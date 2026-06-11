package server

import (
	"context"

	"connectrpc.com/connect"

	v1 "github.com/singaaka/darkside-fleet/gen/go/fleet/v1"
	"github.com/singaaka/darkside-fleet/internal/db/dbgen"
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
	// registry_port is intentionally NOT a setting — it's an internal detail
	// fixed by darkside-fleet (see internal/config/RegistryPort).
	s, err := h.loadSettings(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(s), nil
}

func (h *settingsHandler) loadSettings(ctx context.Context) (*v1.Settings, error) {
	domain, _ := h.s.opts.Store.GetSetting(ctx, "domain")
	paasNodeID, _ := h.s.opts.Store.GetSetting(ctx, "darkside_paas_node_id")
	return &v1.Settings{
		Domain:             domain,
		DarksidePaasNodeId: paasNodeID,
	}, nil
}
