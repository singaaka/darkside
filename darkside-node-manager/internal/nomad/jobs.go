// Package nomad contains HCL job templates for cluster infrastructure services
// deployed by darkside-node-manager: the private image registry, the darkside-paas
// application itself, and the Traefik reverse proxy.
package nomad

import "fmt"

// DarksideRegistryJob returns the HCL2 job spec for the private Docker registry.
// It is pinned to the darkside-paas node via a meta constraint.
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

// DarksidePaasJob returns the HCL2 job spec for the darkside-paas application.
func DarksidePaasJob(datacenter, domain, imageRef string, registryPort int) string {
	return fmt.Sprintf(`job "darkside-paas" {
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

  group "paas" {
    count = 1

    volume "paas-data" {
      type   = "host"
      source = "paas-data"
    }

    network {
      mode = "bridge"
      port "http"  { to = 8080 }
      port "debug" { static = 8000, to = 8080 }
    }

    task "paas" {
      driver = "docker"
      config {
        image = %q
        ports = ["http", "debug"]
        volumes = ["/var/run/docker.sock:/var/run/docker.sock"]
      }

      volume_mount {
        volume      = "paas-data"
        destination = "/data"
      }

      env {
        DARKSIDE_CONFIG = "/data/config.toml"
      }

      service {
        name     = "darkside-paas"
        provider = "consul"
        port     = "http"
        tags     = [
          "traefik.enable=true",
          "traefik.http.routers.paas.rule=Host(` + "`" + `%s` + "`" + `)",
          "traefik.http.routers.paas.entrypoints=websecure",
          "traefik.http.routers.paas.tls.certresolver=letsencrypt",
          "traefik.http.services.paas.loadbalancer.server.port=8080",
          "traefik.http.routers.paas-webhook.rule=Host(` + "`" + `%s` + "`" + `) && PathPrefix(` + "`" + `/webhooks/` + "`" + `)",
          "traefik.http.routers.paas-webhook.entrypoints=websecure",
          "traefik.http.routers.paas-webhook.priority=200",
          "traefik.http.routers.paas-webhook.tls.certresolver=letsencrypt",
          "traefik.http.routers.paas-debug.entrypoints=debug",
          "traefik.http.routers.paas-debug.rule=PathPrefix(` + "`" + `/` + "`" + `)",
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
