package cli

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"copycards/internal/config"
	"copycards/internal/copier"
	"copycards/internal/fbclient"
)

// ListBoards displays an interactive numbered menu of boards for a given org profile
func ListBoards(orgProfile string) error {
	// Load config
	configPath := filepath.Join(os.ExpandEnv("$HOME"), ".config", "copycards", "config.toml")
	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	org, err := cfg.GetOrg(orgProfile)
	if err != nil {
		return err
	}

	// Discover endpoint
	endpoint, err := config.DiscoverEndpoint(org.OrgID, org.APIKey)
	if err != nil {
		return fmt.Errorf("endpoint discovery: %w", err)
	}

	// Create client
	client := fbclient.NewClient(endpoint, org.APIKey, 1)

	// Fetch boards
	boards, err := client.ListBoards()
	if err != nil {
		return fmt.Errorf("fetch boards: %w", err)
	}

	if len(boards) == 0 {
		fmt.Printf("No boards found in organization %q\n", orgProfile)
		return nil
	}

	// Display interactive menu
	fmt.Printf("Boards in organization %q:\n", orgProfile)
	fmt.Println()

	for i, board := range boards {
		// Format: [1] Board Name [board-id]
		fmt.Printf("[%d] %s [%s]\n", i+1, board.Name, board.ID)
	}

	fmt.Println()

	return nil
}

// VerifyBoards runs a preflight check on two boards to ensure they're compatible
func VerifyBoards(srcProfile, dstProfile, srcBoardID, dstBoardID string) error {
	// Load config
	configPath := filepath.Join(os.ExpandEnv("$HOME"), ".config", "copycards", "config.toml")
	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	srcOrg, err := cfg.GetOrg(srcProfile)
	if err != nil {
		return err
	}

	dstOrg, err := cfg.GetOrg(dstProfile)
	if err != nil {
		return err
	}

	// Discover endpoints
	srcEndpoint, err := config.DiscoverEndpoint(srcOrg.OrgID, srcOrg.APIKey)
	if err != nil {
		return fmt.Errorf("discover src endpoint: %w", err)
	}

	dstEndpoint, err := config.DiscoverEndpoint(dstOrg.OrgID, dstOrg.APIKey)
	if err != nil {
		return fmt.Errorf("discover dst endpoint: %w", err)
	}

	// Create clients
	srcClient := fbclient.NewClient(srcEndpoint, srcOrg.APIKey, 1)
	dstClient := fbclient.NewClient(dstEndpoint, dstOrg.APIKey, 1)

	// Run preflight
	pf, err := copier.Preflight(srcClient, dstClient, srcBoardID, dstBoardID)
	if err != nil {
		return fmt.Errorf("preflight check failed: %w", err)
	}

	// Display results
	if pf.Valid {
		fmt.Printf("✓ Boards are compatible\n")
		fmt.Println()
		fmt.Println("Mappings:")
		fmt.Println()

		if len(pf.BinMapping) > 0 {
			fmt.Println("  Bins:")
			for srcID, dstID := range pf.BinMapping {
				fmt.Printf("    %s -> %s\n", srcID, dstID)
			}
			fmt.Println()
		}

		if len(pf.TicketTypeMapping) > 0 {
			fmt.Println("  Ticket Types:")
			for srcID, dstID := range pf.TicketTypeMapping {
				fmt.Printf("    %s -> %s\n", srcID, dstID)
			}
			fmt.Println()
		}

		if len(pf.CustomFieldMapping) > 0 {
			fmt.Println("  Custom Fields:")
			for srcID, dstID := range pf.CustomFieldMapping {
				fmt.Printf("    %s -> %s\n", srcID, dstID)
			}
			fmt.Println()
		}

		if len(pf.UserMapping) > 0 {
			fmt.Println("  Users:")
			for srcID, dstID := range pf.UserMapping {
				fmt.Printf("    %s -> %s\n", srcID, dstID)
			}
			fmt.Println()
		}

		return nil
	}

	fmt.Printf("✗ Boards are NOT compatible\n")
	fmt.Println()
	fmt.Println("Errors:")
	for _, errMsg := range pf.Errors {
		fmt.Printf("  - %s\n", errMsg)
	}

	return nil
}

// InteractiveBoardSelection prompts user to select a board from a numbered list
func InteractiveBoardSelection(orgProfile string) (string, error) {
	// Load config
	configPath := filepath.Join(os.ExpandEnv("$HOME"), ".config", "copycards", "config.toml")
	cfg, err := config.Load(configPath)
	if err != nil {
		return "", fmt.Errorf("load config: %w", err)
	}

	org, err := cfg.GetOrg(orgProfile)
	if err != nil {
		return "", err
	}

	// Discover endpoint
	endpoint, err := config.DiscoverEndpoint(org.OrgID, org.APIKey)
	if err != nil {
		return "", fmt.Errorf("endpoint discovery: %w", err)
	}

	// Create client
	client := fbclient.NewClient(endpoint, org.APIKey, 1)

	// Fetch boards
	boards, err := client.ListBoards()
	if err != nil {
		return "", fmt.Errorf("fetch boards: %w", err)
	}

	if len(boards) == 0 {
		return "", fmt.Errorf("no boards found in organization %q", orgProfile)
	}

	if len(boards) == 1 {
		return boards[0].ID, nil
	}

	// Display menu
	fmt.Printf("Select a board from %q:\n", orgProfile)
	fmt.Println()

	for i, board := range boards {
		fmt.Printf("[%d] %s\n", i+1, board.Name)
	}

	fmt.Println()

	// Read user input
	scanner := bufio.NewScanner(os.Stdin)
	fmt.Print("Enter board number: ")

	if !scanner.Scan() {
		return "", fmt.Errorf("no input provided")
	}

	input := scanner.Text()
	choice, err := strconv.Atoi(input)
	if err != nil {
		return "", fmt.Errorf("invalid input: %s", input)
	}

	if choice < 1 || choice > len(boards) {
		return "", fmt.Errorf("choice out of range: %d (1-%d)", choice, len(boards))
	}

	return boards[choice-1].ID, nil
}
