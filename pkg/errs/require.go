package errs

import "github.com/stretchr/testify/require"

func Equal[T any](t Testing, exp, got T) {
	t.Helper()
	require.Equal(t, exp, got)
}
