package cli

import (
	"bufio"
	"fmt"
	"os"
	"strconv"

	"copycards/internal/copier"
)

// ListBoards displays an interactive numbered menu of boards for a given org profile
func ListBoards(orgProfile string) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	client, err := makeClient(cfg, orgProfile, 1)
	if err != nil {
		return err
	}

	// Fetch boards
	boards, err := client.ListBoards()
	if err != nil {
		return fmt.Errorf("fetch boards: %w", err)
	}

	if len(boards) == 0 {
		fmt.Printf("No boards found in organization %q\n", orgProfile)
		return nil
	}

	// Display menu: [1] Board Name [board-id]
	fmt.Printf("Boards in organization %q:\n", orgProfile)
	fmt.Println()

	for i, board := range boards {
		fmt.Printf("[%d] %s [%s]\n", i+1, board.Name, board.ID)
	}

	fmt.Println()

	return nil
}

// VerifyBoards runs a preflight check on two boards to ensure they're compatible
func VerifyBoards(srcProfile, dstProfile, srcBoardID, dstBoardID string) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	srcClient, err := makeClient(cfg, srcProfile, 1)
	if err != nil {
		return err
	}

	dstClient, err := makeClient(cfg, dstProfile, 1)
	if err != nil {
		return err
	}

	// Run preflight
	pf, err := copier.Preflight(srcClient, dstClient, srcBoardID, dstBoardID)
	if err != nil {
		return fmt.Errorf("preflight check failed: %w", err)
	}

	// Display results
	if pf.Valid {
		fmt.Printf("Boards are compatible\n")
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

	fmt.Println(pf.FormatErrors())

	return nil
}

// InteractiveBoardSelection prompts user to select a board from a numbered list
// Returns the selected board ID
func InteractiveBoardSelection(orgProfile string) (string, error) {
	cfg, err := loadConfig()
	if err != nil {
		return "", err
	}

	client, err := makeClient(cfg, orgProfile, 1)
	if err != nil {
		return "", err
	}

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

