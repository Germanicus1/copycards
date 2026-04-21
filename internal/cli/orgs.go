package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"copycards/internal/config"
	"copycards/internal/fbclient"
)

// ListOrgs lists all configured org profiles with cached endpoint info
func ListOrgs() error {
	// Load config
	configPath := filepath.Join(os.ExpandEnv("$HOME"), ".config", "copycards", "config.toml")
	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	// Get org names and sort for consistent output
	orgNames := cfg.ListOrgNames()
	sort.Strings(orgNames)

	if len(orgNames) == 0 {
		fmt.Println("No organizations configured.")
		return nil
	}

	fmt.Println("Configured organizations:")
	fmt.Println()

	for _, name := range orgNames {
		org, _ := cfg.GetOrg(name)

		// Try to discover or use cached endpoint
		endpoint := org.Endpoint
		if endpoint == "" {
			// Attempt to discover
			discovered, err := config.DiscoverEndpoint(org.OrgID, org.APIKey)
			if err == nil {
				endpoint = discovered
			} else {
				endpoint = "(unable to discover)"
			}
		}

		fmt.Printf("  [%s]\n", name)
		fmt.Printf("    org_id:   %s\n", org.OrgID)
		fmt.Printf("    endpoint: %s\n", endpoint)
		fmt.Println()
	}

	return nil
}

// VerifyOrgAuth verifies that an org's API key is valid by making a test request
func VerifyOrgAuth(orgName string) error {
	// Load config
	configPath := filepath.Join(os.ExpandEnv("$HOME"), ".config", "copycards", "config.toml")
	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	org, err := cfg.GetOrg(orgName)
	if err != nil {
		return err
	}

	// Discover endpoint
	endpoint, err := config.DiscoverEndpoint(org.OrgID, org.APIKey)
	if err != nil {
		return fmt.Errorf("endpoint discovery failed: %w", err)
	}

	// Create client and test with a simple API call
	client := fbclient.NewClient(endpoint, org.APIKey, 1)
	_, err = client.ListBoards()
	if err != nil {
		return fmt.Errorf("auth verification failed: %w", err)
	}

	fmt.Printf("✓ Organization %q is valid and authenticated\n", orgName)
	return nil
}
