# Nomad agent config for the single-host docker-compose dev setup.
#
# Single-node cluster: server + client in the same process. Registers itself
# with consul (on the same compose network) so darkside-paas-deployed apps
# show up in the consul catalog → traefik discovers them via consulCatalog.
#
# Production setups use darkside-node-manager to render a similar config
# (with per-node retry_join + meta) on each cluster node.

datacenter = "dc1"
data_dir   = "/opt/nomad/data"
log_level  = "INFO"
bind_addr  = "0.0.0.0"

server {
  enabled          = true
  bootstrap_expect = 1
}

client {
  enabled = true
  servers = ["127.0.0.1:4647"]

  # Mark this node as a darkside.paas node so the build job's NodeMeta
  # constraint matches.
  meta {
    "darkside.paas" = "true"
  }

  host_volume "docker-sock" {
    path      = "/var/run/docker.sock"
    read_only = false
  }
}

# Register nomad services in consul so traefik's consulCatalog provider
# discovers them. The consul container is named `consul` on the darkside
# compose network.
consul {
  address = "consul:8500"
  # Auto-advertise the nomad client's own services too (optional).
  auto_advertise = true
  server_auto_join = false
  client_auto_join = false
}

plugin "docker" {
  config {
    allow_privileged = true
    volumes {
      enabled = true
    }
    extra_labels = ["job_name", "task_group_name", "task_name"]
  }
}

ports {
  http = 4646
  rpc  = 4647
  serf = 4648
}
