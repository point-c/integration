// Package errs allows for safe error handling in tests.
package errs

// Must returns a function that evaluates the given parameters and fails the tests immediately if err != nil.
func Must[T any](v T, err error) func(Testing) T {
	return func(t Testing) T {
		t.Helper()
		Check(t, err)
		return v
	}
}

// Must2 is the same as Must but for returning two values.
func Must2[T1, T2 any](t1 T1, t2 T2, err error) func(Testing) (T1, T2) {
	return func(t Testing) (T1, T2) {
		t.Helper()
		Check(t, err)
		return t1, t2
	}
}

// Defer checks if something producing an error while in defer.
func Defer(t Testing, fn func() error) {
	t.Helper()
	Check(t, fn())
}

// Check checks for an error, stopping the test and failing if one is encountered.
func Check(t Testing, err error) {
	t.Helper()
	if err != nil {
		t.Errorf("%v", err)
		t.FailNow()
	}
}
