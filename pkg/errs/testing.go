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
	}
)

func NewTestMain(m *testing.M) *TestMain { return &TestMain{M: m} }
func (t *TestMain) Exit() {
	if r := recover(); r != nil {
		if t.code == 0 {
			t.code = 1
		}
		t.Logf("caught panic: %v", r)
	}
	os.Exit(t.code)
}
func (t *TestMain) Run()                              { t.code = t.M.Run() }
func (t *TestMain) Helper()                           {}
func (t *TestMain) Logf(s string, a ...any)           { slog.Info(fmt.Sprintf(s, a...)) }
func (t *TestMain) Errorf(s string, a ...interface{}) { slog.Error(fmt.Sprintf(s, a...)) }
func (t *TestMain) FailNow()                          { panic(nil) }
