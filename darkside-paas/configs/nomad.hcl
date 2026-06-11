# Nomad agent config for darkside.
#
# Single-node dev cluster: server + client in the same process, docker driver,
# host networking so traefik (on the same host) can reach allocated ports.
# Not for production — for that you'd run a real nomad cluster.

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

  host_volume "docker-sock" {
    path      = "/var/run/docker.sock"
    read_only = false
  }
}

plugin "docker" {
  config {
    allow_privileged = false
    volumes {
      enabled = true
    }
  }
}

# The HTTP API is open inside the docker network; traefik and darkside both
# hit it. If you expose this host externally, lock it down with ACLs.
ports {
  http = 4646
  rpc  = 4647
  serf = 4648
}
