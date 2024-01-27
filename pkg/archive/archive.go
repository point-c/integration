// Package archive is a utility that allows for simple archiving of files.
package archive

import (
	"bytes"
	"github.com/point-c/integration/pkg/errs"
	"io"
	"strings"
)

type (
	// Archiver is implemented by a compression method.
	Archiver interface {
		// New produces a new instance of this method.
		New(errs.Testing, io.Writer) Writer
	}
	// Writer represents the archive being written.
	Writer interface {
		io.Closer
		WriteFile(errs.Testing, FileHeader, io.Reader)
		WriteDir(errs.Testing, FileHeader)
	}
)

// Archive is responsible for the actual archiving.
func Archive[A Archiver](t errs.Testing, w io.Writer, files ...FileHeader) {
	ww := (*new(A)).New(t, w)
	defer errs.Defer(t, ww.Close)
	readDir(t, ww, nil, files)
}

func readDir(t errs.Testing, w Writer, path []string, files []FileHeader) {
	for _, f := range files {
		fn := Entry[[]byte]{
			Name: strings.Join(append(path, f.EntryName()), "/"),
			Time: f.EntryTime(),
		}
		switch f := f.(type) {
		case entry[[]byte]:
			w.WriteFile(t, fn, bytes.NewReader(f.EntryContent()))
		case entry[io.Reader]:
			w.WriteFile(t, fn, f.EntryContent())
		case entry[[]FileHeader]:
			w.WriteDir(t, fn)
			readDir(t, w, append(path, f.EntryName()), f.EntryContent())
		}
	}
}
