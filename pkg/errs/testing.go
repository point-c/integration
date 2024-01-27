package errs

import (
	"fmt"
	"github.com/stretchr/testify/require"
	"log/slog"
	"os"
	"testing"
)

type (
	// Testing is implemented by testing.T and testing.B
	Testing interface {
		Helper()
		Logf(string, ...any)
		require.TestingT
	}
	// TestMain is a wrapper to help with testing a TestMain function.
	TestMain struct {
		*testing.M
		code int
		ok   bool
	}
)

// NewTestMain creates a new TestMain wrapping [testing.M].
func NewTestMain(m *testing.M) *TestMain { return &TestMain{M: m} }
func (t *TestMain) Exit() {
	if !t.ok {
		t.Logf("caught panic: %v", recover())
		if t.code == 0 {
			t.code = 1
		}
	}
	os.Exit(t.code)
}

// Run runs the tests saving the return code for exiting later.
func (t *TestMain) Run() { t.code = t.M.Run(); t.ok = true }

// Helper is a noop.
func (t *TestMain) Helper() {}

// Logf logs to [slog.Info].
func (t *TestMain) Logf(s string, a ...any) { slog.Info(fmt.Sprintf(s, a...)) }

// Errorf logs to [slog.Error].
func (t *TestMain) Errorf(s string, a ...interface{}) { slog.Error(fmt.Sprintf(s, a...)) }

// FailNow causes a nil panic.
func (t *TestMain) FailNow() { panic(nil) }
