package speedtest

import (
	"bufio"
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"github.com/librespeed/speedtest-cli/report"
	"github.com/pkg/errors"
	"github.com/point-c/integration/pkg/docker"
	"github.com/point-c/integration/pkg/errs"
	"github.com/point-c/integration/tests/speedtest/internal"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"io"
	"net"
	"net/http"
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

	SpeedtestClientRequest = func(server int) testcontainers.ContainerRequest {
		var netName, hostname string
		switch server {
		case ServerID:
			hostname = Ctx.Server.Config.NetworkName
			netName = speedtestCliServerNet.Name
		case ClientID:
			hostname = Ctx.Client.Config.NetworkName
			netName = speedtestCliClientNet.Name
		default:
			errs.Check(t, fmt.Errorf("invalid id %d", server))
		}

		return testcontainers.ContainerRequest{
			WaitingFor: wait.ForExposedPort(),
			FromDockerfile: testcontainers.FromDockerfile{
				Tag:            "speedtest-cli",
				ContextArchive: internal.Context(t),
				PrintBuildLog:  true,
			},
			Networks: []string{"localhost", netName},
			Env: map[string]string{
				"HOSTNAME": hostname,
				"PORT":     "80",
			},
			ExposedPorts: []string{"8080/tcp"},
		}
	}

	ctx, cancel := context.WithCancel(Ctx)
	defer cancel()
	resultsPrinted, resultsPrintedCancel := context.WithCancel(Ctx)
	defer resultsPrintedCancel()
	go func() {
		defer resultsPrintedCancel()
		var res []Report
		for {
			select {
			case <-ctx.Done():
				writeOutput(t, res, os.Stdout)
				return
			case r := <-Results:
				res = append(res, r)
			}
		}
	}()

	select {
	case <-Ctx.Done():
	default:
		t.Run()
		cancel()
		<-resultsPrinted.Done()
	}
}

func TestServer(t *testing.T) {
	speedtest(t, ServerID, "Server")
}

func TestClient(t *testing.T) {
	speedtest(t, ClientID, "Client")
}

func speedtest(t *testing.T, id int, name string) {
	c, cleanup := Ctx.GetContainer(SpeedtestClientRequest(id))
	defer cleanup()

	ctx, cancel := context.WithCancel(Ctx)
	defer cancel()
	go func() {
		logs := errs.Must(c.Logs(ctx))(t)
		defer logs.Close()
		r := bufio.NewReader(logs)
		var buf bytes.Buffer
		for {
			line, isPrefix, err := r.ReadLine()
			if errors.Is(err, io.EOF) {
				return
			}
			errs.Check(t, err)
			if isPrefix {
				buf.Write(line)
			} else {
				buf.Write(line)
				t.Log(buf.String())
				buf.Reset()
			}
		}
	}()

	pr, pw := io.Pipe()
	go func() {
		defer pw.Close()
		ti := time.NewTicker(time.Millisecond * 500)
		defer ti.Stop()
		for range ti.C {
			if func() bool {
				ctx, cancel := context.WithTimeout(ctx, time.Millisecond*500)
				defer cancel()
				req := errs.Must(http.NewRequestWithContext(ctx,
					http.MethodGet,
					fmt.Sprintf("http://localhost:%d", errs.Must(c.MappedPort(ctx, "8080/tcp"))(t).Int()),
					nil,
				))(t)
				resp, err := http.DefaultClient.Do(req)
				if errors.Is(err, context.DeadlineExceeded) || errors.As(err, new(*net.OpError)) {
					return false
				}
				errs.Check(t, err)
				defer resp.Body.Close()

				switch resp.StatusCode {
				case http.StatusOK:
					errs.Must(io.Copy(pw, resp.Body))(t)
					return true
				case http.StatusTooEarly:
				}
				return false
			}() {
				return
			}
		}
	}()

	var r []Report
	if err := json.NewDecoder(pr).Decode(&r); !errors.Is(err, io.EOF) {
		errs.Check(t, err)
	}
	require.NotEmpty(t, r)
	for i := range r {
		r[i].name = name
		select {
		case Results <- r[i]:
		case <-Ctx.Done():
			return
		}
	}

	var buf bytes.Buffer
	writeOutput(t, r, &buf)
	t.Log("\n" + buf.String())
}

type Report struct {
	report.JSONReport
	name string
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
		row.name = r.name
		row.ts = r.Timestamp.Format(time.RFC1123)
		row.ping, row.jitter = fmtFloat(r.Ping), fmtFloat(r.Jitter)
		row.upload, row.download = fmtFloat(r.Upload), fmtFloat(r.Download)
		writeRow()
	}
	errs.Check(t, tw.Flush())
}
func fmtFloat(f float64) string { return fmt.Sprintf("%.2f", f) }
