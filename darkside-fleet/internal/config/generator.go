// Package config generates Nomad, Consul, and Traefik configuration for each cluster node.
package config

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/singaaka/darkside-fleet/internal/db/dbgen"
)

// RegistryPort is the static port the private Docker registry listens on.
// Hardcoded because it's an internal detail of how darkside-fleet wires the
// cluster — there's no reason for an operator to ever change it, and treating
// it as a setting was just clutter on the UI. If a port change is ever needed,
// flip this constant and any node configs regenerate on the next add/migrate.
const RegistryPort = 5000

// ClusterSettings contains the cluster-wide settings needed for config generation.
// (RegistryPort is fixed; see the constant above.)
type ClusterSettings struct {
	Domain     string
	PaasNodeID string
}

// GenerateNomadHCL produces a nomad.hcl for the given node, aware of all peer IPs.
func GenerateNomadHCL(node dbgen.Node, allNodes []dbgen.Node, settings ClusterSettings) string {
	tags := parseTags(node.Tags)
	isPaas := node.IsDarksidePaasNode == 1

	// Build retry_join list of all OTHER nodes.
	var peers []string
	for _, n := range allNodes {
		if n.ID != node.ID {
			peers = append(peers, fmt.Sprintf(`"%s:4648"`, n.PublicIP))
		}
	}
	retryJoin := strings.Join(peers, ", ")

	nodeMeta := fmt.Sprintf(`"darkside.paas" = "%v"`, isPaas)
	if len(tags) > 0 {
		nodeMeta += fmt.Sprintf("\n    \"node.tags\"   = %q", strings.Join(tags, ","))
	}

	registryAddr := fmt.Sprintf("darkside-registry.service.consul:%d", RegistryPort)

	return fmt.Sprintf(`datacenter = "dc1"
data_dir   = "/opt/nomad/data"

server {
  enabled          = true
  bootstrap_expect = 1
}

client {
  enabled = true
  servers = ["127.0.0.1:4647"]
  meta {
    %s
  }
}

plugin "docker" {
  config {
    allow_privileged = true
    volumes { enabled = true }
    extra_labels = ["job_name", "task_group_name", "task_name", "namespace", "node_name"]
    # Private registry — all nodes trust it.
    auth {
      # insecure registries can also be configured via /etc/docker/daemon.json
    }
  }
}

server_join {
  retry_join = [%s]
}

# Make the private registry reachable by image tag.
# docker daemon is configured with insecure-registries in /etc/docker/daemon.json.
# Registry address: %s
`, nodeMeta, retryJoin, registryAddr)
}

// GenerateConsulHCL produces a consul.hcl for the given node.
func GenerateConsulHCL(node dbgen.Node, allNodes []dbgen.Node) string {
	// Retry join list (all nodes including self for Consul's gossip).
	var peers []string
	for _, n := range allNodes {
		peers = append(peers, fmt.Sprintf(`"%s"`, n.PublicIP))
	}
	retryJoin := strings.Join(peers, ", ")

	return fmt.Sprintf(`datacenter     = "dc1"
data_dir       = "/opt/consul"
node_name      = %q
server         = true
bootstrap_expect = 1
advertise_addr = %q
bind_addr      = %q
client_addr    = "0.0.0.0"
retry_join     = [%s]

ui_config {
  enabled = true
}

connect {
  enabled = false
}
`, node.Name, node.PublicIP, node.PublicIP, retryJoin)
}

// GenerateTraefikYML produces a Traefik static config for a node.
// Traefik is deployed as a Nomad system job and uses this config file.
func GenerateTraefikYML(settings ClusterSettings) string {
	return fmt.Sprintf(`log:
  level: INFO

api:
  dashboard: true
  insecure: false

entryPoints:
  web:
    address: ":80"
    http:
      redirections:
        entryPoint:
          to: websecure
          scheme: https
  websecure:
    address: ":443"
  debug:
    address: ":8000"

providers:
  consulCatalog:
    endpoint:
      address: "http://127.0.0.1:8500"
    exposedByDefault: false
    prefix: traefik
    refreshInterval: 5s

certificatesResolvers:
  letsencrypt:
    acme:
      email: admin@%s
      storage: /data/traefik/acme.json
      httpChallenge:
        entryPoint: web
`, settings.Domain)
}

func parseTags(raw string) []string {
	var tags []string
	if err := json.Unmarshal([]byte(raw), &tags); err != nil {
		return nil
	}
	return tags
}
