package templates

import (
	"bytes"
	"encoding"
	"encoding/binary"
	"encoding/hex"
	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
	errs2 "github.com/point-c/integration/pkg/errs"
	"github.com/point-c/wgapi"
	"math"
	"math/rand"
	"net"
	"text/template"
)

func NewDotPair(t errs2.Testing) (DotServer, DotClient) {
	serverPriv, serverPub := errs2.Must2(wgapi.NewPrivatePublic())(t)
	clientPriv, clientPub := errs2.Must2(wgapi.NewPrivatePublic())(t)
	shared := errs2.Must(wgapi.NewPreshared())(t)
	serverName, clientName := func(suffix string) (string, string) { return "server-" + suffix, "client-" + suffix }(hex.EncodeToString(binary.BigEndian.AppendUint32(nil, uint32(rand.Int()))))
	serverIP, clientIP := func(c uint8) (net.IP, net.IP) { return net.IPv4(192, 168, c, 1), net.IPv4(192, 168, c, 2) }(uint8(rand.Intn(math.MaxUint8) + 1))
	return DotServer{
			NetworkName:    serverName,
			IP:             serverIP,
			Port:           uint16(wgapi.DefaultListenPort),
			Private:        serverPriv,
			Peers:          []DotServerPeer{{NetworkName: clientName, IP: clientIP, Public: clientPub, Shared: shared}},
			FwdNetworkName: clientName,
		}, DotClient{
			NetworkName:  clientName,
			Endpoint:     serverIP.String(), // update to actual endpoint
			IP:           clientIP,
			EndpointPort: uint16(wgapi.DefaultListenPort),
			Private:      clientPriv,
			Public:       serverPub,
			Shared:       shared,
		}
}

type Dot interface {
	ApplyTemplate(errs2.Testing) []byte
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

func (ds DotServer) ApplyTemplate(t errs2.Testing) []byte {
	return caddyfile.Format(ApplyTemplate(t, CaddyfileServer, ds))
}

type DotClient struct {
	NetworkName  string
	IP           net.IP
	Endpoint     string
	EndpointPort uint16
	Private      wgapi.PrivateKey
	Public       wgapi.PublicKey
	Shared       wgapi.PresharedKey
}

func (dc DotClient) ApplyTemplate(t errs2.Testing) []byte {
	return caddyfile.Format(ApplyTemplate(t, CaddyfileClient, dc))
}

type DotDockerfile struct {
	Caddy string   `json:"caddy"`
	Mods  []string `json:"mods"`
}

func (dd DotDockerfile) ApplyTemplate(t errs2.Testing) []byte {
	return ApplyTemplate(t, Dockerfile, dd)
}

func ApplyTemplate(t errs2.Testing, tmpl string, dot any) []byte {
	tm := errs2.Must(template.New("").Funcs(template.FuncMap{
		"txt": func(u encoding.TextMarshaler) string { return string(errs2.Must(u.MarshalText())(t)) },
	}).Parse(tmpl))(t)
	var buf bytes.Buffer
	errs2.Check(t, tm.Execute(&buf, dot))
	return caddyfile.Format(buf.Bytes())
}
