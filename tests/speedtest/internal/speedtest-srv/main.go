package main

import (
	"encoding/json"
	"fmt"
	"github.com/point-c/integration/tests/speedtest/internal/speedtest-srv/speedtest-srv"
	"log/slog"
	"net/http"
)

const (
	ServerHost        = "0.0.0.0"
	ServerPort        = 8080
	DefaultUploadSize = 1024
)

func main() {
	HandleFunc[speedtest_srv.PingRequest, speedtest_srv.PingResponse]("/ping", func(req speedtest_srv.PingRequest) (resp speedtest_srv.PingResponse, err error) {
		if req.Count == 0 {
			req.Count = 12
		}
		resp.Ping, resp.Jitter, err = req.Server().PingAndJitter(int(req.Count))
		return
	})
	HandleFunc[speedtest_srv.DownloadRequest, speedtest_srv.SpeedResponse]("/download", func(req speedtest_srv.DownloadRequest) (resp speedtest_srv.SpeedResponse, err error) {
		if req.Count == 0 {
			req.Count = 3
		}
		resp.Speed, resp.Total, err = req.Server().Download(true, false, false,
			int(req.Count),
			int(max(min(1024, req.Size), 4)),
			max(req.Timeout, 1),
		)
		return
	})
	HandleFunc[speedtest_srv.UploadRequest, speedtest_srv.SpeedResponse]("/upload", func(req speedtest_srv.UploadRequest) (resp speedtest_srv.SpeedResponse, err error) {
		if req.Size == 0 {
			req.Size = DefaultUploadSize
		}
		resp.Speed, resp.Total, err = req.Server().Download(true, false, false,
			int(max(req.Count, 1)),
			int(req.Size),
			max(req.Timeout, 1),
		)
		return
	})
	slog.Info("server started", "hostname", ServerHost, "port", ServerPort)
	slog.Info("server stopped", "err", http.ListenAndServe(fmt.Sprintf("%s:%d", ServerHost, ServerPort), nil))
}

func HandleFunc[Req, Resp any](pattern string, handler func(Req) (Resp, error)) {
	http.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
		var req Req
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			sendJson(w, http.StatusBadRequest, speedtest_srv.GenericResponse[*any]{Err: err})
			return
		}

		if v, ok := any(req).(speedtest_srv.Validator); ok {
			if err := v.Validate(); err != nil {
				sendJson(w, http.StatusBadRequest, speedtest_srv.GenericResponse[*any]{Err: err})
				return
			}
		}

		var resp speedtest_srv.GenericResponse[Resp]
		code := http.StatusOK
		if resp.Resp, resp.Err = handler(req); resp.Err != nil {
			code = http.StatusInternalServerError
		}
		sendJson(w, code, resp)
	})
}

func sendJson[T any](w http.ResponseWriter, code int, v T) {
	w.WriteHeader(code)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("failed to write response", "error", err)
	}
}
