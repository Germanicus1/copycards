package cli

import (
	"fmt"
	"sort"
)

// ListOrgs lists all configured org profiles with cached endpoint info
func ListOrgs() error {
	cfg, err := loadConfig()
	if err != nil {
		return err
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

		// Try to resolve endpoint (config override or discover)
		endpoint, err := resolveEndpoint(org)
		if err != nil {
			endpoint = "(unable to discover)"
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
	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	client, err := makeClient(cfg, orgName)
	if err != nil {
		return err
	}

	_, err = client.ListBoards()
	if err != nil {
		return fmt.Errorf("auth verification failed: %w", err)
	}

	fmt.Printf("Organization %q is valid and authenticated\n", orgName)
	return nil
}
