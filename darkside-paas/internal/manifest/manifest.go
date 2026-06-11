// Package manifest defines and parses the in-repo darkside.toml schema.
//
// darkside.toml lives at the root of each user repo and is the single source
// of truth for how that app is built and deployed. Environments have been
// removed in v2 — the app now has one age keypair configured in the darkside
// dashboard, and a single encrypted env file referenced here.
package manifest

import (
	"fmt"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
)

// Manifest is the parsed darkside.toml.
type Manifest struct {
	Name        string      `toml:"name"`
	Branch      string      `toml:"branch"`     // deploy branch, e.g. "main"
	EnvFile     string      `toml:"env_file"`   // relative path to age-encrypted env file
	KeyID       string      `toml:"key_id"`     // age key ID, e.g. "key-v1"
	Build       Build       `toml:"build"`
	Deploy      Deploy      `toml:"deploy"`
	HealthCheck HealthCheck `toml:"health_check"`
	Hooks       Hooks       `toml:"hooks"`
}

type Build struct {
	Context    string `toml:"context"`
	Dockerfile string `toml:"dockerfile"`
}

type Deploy struct {
	Count       int      `toml:"count"`
	TraefikTags []string `toml:"traefik_tags"`
}

type HealthCheck struct {
	Path     string `toml:"path"`
	Port     int    `toml:"port"`
	Interval string `toml:"interval"`
	Timeout  string `toml:"timeout"`
}

type Hooks struct {
	Pre  string `toml:"pre"`
	Post string `toml:"post"`
	Run  string `toml:"run"`
}

// Parse reads a darkside.toml from bytes, applies defaults, and validates.
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
	if m.Branch == "" {
		m.Branch = "main"
	}
	if m.EnvFile == "" {
		m.EnvFile = "env.age"
	}
	if m.KeyID == "" {
		m.KeyID = "key-v1"
	}
}

func (m *Manifest) validate() error {
	if m.Name == "" {
		return fmt.Errorf("name is required")
	}
	if strings.Contains(m.EnvFile, "..") {
		return fmt.Errorf("env_file must not contain ..")
	}
	if _, err := time.ParseDuration(m.HealthCheck.Interval); err != nil {
		return fmt.Errorf("health_check.interval %q: %w", m.HealthCheck.Interval, err)
	}
	if _, err := time.ParseDuration(m.HealthCheck.Timeout); err != nil {
		return fmt.Errorf("health_check.timeout %q: %w", m.HealthCheck.Timeout, err)
	}
	return nil
}
