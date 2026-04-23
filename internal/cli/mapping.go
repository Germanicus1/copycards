package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"copycards/internal/mapping"
)

// ShowMapping displays the mapping file content
func ShowMapping(srcProfile, dstProfile, srcBoardID string) error {
	m, err := mapping.Load(defaultMappingPath())
	if err != nil {
		return fmt.Errorf("load mapping: %w", err)
	}

	// If no mapping exists, report it
	if m == nil || (len(m.Tickets) == 0 && len(m.Users) == 0) {
		fmt.Println("No mapping file found or mapping is empty.")
		return nil
	}

	// Display mapping summary
	fmt.Printf("Mapping: %s -> %s\n", m.From, m.To)
	fmt.Printf("Boards:  %s -> %s\n", m.SrcBoardID, m.DstBoardID)
	fmt.Println()

	// Display counts
	fmt.Println("Mappings:")
	fmt.Printf("  Tickets:       %d\n", len(m.Tickets))
	fmt.Printf("  Users:         %d\n", len(m.Users))
	fmt.Printf("  Bins:          %d\n", len(m.Bins))
	fmt.Printf("  Ticket Types:  %d\n", len(m.TicketTypes))
	fmt.Printf("  Custom Fields: %d\n", len(m.CustomFields))
	fmt.Printf("  Comments:      %d\n", len(m.Comments))
	fmt.Printf("  Attachments:   %d\n", len(m.Attachments))
	fmt.Printf("  User Groups:   %d\n", len(m.UserGroups))
	fmt.Println()

	// Optionally display detailed mappings for tickets
	if len(m.Tickets) > 0 && len(m.Tickets) <= 20 {
		fmt.Println("Ticket mappings:")
		for srcID, dstID := range m.Tickets {
			fmt.Printf("  %s -> %s\n", srcID, dstID)
		}
		fmt.Println()
	}

	return nil
}

// ResetMapping deletes the mapping file after user confirmation
func ResetMapping(srcProfile, dstProfile, srcBoardID string) error {
	mappingPath := defaultMappingPath()

	// Check if mapping file exists
	if _, err := os.Stat(mappingPath); err != nil {
		if os.IsNotExist(err) {
			fmt.Println("No mapping file found to reset.")
			return nil
		}
		return fmt.Errorf("stat mapping: %w", err)
	}

	// Load and display mapping
	m, err := mapping.Load(mappingPath)
	if err != nil {
		return fmt.Errorf("load mapping: %w", err)
	}

	fmt.Printf("About to reset mapping: %s -> %s\n", m.From, m.To)
	fmt.Printf("Boards:  %s -> %s\n", m.SrcBoardID, m.DstBoardID)
	fmt.Printf("Tickets mapped: %d\n", len(m.Tickets))
	fmt.Println()

	// Prompt for confirmation
	fmt.Print("Are you sure? (type 'yes' to confirm): ")

	scanner := bufio.NewScanner(os.Stdin)
	if !scanner.Scan() {
		return fmt.Errorf("no input provided")
	}

	input := strings.TrimSpace(scanner.Text())
	if input != "yes" {
		fmt.Println("Reset cancelled.")
		return nil
	}

	// Delete mapping file
	if err := os.Remove(mappingPath); err != nil {
		return fmt.Errorf("delete mapping file: %w", err)
	}

	fmt.Println("✓ Mapping file deleted.")
	return nil
}
