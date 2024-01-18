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
		*ctxCncl
		starting    func() *ctxCncl
		terminating func() *ctxCncl
	}
	ctxCncl struct {
		ctx  context.Context
		cncl context.CancelFunc
	}
)

func (cc *ctxCncl) set(ctx context.Context, cncl context.CancelFunc) *ctxCncl {
	cc.ctx, cc.cncl = ctx, cncl
	return cc
}

func Context(t errs.Testing, ctx context.Context, startTO, termTO time.Duration) *TC {
	t.Helper()
	tc := &TC{ctxCncl: new(ctxCncl).set(base(t, ctx))}
	tc.starting = sync.OnceValue(func() *ctxCncl { return new(ctxCncl).set(context.WithTimeout(tc.ctx, startTO)) })
	tc.terminating = sync.OnceValue(func() *ctxCncl { tc.starting().cncl(); return new(ctxCncl).set(context.WithTimeout(tc.ctx, termTO)) })
	return tc
}

func base(t errs.Testing, ctx context.Context) (context.Context, context.CancelFunc) {
	t.Helper()
	ctx, stop := signal.NotifyContext(ctx, os.Kill, os.Interrupt)
	ctx, cancel := context.WithTimeout(ctx, TestingDeadline(t))
	return ctx, func() { t.Helper(); defer cancel(); stop() }
}

func (tc *TC) Cancel()                      { tc.cncl() }
func (tc *TC) Starting() context.Context    { return tc.starting().ctx }
func (tc *TC) Terminating() context.Context { return tc.terminating().ctx }
