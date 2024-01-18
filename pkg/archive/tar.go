package archive

import (
	"archive/tar"
	"bytes"
	errs2 "github.com/point-c/integration/pkg/errs"
	"io"
	"os"
)

type Tar struct{}

type tarWriter struct {
	t errs2.Testing
	w *tar.Writer
}

func (Tar) New(t errs2.Testing, w io.Writer) Writer {
	t.Helper()
	return &tarWriter{t: t, w: tar.NewWriter(w)}
}

func (w *tarWriter) Close() error { w.t.Helper(); return w.w.Close() }

func (w *tarWriter) WriteFile(t errs2.Testing, f FileHeader, r io.Reader) {
	w.t.Helper()
	b := bytes.NewReader(errs2.Must(io.ReadAll(r))(t))
	errs2.Check(t, w.w.WriteHeader(&tar.Header{
		Typeflag:   tar.TypeReg,
		Name:       f.EntryName(),
		Size:       int64(b.Len()),
		Mode:       int64(os.ModePerm),
		ModTime:    f.EntryTime(),
		AccessTime: f.EntryTime(),
		ChangeTime: f.EntryTime(),
	}))
	errs2.Must(io.Copy(w.w, b))(t)
}

func (w *tarWriter) WriteDir(t errs2.Testing, f FileHeader) {
	w.t.Helper()
	errs2.Check(t, w.w.WriteHeader(&tar.Header{
		Typeflag:   tar.TypeDir,
		Name:       f.EntryName(),
		Mode:       int64(os.ModePerm),
		ModTime:    f.EntryTime(),
		AccessTime: f.EntryTime(),
		ChangeTime: f.EntryTime(),
	}))
}
