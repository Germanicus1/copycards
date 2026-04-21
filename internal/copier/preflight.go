package copier

import (
	"fmt"

	"copycards/internal/fbclient"
	"copycards/internal/mapping"
)

type PreflightResult struct {
	Valid              bool
	BinMapping         map[string]string // src bin id -> dst bin id
	TicketTypeMapping  map[string]string // src type id -> dst type id
	CustomFieldMapping map[string]string // src field id -> dst field id
	UserMapping        map[string]string // src user id -> dst user id
	Errors             []string
}

// Preflight checks if src and dst boards are compatible
func Preflight(srcClient, dstClient *fbclient.Client, srcBoardID, dstBoardID string) (*PreflightResult, error) {
	result := &PreflightResult{
		BinMapping:         make(map[string]string),
		TicketTypeMapping:  make(map[string]string),
		CustomFieldMapping: make(map[string]string),
		UserMapping:        make(map[string]string),
		Errors:             []string{},
	}

	// Fetch boards
	srcBoard, err := srcClient.GetBoard(srcBoardID)
	if err != nil {
		return nil, fmt.Errorf("fetch src board: %w", err)
	}

	dstBoard, err := dstClient.GetBoard(dstBoardID)
	if err != nil {
		return nil, fmt.Errorf("fetch dst board: %w", err)
	}

	// Check bin names match
	srcBinsByName := make(map[string]*fbclient.Bin)
	for i := range srcBoard.Bins {
		srcBinsByName[srcBoard.Bins[i].Name] = &srcBoard.Bins[i]
	}

	dstBinsByName := make(map[string]*fbclient.Bin)
	for i := range dstBoard.Bins {
		dstBinsByName[dstBoard.Bins[i].Name] = &dstBoard.Bins[i]
	}

	// Exact match: every src bin must exist in dst with same name
	for srcName, srcBin := range srcBinsByName {
		if dstBin, ok := dstBinsByName[srcName]; ok {
			result.BinMapping[srcBin.ID] = dstBin.ID
		} else {
			result.Errors = append(result.Errors, fmt.Sprintf("src bin %q not found in dst", srcName))
		}
	}

	// Check ticket types match by name
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
		if dstType, ok := dstTypesByName[srcTypes[i].Name]; ok {
			result.TicketTypeMapping[srcTypes[i].ID] = dstType.ID
		} else {
			result.Errors = append(result.Errors, fmt.Sprintf("ticket type %q not found in dst", srcTypes[i].Name))
		}
	}

	// Check custom fields match by name + type
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
		key := fmt.Sprintf("%s:%d", srcFields[i].Name, srcFields[i].Type)
		if dstField, ok := dstFieldsByNameType[key]; ok {
			result.CustomFieldMapping[srcFields[i].ID] = dstField.ID
		} else {
			result.Errors = append(result.Errors, fmt.Sprintf("custom field %q (type %d) not found in dst", srcFields[i].Name, srcFields[i].Type))
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
		} else {
			// Don't error on missing user; will be caught during ticket copy
		}
	}

	result.Valid = len(result.Errors) == 0
	return result, nil
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
