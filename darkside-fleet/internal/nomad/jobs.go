// Package nomad contains HCL job templates for cluster infrastructure services
// deployed by darkside-fleet: the private image registry, the darkside
// application itself, and the Traefik reverse proxy.
package nomad

import "fmt"

// DarksideRegistryJob returns the HCL2 job spec for the private Docker registry.
// It is pinned to the darkside host node via a meta constraint.
func DarksideRegistryJob(datacenter string, registryPort int) string {
	return fmt.Sprintf(`job "darkside-registry" {
  datacenters = [%q]
  type        = "service"

  constraint {
    attribute = "$${meta[\"darkside.paas\"]}"
    value     = "true"
  }

  group "registry" {
    count = 1

    volume "registry-data" {
      type   = "host"
      source = "registry-data"
    }

    network {
      mode = "host"
      port "registry" { static = %d }
    }

    task "registry" {
      driver = "docker"
      config {
        image = "registry:2"
        ports = ["registry"]
      }

      volume_mount {
        volume      = "registry-data"
        destination = "/var/lib/registry"
      }

      service {
        name     = "darkside-registry"
        provider = "consul"
        port     = "registry"
      }

      resources {
        cpu    = 100
        memory = 128
      }
    }
  }
}
`, datacenter, registryPort)
}

// DarksideJob returns the HCL2 job spec for the darkside application.
func DarksideJob(datacenter, domain, imageRef string, registryPort int) string {
	return fmt.Sprintf(`job "darkside" {
  datacenters = [%q]
  type        = "service"

  constraint {
    attribute = "$${meta[\"darkside.paas\"]}"
    value     = "true"
  }

  update {
    max_parallel      = 1
    health_check      = "checks"
    min_healthy_time  = "10s"
    healthy_deadline  = "3m"
    progress_deadline = "5m"
    auto_revert       = true
  }

  group "darkside" {
    count = 1

    volume "darkside-data" {
      type   = "host"
      source = "darkside-data"
    }

    network {
      mode = "bridge"
      port "http"  { to = 8080 }
      port "debug" { static = 8000, to = 8080 }
    }

    task "darkside" {
      driver = "docker"
      config {
        image = %q
        ports = ["http", "debug"]
        volumes = ["/var/run/docker.sock:/var/run/docker.sock"]
      }

      volume_mount {
        volume      = "darkside-data"
        destination = "/data"
      }

      env {
        DARKSIDE_CONFIG = "/data/config.toml"
      }

      service {
        name     = "darkside"
        provider = "consul"
        port     = "http"
        tags     = [
          "traefik.enable=true",
          "traefik.http.routers.darkside.rule=Host(` + "`" + `%s` + "`" + `)",
          "traefik.http.routers.darkside.entrypoints=websecure",
          "traefik.http.routers.darkside.tls.certresolver=letsencrypt",
          "traefik.http.services.darkside.loadbalancer.server.port=8080",
          "traefik.http.routers.darkside-webhook.rule=Host(` + "`" + `%s` + "`" + `) && PathPrefix(` + "`" + `/webhooks/` + "`" + `)",
          "traefik.http.routers.darkside-webhook.entrypoints=websecure",
          "traefik.http.routers.darkside-webhook.priority=200",
          "traefik.http.routers.darkside-webhook.tls.certresolver=letsencrypt",
          "traefik.http.routers.darkside-debug.entrypoints=debug",
          "traefik.http.routers.darkside-debug.rule=PathPrefix(` + "`" + `/` + "`" + `)",
        ]

        check {
          type     = "http"
          path     = "/healthz"
          interval = "30s"
          timeout  = "5s"
        }
      }

      resources {
        cpu    = 300
        memory = 256
      }
    }
  }
}
`, datacenter, imageRef, domain, domain)
}

// TraefikSystemJob returns the HCL2 job for Traefik running on every node (system type).
func TraefikSystemJob(datacenter string) string {
	return fmt.Sprintf(`job "traefik" {
  datacenters = [%q]
  type        = "system"

  group "traefik" {
    network {
      mode = "host"
      port "web"       { static = 80 }
      port "websecure" { static = 443 }
      port "debug"     { static = 8000 }
    }

    task "traefik" {
      driver = "docker"
      config {
        image        = "traefik:v3.7"
        network_mode = "host"
        volumes      = [
          "/etc/traefik/traefik.yml:/etc/traefik/traefik.yml:ro",
          "/data/traefik:/data/traefik",
        ]
        ports = ["web", "websecure", "debug"]
      }

      service {
        name     = "traefik"
        provider = "consul"
        port     = "web"
      }

      resources {
        cpu    = 200
        memory = 128
      }
    }
  }
}
`, datacenter)
}
