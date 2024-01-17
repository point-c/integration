package integration

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"github.com/point-c/integration/errs"
	"testing"
)

var (
	//go:embed Dockerfile
	Dockerfile string
	//go:embed modules.json
	Config []byte
	//go:embed Caddyfile.client
	CaddyfileClient string
	//go:embed Caddyfile.server
	CaddyfileServer string
)

func DeJSON[T any](t testing.TB, b []byte) (v T) {
	t.Helper()
	errs.Check(t, json.NewDecoder(bytes.NewReader(b)).Decode(&v))
	return
}
