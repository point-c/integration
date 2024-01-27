package speedtest_srv

import (
	"errors"
	"fmt"
	"github.com/librespeed/speedtest-cli/defs"
	"net"
	"time"
)

const (
	DefaultUploadSize = 1024
	DefaultTimeout    = time.Second * 10
)

var ListenAddress = net.TCPAddr{IP: net.IPv4zero, Port: 8080}

const (
	MethodSpeedTestPing     = "SpeedTest.Ping"
	MethodSpeedTestDownload = "SpeedTest.Download"
	MethodSpeedTestUpload   = "SpeedTest.Upload"
)

type SpeedTest struct{}

type (
	PingRequest struct {
		ServerInfo
		Count uint
	}
	PingResponse struct {
		Ping   float64 `json:"ping_ms"`
		Jitter float64 `json:"jitter_ms"`
	}
)

func (st *SpeedTest) Ping(req PingRequest, resp *PingResponse) (err error) {
	if err = req.Validate(); err != nil {
		return err
	}
	if req.Count == 0 {
		req.Count = 12
	}
	resp.Ping, resp.Jitter, err = req.Server().PingAndJitter(int(req.Count))
	return
}

type (
	DownloadRequest struct {
		ServerInfo
		Count   uint          `json:"count"`
		Size    uint          `json:"size_mb"`
		Timeout time.Duration `json:"timeout"`
	}
	UploadRequest struct {
		ServerInfo
		Count   uint          `json:"count"`
		Size    uint          `json:"size_kb"`
		Timeout time.Duration `json:"timeout"`
	}
	SpeedResponse struct {
		Speed float64 `json:"average_mbps"`
		Total int     `json:"total_bytes"`
	}
)

func (st *SpeedTest) Download(req DownloadRequest, resp *SpeedResponse) (err error) {
	if err = req.Validate(); err != nil {
		return err
	}
	if req.Count == 0 {
		req.Count = 3
	}
	if req.Size == 0 {
		req.Size = 100
	}
	if req.Timeout == 0 {
		req.Timeout = DefaultTimeout
	}
	resp.Speed, resp.Total, err = req.Server().Download(true, false, false,
		int(req.Count),
		int(max(min(1024, req.Size), 4)),
		req.Timeout,
	)
	return
}

func (st *SpeedTest) Upload(req UploadRequest, resp *SpeedResponse) (err error) {
	if err = req.Validate(); err != nil {
		return err
	}
	if req.Size == 0 {
		req.Size = DefaultUploadSize
	}
	if req.Count == 0 {
		req.Count = 3
	}
	if req.Timeout == 0 {
		req.Timeout = DefaultTimeout
	}
	resp.Speed, resp.Total, err = req.Server().Upload(false, true, false, false,
		int(req.Count),
		int(req.Size),
		req.Timeout,
	)
	return
}

type ServerInfo struct {
	Name     string
	Hostname string
	Port     uint16
}

func (s ServerInfo) Validate() (err error) {
	if s.Hostname == "" {
		err = errors.Join(err, errors.New("hostname cannot be empty"))
	}
	if s.Port == 0 {
		err = errors.Join(err, errors.New("port cannot be 0"))
	}
	return
}

func (s ServerInfo) Server() *defs.Server {
	return &defs.Server{
		ID:          1,
		Name:        s.Name,
		Server:      fmt.Sprintf("http://%s:%d", s.Hostname, s.Port),
		DownloadURL: "garbage.php",
		UploadURL:   "empty.php",
		PingURL:     "empty.php",
		GetIPURL:    "getIP.php",
		NoICMP:      true,
	}
}
