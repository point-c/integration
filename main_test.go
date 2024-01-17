package integration

import (
	"context"
	_ "embed"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	_ "github.com/caddyserver/caddy/v2/caddyconfig/httpcaddyfile"
	_ "github.com/caddyserver/caddy/v2/modules/standard"
	_ "github.com/point-c/caddy"
	_ "github.com/point-c/caddy-randhandler"
	_ "github.com/point-c/caddy-wg"
	"github.com/point-c/integration/archive"
	"github.com/point-c/integration/errs"
	"github.com/point-c/simplewg"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/network"
	"github.com/testcontainers/testcontainers-go/wait"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

const (
	DockerfileName = "Dockerfile"
	CaddyfileName  = "Caddyfile"
)

func TestPointc(t *testing.T) {
	dockerfile := archive.Entry[[]byte]{
		Name:    DockerfileName,
		Time:    time.Now(),
		Content: DeJSON[DotDockerfile](t, Config).ApplyTemplate(t),
	}

	ctx, cancel := TestingContext(t, context.Background())
	defer cancel()
	ctx, cancel = signal.NotifyContext(ctx, os.Kill, os.Interrupt, syscall.SIGKILL, syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	serverCfg, clientCfg := NewDotPair(t)
	WriteDebugZip(t, dockerfile, serverCfg, clientCfg)

	intNet, cleanup := GetInternalNet(t, ctx)
	defer cleanup()

	server, cleanup := GetCaddy(t, ctx, testcontainers.ContainerRequest{
		FromDockerfile: GetBuildContext(t, dockerfile)(serverCfg),
		Name:           serverCfg.NetworkName,
		ExposedPorts:   []string{"80/tcp"},
		WaitingFor: wait.ForAll(
			wait.ForLog(`{"level":"info","ts":[0-9]+\.[0-9]+,"msg":"Interface state changed","Old":"Down","Want":"Up","Now":"Up"}`).AsRegexp(),
			wait.ForLog(`{"level":"info","ts":[0-9]+\.[0-9]+,"msg":"serving initial configuration"}`).AsRegexp(),
			wait.ForListeningPort("80/tcp"),
		),
		Hostname: serverCfg.NetworkName,
		Networks: []string{"localhost", intNet.Name},
	})
	defer cleanup()

	clientCfg.Endpoint = serverCfg.NetworkName
	clientCfg.EndpointPort = 51820
	client, cleanup := GetCaddy(t, ctx, testcontainers.ContainerRequest{
		Name:           clientCfg.NetworkName,
		Networks:       []string{"localhost", intNet.Name},
		FromDockerfile: GetBuildContext(t, dockerfile)(clientCfg),
		ExposedPorts:   []string{"80/tcp"},
		WaitingFor: wait.ForAll(
			wait.ForLog(`{"level":"info","ts":[0-9]+\.[0-9]+,"msg":"Interface state changed","Old":"Down","Want":"Up","Now":"Up"}`).AsRegexp(),
			wait.ForLog(`{"level":"info","ts":[0-9]+\.[0-9]+,"msg":"serving initial configuration"}`).AsRegexp(),
			wait.ForListeningPort("80/tcp"),
		),
	})
	defer cleanup()
	RunSubtests(t, ctx, errs.Must(server.MappedPort(ctx, "80/tcp"))(t).Int(), errs.Must(client.MappedPort(ctx, "80/tcp"))(t).Int())
	require.NoError(t, ctx.Err())
}

func RunSubtests(t *testing.T, ctx context.Context, serverPort, clientPort int) {
	seed, seedStr := MakeSeed()
	tt := []struct {
		Bytes     uint
		Kilobytes uint
		Megabytes uint
		Gigabytes uint
	}{
		{Bytes: 1},
		{Bytes: 128},
		{Bytes: 256},
		{Bytes: 512},
		{Kilobytes: 1},
		{Kilobytes: 128},
		{Kilobytes: 256},
		{Kilobytes: 512},
		{Megabytes: 1},
		{Megabytes: 128},
		{Megabytes: 256},
		{Megabytes: 512},
		{Gigabytes: 1},
	}

	for _, tt := range tt {
		size, sizeStr := ParseSize(tt.Gigabytes, tt.Megabytes, tt.Kilobytes, tt.Bytes)
		t.Run(fmt.Sprintf("requesting %s with seed of %s", sizeStr, seedStr), func(t *testing.T) {
			var w simplewg.Wg
			var clientR, serverR []byte
			w.Go(func() { clientR = GetRandBytes(t, "client", ctx, uint16(clientPort), seed, size) })
			w.Go(func() { serverR = GetRandBytes(t, "server", ctx, uint16(serverPort), seed, size) })
			w.Wait()
			require.Equal(t, clientR, serverR)
		})
	}
}

func MakeSeed() (seed int64, str string) {
	seed = time.Now().UnixMilli()
	str = "0x" + hex.EncodeToString(binary.BigEndian.AppendUint64(nil, uint64(seed)))
	return
}

func ParseSize(gb, mb, kb, b uint) (size int64, str string) {
	for _, s := range []struct{ s, u, m uint }{{s: gb, u: 'G', m: 1024 * 1024 * 1024}, {s: mb, u: 'M', m: 1024 * 1024}, {s: kb, u: 'K', m: 1024}, {s: b, u: 'b'}} {
		size += int64(s.s * s.m)
		if s.s != 0 {
			str += fmt.Sprintf("%d%c", s.s, s.u)
		}
	}
	return
}

func GetRandBytes(t testing.TB, requestedName string, ctx context.Context, port uint16, seed, size int64) []byte {
	req := errs.Must(http.NewRequest(http.MethodGet, fmt.Sprintf("http://localhost:%d", int(port)), nil))(t).WithContext(ctx)
	req.Header = http.Header{
		"Rand-Seed":        []string{fmt.Sprintf("%d", seed)},
		"Rand-Size":        []string{fmt.Sprintf("%d", size)},
		"Docker-Requested": []string{requestedName},
	}

	defer func(start time.Time) {
		t.Logf("took %s to download from %s", time.Now().Sub(start).String(), requestedName)
	}(time.Now())
	resp := errs.Must(http.DefaultClient.Do(req))(t)
	defer errs.Defer(t, resp.Body.Close)
	return errs.Must(io.ReadAll(resp.Body))(t)
}

func WriteDebugZip(t testing.TB, dockerfile archive.FileHeader, serverCfg, clientCfg Dot) func() {
	now := time.Now()
	_ = os.Mkdir("test_output", os.ModePerm)
	return func() {
		f := errs.Must(os.Create(filepath.Join("test_output", now.Format("2006-01-02T15:04:05Z07:00")+".zip")))(t)
		defer errs.Defer(t, f.Close)
		errs.Must(io.Copy(f, archive.Archive[archive.Zip](t, dockerfile,
			archive.Entry[[]byte]{Name: "server/Caddyfile", Time: now, Content: serverCfg.ApplyTemplate(t)},
			//archive.Entry[[]byte]{Name: "server/stdout.log", Time: now, Content: serverLogs.StdoutFile()},
			//archive.Entry[[]byte]{Name: "server/stderr.log", Time: now, Content: serverLogs.StderrFile()},
			archive.Entry[[]byte]{Name: "client/Caddyfile", Time: now, Content: clientCfg.ApplyTemplate(t)},
			//archive.Entry[[]byte]{Name: "client/stdout.log", Time: now, Content: clientLogs.StdoutFile()},
			//archive.Entry[[]byte]{Name: "client/stderr.log", Time: now, Content: clientLogs.StderrFile()},
		)))(t)
	}
}

func GetBuildContext(t testing.TB, dockerfile archive.Entry[[]byte]) func(Dot) testcontainers.FromDockerfile {
	t.Helper()
	return func(d Dot) testcontainers.FromDockerfile {
		t.Helper()
		return testcontainers.FromDockerfile{
			ContextArchive: archive.Archive[archive.Tar](t, dockerfile, archive.Entry[[]byte]{
				Name:    CaddyfileName,
				Time:    time.Now(),
				Content: d.ApplyTemplate(t),
			}),
		}
	}
}

func GetInternalNet(t testing.TB, ctx context.Context) (*testcontainers.DockerNetwork, func()) {
	t.Helper()
	internalNet := errs.Must(network.New(ctx, network.WithCheckDuplicate(), network.WithInternal(), network.WithAttachable()))(t)
	return internalNet, func() {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()
		errs.Check(t, internalNet.Remove(ctx))
	}
}

func GetCaddy(t testing.TB, ctx context.Context, req testcontainers.ContainerRequest) (testcontainers.Container, func()) {
	t.Helper()
	log.Printf("building container with config")
	c := errs.Must(testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{ContainerRequest: req}))(t)
	errs.Check(t, c.Start(ctx))
	return c, func() {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()
		errs.Check(t, c.Terminate(ctx))
	}
}
