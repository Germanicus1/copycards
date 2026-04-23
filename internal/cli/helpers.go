package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"copycards/internal/config"
	"copycards/internal/fbclient"
)

// defaultConfigPath returns the default config file path
// This is called from CLI commands; if configPath is set globally, use that
var GlobalConfigPath string

func defaultConfigPath() string {
	if GlobalConfigPath != "" {
		return GlobalConfigPath
	}
	return filepath.Join(os.ExpandEnv("$HOME"), ".config", "copycards", "config.toml")
}

// defaultMappingPath returns the default mapping file path
func defaultMappingPath() string {
	return filepath.Join(os.ExpandEnv("$HOME"), ".copycard", "mapping.json")
}

// resolveEndpoint returns the configured endpoint if set, otherwise builds
// the deterministic one from the org ID.
func resolveEndpoint(org *config.OrgConfig) string {
	if org.Endpoint != "" {
		return org.Endpoint
	}
	return config.BuildEndpoint(org.OrgID)
}

// makeClient loads config, resolves endpoint, and returns an fbclient
func makeClient(cfg *config.Config, profileName string) (*fbclient.Client, error) {
	org, err := cfg.GetOrg(profileName)
	if err != nil {
		return nil, err
	}
	return fbclient.NewClient(resolveEndpoint(org), org.APIKey), nil
}

// loadConfig loads the config from the default path
func loadConfig() (*config.Config, error) {
	cfg, err := config.Load(defaultConfigPath())
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}
	return cfg, nil
}
