package cntx

import (
	"context"
	"github.com/point-c/integration/pkg/errs"
	"os"
	"os/signal"
	"sync"
	"time"
)

type (
	TC struct {
		*CtxCncl
		starting    func() *CtxCncl
		terminating func() *CtxCncl
	}
	CtxCncl struct {
		context.Context
		cncl context.CancelFunc
	}
)

func (cc *CtxCncl) set(ctx context.Context, cncl context.CancelFunc) *CtxCncl {
	cc.Context, cc.cncl = ctx, cncl
	return cc
}

func Context(t errs.Testing, ctx context.Context, startTO, termTO time.Duration) *TC {
	tc := &TC{CtxCncl: new(CtxCncl).set(base(t, ctx))}
	tc.starting = sync.OnceValue(func() *CtxCncl { return new(CtxCncl).set(context.WithTimeout(tc, startTO)) })
	tc.terminating = sync.OnceValue(func() *CtxCncl { tc.starting().cncl(); return new(CtxCncl).set(context.WithTimeout(tc, termTO)) })
	return tc
}

func base(t errs.Testing, ctx context.Context) (context.Context, context.CancelFunc) {
	ctx, stop := signal.NotifyContext(ctx, os.Kill, os.Interrupt)
	ctx, cancel := context.WithTimeout(ctx, TestingDeadline(t))
	return ctx, func() { defer cancel(); stop() }
}

func (cc *CtxCncl) Cancel()                 { cc.cncl() }
func (tc *TC) Starting() context.Context    { return tc.starting() }
func (tc *TC) Terminating() context.Context { return tc.terminating() }
