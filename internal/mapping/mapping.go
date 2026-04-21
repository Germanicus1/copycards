package mapping

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type Mapping struct {
	From            string            `json:"from"`
	To              string            `json:"to"`
	SrcBoardID      string            `json:"srcBoard"`
	DstBoardID      string            `json:"dstBoard"`
	Users           map[string]string `json:"users,omitempty"`
	UserGroups      map[string]string `json:"userGroups,omitempty"`
	TicketTypes     map[string]string `json:"ticketTypes,omitempty"`
	CustomFields    map[string]string `json:"customFields,omitempty"`
	Bins            map[string]string `json:"bins,omitempty"`
	Tickets         map[string]string `json:"tickets,omitempty"`
	Comments        map[string]string `json:"comments,omitempty"`
	Attachments     map[string]string `json:"attachments,omitempty"`
}

// Load reads mapping from disk, returns empty mapping if file doesn't exist
func Load(path string) (*Mapping, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Mapping{
				Users:        make(map[string]string),
				UserGroups:   make(map[string]string),
				TicketTypes:  make(map[string]string),
				CustomFields: make(map[string]string),
				Bins:         make(map[string]string),
				Tickets:      make(map[string]string),
				Comments:     make(map[string]string),
				Attachments:  make(map[string]string),
			}, nil
		}
		return nil, fmt.Errorf("read mapping: %w", err)
	}

	var m Mapping
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parse mapping: %w", err)
	}
	return &m, nil
}

// Save writes mapping to disk, creating directories as needed
func (m *Mapping) Save(path string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}

	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal mapping: %w", err)
	}

	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("write mapping: %w", err)
	}
	return nil
}

// RecordUser adds a user mapping
func (m *Mapping) RecordUser(srcID, dstID string) {
	if m.Users == nil {
		m.Users = make(map[string]string)
	}
	m.Users[srcID] = dstID
}

// RecordTicket adds a ticket mapping
func (m *Mapping) RecordTicket(srcID, dstID string) {
	if m.Tickets == nil {
		m.Tickets = make(map[string]string)
	}
	m.Tickets[srcID] = dstID
}

// GetTicketDst returns the destination ticket ID, or "" if not mapped
func (m *Mapping) GetTicketDst(srcID string) string {
	if m.Tickets == nil {
		return ""
	}
	return m.Tickets[srcID]
}

// GetUserDst returns the destination user ID, or "" if not mapped
func (m *Mapping) GetUserDst(srcID string) string {
	if m.Users == nil {
		return ""
	}
	return m.Users[srcID]
}

// GetBinDst returns the destination bin ID, or "" if not mapped
func (m *Mapping) GetBinDst(srcID string) string {
	if m.Bins == nil {
		return ""
	}
	return m.Bins[srcID]
}

// GetTicketTypeDst returns the destination ticket type ID, or "" if not mapped
func (m *Mapping) GetTicketTypeDst(srcID string) string {
	if m.TicketTypes == nil {
		return ""
	}
	return m.TicketTypes[srcID]
}

// GetCustomFieldDst returns the destination custom field ID, or "" if not mapped
func (m *Mapping) GetCustomFieldDst(srcID string) string {
	if m.CustomFields == nil {
		return ""
	}
	return m.CustomFields[srcID]
}

// GetCommentDst returns the destination comment ID, or "" if not mapped
func (m *Mapping) GetCommentDst(srcID string) string {
	if m.Comments == nil {
		return ""
	}
	return m.Comments[srcID]
}

// GetAttachmentDst returns the destination attachment ID, or "" if not mapped
func (m *Mapping) GetAttachmentDst(srcID string) string {
	if m.Attachments == nil {
		return ""
	}
	return m.Attachments[srcID]
}

// GetUserGroupDst returns the destination user group ID, or "" if not mapped
func (m *Mapping) GetUserGroupDst(srcID string) string {
	if m.UserGroups == nil {
		return ""
	}
	return m.UserGroups[srcID]
}

// RecordUserGroup adds a user group mapping
func (m *Mapping) RecordUserGroup(srcID, dstID string) {
	if m.UserGroups == nil {
		m.UserGroups = make(map[string]string)
	}
	m.UserGroups[srcID] = dstID
}

// RecordComment adds a comment mapping
func (m *Mapping) RecordComment(srcID, dstID string) {
	if m.Comments == nil {
		m.Comments = make(map[string]string)
	}
	m.Comments[srcID] = dstID
}

// RecordAttachment adds an attachment mapping
func (m *Mapping) RecordAttachment(srcID, dstID string) {
	if m.Attachments == nil {
		m.Attachments = make(map[string]string)
	}
	m.Attachments[srcID] = dstID
}
