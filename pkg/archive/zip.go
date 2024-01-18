package archive

import (
	"archive/zip"
	errs2 "github.com/point-c/integration/pkg/errs"
	"io"
	"io/fs"
	"os"
)

type Zip struct{}

type zipWriter struct {
	t errs2.Testing
	w *zip.Writer
}

func (Zip) New(t errs2.Testing, w io.Writer) Writer {
	t.Helper()
	return &zipWriter{t: t, w: zip.NewWriter(w)}
}

func (w *zipWriter) Close() error { w.t.Helper(); return w.w.Close() }

func (w *zipWriter) WriteFile(t errs2.Testing, f FileHeader, r io.Reader) {
	w.t.Helper()
	errs2.Must(io.Copy(errs2.Must(w.w.CreateHeader(zipHeader(f, zip.Deflate, os.ModePerm)))(t), r))(t)
}

func (w *zipWriter) WriteDir(t errs2.Testing, f FileHeader) {
	w.t.Helper()
	errs2.Must(w.w.CreateHeader(zipHeader(f, zip.Store, os.ModePerm|os.ModeDir)))(t)
}

func zipHeader(f FileHeader, method uint16, mode fs.FileMode) *zip.FileHeader {
	hdr := zip.FileHeader{Name: f.EntryName(), Modified: f.EntryTime(), Method: method}
	hdr.SetMode(mode)
	return &hdr
}
