package caddyfile

import (
	"fmt"
	"github.com/caddyserver/caddy/v2/caddyconfig"
	_ "github.com/caddyserver/caddy/v2/caddyconfig/httpcaddyfile"
	_ "github.com/caddyserver/caddy/v2/modules/standard"
	_ "github.com/point-c/caddy/module"
	"github.com/point-c/integration/pkg/errs"
	"github.com/point-c/integration/pkg/templates"
	"github.com/point-c/wgapi"
	"github.com/stretchr/testify/require"
	"net"
	"testing"
)

func TestCaddyfile(t *testing.T) {
	serverPriv, serverPub := errs.Must2(wgapi.NewPrivatePublic())(t)
	clientPriv, clientPub := errs.Must2(wgapi.NewPrivatePublic())(t)
	shared := errs.Must(wgapi.NewPreshared())(t)
	serverIP := net.IPv4(192, 168, 199, 1)
	clientIP := net.IPv4(192, 168, 199, 2)
	wgPort := uint16(51820)
	clientName, serverName := "test-client", "test-server"

	tt := []struct {
		Name string
		Dot  templates.Dot
		Exp  []byte
	}{
		{
			Name: "client",
			Dot: templates.DotClient{
				NetworkName:  clientName,
				IP:           clientIP,
				Endpoint:     "localhost",
				EndpointPort: wgPort,
				Private:      clientPriv,
				Public:       serverPub,
				Shared:       shared,
				Directive:    "route {\nrand\n}",
			},
			Exp: caddyconfig.JSON(Cfg{
				Apps: CfgApps{
					Http: CfgAppsHttp{
						Servers: map[string]CfgAppsHttpServers{
							"srv0": {
								Listen: []string{":80"},
								Logs:   new(struct{}),
								ListenerWrappers: []CfgAppsHttpServersLW{
									{
										Listeners: []CfgAppsHttpServersLWL{
											{
												Listener: "point-c",
												Name:     clientName,
												Port:     80,
											},
										},
										Wrapper: "merge",
									},
								},
								Routes: []CfgAppsHttpServersRoutes{
									{
										Handle: []CfgAppsHttpServersRoutesHandle{
											{
												Handler: "subroute",
												Routes: []CfgAppsHttpServersRoutes{
													{
														Handle: []CfgAppsHttpServersRoutesHandle{
															{
																Handler: "rand",
															},
														},
													},
												},
											},
										},
									},
								},
							},
						},
					},
					PointC: CfgAppsPointc{
						Networks: []any{
							CfgAppsPointcNetworksClient{
								Name:      clientName,
								Endpoint:  fmt.Sprintf("localhost:%d", wgPort),
								IP:        clientIP.String(),
								Preshared: string(errs.Must(shared.MarshalText())(t)),
								Public:    string(errs.Must(serverPub.MarshalText())(t)),
								Private:   string(errs.Must(clientPriv.MarshalText())(t)),
								Type:      "wgclient",
							},
						},
					},
				},
			}, nil),
		},
		{
			Name: "server",
			Dot: templates.DotServer{
				NetworkName:    serverName,
				IP:             serverIP,
				Port:           wgPort,
				Private:        serverPriv,
				Peers:          []templates.DotServerPeer{{NetworkName: clientName, IP: clientIP, Public: clientPub, Shared: shared}},
				FwdNetworkName: clientName,
			},
			Exp: caddyconfig.JSON(Cfg{
				Apps: CfgApps{
					Http: CfgAppsHttp{
						Servers: map[string]CfgAppsHttpServers{
							"srv0": {
								Listen: []string{"stub://0.0.0.0:80"},
							},
						},
					},
					PointC: CfgAppsPointc{
						Networks: []any{
							map[string]any{
								"addr":     "0.0.0.0",
								"hostname": "sys",
								"type":     "system",
							},
							CfgAppsPointcNetworksServer{
								Hostname:   serverName,
								Ip:         serverIP.String(),
								ListenPort: int(wgPort),
								Peers: []CfgAppsPointcNetworksServerPeer{
									{
										Hostname:  clientName,
										Ip:        clientIP.String(),
										Preshared: string(errs.Must(shared.MarshalText())(t)),
										Public:    string(errs.Must(clientPub.MarshalText())(t)),
									},
								},
								Private: string(errs.Must(serverPriv.MarshalText())(t)),
								Type:    "wgserver",
							},
						},
						NetOps: []any{
							CfgAppsPointcNetOpsForward{
								Forwards: []any{
									CfgAppsPointcNetOpsForwardTCP{
										Forward: "tcp",
										Ports:   "80:80",
									},
								},
								Hosts: "sys:" + clientName,
								Op:    "forward",
							},
						},
					},
				},
			}, nil),
		},
	}
	for _, tt := range tt {
		t.Run(tt.Name, func(t *testing.T) {
			b := tt.Dot.ApplyTemplate(t)
			adapter := caddyconfig.GetAdapter("caddyfile")
			require.NotNil(t, adapter)
			b, warn, err := adapter.Adapt(b, nil)
			require.NoError(t, err)
			require.Empty(t, warn)
			require.JSONEq(t, string(tt.Exp), string(b))
		})
	}
}

type (
	Cfg struct {
		Apps CfgApps `json:"apps"`
	}
	CfgApps struct {
		Http   CfgAppsHttp   `json:"http"`
		PointC CfgAppsPointc `json:"point-c"`
	}
	CfgAppsPointc struct {
		Networks []any `json:"networks"`
		NetOps   []any `json:"net-ops,omitempty"`
	}
	CfgAppsPointcNetworksClient struct {
		Name      string `json:"name"`
		Endpoint  string `json:"endpoint"`
		IP        string `json:"ip"`
		Preshared string `json:"preshared"`
		Public    string `json:"public"`
		Private   string `json:"private"`
		Type      string `json:"type"`
	}
	CfgAppsPointcNetworksServer struct {
		Hostname   string                            `json:"hostname"`
		Ip         string                            `json:"ip"`
		ListenPort int                               `json:"listen-port"`
		Peers      []CfgAppsPointcNetworksServerPeer `json:"peers"`
		Private    string                            `json:"private"`
		Type       string                            `json:"type"`
	}
	CfgAppsPointcNetOpsForward struct {
		Forwards []any  `json:"forwards"`
		Hosts    string `json:"hosts"`
		Op       string `json:"op"`
	}
	CfgAppsPointcNetOpsForwardTCP struct {
		Forward string    `json:"forward"`
		Ports   string    `json:"ports"`
		Buf     *struct{} `json:"buf"`
	}
	CfgAppsPointcNetworksServerPeer struct {
		Hostname  string `json:"hostname"`
		Ip        string `json:"ip"`
		Preshared string `json:"preshared"`
		Public    string `json:"public"`
	}
	CfgAppsHttp struct {
		Servers map[string]CfgAppsHttpServers `json:"servers"`
	}
	CfgAppsHttpServers struct {
		Listen           []string                   `json:"listen"`
		Routes           []CfgAppsHttpServersRoutes `json:"routes,omitempty"`
		ListenerWrappers []CfgAppsHttpServersLW     `json:"listener_wrappers,omitempty"`
		Logs             *struct{}                  `json:"logs,omitempty"`
	}
	CfgAppsHttpServersLW struct {
		Listeners []CfgAppsHttpServersLWL `json:"listeners"`
		Wrapper   string                  `json:"wrapper"`
	}
	CfgAppsHttpServersLWL struct {
		Listener string `json:"listener"`
		Name     string `json:"name"`
		Port     uint16 `json:"port"`
	}
	CfgAppsHttpServersRoutes struct {
		Handle []CfgAppsHttpServersRoutesHandle `json:"handle"`
	}
	CfgAppsHttpServersRoutesHandle struct {
		Handler string                     `json:"handler"`
		Routes  []CfgAppsHttpServersRoutes `json:"routes,omitempty"`
	}
)
