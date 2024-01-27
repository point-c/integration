//go:build docker

package main

import (
	"log/slog"
	"net"
	"net/rpc"
	"os"
	"speedtest/speedtest-srv"
)

func main() {
	if err := rpc.Register(new(speedtest_srv.SpeedTest)); err != nil {
		slog.Error("failed to register rpc", "err", err)
		os.Exit(1)
	}
	slog.Info("starting server", "hostname", speedtest_srv.ListenAddress.IP.String(), "port", speedtest_srv.ListenAddress.Port)
	ln, err := net.Listen("tcp", speedtest_srv.ListenAddress.String())
	if err != nil {
		slog.Error("failed to listen", "address", speedtest_srv.ListenAddress.IP.String(), "err", err)
		os.Exit(1)
	}
	slog.Info("server started", "hostname", speedtest_srv.ListenAddress.IP.String(), "port", speedtest_srv.ListenAddress.Port)
	for {
		conn, err := ln.Accept()
		if err != nil {
			slog.Error("server stopped", "err", err)
			os.Exit(1)
		}
		go rpc.ServeConn(conn)
	}
}
