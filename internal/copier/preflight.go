package copier

import (
	"fmt"
	"strings"

	"copycards/internal/fbclient"
	"copycards/internal/mapping"
)

// MissingItem describes one thing the dst board lacks.
// Details is an optional diagnostic note (e.g. "exists in dst org but not on this board").
type MissingItem struct {
	Name    string
	Details string
}

type PreflightResult struct {
	Valid              bool
	BinMapping         map[string]string // src bin id -> dst bin id
	TicketTypeMapping  map[string]string // src type id -> dst type id
	CustomFieldMapping map[string]string // src field id -> dst field id
	UserMapping        map[string]string // src user id -> dst user id
	MissingBins        []MissingItem
	MissingTypes       []MissingItem
	MissingFields      []MissingItem
}

// Preflight checks if src and dst boards are compatible
func Preflight(srcClient, dstClient *fbclient.Client, srcBoardID, dstBoardID string) (*PreflightResult, error) {
	result := &PreflightResult{
		BinMapping:         make(map[string]string),
		TicketTypeMapping:  make(map[string]string),
		CustomFieldMapping: make(map[string]string),
		UserMapping:        make(map[string]string),
	}

	// Fetch boards
	fmt.Println("Fetching board details...")
	srcBoard, err := srcClient.GetBoard(srcBoardID)
	if err != nil {
		return nil, fmt.Errorf("fetch src board: %w", err)
	}
	fmt.Printf("  Source board has %d bins in its bin list\n", len(srcBoard.Bins))

	dstBoard, err := dstClient.GetBoard(dstBoardID)
	if err != nil {
		return nil, fmt.Errorf("fetch dst board: %w", err)
	}
	fmt.Printf("  Destination board has %d bins in its bin list\n", len(dstBoard.Bins))

	// Check bin names match - fetch all bins and filter by board
	fmt.Println("Loading source bins...")
	srcAllBins, err := srcClient.ListBins()
	if err != nil {
		return nil, fmt.Errorf("fetch src bins: %w", err)
	}

	srcBinsMap := make(map[string]fbclient.Bin)
	for _, bin := range srcAllBins {
		srcBinsMap[bin.ID] = bin
	}

	srcBinsByName := make(map[string]string) // name -> id
	srcMissingCount := 0
	for _, binID := range srcBoard.Bins {
		bin, ok := srcBinsMap[binID]
		if !ok {
			srcMissingCount++
			continue
		}
		srcBinsByName[bin.Name] = bin.ID
	}
	if srcMissingCount > 0 {
		fmt.Printf("  Loaded %d bins (skipped %d missing)\n", len(srcBinsByName), srcMissingCount)
	} else {
		fmt.Printf("  Loaded %d bins\n", len(srcBinsByName))
		for name := range srcBinsByName {
			fmt.Printf("    - %q\n", name)
		}
	}

	fmt.Println("Loading destination bins...")
	dstAllBins, err := dstClient.ListBins()
	if err != nil {
		return nil, fmt.Errorf("fetch dst bins: %w", err)
	}

	dstBinsMap := make(map[string]fbclient.Bin)
	for _, bin := range dstAllBins {
		dstBinsMap[bin.ID] = bin
	}

	// Lookup of every bin in the dst org by name, for the diagnostic on missing bins.
	dstAllBinsByName := make(map[string]bool)
	for _, bin := range dstAllBins {
		dstAllBinsByName[bin.Name] = true
	}

	dstBinsByName := make(map[string]string) // name -> id (bins actually on the dst board)
	dstMissingCount := 0
	for _, binID := range dstBoard.Bins {
		bin, ok := dstBinsMap[binID]
		if !ok {
			dstMissingCount++
			continue
		}
		dstBinsByName[bin.Name] = bin.ID
	}
	if dstMissingCount > 0 {
		fmt.Printf("  Loaded %d bins (skipped %d missing)\n", len(dstBinsByName), dstMissingCount)
	} else {
		fmt.Printf("  Loaded %d bins\n", len(dstBinsByName))
		for name := range dstBinsByName {
			fmt.Printf("    - %q\n", name)
		}
	}

	// Exact match: every src bin must exist on the dst board with the same name.
	for srcName, srcID := range srcBinsByName {
		if dstID, ok := dstBinsByName[srcName]; ok {
			result.BinMapping[srcID] = dstID
			continue
		}

		details := "not found in dst org"
		if dstAllBinsByName[srcName] {
			details = "exists in dst org but not on this board"
		}
		result.MissingBins = append(result.MissingBins, MissingItem{Name: srcName, Details: details})
	}

	// Collect ticket types and fields actually used on the board
	fmt.Println("Running preflight validation...")
	usedTypeIDs := make(map[string]bool)
	usedFieldIDs := make(map[string]bool)

	for binIdx, binID := range srcBoard.Bins {
		fmt.Printf("\r  Scanning: %d/%d...", binIdx+1, len(srcBoard.Bins))
		tickets, err := srcClient.ListTicketsByBin(binID)
		if err != nil {
			return nil, fmt.Errorf("fetch tickets in bin %s: %w", binID, err)
		}
		for _, ticket := range tickets {
			if ticket.TicketTypeID != "" {
				usedTypeIDs[ticket.TicketTypeID] = true
			}
			for fieldID := range ticket.CustomFields {
				usedFieldIDs[fieldID] = true
			}
		}
	}
	fmt.Println() // Move to next line after scanning

	// Check ticket types match by name (only used types)
	srcTypes, err := srcClient.ListTicketTypes()
	if err != nil {
		return nil, fmt.Errorf("fetch src ticket types: %w", err)
	}

	dstTypes, err := dstClient.ListTicketTypes()
	if err != nil {
		return nil, fmt.Errorf("fetch dst ticket types: %w", err)
	}

	dstTypesByName := make(map[string]*fbclient.TicketType)
	for i := range dstTypes {
		dstTypesByName[dstTypes[i].Name] = &dstTypes[i]
	}

	for i := range srcTypes {
		if !usedTypeIDs[srcTypes[i].ID] {
			continue // Skip unused types
		}
		if dstType, ok := dstTypesByName[srcTypes[i].Name]; ok {
			result.TicketTypeMapping[srcTypes[i].ID] = dstType.ID
		} else {
			result.MissingTypes = append(result.MissingTypes, MissingItem{Name: srcTypes[i].Name})
		}
	}

	// Check custom fields match by name + type (only used fields)
	srcFields, err := srcClient.ListCustomFields()
	if err != nil {
		return nil, fmt.Errorf("fetch src custom fields: %w", err)
	}

	dstFields, err := dstClient.ListCustomFields()
	if err != nil {
		return nil, fmt.Errorf("fetch dst custom fields: %w", err)
	}

	dstFieldsByNameType := make(map[string]*fbclient.CustomField)
	for i := range dstFields {
		key := fmt.Sprintf("%s:%d", dstFields[i].Name, dstFields[i].Type)
		dstFieldsByNameType[key] = &dstFields[i]
	}

	for i := range srcFields {
		if !usedFieldIDs[srcFields[i].ID] {
			continue // Skip unused fields
		}
		key := fmt.Sprintf("%s:%d", srcFields[i].Name, srcFields[i].Type)
		if dstField, ok := dstFieldsByNameType[key]; ok {
			result.CustomFieldMapping[srcFields[i].ID] = dstField.ID
		} else {
			result.MissingFields = append(result.MissingFields, MissingItem{
				Name:    srcFields[i].Name,
				Details: fmt.Sprintf("type %d", srcFields[i].Type),
			})
		}
	}

	// Build user map by email
	srcUsers, err := srcClient.ListUsers()
	if err != nil {
		return nil, fmt.Errorf("fetch src users: %w", err)
	}

	dstUsers, err := dstClient.ListUsers()
	if err != nil {
		return nil, fmt.Errorf("fetch dst users: %w", err)
	}

	dstUsersByEmail := make(map[string]*fbclient.User)
	for i := range dstUsers {
		dstUsersByEmail[dstUsers[i].Email] = &dstUsers[i]
	}

	for i := range srcUsers {
		if dstUser, ok := dstUsersByEmail[srcUsers[i].Email]; ok {
			result.UserMapping[srcUsers[i].ID] = dstUser.ID
		}
		// Missing users aren't fatal; they surface during ticket copy.
	}

	result.Valid = len(result.MissingBins) == 0 && len(result.MissingTypes) == 0 && len(result.MissingFields) == 0

	if result.Valid {
		fmt.Printf("✓ Boards are compatible. Found %d used types and %d used fields.\n", len(usedTypeIDs), len(usedFieldIDs))
	}

	return result, nil
}

// FormatErrors renders a human-readable summary of the compatibility problems.
// Returns an empty string if the result is valid.
func (pf *PreflightResult) FormatErrors() string {
	if pf.Valid {
		return ""
	}

	var b strings.Builder
	b.WriteString("Boards are not compatible.\n")

	writeSection := func(heading string, items []MissingItem) {
		if len(items) == 0 {
			return
		}
		b.WriteString("\n")
		fmt.Fprintf(&b, "%s (%d):\n", heading, len(items))
		for _, item := range items {
			if item.Details != "" {
				fmt.Fprintf(&b, "  - %q — %s\n", item.Name, item.Details)
			} else {
				fmt.Fprintf(&b, "  - %q\n", item.Name)
			}
		}
	}

	writeSection("Missing bins", pf.MissingBins)
	writeSection("Missing ticket types", pf.MissingTypes)
	writeSection("Missing custom fields", pf.MissingFields)

	return strings.TrimRight(b.String(), "\n")
}

// ApplyMappingToResult stores preflight mappings in the mapping file
func ApplyMappingToResult(m *mapping.Mapping, pf *PreflightResult) {
	for srcID, dstID := range pf.TicketTypeMapping {
		if m.TicketTypes == nil {
			m.TicketTypes = make(map[string]string)
		}
		m.TicketTypes[srcID] = dstID
	}

	for srcID, dstID := range pf.CustomFieldMapping {
		if m.CustomFields == nil {
			m.CustomFields = make(map[string]string)
		}
		m.CustomFields[srcID] = dstID
	}

	for srcID, dstID := range pf.BinMapping {
		if m.Bins == nil {
			m.Bins = make(map[string]string)
		}
		m.Bins[srcID] = dstID
	}

	for srcID, dstID := range pf.UserMapping {
		m.RecordUser(srcID, dstID)
	}
}
