package copier

import (
	"fmt"

	"copycards/internal/fbclient"
	"copycards/internal/mapping"
)

// CopyTicketOptions controls ticket copy behavior
type CopyTicketOptions struct {
	IncludeAttachments bool
	IncludeComments    bool
	WithChildren       bool
	Force              bool
}

// CopyTicket copies a single ticket from src to dst
func CopyTicket(srcClient, dstClient *fbclient.Client, srcTicketID, dstBoardID string, m *mapping.Mapping, opts CopyTicketOptions) (string, error) {
	// Check if already copied
	dstTicketID := m.GetTicketDst(srcTicketID)
	if dstTicketID != "" && !opts.Force {
		return dstTicketID, nil
	}

	// Fetch src ticket
	srcTicket, err := srcClient.GetTicket(srcTicketID)
	if err != nil {
		return "", fmt.Errorf("fetch src ticket: %w", err)
	}

	// Allocate new ID
	newID, err := AllocateTicketID(dstClient)
	if err != nil {
		return "", fmt.Errorf("allocate ticket ID: %w", err)
	}

	// Translate fields
	dstTicket, err := TranslateTicket(srcTicket, newID, dstBoardID, m, dstClient)
	if err != nil {
		return "", fmt.Errorf("translate ticket: %w", err)
	}

	// Create on dst
	if err := dstClient.CreateTicket(dstTicket); err != nil {
		return "", fmt.Errorf("create ticket on dst: %w", err)
	}

	// Record mapping
	m.RecordTicket(srcTicketID, newID)

	// Copy attachments if requested
	if opts.IncludeAttachments {
		if err := CopyAttachments(srcClient, dstClient, srcTicketID, newID, m); err != nil {
			return newID, fmt.Errorf("copy attachments: %w", err)
		}
	}

	// Copy comments if requested
	if opts.IncludeComments {
		if err := CopyComments(srcClient, dstClient, srcTicketID, newID, m); err != nil {
			return newID, fmt.Errorf("copy comments: %w", err)
		}
	}

	return newID, nil
}

// TranslateTicket converts src ticket to dst format, applying ID mappings
func TranslateTicket(srcTicket *fbclient.Ticket, newID, dstBoardID string, m *mapping.Mapping, dstClient *fbclient.Client) (*fbclient.Ticket, error) {
	dst := &fbclient.Ticket{
		ID:           newID,
		Name:         srcTicket.Name,
		BinID:        m.Bins[srcTicket.BinID],
		TicketTypeID: m.TicketTypes[srcTicket.TicketTypeID],
		Order:        srcTicket.Order,
		Description:  srcTicket.Description,
	}

	// Validate bin mapping
	if dst.BinID == "" {
		return nil, fmt.Errorf("no bin mapping for %s", srcTicket.BinID)
	}

	// Validate ticket type mapping
	if dst.TicketTypeID == "" {
		return nil, fmt.Errorf("no ticket type mapping for %s", srcTicket.TicketTypeID)
	}

	// Translate assigned users — FAIL if any unmapped
	if len(srcTicket.AssignedIDs) > 0 {
		dst.AssignedIDs = make([]string, 0)
		for _, srcUserID := range srcTicket.AssignedIDs {
			dstUserID, ok := m.Users[srcUserID]
			if !ok {
				return nil, fmt.Errorf("unmapped user assignment: %s", srcUserID)
			}
			dst.AssignedIDs = append(dst.AssignedIDs, dstUserID)
		}
	}

	// Translate watched users — FAIL if any unmapped
	if len(srcTicket.WatchIDs) > 0 {
		dst.WatchIDs = make([]string, 0)
		for _, srcUserID := range srcTicket.WatchIDs {
			dstUserID, ok := m.Users[srcUserID]
			if !ok {
				return nil, fmt.Errorf("unmapped user watch: %s", srcUserID)
			}
			dst.WatchIDs = append(dst.WatchIDs, dstUserID)
		}
	}

	// Translate custom fields
	if len(srcTicket.CustomFields) > 0 {
		dst.CustomFields = make(map[string]interface{})
		for srcFieldID, value := range srcTicket.CustomFields {
			dstFieldID, ok := m.CustomFields[srcFieldID]
			if !ok {
				return nil, fmt.Errorf("unmapped custom field: %s", srcFieldID)
			}
			dst.CustomFields[dstFieldID] = value
		}
	}

	// Translate checklists (inner IDs regenerated per spec)
	if len(srcTicket.Checklists) > 0 {
		dst.Checklists = make(map[string]fbclient.Checklist)
		for _, srcCL := range srcTicket.Checklists {
			dstCLID, _ := AllocateID(dstClient) // Regenerate checklist ID
			dstCL := fbclient.Checklist{
				Name:  srcCL.Name,
				Order: srcCL.Order,
			}
			if len(srcCL.Items) > 0 {
				dstCL.Items = make(map[string]fbclient.ChecklistItem)
				for _, srcItem := range srcCL.Items {
					dstItemID, _ := AllocateID(dstClient) // Regenerate item ID
					dstCL.Items[dstItemID] = fbclient.ChecklistItem{
						Name:    srcItem.Name,
						Order:   srcItem.Order,
						Checked: srcItem.Checked,
					}
				}
			}
			dst.Checklists[dstCLID] = dstCL
		}
	}

	// Copy date/effort fields verbatim
	dst.PlannedStartDate = srcTicket.PlannedStartDate
	dst.DueDate = srcTicket.DueDate

	// Handle parent/child: defer to second pass

	return dst, nil
}

// AllocateTicketID fetches a new ID from dst /ids endpoint
func AllocateTicketID(client *fbclient.Client) (string, error) {
	return AllocateID(client)
}

// AllocateID is a helper to get IDs from /ids endpoint
func AllocateID(client *fbclient.Client) (string, error) {
	if client == nil {
		return "", fmt.Errorf("client cannot be nil")
	}
	ids, err := client.GetIDs(1)
	if err != nil {
		return "", fmt.Errorf("allocate ID: %w", err)
	}
	if len(ids) == 0 {
		return "", fmt.Errorf("no IDs returned from /ids endpoint")
	}
	return ids[0], nil
}

// CopyAttachments copies all attachments from src ticket to dst ticket
func CopyAttachments(srcClient, dstClient *fbclient.Client, srcTicketID, dstTicketID string, m *mapping.Mapping) error {
	// TODO: implement in Task 7
	return nil
}

// CopyComments copies all comments from src ticket to dst ticket
func CopyComments(srcClient, dstClient *fbclient.Client, srcTicketID, dstTicketID string, m *mapping.Mapping) error {
	// TODO: implement in Task 7
	return nil
}
