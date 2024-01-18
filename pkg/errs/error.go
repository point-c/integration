package errs

import (
	"github.com/stretchr/testify/require"
)

func Must[T any](v T, err error) func(Testing) T {
	return func(t Testing) T {
		t.Helper()
		Check(t, err)
		return v
	}
}

func Must2[T1, T2 any](t1 T1, t2 T2, err error) func(Testing) (T1, T2) {
	return func(t Testing) (T1, T2) {
		t.Helper()
		Check(t, err)
		return t1, t2
	}
}

func Defer(t Testing, fn func() error) {
	t.Helper()
	require.NoError(t, fn())
}

func Check(t Testing, err error) {
	t.Helper()
	require.NoError(t, err)
}
