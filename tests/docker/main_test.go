package docker

import (
	"bytes"
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
	"github.com/point-c/integration/pkg/archive"
	"github.com/point-c/integration/pkg/cntx"
	"github.com/point-c/integration/pkg/errs"
	"github.com/point-c/integration/pkg/templates"
	"github.com/point-c/simplewg"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/network"
	"github.com/testcontainers/testcontainers-go/wait"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"
)

const (
	DockerfileName = "Dockerfile"
	CaddyfileName  = "Caddyfile"
)

var (
	ServerPort uint16
	ClientPort uint16
	Ctx        *cntx.TC
)

func TestMain(m *testing.M) {
	t := errs.NewTestMain(m)
	defer t.Exit()
	Ctx = cntx.Context(t, context.Background(), time.Minute*2, time.Minute*2)
	defer Ctx.Cancel()

	clientDockerfile, serverDockerfile := GetDockerfiles(t)
	serverCfg, clientCfg := templates.NewDotPair(t)
	writeDebugZip := WriteDebugZip(t, clientDockerfile, serverDockerfile, serverCfg, clientCfg)
	writeDebugZip()
	defer writeDebugZip()

	intNet, cleanup := GetInternalNet(t, Ctx)
	defer cleanup()
	server, cleanup := GetCaddy(t, Ctx, testcontainers.ContainerRequest{
		FromDockerfile: GetBuildContext(t, serverDockerfile)(serverCfg),
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
	client, cleanup := GetCaddy(t, Ctx, testcontainers.ContainerRequest{
		Name:           clientCfg.NetworkName,
		Networks:       []string{"localhost", intNet.Name},
		FromDockerfile: GetBuildContext(t, clientDockerfile)(clientCfg),
		ExposedPorts:   []string{"80/tcp"},
		WaitingFor: wait.ForAll(
			wait.ForLog(`{"level":"info","ts":[0-9]+\.[0-9]+,"msg":"Interface state changed","Old":"Down","Want":"Up","Now":"Up"}`).AsRegexp(),
			wait.ForLog(`{"level":"info","ts":[0-9]+\.[0-9]+,"msg":"serving initial configuration"}`).AsRegexp(),
			wait.ForListeningPort("80/tcp"),
		),
	})
	defer cleanup()
	ServerPort = uint16(errs.Must(server.MappedPort(Ctx.Starting(), "80/tcp"))(t).Int())
	ClientPort = uint16(errs.Must(client.MappedPort(Ctx.Starting(), "80/tcp"))(t).Int())
	require.NoError(t, Ctx.Starting().Err())
	t.Run()
}

func TestRandDownload(t *testing.T) {
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
			w.Go(func() { clientR = GetRandBytes(t, "client", Ctx.Starting(), ClientPort, seed, size) })
			w.Go(func() { serverR = GetRandBytes(t, "server", Ctx.Starting(), ServerPort, seed, size) })
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

func GetRandBytes(t errs.Testing, requestedName string, ctx context.Context, port uint16, seed, size int64) []byte {
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

func WriteDebugZip(t errs.Testing, clientDockerfile, serverDockerfile archive.Entry[[]byte], serverCfg, clientCfg templates.Dot) func() {
	now := time.Now()
	_ = os.Mkdir("test_output", os.ModePerm)
	return func() {
		f := errs.Must(os.Create(filepath.Join("test_output", now.Format("2006-01-02T15:04:05Z07:00")+".zip")))(t)
		defer errs.Defer(t, f.Close)
		archive.Archive[archive.Zip](t, f,
			archive.Entry[[]byte]{Name: "server/" + DockerfileName, Time: serverDockerfile.EntryTime(), Content: serverDockerfile.EntryContent()},
			archive.Entry[[]byte]{Name: "server/" + CaddyfileName, Time: now, Content: serverCfg.ApplyTemplate(t)},
			//archive.Entry[[]byte]{Name: "server/stdout.log", Time: now, Content: serverLogs.StdoutFile()},
			//archive.Entry[[]byte]{Name: "server/stderr.log", Time: now, Content: serverLogs.StderrFile()},
			archive.Entry[[]byte]{Name: "client/" + DockerfileName, Time: clientDockerfile.EntryTime(), Content: clientDockerfile.EntryContent()},
			archive.Entry[[]byte]{Name: "client/" + CaddyfileName, Time: now, Content: clientCfg.ApplyTemplate(t)},
			//archive.Entry[[]byte]{Name: "client/stdout.log", Time: now, Content: clientLogs.StdoutFile()},
			//archive.Entry[[]byte]{Name: "client/stderr.log", Time: now, Content: clientLogs.StderrFile()},
		)
	}
}

func GetBuildContext(t errs.Testing, dockerfile archive.Entry[[]byte]) func(templates.Dot) testcontainers.FromDockerfile {
	return func(d templates.Dot) testcontainers.FromDockerfile {
		var buf bytes.Buffer
		archive.Archive[archive.Tar](t, &buf, dockerfile, archive.Entry[[]byte]{
			Name:    CaddyfileName,
			Time:    time.Now(),
			Content: d.ApplyTemplate(t),
		})
		return testcontainers.FromDockerfile{ContextArchive: &buf}
	}
}

func GetInternalNet(t errs.Testing, ctx *cntx.TC) (*testcontainers.DockerNetwork, func()) {
	internalNet := errs.Must(network.New(ctx.Starting(), network.WithCheckDuplicate(), network.WithInternal(), network.WithAttachable()))(t)
	return internalNet, func() { errs.Check(t, internalNet.Remove(ctx.Terminating())) }
}

func GetCaddy(t errs.Testing, ctx *cntx.TC, req testcontainers.ContainerRequest) (testcontainers.Container, func()) {
	t.Helper()
	log.Printf("building container with config")
	c := errs.Must(testcontainers.GenericContainer(ctx.Starting(), testcontainers.GenericContainerRequest{ContainerRequest: req}))(t)
	errs.Check(t, c.Start(ctx.Starting()))
	return c, func() {
		errs.Check(t, c.Terminate(ctx.Terminating()))
	}
}

func GetDockerfiles(t errs.Testing) (server, client archive.Entry[[]byte]) {
	now := time.Now()
	return archive.Entry[[]byte]{
			Name:    DockerfileName,
			Time:    now,
			Content: templates.DeJSON[templates.DotDockerfile](t, templates.ClientConfig).ApplyTemplate(t),
		}, archive.Entry[[]byte]{
			Name:    DockerfileName,
			Time:    now,
			Content: templates.DeJSON[templates.DotDockerfile](t, templates.ServerConfig).ApplyTemplate(t),
		}
}
