package config

import (
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/BurntSushi/toml"
)

// Config is the operator-supplied config mounted into the darkside container.
// App-specific config lives in each app's repo as a separate darkside.toml.
type Config struct {
	// ExternalURL is the full HTTPS URL where darkside is reachable. Goes into
	// GitHub webhook + manifest callback URLs.
	ExternalURL string `toml:"external_url"`

	// DataDir is the writable volume for sqlite, cloned repos, etc.
	DataDir string `toml:"data_dir"`

	// Listen is the host:port the binary binds inside the container.
	Listen string `toml:"listen"`

	// NomadAddr is the URL of the nomad HTTP API.
	NomadAddr string `toml:"nomad_addr"`

	// DockerHost is the docker daemon endpoint.
	DockerHost string `toml:"docker_host"`
}

func Load(path string) (*Config, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config %s: %w", path, err)
	}
	var c Config
	if err := toml.Unmarshal(b, &c); err != nil {
		return nil, fmt.Errorf("parse config %s: %w", path, err)
	}
	c.applyDefaults()
	if err := c.validate(); err != nil {
		return nil, err
	}
	return &c, nil
}

func (c *Config) applyDefaults() {
	if c.Listen == "" {
		c.Listen = ":8080"
	}
	if c.DataDir == "" {
		c.DataDir = "/data"
	}
	if c.NomadAddr == "" {
		c.NomadAddr = "http://host.docker.internal:4646"
	}
	if c.DockerHost == "" {
		c.DockerHost = "unix:///var/run/docker.sock"
	}
	c.ExternalURL = strings.TrimRight(c.ExternalURL, "/")
}

func (c *Config) validate() error {
	u, err := url.Parse(c.ExternalURL)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return fmt.Errorf("config: external_url must be a full URL (got %q)", c.ExternalURL)
	}
	return nil
}

// Host returns the hostname portion of ExternalURL (e.g. darkside.example.com).
// Used for display in the UI / responses; the host rule in compose comes from
// the DARKSIDE_HOST .env var.
func (c *Config) Host() string {
	u, err := url.Parse(c.ExternalURL)
	if err != nil {
		return ""
	}
	return u.Host
}
