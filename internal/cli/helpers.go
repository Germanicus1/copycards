package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"copycards/internal/config"
	"copycards/internal/fbclient"
)

// defaultConfigPath returns the default config file path
func defaultConfigPath() string {
	return filepath.Join(os.ExpandEnv("$HOME"), ".config", "copycards", "config.toml")
}

// defaultMappingPath returns the default mapping file path
func defaultMappingPath() string {
	return filepath.Join(os.ExpandEnv("$HOME"), ".copycard", "mapping.json")
}

// resolveEndpoint returns the configured endpoint or discovers it from the API
func resolveEndpoint(org *config.OrgConfig) (string, error) {
	if org.Endpoint != "" {
		return org.Endpoint, nil
	}
	endpoint, err := config.DiscoverEndpoint(org.OrgID, org.APIKey)
	if err != nil {
		return "", fmt.Errorf("endpoint discovery: %w", err)
	}
	return endpoint, nil
}

// makeClient loads config, resolves endpoint, and returns an fbclient
func makeClient(cfg *config.Config, profileName string, concurrency int) (*fbclient.Client, error) {
	org, err := cfg.GetOrg(profileName)
	if err != nil {
		return nil, err
	}
	endpoint, err := resolveEndpoint(org)
	if err != nil {
		return nil, fmt.Errorf("org %q: %w", profileName, err)
	}
	return fbclient.NewClient(endpoint, org.APIKey, concurrency), nil
}

// loadConfig loads the config from the default path
func loadConfig() (*config.Config, error) {
	cfg, err := config.Load(defaultConfigPath())
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}
	return cfg, nil
}
