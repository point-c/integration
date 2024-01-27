package internal

import (
	"bytes"
	_ "embed"
	"github.com/point-c/integration/pkg/archive"
	"github.com/point-c/integration/pkg/errs"
	"io"
	"sync"
	"time"
)

var (
	//go:embed speedtest-srv/Dockerfile
	Dockerfile []byte
	//go:embed speedtest-srv/main.go
	Main []byte
	//go:embed speedtest-srv/speedtest-srv/model.go
	Models  []byte
	ctx     []byte
	ctxOnce sync.Once
)

func Context(t errs.Testing) io.Reader {
	ctxOnce.Do(func() {
		var buf bytes.Buffer
		archive.Archive[archive.Tar](t, &buf,
			archive.Entry[[]byte]{
				Name:    "Dockerfile",
				Time:    time.Now(),
				Content: Dockerfile,
			},
			archive.Entry[[]byte]{
				Name:    "main.go",
				Time:    time.Now(),
				Content: Main,
			},
			archive.Entry[[]archive.FileHeader]{
				Name: "speedtest-srv",
				Time: time.Now(),
				Content: []archive.FileHeader{
					archive.Entry[[]byte]{
						Name:    "model.go",
						Time:    time.Now(),
						Content: Models,
					},
				},
			},
		)
		ctx = buf.Bytes()
	})
	return bytes.NewReader(ctx)
}
