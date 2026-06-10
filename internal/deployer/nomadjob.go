package deployer

import (
	"bytes"
	"fmt"
	"sort"
	"strings"
	"text/template"

	"github.com/singaaka/darkside/internal/manifest"
)

// nomadJobInput is the template input for the HCL2 nomad job template.
type nomadJobInput struct {
	JobName        string
	Datacenter     string
	Count          int
	Port           int
	HealthPath     string
	HealthInterval string
	HealthTimeout  string
	Image          string
	Command        string            // optional, overrides image CMD
	Tags           []string          // service tags (traefik etc.)
	Env            map[string]string // injected as task env
}

// hclQuoted turns a Go string into an HCL2 double-quoted literal. HCL2's
// escape rules: \\ \" \n \t \r and ${ %{ for interpolation markers.
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

const nomadJobTmpl = `job "{{.JobName}}" {
  datacenters = ["{{.Datacenter}}"]
  type        = "service"

  update {
    max_parallel     = 1
    health_check     = "checks"
    min_healthy_time = "10s"
    healthy_deadline = "5m"
    progress_deadline = "10m"
    auto_revert      = true
    canary           = 0
  }

  group "app" {
    count = {{.Count}}

    network {
      port "http" {
        to = {{.Port}}
      }
    }

    service {
      name     = "{{.JobName}}"
      provider = "nomad"
      port     = "http"
      tags     = [
{{range .Tags}}        {{quoted .}},
{{end}}      ]

      check {
        type     = "http"
        path     = {{quoted .HealthPath}}
        port     = "http"
        interval = {{quoted .HealthInterval}}
        timeout  = {{quoted .HealthTimeout}}
      }
    }

    task "app" {
      driver = "docker"
      config {
        image = {{quoted .Image}}
        ports = ["http"]
{{if .Command}}        command = "sh"
        args = ["-c", {{quoted .Command}}]
{{end}}      }
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

// RenderNomadJob produces the HCL2 job spec. envMap is iterated in sorted key
// order so the rendered text is deterministic — useful when diffing nomad
// jobs across deployments.
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

	// Deterministic env iteration.
	keys := make([]string, 0, len(envMap))
	for k := range envMap {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	sortedEnv := make(map[string]string, len(envMap))
	for _, k := range keys {
		sortedEnv[k] = envMap[k]
	}

	// Stripped tags — drop empty + dedupe to be polite.
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

