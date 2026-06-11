package server

import (
	"context"
	"database/sql"
	"errors"

	"connectrpc.com/connect"

	v1 "github.com/singaaka/darkside-fleet/gen/go/fleet/v1"
)

type clusterConfigHandler struct{ s *Server }

func (h *clusterConfigHandler) GetNodeConfig(ctx context.Context, req *connect.Request[v1.GetNodeConfigRequest]) (*connect.Response[v1.NodeConfig], error) {
	cfg, err := h.s.opts.Store.GetClusterConfig(ctx, req.Msg.NodeId)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("config not found — node may not have been provisioned yet"))
	}
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&v1.NodeConfig{
		NodeId:        cfg.NodeID,
		NomadHcl:      cfg.NomadHcl,
		ConsulHcl:     cfg.ConsulHcl,
		TraefikYml:    cfg.TraefikYml,
		UpdatedAtUnix: cfg.UpdatedAt.Unix(),
	}), nil
}
