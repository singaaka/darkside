package server

import (
	"context"
	"time"

	darksidev1 "github.com/singaaka/darkside-paas/gen/go/darkside/v1"

	"connectrpc.com/connect"
)

type healthHandler struct {
	version string
	domain  string
}

func (h *healthHandler) Ping(_ context.Context, _ *connect.Request[darksidev1.PingRequest]) (*connect.Response[darksidev1.PingResponse], error) {
	return connect.NewResponse(&darksidev1.PingResponse{
		Message:       "pong",
		TimestampUnix: time.Now().Unix(),
	}), nil
}

func (h *healthHandler) Info(_ context.Context, _ *connect.Request[darksidev1.InfoRequest]) (*connect.Response[darksidev1.InfoResponse], error) {
	return connect.NewResponse(&darksidev1.InfoResponse{
		Version: h.version,
		Domain:  h.domain,
	}), nil
}
