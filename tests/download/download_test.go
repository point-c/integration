package download

import (
	"context"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"github.com/point-c/integration/pkg/cntx"
	"github.com/point-c/integration/pkg/docker"
	"github.com/point-c/integration/pkg/errs"
	"github.com/point-c/simplewg"
	"github.com/stretchr/testify/require"
	"io"
	"net/http"
	"testing"
	"time"
)

const (
	StartupTimeout  = time.Minute * 2
	ShutdownTimeout = time.Second * 5
)

var (
	ServerPort uint16
	ClientPort uint16
	Ctx        *cntx.TC
)

func TestMain(m *testing.M) {
	t := errs.NewTestMain(m)
	defer t.Exit()

	cfg := docker.NewMainContext(t, StartupTimeout, ShutdownTimeout, "route {\nrand\n}")
	Ctx = cfg.Ctx
	defer Ctx.Cancel()

	cfg.WriteDebugZip()
	go func() {
		for range time.Tick(time.Second * 5) {
			cfg.WriteDebugZip()
		}
	}()

	intNet, cleanup := cfg.GetInternalNet()
	defer cleanup()

	networks := []string{"localhost", intNet.Name}
	exposedPorts := []string{"80/tcp"}
	server, cleanup := cfg.Server.StartContainer(networks, exposedPorts, "80/tcp")
	defer cleanup()
	client, cleanup := cfg.Client.StartContainer(networks, exposedPorts, "80/tcp")
	defer cleanup()

	ServerPort = uint16(errs.Must(server.MappedPort(Ctx.Starting(), "80/tcp"))(t).Int())
	ClientPort = uint16(errs.Must(client.MappedPort(Ctx.Starting(), "80/tcp"))(t).Int())
	require.NoError(t, Ctx.Starting().Err())
	t.Run()
}

func TestDownload(t *testing.T) {
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
	for _, s := range []struct{ s, u, m uint }{
		{s: gb, u: 'G', m: 1024 * 1024 * 1024},
		{s: mb, u: 'M', m: 1024 * 1024},
		{s: kb, u: 'K', m: 1024},
		{s: b, u: 'b'},
	} {
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
