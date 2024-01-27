package speedtest_srv

import (
	"errors"
	"fmt"
	"github.com/librespeed/speedtest-cli/defs"
	"time"
)

type Validator interface {
	Validate() error
}

type GenericResponse[T any] struct {
	Err  error `json:"err,omitempty"`
	Resp T     `json:"resp,omitempty"`
}

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

type (
	DownloadRequest struct {
		ServerInfo
		Count   uint          `json:"count"`
		Size    uint          `json:"size_mb"`
		Timeout time.Duration `json:"timeout"`
	}
	SpeedResponse struct {
		Speed float64 `json:"average_mbps"`
		Total int     `json:"total_bytes"`
	}
)

type (
	UploadRequest struct {
		ServerInfo
		Count   uint          `json:"count"`
		Size    uint          `json:"size_kb"`
		Timeout time.Duration `json:"timeout"`
	}
)

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
		Server:      fmt.Sprintf("//%s:%d", s.Hostname, s.Port),
		DownloadURL: "garbage.php",
		UploadURL:   "empty.php",
		PingURL:     "empty.php",
		GetIPURL:    "getIP.php",
		NoICMP:      true,
	}
}
