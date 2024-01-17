package archive

import (
	"bytes"
	"github.com/point-c/integration/errs"
	"io"
	"testing"
)

type (
	Archiver interface {
		New(testing.TB, io.Writer) Writer
	}
	Writer interface {
		io.Closer
		WriteFile(testing.TB, FileHeader, io.Reader)
		WriteDir(testing.TB, FileHeader)
	}
)

func Archive[A Archiver](t testing.TB, files ...FileHeader) io.Reader {
	t.Helper()
	pr, pw := io.Pipe()
	w := (*new(A)).New(t, pw)
	go func() {
		defer errs.Defer(t, pw.Close)
		defer errs.Defer(t, w.Close)
		readDir(t, w, files...)
	}()
	return pr
}

func readDir(t testing.TB, w Writer, files ...FileHeader) {
	for _, f := range files {
		switch f := f.(type) {
		case entry[[]byte]:
			w.WriteFile(t, f, bytes.NewReader(f.EntryContent()))
		case entry[io.Reader]:
			w.WriteFile(t, f, f.EntryContent())
		case entry[[]FileHeader]:
			w.WriteDir(t, f)
			readDir(t, w, f.EntryContent()...)
		}
	}
}
