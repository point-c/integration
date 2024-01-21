package docker

import (
	"bytes"
	"context"
	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
	"github.com/point-c/integration/pkg/archive"
	"github.com/point-c/integration/pkg/cntx"
	"github.com/point-c/integration/pkg/errs"
	"github.com/point-c/integration/pkg/templates"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/network"
	"github.com/testcontainers/testcontainers-go/wait"
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

type MainContext struct {
	t      errs.Testing
	Client MainContextEntry[templates.DotClient]
	Server MainContextEntry[templates.DotServer]
	Now    time.Time
	Ctx    *cntx.TC
}

func NewMainContext(t errs.Testing, startTO, stopTO time.Duration, clientDirective string) *MainContext {
	_ = os.Mkdir("test_output", os.ModePerm)
	ctx := MainContext{Now: time.Now(), t: t}
	ctx.Client.p = &ctx
	ctx.Server.p = &ctx
	ctx.Ctx = cntx.Context(t, context.Background(), startTO, stopTO)
	ctx.Client.Config, ctx.Server.Config = templates.NewDotPair(t)
	ctx.Client.Config.Endpoint = ctx.Server.Config.NetworkName
	ctx.Client.Config.EndpointPort = 51820
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

func (ctx *MainContext) GetInternalNet() (*testcontainers.DockerNetwork, func()) {
	return ctx.GetNet(network.WithInternal())
}

func (ctx *MainContext) GetNet(opts ...network.NetworkCustomizer) (*testcontainers.DockerNetwork, func()) {
	internalNet := errs.Must(network.New(ctx.Ctx.Starting(), append(opts, network.WithCheckDuplicate(), network.WithAttachable())...))(ctx.t)
	return internalNet, func() { errs.Check(ctx.t, internalNet.Remove(ctx.Ctx.Terminating())) }
}

type LockedBuf struct {
	buf  bytes.Buffer
	lock sync.RWMutex
}

func (l *LockedBuf) Accept(log testcontainers.Log) {
	l.lock.Lock()
	defer l.lock.Unlock()
	l.buf.Write(log.Content)
}

func (l *LockedBuf) Bytes() []byte {
	l.lock.RLock()
	defer l.lock.RUnlock()
	return l.buf.Bytes()
}

type MainContextEntry[D interface {
	templates.Dot
	GetNetworkName() string
}] struct {
	p          *MainContext
	Dockerfile archive.Entry[[]byte]
	Caddyfile  archive.Entry[[]byte]
	Config     D
	Logs       LockedBuf
}

func (mce *MainContextEntry[D]) StartContainer(networks []string, exposed []string) (testcontainers.Container, func()) {
	// Generate dockerfile context
	var buf bytes.Buffer
	archive.Archive[archive.Tar](mce.p.t, &buf, mce.Caddyfile, mce.Dockerfile)
	// Create container
	c := errs.Must(testcontainers.GenericContainer(mce.p.Ctx.Starting(), testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			FromDockerfile: testcontainers.FromDockerfile{ContextArchive: &buf},
			Name:           mce.Config.GetNetworkName(),
			Hostname:       mce.Config.GetNetworkName(),
			Networks:       networks,
			ExposedPorts:   exposed,
			WaitingFor: wait.ForAll(
				wait.ForLog(`{"level":"info","ts":[0-9]+\.[0-9]+,"msg":"Interface state changed","Old":"Down","Want":"Up","Now":"Up"}`).AsRegexp(),
				wait.ForLog(`{"level":"info","ts":[0-9]+\.[0-9]+,"msg":"serving initial configuration"}`).AsRegexp(),
				wait.ForListeningPort("80/tcp"),
			),
		},
	}))(mce.p.t)
	// Start container
	errs.Check(mce.p.t, c.Start(mce.p.Ctx.Starting()))
	// Start log reader before starting
	c.FollowOutput(&mce.Logs)
	errs.Check(mce.p.t, c.StartLogProducer(mce.p.Ctx.Starting()))
	return c, func() {
		defer errs.Check(mce.p.t, c.Terminate(mce.p.Ctx.Terminating()))
		errs.Check(mce.p.t, c.StopLogProducer())
	}
}
