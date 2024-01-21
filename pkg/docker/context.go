package docker

import (
	"bytes"
	"context"
	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
	"github.com/docker/go-connections/nat"
	"github.com/point-c/integration/pkg/archive"
	"github.com/point-c/integration/pkg/cntx"
	"github.com/point-c/integration/pkg/errs"
	"github.com/point-c/integration/pkg/templates"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/network"
	"github.com/testcontainers/testcontainers-go/wait"
	"os"
	"path/filepath"
	"time"
)

const (
	DockerfileName = "Dockerfile"
	CaddyfileName  = "Caddyfile"
	LogName        = "caddy.log"
)

type MainContext struct {
	context.Context
	cancel context.CancelFunc
	t      errs.Testing
	Client MainContextEntry[templates.DotClient]
	Server MainContextEntry[templates.DotServer]
	Now    time.Time
}

func NewMainContext(t errs.Testing, clientDirective string) *MainContext {
	_ = os.Mkdir("test_output", os.ModePerm)
	ctx := MainContext{
		t:   t,
		Now: time.Now(),
	}
	ctx.Client.p = &ctx
	ctx.Server.p = &ctx
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
				//archive.Entry[[]byte]{Name: LogName, Time: ctx.Now, Content: ctx.Client.Logs.Bytes()},
				archive.Entry[[]byte]{Name: LogName, Time: ctx.Now, Content: nil},
			},
		},
		archive.Entry[[]archive.FileHeader]{
			Name: "server",
			Time: ctx.Now,
			Content: []archive.FileHeader{
				ctx.Server.Caddyfile,
				ctx.Server.Dockerfile,
				//archive.Entry[[]byte]{Name: LogName, Time: ctx.Now, Content: ctx.Server.Logs.Bytes()},
				archive.Entry[[]byte]{Name: LogName, Time: ctx.Now, Content: nil},
			},
		},
	)
}

func (ctx *MainContext) GetInternalNet() (*testcontainers.DockerNetwork, func()) {
	return ctx.GetNet(network.WithInternal())
}

func (ctx *MainContext) GetNet(ctx context.Context, opts ...network.NetworkCustomizer) (*testcontainers.DockerNetwork, func()) {
	internalNet := errs.Must(network.New(ctx.Ctx.Starting(), append(opts, network.WithCheckDuplicate(), network.WithAttachable())...))(ctx.t)
	return internalNet, func() { errs.Check(ctx.t, internalNet.Remove(ctx.Ctx.Terminating())) }
}

type Container struct {
	//lock    sync.RWMutex
	//running bool
	//b1, b2  bytes.Buffer
	//w       io.Writer
	C testcontainers.Container
}

//type containerLogs Container
//
//func (l *containerLogs) Accept(log testcontainers.Log) {
//	l.lock.Lock()
//	defer l.lock.Unlock()
//	_, _ = l.w.Write(log.Content)
//}

//func (l *Container) Read(b []byte) (n int, err error) {
//	for {
//		running := func() bool {
//			l.lock.RLock()
//			defer l.lock.RUnlock()
//			n, err = l.b1.Read(b)
//			return l.running
//		}()
//		if !running || n > 0 || !errors.Is(err, io.EOF) {
//			return
//		}
//		runtime.Gosched()
//	}
//}

//func (l *Container) Bytes() []byte {
//	l.lock.RLock()
//	defer l.lock.RUnlock()
//	return l.b2.Bytes()
//}

func (l *Container) StartContainer(t errs.Testing, ctx *cntx.TC, req testcontainers.ContainerRequest) (testcontainers.Container, func()) {
	//l.lock.Lock()
	//defer l.lock.Unlock()
	//if l.running {
	//	return l.C, func() {}
	//}
	//l.b1.Reset()
	//l.b2.Reset()

	//req.LifecycleHooks = append(req.LifecycleHooks, testcontainers.ContainerLifecycleHooks{
	//	PostTerminates: []testcontainers.ContainerHook{func(context.Context, testcontainers.Container) error {
	//		l.lock.Lock()
	//		defer l.lock.Lock()
	//		l.running = false
	//		return nil
	//	}},
	//})

	l.C = errs.Must(testcontainers.GenericContainer(ctx.Starting(), testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	}))(t)
	//l.running = true
	// Start log reader after starting
	//l.w = io.MultiWriter(&l.b1, &l.b2)
	//l.C.FollowOutput((*containerLogs)(l))
	// Starts its own goroutine
	//ctxLogs, cancelLogs := context.WithCancel(ctx)
	//errs.Check(t, l.C.StartLogProducer(ctxLogs))
	return l.C, func() {
		//cancelLogs()
		ctx, cancel := context.WithTimeout(ctx.Terminating(), time.Second*10)
		defer cancel()
		to := time.Second * 5
		errs.Check(t, l.C.Stop(ctx, &to))
		errs.Check(t, l.C.Terminate(ctx))
	}
}

type MainContextEntry[D interface {
	templates.Dot
	GetNetworkName() string
}] struct {
	p          *MainContext
	Dockerfile archive.Entry[[]byte]
	Caddyfile  archive.Entry[[]byte]
	Config     D
	Logs       Container
}

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

	return mce.Logs.StartContainer(mce.p.t, mce.p.Ctx, testcontainers.ContainerRequest{
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
}
