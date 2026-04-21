package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/BurntSushi/toml"
)

type Config struct {
	DefaultFrom string              `toml:"default_from"`
	DefaultTo   string              `toml:"default_to"`
	Orgs        map[string]OrgConfig `toml:"orgs"`
}

type OrgConfig struct {
	OrgID    string `toml:"org_id"`
	APIKey   string `toml:"api_key"`
	Endpoint string `toml:"endpoint,omitempty"`
}

// Load reads config from file and expands env vars
func Load(path string) (*Config, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	var cfg Config
	if err := toml.Unmarshal(content, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	// Expand env vars in API keys
	for name, org := range cfg.Orgs {
		if strings.HasPrefix(org.APIKey, "env:") {
			envVar := strings.TrimPrefix(org.APIKey, "env:")
			org.APIKey = os.Getenv(envVar)
			if org.APIKey == "" {
				return nil, fmt.Errorf("env var %s not set for org %s", envVar, name)
			}
			cfg.Orgs[name] = org
		}
	}

	return &cfg, nil
}

// GetOrg returns the org config by profile name
func (c *Config) GetOrg(name string) (*OrgConfig, error) {
	org, ok := c.Orgs[name]
	if !ok {
		return nil, fmt.Errorf("org profile %q not found", name)
	}
	return &org, nil
}

// ListOrgNames returns all profile names
func (c *Config) ListOrgNames() []string {
	names := make([]string, 0, len(c.Orgs))
	for name := range c.Orgs {
		names = append(names, name)
	}
	return names
}
