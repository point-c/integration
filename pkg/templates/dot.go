package templates

import (
	"bytes"
	"encoding"
	"encoding/binary"
	"encoding/hex"
	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
	"github.com/point-c/integration/pkg/errs"
	"github.com/point-c/wgapi"
	"math"
	"math/rand"
	"net"
	"text/template"
)

func NewDotPair(t errs.Testing) (DotClient, DotServer) {
	serverPriv, serverPub := errs.Must2(wgapi.NewPrivatePublic())(t)
	clientPriv, clientPub := errs.Must2(wgapi.NewPrivatePublic())(t)
	shared := errs.Must(wgapi.NewPreshared())(t)
	serverName, clientName := func(suffix string) (string, string) { return "server-" + suffix, "client-" + suffix }(hex.EncodeToString(binary.BigEndian.AppendUint32(nil, uint32(rand.Int()))))
	serverIP, clientIP := func(c uint8) (net.IP, net.IP) { return net.IPv4(192, 168, c, 1), net.IPv4(192, 168, c, 2) }(uint8(rand.Intn(math.MaxUint8) + 1))
	return DotClient{
			NetworkName:  clientName,
			Endpoint:     serverIP.String(), // update to actual endpoint
			IP:           clientIP,
			EndpointPort: uint16(wgapi.DefaultListenPort),
			Private:      clientPriv,
			Public:       serverPub,
			Shared:       shared,
		}, DotServer{
			NetworkName:    serverName,
			IP:             serverIP,
			Port:           uint16(wgapi.DefaultListenPort),
			Private:        serverPriv,
			Peers:          []DotServerPeer{{NetworkName: clientName, IP: clientIP, Public: clientPub, Shared: shared}},
			FwdNetworkName: clientName,
		}
}

type Dot interface {
	ApplyTemplate(errs.Testing) []byte
}

type (
	DotServer struct {
		NetworkName    string
		FwdNetworkName string
		IP             net.IP
		Port           uint16
		Private        wgapi.PrivateKey
		Peers          []DotServerPeer
	}
	DotServerPeer struct {
		NetworkName string
		IP          net.IP
		Public      wgapi.PublicKey
		Shared      wgapi.PresharedKey
	}
)

func (ds DotServer) ApplyTemplate(t errs.Testing) []byte {
	return caddyfile.Format(ApplyTemplate(t, CaddyfileServer, ds))
}

func (ds DotServer) GetNetworkName() string {
	return ds.NetworkName
}

type DotClient struct {
	NetworkName  string
	IP           net.IP
	Endpoint     string
	EndpointPort uint16
	Private      wgapi.PrivateKey
	Public       wgapi.PublicKey
	Shared       wgapi.PresharedKey
	Directive    string
}

func (dc DotClient) ApplyTemplate(t errs.Testing) []byte {
	return caddyfile.Format(ApplyTemplate(t, CaddyfileClient, dc))
}

func (dc DotClient) GetNetworkName() string {
	return dc.NetworkName
}

type DotDockerfile struct {
	Caddy string   `json:"caddy"`
	Mods  []string `json:"mods"`
}

func (dd DotDockerfile) ApplyTemplate(t errs.Testing) []byte {
	return ApplyTemplate(t, Dockerfile, dd)
}

func ApplyTemplate(t errs.Testing, tmpl string, dot any) []byte {
	tm := errs.Must(template.New("").Funcs(template.FuncMap{
		"txt": func(u encoding.TextMarshaler) string { return string(errs.Must(u.MarshalText())(t)) },
	}).Parse(tmpl))(t)
	var buf bytes.Buffer
	errs.Check(t, tm.Execute(&buf, dot))
	return caddyfile.Format(buf.Bytes())
}
