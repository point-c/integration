package integration

import (
	"context"
	"testing"
	"time"
)

const DefaultTestTimeout = time.Minute * 5

func TestingDeadline(t testing.TB) time.Time {
	t.Helper()
	if t, ok := t.(*testing.T); ok {
		if d, ok := t.Deadline(); ok {
			return d
		}
	}
	return time.Now().Add(DefaultTestTimeout)
}

func TestingContext(t testing.TB, ctx context.Context) (context.Context, context.CancelFunc) {
	t.Helper()
	return context.WithDeadline(ctx, TestingDeadline(t))
}
