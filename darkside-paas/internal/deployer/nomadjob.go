package deployer

import (
	"bytes"
	"fmt"
	"sort"
	"strings"
	"text/template"

	"github.com/singaaka/darkside-paas/internal/manifest"
)

// nomadJobInput is the template data for a long-lived service job (user apps).
type nomadJobInput struct {
	JobName        string
	Datacenter     string
	Count          int
	Port           int
	HealthPath     string
	HealthInterval string
	HealthTimeout  string
	Image          string
	Command        string
	Tags           []string
	Env            map[string]string
}

func hclQuoted(s string) string {
	r := strings.NewReplacer(
		`\`, `\\`,
		`"`, `\"`,
		"\n", `\n`,
		"\r", `\r`,
		"\t", `\t`,
		"${", `$${`,
		"%{", `%%{`,
	)
	return `"` + r.Replace(s) + `"`
}

// Service job uses CNI bridge mode and Consul service discovery. Traefik reads
// service tags from the Consul catalog provider.
const nomadJobTmpl = `job "{{.JobName}}" {
  datacenters = ["{{.Datacenter}}"]
  type        = "service"

  update {
    max_parallel      = 1
    health_check      = "checks"
    min_healthy_time  = "10s"
    healthy_deadline  = "5m"
    progress_deadline = "10m"
    auto_revert       = true
    canary            = 0
  }

  group "app" {
    count = {{.Count}}

    network {
      mode = "bridge"
      port "http" { to = {{.Port}} }
    }

    task "app" {
      driver = "docker"
      config {
        image = {{quoted .Image}}
{{if .Command}}        command = "sh"
        args    = ["-c", {{quoted .Command}}]
{{end}}      }

      service {
        name     = "{{.JobName}}"
        provider = "consul"
        port     = "http"
        tags = [
{{range .Tags}}          {{quoted .}},
{{end}}        ]

        check {
          type     = "http"
          path     = {{quoted .HealthPath}}
          interval = {{quoted .HealthInterval}}
          timeout  = {{quoted .HealthTimeout}}
        }
      }

      env {
{{range $k, $v := .Env}}        {{$k}} = {{quoted $v}}
{{end}}      }

      resources {
        cpu    = 200
        memory = 256
      }
    }
  }
}
`

var nomadJobTemplate = template.Must(template.New("nomadjob").
	Funcs(template.FuncMap{"quoted": hclQuoted}).
	Parse(nomadJobTmpl))

// RenderNomadJob produces the HCL2 service job spec for user apps.
func RenderNomadJob(jobName, datacenter, image string, m *manifest.Manifest, envMap map[string]string) (string, error) {
	if datacenter == "" {
		datacenter = "dc1"
	}
	port := m.HealthCheck.Port
	if port == 0 {
		port = 8080
	}
	healthPath := m.HealthCheck.Path
	if healthPath == "" {
		healthPath = "/"
	}

	keys := make([]string, 0, len(envMap))
	for k := range envMap {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	sortedEnv := make(map[string]string, len(envMap))
	for _, k := range keys {
		sortedEnv[k] = envMap[k]
	}

	tags := make([]string, 0, len(m.Deploy.TraefikTags))
	seen := map[string]struct{}{}
	for _, t := range m.Deploy.TraefikTags {
		t = strings.TrimSpace(t)
		if t == "" {
			continue
		}
		if _, ok := seen[t]; ok {
			continue
		}
		seen[t] = struct{}{}
		tags = append(tags, t)
	}

	in := nomadJobInput{
		JobName:        jobName,
		Datacenter:     datacenter,
		Count:          m.Deploy.Count,
		Port:           port,
		HealthPath:     healthPath,
		HealthInterval: m.HealthCheck.Interval,
		HealthTimeout:  m.HealthCheck.Timeout,
		Image:          image,
		Command:        m.Hooks.Run,
		Tags:           tags,
		Env:            sortedEnv,
	}
	var out bytes.Buffer
	if err := nomadJobTemplate.Execute(&out, in); err != nil {
		return "", fmt.Errorf("render nomad job: %w", err)
	}
	return out.String(), nil
}

// ── Batch job templates ────────────────────────────────────────────────────────

// BatchJobInput is the template data for ephemeral build/hook batch jobs.
type BatchJobInput struct {
	JobName    string
	Datacenter string
	Image      string
	Command    string
	Env        map[string]string
	// NodeMeta constrains the job to a specific node (e.g. the builder node).
	NodeMeta map[string]string
}

const batchJobTmpl = `job "{{.JobName}}" {
  datacenters = ["{{.Datacenter}}"]
  type        = "batch"

{{if .NodeMeta}}  constraint {
{{range $k, $v := .NodeMeta}}    attribute = "$${meta.{{$k}}}"
    value     = {{quoted $v}}
{{end}}  }
{{end}}
  group "run" {
    task "run" {
      driver = "docker"
      config {
        image      = {{quoted .Image}}
        privileged = true
        volumes    = ["/var/run/docker.sock:/var/run/docker.sock"]
        command    = "sh"
        args       = ["-c", {{quoted .Command}}]
      }

      env {
{{range $k, $v := .Env}}        {{$k}} = {{quoted $v}}
{{end}}      }

      resources {
        cpu    = 500
        memory = 512
      }
    }
  }
}
`

var batchJobTemplate = template.Must(template.New("batchjob").
	Funcs(template.FuncMap{"quoted": hclQuoted}).
	Parse(batchJobTmpl))

// RenderBatchJob produces an HCL2 batch job spec for builds and hooks.
func RenderBatchJob(in BatchJobInput) (string, error) {
	if in.Datacenter == "" {
		in.Datacenter = "dc1"
	}
	keys := make([]string, 0, len(in.Env))
	for k := range in.Env {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	sortedEnv := make(map[string]string, len(in.Env))
	for _, k := range keys {
		sortedEnv[k] = in.Env[k]
	}
	in.Env = sortedEnv

	var out bytes.Buffer
	if err := batchJobTemplate.Execute(&out, in); err != nil {
		return "", fmt.Errorf("render batch job: %w", err)
	}
	return out.String(), nil
}
