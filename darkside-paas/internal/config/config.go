package config

import (
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/BurntSushi/toml"
)

// Config is the operator-supplied config mounted into the darkside container.
type Config struct {
	ExternalURL  string `toml:"external_url"`
	DataDir      string `toml:"data_dir"`
	Listen       string `toml:"listen"`
	NomadAddr    string `toml:"nomad_addr"`
	RegistryAddr string `toml:"registry_addr"` // private docker registry, e.g. "127.0.0.1:5000"
	DockerHost   string `toml:"docker_host"`
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
		c.NomadAddr = "http://127.0.0.1:4646"
	}
	if c.RegistryAddr == "" {
		c.RegistryAddr = "darkside-registry.service.consul:5000"
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

// Host returns the hostname portion of ExternalURL.
func (c *Config) Host() string {
	u, err := url.Parse(c.ExternalURL)
	if err != nil {
		return ""
	}
	return u.Host
}
