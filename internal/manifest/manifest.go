// Package manifest defines and parses the in-repo darkside.toml schema.
//
// darkside.toml lives at the root of each user repo and is the source of truth
// for how that app is built and deployed. Operator config (the platform
// itself) is separate — see internal/config.
package manifest

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
)

// Manifest is the parsed darkside.toml.
type Manifest struct {
	Name         string            `toml:"name"`
	Build        Build             `toml:"build"`
	Deploy       Deploy            `toml:"deploy"`
	HealthCheck  HealthCheck       `toml:"health_check"`
	Hooks        Hooks             `toml:"hooks"`
	Branches     map[string]string `toml:"branches"`
	Environments []Environment     `toml:"environments"`
}

type Build struct {
	Context    string `toml:"context"`    // default: "."
	Dockerfile string `toml:"dockerfile"` // default: "Dockerfile"
}

type Deploy struct {
	Count       int      `toml:"count"`        // default: 1
	TraefikTags []string `toml:"traefik_tags"` // service tags, passed through to nomad as-is
}

type HealthCheck struct {
	Path     string `toml:"path"`
	Port     int    `toml:"port"`
	Interval string `toml:"interval"` // e.g. "30s"; default "30s"
	Timeout  string `toml:"timeout"`  // e.g. "5s";  default "5s"
}

type Hooks struct {
	// Pre runs as `docker run --rm <image>` before the rolling deploy starts.
	// Empty = no hook.
	Pre string `toml:"pre"`
	// Post runs the same way after deploy is healthy.
	Post string `toml:"post"`
	// Run overrides the built image's default CMD when launched by nomad.
	// Empty = use the image's CMD.
	Run string `toml:"run"`
}

type Environment struct {
	Name    string `toml:"name"`
	EnvFile string `toml:"env_file"` // path relative to repo root; e.g. env.production.age
}

var nameRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,38}[a-z0-9]$|^[a-z0-9]$`)
var envNameRe = regexp.MustCompile(`^[a-z][a-z0-9-]{0,19}$`)

// Parse reads a darkside.toml from bytes and applies defaults + validation.
func Parse(data []byte) (*Manifest, error) {
	var m Manifest
	if err := toml.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parse darkside.toml: %w", err)
	}
	m.applyDefaults()
	if err := m.validate(); err != nil {
		return nil, err
	}
	return &m, nil
}

func (m *Manifest) applyDefaults() {
	if m.Build.Context == "" {
		m.Build.Context = "."
	}
	if m.Build.Dockerfile == "" {
		m.Build.Dockerfile = "Dockerfile"
	}
	if m.Deploy.Count == 0 {
		m.Deploy.Count = 1
	}
	if m.HealthCheck.Interval == "" {
		m.HealthCheck.Interval = "30s"
	}
	if m.HealthCheck.Timeout == "" {
		m.HealthCheck.Timeout = "5s"
	}
}

func (m *Manifest) validate() error {
	if !nameRe.MatchString(m.Name) {
		return fmt.Errorf("name %q must be lowercase slug [a-z0-9-]", m.Name)
	}
	if len(m.Environments) == 0 {
		return errors.New("at least one [[environments]] block is required")
	}
	envNames := make(map[string]struct{}, len(m.Environments))
	for i, e := range m.Environments {
		if !envNameRe.MatchString(e.Name) {
			return fmt.Errorf("environments[%d].name %q must match [a-z][a-z0-9-]{0,19}", i, e.Name)
		}
		if e.EnvFile == "" {
			return fmt.Errorf("environments[%d].env_file is required", i)
		}
		if strings.Contains(e.EnvFile, "..") {
			return fmt.Errorf("environments[%d].env_file %q must not contain ..", i, e.EnvFile)
		}
		if _, dup := envNames[e.Name]; dup {
			return fmt.Errorf("environments[%d]: duplicate name %q", i, e.Name)
		}
		envNames[e.Name] = struct{}{}
	}
	if len(m.Branches) == 0 {
		return errors.New("[branches] must declare at least one branch → environment mapping")
	}
	for branch, env := range m.Branches {
		if _, ok := envNames[env]; !ok {
			return fmt.Errorf("branches.%s → %q has no matching [[environments]] block", branch, env)
		}
	}
	if _, err := time.ParseDuration(m.HealthCheck.Interval); err != nil {
		return fmt.Errorf("health_check.interval %q: %w", m.HealthCheck.Interval, err)
	}
	if _, err := time.ParseDuration(m.HealthCheck.Timeout); err != nil {
		return fmt.Errorf("health_check.timeout %q: %w", m.HealthCheck.Timeout, err)
	}
	return nil
}

// EnvironmentByName finds the [[environments]] entry. Returns nil if missing.
func (m *Manifest) EnvironmentByName(name string) *Environment {
	for i := range m.Environments {
		if m.Environments[i].Name == name {
			return &m.Environments[i]
		}
	}
	return nil
}

// EnvForBranch returns the env name that branch deploys to, or "".
func (m *Manifest) EnvForBranch(branch string) string {
	return m.Branches[branch]
}
