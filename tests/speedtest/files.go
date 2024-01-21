package speedtest

import (
	_ "embed"
	"github.com/point-c/integration/pkg/errs"
	"github.com/point-c/integration/pkg/templates"
)

//go:embed Dockerfile
var Dockerfile []byte

//go:embed servers.json
var ServersJSON string

type Dot struct {
	Client string
	Server string
}

func (d Dot) ApplyTemplate(t errs.Testing) []byte {
	return templates.ApplyTemplate(t, ServersJSON, d)
}
