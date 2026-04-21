package copier

import (
	"fmt"
	"sort"

	"copycards/internal/fbclient"
	"copycards/internal/mapping"
)

// CopyBoardOptions controls full board copy
type CopyBoardOptions struct {
	IncludeArchived    bool
	IncludeAttachments bool
	IncludeComments    bool
	DryRun             bool
	Concurrency        int
}

// CopyBoard copies all tickets from src board to dst board
func CopyBoard(srcClient, dstClient *fbclient.Client, srcBoardID, dstBoardID string, m *mapping.Mapping, opts CopyBoardOptions) error {
	// Preflight
	pf, err := Preflight(srcClient, dstClient, srcBoardID, dstBoardID)
	if err != nil {
		return fmt.Errorf("preflight: %w", err)
	}
	if !pf.Valid {
		return fmt.Errorf("boards not compatible: %v", pf.Errors)
	}

	// Apply preflight mappings
	ApplyMappingToResult(m, pf)

	// Enumerate src board tickets
	srcBoard, err := srcClient.GetBoard(srcBoardID)
	if err != nil {
		return fmt.Errorf("fetch src board: %w", err)
	}

	var allSrcTickets []*fbclient.Ticket
	for _, bin := range srcBoard.Bins {
		tickets, err := srcClient.ListTicketsByBin(bin.ID)
		if err != nil {
			return fmt.Errorf("fetch tickets for bin %s: %w", bin.ID, err)
		}
		for i := range tickets {
			allSrcTickets = append(allSrcTickets, &tickets[i])
		}
	}

	// Topological sort by order field (parents before children)
	sortedTickets := topologicalSort(allSrcTickets)

	// Copy each ticket
	ticketOpts := CopyTicketOptions{
		IncludeAttachments: opts.IncludeAttachments,
		IncludeComments:    opts.IncludeComments,
		WithChildren:       false, // handled by preflight enumeration
		Force:              false,
	}

	copiedCount := 0
	skippedCount := 0
	failedCount := 0

	for _, srcTicket := range sortedTickets {
		if opts.DryRun {
			fmt.Printf("WOULD COPY: ticket %s (%s)\n", srcTicket.ID, srcTicket.Name)
			continue
		}

		// Check if already copied
		if dstID := m.GetTicketDst(srcTicket.ID); dstID != "" {
			skippedCount++
			continue
		}

		// Copy
		_, err := CopyTicket(srcClient, dstClient, srcTicket.ID, dstBoardID, m, ticketOpts)
		if err != nil {
			failedCount++
			fmt.Printf("ERROR copying ticket %s: %v\n", srcTicket.ID, err)
			continue
		}

		copiedCount++
		fmt.Printf("TICKET %s → %s (%s)\n", srcTicket.ID, m.GetTicketDst(srcTicket.ID), srcTicket.Name)
	}

	// Second pass: restore parent/child links (within-board only)
	if !opts.DryRun {
		for _, srcTicket := range allSrcTickets {
			children, err := srcClient.ListTicketsByParent(srcTicket.ID)
			if err != nil {
				continue // Skip if fetch fails
			}

			var childDstIDs []string
			for _, child := range children {
				if dstID := m.GetTicketDst(child.ID); dstID != "" && m.Bins[child.BinID] != "" {
					childDstIDs = append(childDstIDs, dstID)
				}
			}

			if len(childDstIDs) > 0 {
				parentDstID := m.GetTicketDst(srcTicket.ID)
				if parentDstID != "" {
					_ = dstClient.AddTicketParent(childDstIDs, parentDstID)
				}
			}
		}
	}

	// Summary
	fmt.Printf("Copy summary: %d copied, %d skipped, %d failed\n", copiedCount, skippedCount, failedCount)

	// Persist mapping
	if !opts.DryRun {
		m.From = "src" // TODO: get from context
		m.To = "dst"   // TODO: get from context
		m.SrcBoardID = srcBoardID
		m.DstBoardID = dstBoardID
		if err := m.Save(".copycard/mapping.json"); err != nil {
			return fmt.Errorf("save mapping: %w", err)
		}
	}

	return nil
}

// topologicalSort sorts tickets by order field (simple sort for now)
// Parents are placed before children based on order field
func topologicalSort(tickets []*fbclient.Ticket) []*fbclient.Ticket {
	// Sort by order field (ascending)
	sort.Slice(tickets, func(i, j int) bool {
		return tickets[i].Order < tickets[j].Order
	})

	return tickets
}
