package errs

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func Must[T any](v T, err error) func(testing.TB) T {
	return func(t testing.TB) T {
		t.Helper()
		Check(t, err)
		return v
	}
}

func Must2[T1, T2 any](t1 T1, t2 T2, err error) func(testing.TB) (T1, T2) {
	return func(t testing.TB) (T1, T2) {
		t.Helper()
		Check(t, err)
		return t1, t2
	}
}

func Defer(t testing.TB, fn func() error) {
	t.Helper()
	require.NoError(t, fn())
}

func Check(t testing.TB, err error) {
	t.Helper()
	require.NoError(t, err)
}
