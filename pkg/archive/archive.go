package archive

import (
	"bytes"
	errs2 "github.com/point-c/integration/pkg/errs"
	"io"
)

type (
	Archiver interface {
		New(errs2.Testing, io.Writer) Writer
	}
	Writer interface {
		io.Closer
		WriteFile(errs2.Testing, FileHeader, io.Reader)
		WriteDir(errs2.Testing, FileHeader)
	}
)

func Archive[A Archiver](t errs2.Testing, w io.Writer, files ...FileHeader) {
	t.Helper()
	ww := (*new(A)).New(t, w)
	defer errs2.Defer(t, ww.Close)
	readDir(t, ww, files...)
}

func readDir(t errs2.Testing, w Writer, files ...FileHeader) {
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
