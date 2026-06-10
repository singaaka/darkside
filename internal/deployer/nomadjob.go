package deployer

import (
	"bytes"
	"fmt"
	"sort"
	"strings"
	"text/template"

	"github.com/singaaka/darkside/internal/manifest"
)

// dockerNetwork is the compose bridge that everything in darkside lives on:
// traefik, darkside itself, nomad, and (per the docker config emitted below)
// every alloc nomad spawns. Keep this in sync with `networks.darkside.name` in
// docker-compose.yml.
const dockerNetwork = "darkside"

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
	Command        string
	Tags           []string
	Env            map[string]string
	Network        string
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

// The template puts the service block INSIDE the task and uses
// `address_mode = "driver"`. That tells nomad to advertise the container's
// IP on the docker network rather than a host-mapped port — which is the
// whole point of having allocs share the `darkside` bridge with traefik.
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

    task "app" {
      driver = "docker"
      config {
        image        = {{quoted .Image}}
        network_mode = {{quoted .Network}}
{{if .Command}}        command = "sh"
        args    = ["-c", {{quoted .Command}}]
{{end}}      }

      service {
        name         = "{{.JobName}}"
        provider     = "nomad"
        port         = "{{.Port}}"
        address_mode = "driver"
        tags = [
{{range .Tags}}          {{quoted .}},
{{end}}        ]

        check {
          type         = "http"
          path         = {{quoted .HealthPath}}
          port         = "{{.Port}}"
          interval     = {{quoted .HealthInterval}}
          timeout      = {{quoted .HealthTimeout}}
          address_mode = "driver"
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
		Network:        dockerNetwork,
	}
	var out bytes.Buffer
	if err := nomadJobTemplate.Execute(&out, in); err != nil {
		return "", fmt.Errorf("render nomad job: %w", err)
	}
	return out.String(), nil
}
