package docker

import (
	"github.com/point-c/integration/pkg/errs"
	"testing"
	"time"
)

const DefaultTestTimeout = time.Minute * 5

// TestingDeadline gets the deadline that is passed to testing. If it is not set it returns the current time added to the default timeout.
func TestingDeadline(t errs.Testing) time.Time {
	if t, ok := t.(*testing.T); ok {
		if d, ok := t.Deadline(); ok {
			return d
		}
	}
	return time.Now().Add(DefaultTestTimeout)
}
