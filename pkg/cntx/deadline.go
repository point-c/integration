package cntx

import (
	"github.com/point-c/integration/pkg/errs"
	"testing"
	"time"
)

const DefaultTestTimeout = time.Minute * 5

func TestingDeadline(t errs.Testing) time.Duration {
	t.Helper()
	if t, ok := t.(*testing.T); ok {
		if d, ok := t.Deadline(); ok {
			return d.Sub(time.Now())
		}
	}
	return DefaultTestTimeout
}
