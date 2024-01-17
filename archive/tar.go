package archive

import (
	"archive/tar"
	"bytes"
	"github.com/point-c/integration/errs"
	"io"
	"os"
	"testing"
)

type Tar struct{}

type tarWriter struct {
	t testing.TB
	w *tar.Writer
}

func (Tar) New(t testing.TB, w io.Writer) Writer {
	t.Helper()
	return &tarWriter{t: t, w: tar.NewWriter(w)}
}

func (w *tarWriter) Close() error { w.t.Helper(); return w.w.Close() }

func (w *tarWriter) WriteFile(t testing.TB, f FileHeader, r io.Reader) {
	w.t.Helper()
	b := bytes.NewReader(errs.Must(io.ReadAll(r))(t))
	errs.Check(t, w.w.WriteHeader(&tar.Header{
		Typeflag:   tar.TypeReg,
		Name:       f.EntryName(),
		Size:       int64(b.Len()),
		Mode:       int64(os.ModePerm),
		ModTime:    f.EntryTime(),
		AccessTime: f.EntryTime(),
		ChangeTime: f.EntryTime(),
	}))
	errs.Must(io.Copy(w.w, b))(t)
}

func (w *tarWriter) WriteDir(t testing.TB, f FileHeader) {
	w.t.Helper()
	errs.Check(t, w.w.WriteHeader(&tar.Header{
		Typeflag:   tar.TypeDir,
		Name:       f.EntryName(),
		Mode:       int64(os.ModePerm),
		ModTime:    f.EntryTime(),
		AccessTime: f.EntryTime(),
		ChangeTime: f.EntryTime(),
	}))
}
