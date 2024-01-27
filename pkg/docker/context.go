// Package docker assists with creating containers and networks for docker.
package docker

import (
	"bytes"
	"context"
	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
	"github.com/docker/go-connections/nat"
	"github.com/point-c/integration/pkg/archive"
	"github.com/point-c/integration/pkg/errs"
	"github.com/point-c/integration/pkg/templates"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/network"
	"github.com/testcontainers/testcontainers-go/wait"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const (
	DockerfileName = "Dockerfile"
	CaddyfileName  = "Caddyfile"
	LogName        = "caddy.log"
)

// MainContext contains the overall context for the application and configs.
type MainContext struct {
	context.Context
	cancel context.CancelFunc
	t      errs.Testing
	Client MainContextEntry[templates.DotClient]
	Server MainContextEntry[templates.DotServer]
	Now    time.Time
}

// NewMainContext creates a new context. clientDirective is passed to the client's Caddyfile as the handler for the `:80` route.
func NewMainContext(t errs.Testing, clientDirective string) *MainContext {
	_ = os.Mkdir("test_output", os.ModePerm)
	ctx := MainContext{
		t:   t,
		Now: time.Now(),
	}
	ctx.Context, ctx.cancel = context.WithDeadline(context.Background(), TestingDeadline(t))

	ctx.Client.p = &ctx
	ctx.Server.p = &ctx
	ctx.Client.Config, ctx.Server.Config = templates.NewDotPair(t)
	ctx.Client.Config.Directive = clientDirective

	ctx.Client.Dockerfile, ctx.Server.Dockerfile = archive.Entry[[]byte]{
		Name:    DockerfileName,
		Time:    ctx.Now,
		Content: templates.DeJSON[templates.DotDockerfile](t, templates.ClientConfig).ApplyTemplate(t),
	}, archive.Entry[[]byte]{
		Name:    DockerfileName,
		Time:    ctx.Now,
		Content: templates.DeJSON[templates.DotDockerfile](t, templates.ServerConfig).ApplyTemplate(t),
	}
	ctx.Client.Caddyfile, ctx.Server.Caddyfile = archive.Entry[[]byte]{
		Name:    CaddyfileName,
		Time:    ctx.Now,
		Content: caddyfile.Format(ctx.Client.Config.ApplyTemplate(t)),
	}, archive.Entry[[]byte]{
		Name:    CaddyfileName,
		Time:    ctx.Now,
		Content: caddyfile.Format(ctx.Server.Config.ApplyTemplate(t)),
	}
	return &ctx
}

func (ctx *MainContext) Cancel() { ctx.cancel() }

// WriteDebugZip writes information about the caddy processes for debugging.
// The zip contains the server and client's caddyfile and dockerfile, along with any logs if they exist.
func (ctx *MainContext) WriteDebugZip() {
	f := errs.Must(os.Create(filepath.Join("test_output", ctx.Now.Format("2006-01-02T15:04:05Z07:00")+".zip")))(ctx.t)
	defer errs.Defer(ctx.t, f.Close)
	archive.Archive[archive.Zip](ctx.t, f,
		archive.Entry[[]archive.FileHeader]{
			Name: "client",
			Time: ctx.Now,
			Content: []archive.FileHeader{
				ctx.Client.Caddyfile,
				ctx.Client.Dockerfile,
				archive.Entry[[]byte]{Name: LogName, Time: ctx.Now, Content: ctx.Client.Logs.Bytes()},
			},
		},
		archive.Entry[[]archive.FileHeader]{
			Name: "server",
			Time: ctx.Now,
			Content: []archive.FileHeader{
				ctx.Server.Caddyfile,
				ctx.Server.Dockerfile,
				archive.Entry[[]byte]{Name: LogName, Time: ctx.Now, Content: ctx.Server.Logs.Bytes()},
			},
		},
	)
}

// GetInternalNet gets a docker network with no external connection.
func (ctx *MainContext) GetInternalNet() (*testcontainers.DockerNetwork, func()) {
	return ctx.GetNet(network.WithInternal())
}

// GetNet gets a network with the given options. Use the returned func to cleanup the container after usage.
func (ctx *MainContext) GetNet(opts ...network.NetworkCustomizer) (*testcontainers.DockerNetwork, func()) {
	c, cn := context.WithTimeout(ctx, time.Second*10)
	defer cn()
	internalNet := errs.Must(network.New(c, append(opts, network.WithCheckDuplicate(), network.WithAttachable())...))(ctx.t)
	return internalNet, func() {
		c, cn := context.WithTimeout(context.Background(), time.Second*10)
		defer cn()
		errs.Check(ctx.t, internalNet.Remove(c))
	}
}

// GetContainer creates a new docker container with the given request. The container will be started before returning.
// Use cleanup to remove all container resources after running.
func (ctx *MainContext) GetContainer(req testcontainers.ContainerRequest) (tc testcontainers.Container, cleanup func()) {
	c, cn := context.WithTimeout(ctx, time.Minute*5)
	defer cn()
	tc = errs.Must(testcontainers.GenericContainer(c, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	}))(ctx.t)
	return tc, func() {
		c, cn := context.WithTimeout(context.Background(), time.Second*20)
		defer cn()
		to := time.Second * 10
		errs.Check(ctx.t, tc.Stop(c, &to))
	}
}

type (
	// MainContextEntry is either a server or client definition.
	MainContextEntry[D interface {
		templates.Dot
		NamedNetwork
	}] struct {
		p          *MainContext
		Dockerfile archive.Entry[[]byte]
		Caddyfile  archive.Entry[[]byte]
		Config     D
		Logs       lockedBuf
	}
	// NamedNetwork is used to specify the server and client data.
	NamedNetwork interface {
		GetNetworkName() string
	}
)

// StartContainer starts the container specified by this configuration.
func (mce *MainContextEntry[D]) StartContainer(networks []string, exposed []string, waitPort ...nat.Port) (testcontainers.Container, func()) {
	// Generate dockerfile context
	var buf bytes.Buffer
	archive.Archive[archive.Tar](mce.p.t, &buf, mce.Caddyfile, mce.Dockerfile)
	// Start container
	waitFor := []wait.Strategy{
		wait.ForLog(`{"level":"info","ts":[0-9]+\.[0-9]+,"msg":"Interface state changed","Old":"Down","Want":"Up","Now":"Up"}`).AsRegexp(),
		wait.ForLog(`{"level":"info","ts":[0-9]+\.[0-9]+,"msg":"serving initial configuration"}`).AsRegexp(),
	}
	if len(waitPort) > 0 {
		waitFor = append(waitFor, wait.ForListeningPort(waitPort[0]))
	}

	logsCtx, logsCancel := context.WithCancel(mce.p)
	c, cleanup := mce.p.GetContainer(testcontainers.ContainerRequest{
		FromDockerfile: testcontainers.FromDockerfile{
			ContextArchive: &buf,
			PrintBuildLog:  true,
		},
		Name:         mce.Config.GetNetworkName(),
		Hostname:     mce.Config.GetNetworkName(),
		Networks:     networks,
		ExposedPorts: exposed,
		WaitingFor:   wait.ForAll(waitFor...),
	})
	cleanup = func(f func()) func() { return func() { logsCancel(); f() } }(cleanup)

	panicked := true
	defer func() {
		if panicked {
			defer cleanup()
		}
	}()

	go func() {
		_, _ = io.Copy(&mce.Logs, errs.Must(c.Logs(logsCtx))(mce.p.t))
	}()

	panicked = false
	return c, cleanup
}

type lockedBuf struct {
	b bytes.Buffer
	l sync.Mutex
}

func (l *lockedBuf) Bytes() []byte {
	l.l.Lock()
	defer l.l.Unlock()
	return l.b.Bytes()
}

func (l *lockedBuf) Write(p []byte) (n int, err error) {
	l.l.Lock()
	defer l.l.Unlock()
	return l.b.Write(p)
}
