package server

import (
	"context"
	"fmt"

	"connectrpc.com/connect"

	v1 "github.com/singaaka/darkside-node-manager/gen/go/nm/v1"
	"github.com/singaaka/darkside-node-manager/internal/db/dbgen"
)

type settingsHandler struct{ s *Server }

func (h *settingsHandler) Get(ctx context.Context, _ *connect.Request[v1.GetSettingsRequest]) (*connect.Response[v1.Settings], error) {
	s, err := h.loadSettings(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(s), nil
}

func (h *settingsHandler) Update(ctx context.Context, req *connect.Request[v1.UpdateSettingsRequest]) (*connect.Response[v1.Settings], error) {
	if req.Msg.Domain != "" {
		if err := h.s.opts.Store.SetSetting(ctx, dbgen.SetSettingParams{Key: "domain", Value: req.Msg.Domain}); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}
	if req.Msg.RegistryPort > 0 {
		if err := h.s.opts.Store.SetSetting(ctx, dbgen.SetSettingParams{Key: "registry_port", Value: fmt.Sprintf("%d", req.Msg.RegistryPort)}); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}
	s, err := h.loadSettings(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(s), nil
}

func (h *settingsHandler) loadSettings(ctx context.Context) (*v1.Settings, error) {
	domain, _ := h.s.opts.Store.GetSetting(ctx, "domain")
	regPortStr, _ := h.s.opts.Store.GetSetting(ctx, "registry_port")
	paasNodeID, _ := h.s.opts.Store.GetSetting(ctx, "darkside_paas_node_id")

	regPort := int32(5000)
	if regPortStr != "" {
		var p int
		fmt.Sscanf(regPortStr, "%d", &p)
		regPort = int32(p)
	}
	return &v1.Settings{
		Domain:             domain,
		RegistryPort:       regPort,
		DarksidePaasNodeId: paasNodeID,
	}, nil
}
