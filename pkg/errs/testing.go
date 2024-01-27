package errs

import (
	"fmt"
	"github.com/stretchr/testify/require"
	"log/slog"
	"os"
	"testing"
)

type (
	Testing interface {
		Helper()
		Logf(string, ...any)
		require.TestingT
	}
	TestMain struct {
		*testing.M
		code int
		ok   bool
	}
)

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
func (t *TestMain) Run()                              { t.code = t.M.Run(); t.ok = true }
func (t *TestMain) Helper()                           {}
func (t *TestMain) Logf(s string, a ...any)           { slog.Info(fmt.Sprintf(s, a...)) }
func (t *TestMain) Errorf(s string, a ...interface{}) { slog.Error(fmt.Sprintf(s, a...)) }
func (t *TestMain) FailNow()                          { panic(nil) }
