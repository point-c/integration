package archive

import (
	"archive/zip"
	"github.com/point-c/integration/errs"
	"io"
	"io/fs"
	"os"
	"testing"
)

type Zip struct{}

type zipWriter struct {
	t testing.TB
	w *zip.Writer
}

func (Zip) New(t testing.TB, w io.Writer) Writer {
	t.Helper()
	return &zipWriter{t: t, w: zip.NewWriter(w)}
}

func (w *zipWriter) Close() error { w.t.Helper(); return w.w.Close() }

func (w *zipWriter) WriteFile(t testing.TB, f FileHeader, r io.Reader) {
	w.t.Helper()
	errs.Must(io.Copy(errs.Must(w.w.CreateHeader(zipHeader(f, zip.Deflate, os.ModePerm)))(t), r))(t)
}

func (w *zipWriter) WriteDir(t testing.TB, f FileHeader) {
	w.t.Helper()
	errs.Must(w.w.CreateHeader(zipHeader(f, zip.Store, os.ModePerm|os.ModeDir)))(t)
}

func zipHeader(f FileHeader, method uint16, mode fs.FileMode) *zip.FileHeader {
	hdr := zip.FileHeader{Name: f.EntryName(), Modified: f.EntryTime(), Method: method}
	hdr.SetMode(mode)
	return &hdr
}
