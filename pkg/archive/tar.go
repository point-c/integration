package archive

import (
	"archive/tar"
	"bytes"
	"github.com/point-c/integration/pkg/errs"
	"io"
	"os"
)

type Tar struct{}

type tarWriter struct {
	t errs.Testing
	w *tar.Writer
}

func (Tar) New(t errs.Testing, w io.Writer) Writer {
	return &tarWriter{t: t, w: tar.NewWriter(w)}
}

func (w *tarWriter) Close() error { return w.w.Close() }

func (w *tarWriter) WriteFile(t errs.Testing, f FileHeader, r io.Reader) {
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

func (w *tarWriter) WriteDir(t errs.Testing, f FileHeader) {
	errs.Check(t, w.w.WriteHeader(&tar.Header{
		Typeflag:   tar.TypeDir,
		Name:       f.EntryName(),
		Mode:       int64(os.ModePerm),
		ModTime:    f.EntryTime(),
		AccessTime: f.EntryTime(),
		ChangeTime: f.EntryTime(),
	}))
}
