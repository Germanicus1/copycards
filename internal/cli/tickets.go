package cli

import (
	"fmt"

	"copycards/internal/copier"
	"copycards/internal/mapping"
)

// CopyTicketsOptions holds flags for the tickets copy command
type CopyTicketsOptions struct {
	DryRun              bool
	IncludeAttachments  bool
	IncludeComments     bool
	Concurrency         int
}

// CopyTickets copies all tickets between two boards
func CopyTickets(srcProfile, dstProfile, srcBoardID, dstBoardID string, opts CopyTicketsOptions) error {
	// Clamp concurrency
	if opts.Concurrency < 1 {
		opts.Concurrency = 1
	}
	if opts.Concurrency > 500 {
		opts.Concurrency = 500
	}

	fmt.Println("Loading configuration...")
	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	fmt.Println("Connecting to source organization...")
	srcClient, err := makeClient(cfg, srcProfile, opts.Concurrency)
	if err != nil {
		return err
	}

	fmt.Println("Connecting to destination organization...")
	dstClient, err := makeClient(cfg, dstProfile, opts.Concurrency)
	if err != nil {
		return err
	}

	fmt.Println("Loading mapping file...")
	// Load or create mapping file
	m, err := mapping.Load(defaultMappingPath())
	if err != nil {
		return fmt.Errorf("load mapping: %w", err)
	}

	// Ensure maps are initialized
	if m.Users == nil {
		m.Users = make(map[string]string)
	}
	if m.TicketTypes == nil {
		m.TicketTypes = make(map[string]string)
	}
	if m.CustomFields == nil {
		m.CustomFields = make(map[string]string)
	}
	if m.Bins == nil {
		m.Bins = make(map[string]string)
	}
	if m.Tickets == nil {
		m.Tickets = make(map[string]string)
	}
	if m.Comments == nil {
		m.Comments = make(map[string]string)
	}
	if m.Attachments == nil {
		m.Attachments = make(map[string]string)
	}
	if m.UserGroups == nil {
		m.UserGroups = make(map[string]string)
	}

	// Set mapping context
	m.From = srcProfile
	m.To = dstProfile
	m.SrcBoardID = srcBoardID
	m.DstBoardID = dstBoardID

	// Run board copy
	boardOpts := copier.CopyBoardOptions{
		IncludeAttachments: opts.IncludeAttachments,
		IncludeComments:    opts.IncludeComments,
		DryRun:             opts.DryRun,
		Concurrency:        opts.Concurrency,
	}

	if err := copier.CopyBoard(srcClient, dstClient, srcBoardID, dstBoardID, m, boardOpts); err != nil {
		return fmt.Errorf("copy board: %w", err)
	}

	// Save mapping if not a dry run
	if !opts.DryRun {
		if err := m.Save(defaultMappingPath()); err != nil {
			return fmt.Errorf("save mapping: %w", err)
		}
	}

	return nil
}

// CopyTicket copies a single ticket from src to dst
func CopyTicket(srcProfile, dstProfile, ticketID, dstBoardID string, opts struct {
	WithChildren       bool
	IncludeAttachments bool
	IncludeComments    bool
	DryRun             bool
}) error {
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

	// Load or create mapping file
	m, err := mapping.Load(defaultMappingPath())
	if err != nil {
		return fmt.Errorf("load mapping: %w", err)
	}

	// Ensure maps are initialized
	if m.Users == nil {
		m.Users = make(map[string]string)
	}
	if m.TicketTypes == nil {
		m.TicketTypes = make(map[string]string)
	}
	if m.CustomFields == nil {
		m.CustomFields = make(map[string]string)
	}
	if m.Bins == nil {
		m.Bins = make(map[string]string)
	}
	if m.Tickets == nil {
		m.Tickets = make(map[string]string)
	}
	if m.Comments == nil {
		m.Comments = make(map[string]string)
	}
	if m.Attachments == nil {
		m.Attachments = make(map[string]string)
	}
	if m.UserGroups == nil {
		m.UserGroups = make(map[string]string)
	}

	// Set mapping context
	m.From = srcProfile
	m.To = dstProfile
	m.DstBoardID = dstBoardID

	if opts.DryRun {
		fmt.Printf("DRY RUN: Would copy ticket %s\n", ticketID)
		return nil
	}

	// Copy the ticket
	ticketOpts := copier.CopyTicketOptions{
		IncludeAttachments: opts.IncludeAttachments,
		IncludeComments:    opts.IncludeComments,
		WithChildren:       opts.WithChildren,
		Force:              false,
	}

	newID, err := copier.CopyTicket(srcClient, dstClient, ticketID, dstBoardID, m, ticketOpts)
	if err != nil {
		return fmt.Errorf("copy ticket: %w", err)
	}

	fmt.Printf("Ticket copied: %s -> %s\n", ticketID, newID)

	// Save mapping
	if err := m.Save(defaultMappingPath()); err != nil {
		return fmt.Errorf("save mapping: %w", err)
	}

	return nil
}

// DiffBoards shows tickets in src that haven't been copied to dst yet
func DiffBoards(srcProfile, dstProfile, srcBoardID, dstBoardID string) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	srcClient, err := makeClient(cfg, srcProfile, 1)
	if err != nil {
		return err
	}

	_, err = makeClient(cfg, dstProfile, 1)
	if err != nil {
		return err
	}

	// Load mapping
	m, err := mapping.Load(defaultMappingPath())
	if err != nil {
		return fmt.Errorf("load mapping: %w", err)
	}

	// Fetch src board
	srcBoard, err := srcClient.GetBoard(srcBoardID)
	if err != nil {
		return fmt.Errorf("fetch src board: %w", err)
	}

	// Enumerate all src tickets
	var srcTickets []*TicketInfo
	for _, binID := range srcBoard.Bins {
		tickets, err := srcClient.ListTicketsByBin(binID)
		if err != nil {
			return fmt.Errorf("fetch tickets for bin %s: %w", binID, err)
		}
		for i := range tickets {
			srcTickets = append(srcTickets, &TicketInfo{
				ID:   tickets[i].ID,
				Name: tickets[i].Name,
			})
		}
	}

	// Find tickets not yet copied
	var notCopied []*TicketInfo
	for _, ticket := range srcTickets {
		if m.GetTicketDst(ticket.ID) == "" {
			notCopied = append(notCopied, ticket)
		}
	}

	if len(notCopied) == 0 {
		fmt.Println("All tickets have been copied.")
		return nil
	}

	fmt.Printf("Tickets in %s not yet copied to %s:\n", srcProfile, dstProfile)
	fmt.Println()

	for _, ticket := range notCopied {
		fmt.Printf("  %s - %s\n", ticket.ID, ticket.Name)
	}

	fmt.Println()
	fmt.Printf("Total: %d ticket(s) remaining\n", len(notCopied))

	return nil
}

// TicketInfo is a lightweight struct for displaying ticket info
// This is used instead of fbclient.Ticket in diff output to keep output small
type TicketInfo struct {
	ID   string
	Name string
}
