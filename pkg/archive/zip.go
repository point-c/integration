package archive

import (
	"archive/zip"
	"github.com/point-c/integration/pkg/errs"
	"io"
	"io/fs"
	"os"
)

// Zip allows writing .zip archives.
type Zip struct{}

type zipWriter struct {
	t errs.Testing
	w *zip.Writer
}

func (Zip) New(t errs.Testing, w io.Writer) Writer {
	return &zipWriter{t: t, w: zip.NewWriter(w)}
}

func (w *zipWriter) Close() error { return w.w.Close() }

func (w *zipWriter) WriteFile(t errs.Testing, f FileHeader, r io.Reader) {
	errs.Must(io.Copy(errs.Must(w.w.CreateHeader(zipHeader(f, zip.Deflate, os.ModePerm)))(t), r))(t)
}

func (w *zipWriter) WriteDir(t errs.Testing, f FileHeader) {
	errs.Must(w.w.CreateHeader(zipHeader(f, zip.Store, os.ModePerm|os.ModeDir)))(t)
}

func zipHeader(f FileHeader, method uint16, mode fs.FileMode) *zip.FileHeader {
	hdr := zip.FileHeader{Name: f.EntryName(), Modified: f.EntryTime(), Method: method}
	hdr.SetMode(mode)
	return &hdr
}
