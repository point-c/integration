package speedtest

import (
	"bytes"
	_ "embed"
	"fmt"
	"github.com/librespeed/speedtest-cli/report"
	"github.com/point-c/integration/pkg/archive"
	"github.com/point-c/integration/pkg/cntx"
	"github.com/point-c/integration/pkg/docker"
	"github.com/point-c/integration/pkg/errs"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"io"
	"strconv"
	"strings"
	"testing"
	"text/tabwriter"
	"time"
)

const (
	StartupTimeout  = time.Minute * 2
	ShutdownTimeout = time.Second * 15
)

const (
	SpeedTestServerName = "speedtest_server"
)

const (
	ClientID = 1
	ServerID = 2
)

var (
	Ctx                    *cntx.TC
	SpeedtestClientRequest func(int) testcontainers.ContainerRequest
)

func TestMain(m *testing.M) {
	t := errs.NewTestMain(m)
	defer t.Exit()

	cfg := docker.NewMainContext(t, StartupTimeout, ShutdownTimeout, fmt.Sprintf("reverse_proxy %s:80", SpeedTestServerName))
	Ctx = cfg.Ctx
	defer Ctx.Cancel()

	go func() {
		t := time.Tick(time.Second * 5)
		fn := cfg.Ctx.Starting
		loop := true
		doneFns := []func(){func() { fn = cfg.Ctx.Terminating }, func() { loop = false }}
		for loop {
			cfg.WriteDebugZip()
			select {
			case <-fn().Done():
				doneFns[0]()
				doneFns = doneFns[1:]
			case <-t:
			}
		}
	}()

	intNet, cleanup := cfg.GetInternalNet()
	defer cleanup()
	speedtestServerNet, cleanup := cfg.GetInternalNet()
	defer cleanup()
	speedtestClientNet, cleanup := cfg.GetInternalNet()
	defer cleanup()

	_, cleanup = new(docker.Container).StartContainer(t, Ctx, testcontainers.ContainerRequest{
		Image:    "adolfintel/speedtest",
		Hostname: SpeedTestServerName,
		Networks: []string{speedtestServerNet.Name},
		Env:      map[string]string{"MODE": "backend"},
	})
	defer cleanup()
	_, cleanup = cfg.Server.StartContainer([]string{intNet.Name, speedtestClientNet.Name}, nil)
	defer cleanup()
	_, cleanup = cfg.Client.StartContainer([]string{intNet.Name, speedtestServerNet.Name, speedtestClientNet.Name}, nil)
	defer cleanup()

	var contextArchive bytes.Buffer
	archive.Archive[archive.Tar](t, &contextArchive,
		archive.Entry[[]byte]{Name: docker.DockerfileName, Time: time.Now(), Content: Dockerfile},
		archive.Entry[[]byte]{Name: "servers.json", Time: time.Now(), Content: Dot{
			Client: cfg.Client.Config.NetworkName,
			Server: cfg.Server.Config.NetworkName,
		}.ApplyTemplate(t)},
	)
	SpeedtestClientRequest = func(server int) testcontainers.ContainerRequest {
		return testcontainers.ContainerRequest{
			FromDockerfile: testcontainers.FromDockerfile{
				Tag:            "speedtest-cli",
				ContextArchive: &contextArchive,
				PrintBuildLog:  true,
			},
			Networks: []string{speedtestClientNet.Name},
			Cmd: []string{"/usr/local/bin/librespeed-cli",
				"--no-icmp",
				"--json",
				"--local-json", "/servers.json",
				"--server", strconv.Itoa(server),
			},
		}
	}

	require.NoError(t, Ctx.Starting().Err())
	t.Run()
}

func TestServer(t *testing.T) {
	speedtest(t, ServerID)
}

func TestClient(t *testing.T) {
	speedtest(t, ClientID)
}

func speedtest(t *testing.T, id int) {
	var c docker.Container
	_, cleanup := c.StartContainer(t, Ctx, SpeedtestClientRequest(id))
	defer cleanup()

	//var r []report.JSONReport
	//errs.Check(t, json.NewDecoder(&c).Decode(&r))
	//require.NotEmpty(t, r)
	//
	//var buf bytes.Buffer
	//writeOutput(t, r, &buf)
	//t.Log("\n" + buf.String())
}

func writeOutput(t errs.Testing, r []report.JSONReport, w io.Writer) {
	tw := tabwriter.NewWriter(w, 0, 0, 1, ' ', 0)
	row := struct{ ts, ping, jitter, upload, download string }{
		ts:       "Timestamp",
		ping:     "Ping (ms)",
		jitter:   "Jitter (ms)",
		upload:   "Upload (Mbps)",
		download: "Download (Mbps)",
	}
	writeRow := func() {
		errs.Must(tw.Write([]byte(strings.Join([]string{row.ts, row.ping, row.jitter, row.upload, row.download}, "\t") + "\n")))(t)
	}
	writeRow()
	for _, r := range r {
		row.ts = r.Timestamp.Format(time.RFC1123)
		row.ping, row.jitter = fmtFloat(r.Ping), fmtFloat(r.Jitter)
		row.upload, row.download = fmtFloat(r.Upload), fmtFloat(r.Download)
		writeRow()
	}
	errs.Check(t, tw.Flush())
}
func fmtFloat(f float64) string { return fmt.Sprintf("%.2f", f) }
