package server

import "net/http"

// handleOperatorConfig exposes the runtime-resolved operator config so the
// dashboard can show what `darkside.toml` produced. Read-only by design —
// changes are made by editing the file and restarting.
//
// GET /api/v1/operator-config
func (s *Server) handleOperatorConfig(w http.ResponseWriter, _ *http.Request) {
	cfg := s.opts.Config
	jsonOK(w, map[string]any{
		"external_url":  cfg.ExternalURL,
		"host":          cfg.Host(),
		"listen":        cfg.Listen,
		"data_dir":      cfg.DataDir,
		"nomad_addr":    cfg.NomadAddr,
		"registry_addr": cfg.RegistryAddr,
		"docker_host":   cfg.DockerHost,
		"version":       s.opts.Version,
	})
}
