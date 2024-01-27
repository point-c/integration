package speedtest

import (
	"context"
	_ "embed"
	"fmt"
	"github.com/point-c/integration/pkg/docker"
	"github.com/point-c/integration/pkg/errs"
	"github.com/point-c/integration/tests/speedtest/internal"
	speedtest_srv "github.com/point-c/integration/tests/speedtest/internal/speedtest-srv/speedtest-srv"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"io"
	"net"
	"net/rpc"
	"os"
	"strings"
	"testing"
	"text/tabwriter"
	"time"
)

const (
	SpeedTestServerName = "speedtest_server"
)

const (
	ClientID = 1
	ServerID = 2
)

var (
	Ctx                    *docker.MainContext
	SpeedtestClientRequest func(int) testcontainers.ContainerRequest
	Results                = make(chan Report)
)

func TestMain(m *testing.M) {
	t := errs.NewTestMain(m)
	defer t.Exit()

	Ctx = docker.NewMainContext(t, fmt.Sprintf("reverse_proxy %s:80", SpeedTestServerName))
	defer Ctx.Cancel()
	defer collectAndDefer(t)()

	go func() {
		t := time.Tick(time.Second * 5)
		for {
			Ctx.WriteDebugZip()
			select {
			case <-Ctx.Done():
				Ctx.WriteDebugZip()
				return
			case <-t:
			}
		}
	}()

	intNet, cleanup := Ctx.GetInternalNet()
	defer cleanup()
	speedtestServerNet, cleanup := Ctx.GetInternalNet()
	defer cleanup()
	speedtestCliClientNet, cleanup := Ctx.GetInternalNet()
	defer cleanup()
	speedtestCliServerNet, cleanup := Ctx.GetInternalNet()
	defer cleanup()

	_, cleanup = Ctx.GetContainer(testcontainers.ContainerRequest{
		Image:    "adolfintel/speedtest",
		Hostname: SpeedTestServerName,
		Networks: []string{speedtestServerNet.Name},
		Env:      map[string]string{"MODE": "backend"},
	})
	defer cleanup()
	_, cleanup = Ctx.Server.StartContainer([]string{intNet.Name, speedtestCliServerNet.Name}, nil)
	defer cleanup()
	_, cleanup = Ctx.Client.StartContainer([]string{intNet.Name, speedtestServerNet.Name, speedtestCliClientNet.Name}, nil)
	defer cleanup()

	SpeedtestClientRequest = SpeedtestClientRequestFn(t, speedtestCliServerNet.Name, speedtestCliClientNet.Name)
	select {
	case <-Ctx.Done():
	default:
		t.Run()
	}
}

func TestServer(t *testing.T) {
	speedtest(t, ServerID, "VPN", 3)
}

func TestClient(t *testing.T) {
	speedtest(t, ClientID, "Caddy", 3)
}

func speedtest(t *testing.T, id int, name string, count uint) {
	c, cleanup := Ctx.GetContainer(SpeedtestClientRequest(id))
	defer cleanup()

	var hostname string
	switch id {
	case ServerID:
		hostname = Ctx.Server.Config.NetworkName
	case ClientID:
		hostname = Ctx.Client.Config.NetworkName
	}

	ctx, cancel := context.WithCancel(Ctx)
	defer cancel()

	serverInfo := speedtest_srv.ServerInfo{Name: name, Hostname: hostname, Port: 80}
	for i := uint(0); i < count; i++ {
		t.Run(fmt.Sprintf("speedtesting: %s %d", name, i+1), func(t *testing.T) {
			rep := Report{Timestamp: time.Now(), Name: fmt.Sprintf("%s-%d", name, i+1)}
			addr := fmt.Sprintf("localhost:%d", errs.Must(c.MappedPort(ctx, "8080/tcp"))(t).Int())
			t.Logf("dialing speedtest server: %s", addr)
			conn := errs.Must(new(net.Dialer).DialContext(Ctx, "tcp", addr))(t)
			client := rpc.NewClient(conn)
			defer errs.Defer(t, client.Close)

			t.Run("ping", func(t *testing.T) {
				var pingResp speedtest_srv.PingResponse
				errs.Check(t, client.Call(speedtest_srv.MethodSpeedTestPing, speedtest_srv.PingRequest{ServerInfo: serverInfo}, &pingResp))
				rep.Ping, rep.Jitter = pingResp.Ping, pingResp.Jitter
			})
			t.Run("download", func(t *testing.T) {
				var sr speedtest_srv.SpeedResponse
				errs.Check(t, client.Call(speedtest_srv.MethodSpeedTestDownload, speedtest_srv.DownloadRequest{ServerInfo: serverInfo}, &sr))
				rep.Download = sr.Speed
			})
			t.Run("upload", func(t *testing.T) {
				var sr speedtest_srv.SpeedResponse
				errs.Check(t, client.Call(speedtest_srv.MethodSpeedTestUpload, speedtest_srv.UploadRequest{ServerInfo: serverInfo}, &sr))
				rep.Upload = sr.Speed
			})

			select {
			case Results <- rep:
			case <-Ctx.Done():
				return
			}
		})
	}
}

type Report struct {
	Ping, Jitter, Upload, Download float64
	Timestamp                      time.Time
	Name                           string
}

func writeOutput(t errs.Testing, r []Report, w io.Writer) {
	tw := tabwriter.NewWriter(w, 0, 0, 1, ' ', 0)
	row := struct{ name, ts, ping, jitter, upload, download string }{
		name:     "Name",
		ts:       "Timestamp",
		ping:     "Ping (ms)",
		jitter:   "Jitter (ms)",
		upload:   "Upload (Mbps)",
		download: "Download (Mbps)",
	}
	writeRow := func() {
		errs.Must(tw.Write([]byte(strings.Join([]string{row.name, row.ts, row.ping, row.jitter, row.upload, row.download}, "\t") + "\n")))(t)
	}
	writeRow()
	for _, r := range r {
		row.name = r.Name
		row.ts = r.Timestamp.Format(time.RFC1123)
		row.ping, row.jitter = fmtFloat(r.Ping), fmtFloat(r.Jitter)
		row.upload, row.download = fmtFloat(r.Upload), fmtFloat(r.Download)
		writeRow()
	}
	errs.Check(t, tw.Flush())
}
func fmtFloat(f float64) string { return fmt.Sprintf("%.2f", f) }

func SpeedtestClientRequestFn(t errs.Testing, serverNet, clientNet string) func(server int) testcontainers.ContainerRequest {
	return func(server int) testcontainers.ContainerRequest {
		var netName string
		switch server {
		case ServerID:
			netName = serverNet
		case ClientID:
			netName = clientNet
		default:
			errs.Check(t, fmt.Errorf("invalid id %d", server))
		}

		return testcontainers.ContainerRequest{
			WaitingFor: wait.ForLog(".*server started.*").AsRegexp(),
			FromDockerfile: testcontainers.FromDockerfile{
				Tag:            "speedtest-cli",
				ContextArchive: internal.Context(t),
				PrintBuildLog:  true,
			},
			Networks:     []string{"localhost", netName},
			ExposedPorts: []string{"8080/tcp"},
		}
	}
}

func collectAndDefer(t errs.Testing) func() {
	ctx, cancel := context.WithCancel(Ctx)
	resultsPrinted, resultsPrintedCancel := context.WithCancel(Ctx)

	var res []Report
	go func() {
		defer resultsPrintedCancel()
		for {
			select {
			case <-ctx.Done():
				return
			case r := <-Results:
				res = append(res, r)
			}
		}
	}()

	return func() {
		defer func() {
			<-resultsPrinted.Done()
			writeOutput(t, res, os.Stdout)
		}()
		cancel()
	}
}
