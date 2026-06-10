package config

import (
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/BurntSushi/toml"
)

// Config is the operator-supplied configuration mounted into the darkside
// container. Everything that's specific to *this deployment of the platform*
// (the domain, the data dir, etc.) lives here. App-specific config lives in
// each app's darkside.toml inside its repo.
type Config struct {
	// Domain is the root domain the stack runs on, e.g. "example.com".
	// darkside itself is served at darkside.<Domain> and all deployed apps
	// get subdomains under <Domain>.
	Domain string `toml:"domain"`

	// ExternalURL is the full URL where darkside is reachable from the public
	// internet — GitHub needs this for webhook + manifest redirect URLs.
	// Defaults to https://darkside.<Domain> if not set.
	ExternalURL string `toml:"external_url"`

	// DataDir is where the sqlite DB, cloned repos, and other runtime state
	// live. Must be a writable volume.
	DataDir string `toml:"data_dir"`

	// Listen is the host:port the darkside HTTP server binds inside the
	// container. Traefik fronts this; you usually don't need to change it.
	Listen string `toml:"listen"`

	// NomadAddr is the URL of the nomad HTTP API, e.g. http://nomad:4646.
	NomadAddr string `toml:"nomad_addr"`

	// DockerHost is the docker daemon endpoint. Default is the mounted unix
	// socket. Set this if you're proxying docker.
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
		c.NomadAddr = "http://nomad:4646"
	}
	if c.DockerHost == "" {
		c.DockerHost = "unix:///var/run/docker.sock"
	}
	if c.ExternalURL == "" && c.Domain != "" {
		c.ExternalURL = "https://darkside." + c.Domain
	}
	c.ExternalURL = strings.TrimRight(c.ExternalURL, "/")
}

func (c *Config) validate() error {
	if c.Domain == "" {
		return fmt.Errorf("config: domain is required (root domain like example.com)")
	}
	u, err := url.Parse(c.ExternalURL)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return fmt.Errorf("config: external_url must be a full URL (got %q)", c.ExternalURL)
	}
	return nil
}
