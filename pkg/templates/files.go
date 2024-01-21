package templates

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"github.com/point-c/integration/pkg/errs"
)

var (
	//go:embed Dockerfile
	Dockerfile string
	//go:embed client_modules.json
	ClientConfig []byte
	//go:embed server_modules.json
	ServerConfig []byte
	//go:embed Caddyfile.client
	CaddyfileClient string
	//go:embed Caddyfile.server
	CaddyfileServer string
)

func DeJSON[T any](t errs.Testing, b []byte) (v T) {
	errs.Check(t, json.NewDecoder(bytes.NewReader(b)).Decode(&v))
	return
}
