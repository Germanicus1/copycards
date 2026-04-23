package copier

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"copycards/internal/copier"
	"copycards/internal/fbclient"
	"copycards/internal/mapping"
)

func TestTranslateTicket(t *testing.T) {
	srcTicket := &fbclient.Ticket{
		ID:           "src-ticket-1",
		Name:         "Test Ticket",
		BinID:        "src-bin-1",
		TicketTypeID: "src-type-1",
		Order:        10.0,
		Description:  "A test ticket",
		AssignedIDs:  []string{"src-user-1"},
		CustomFields: map[string]interface{}{
			"src-field-1": 42,
		},
	}

	m := &mapping.Mapping{
		Bins: map[string]string{
			"src-bin-1": "dst-bin-1",
		},
		TicketTypes: map[string]string{
			"src-type-1": "dst-type-1",
		},
		CustomFields: map[string]string{
			"src-field-1": "dst-field-1",
		},
		Users: map[string]string{
			"src-user-1": "dst-user-1",
		},
	}

	dstTicket, err := copier.TranslateTicket(srcTicket, "dst-ticket-1", "dst-board-1", m, nil)
	if err != nil {
		t.Fatalf("TranslateTicket failed: %v", err)
	}

	if dstTicket.ID != "dst-ticket-1" {
		t.Errorf("ID mismatch: %s", dstTicket.ID)
	}
	if dstTicket.BinID != "dst-bin-1" {
		t.Errorf("BinID mismatch: %s", dstTicket.BinID)
	}
	if dstTicket.TicketTypeID != "dst-type-1" {
		t.Errorf("TicketTypeID mismatch: %s", dstTicket.TicketTypeID)
	}
	if len(dstTicket.AssignedIDs) != 1 || dstTicket.AssignedIDs[0] != "dst-user-1" {
		t.Errorf("AssignedIDs mismatch: %v", dstTicket.AssignedIDs)
	}
	if dstTicket.CustomFields["dst-field-1"] != 42 {
		t.Errorf("CustomFields mismatch: %v", dstTicket.CustomFields)
	}
	if dstTicket.Name != "Test Ticket" {
		t.Errorf("Name mismatch: %s", dstTicket.Name)
	}
	if dstTicket.Description != "A test ticket" {
		t.Errorf("Description mismatch: %s", dstTicket.Description)
	}
}

func TestTranslateTicketUnmappedUser(t *testing.T) {
	srcTicket := &fbclient.Ticket{
		ID:           "src-ticket-1",
		Name:         "Test Ticket",
		BinID:        "src-bin-1",
		TicketTypeID: "src-type-1",
		AssignedIDs:  []string{"unmapped-user"},
	}

	m := &mapping.Mapping{
		Bins: map[string]string{
			"src-bin-1": "dst-bin-1",
		},
		TicketTypes: map[string]string{
			"src-type-1": "dst-type-1",
		},
		Users: map[string]string{},
	}

	_, err := copier.TranslateTicket(srcTicket, "dst-ticket-1", "dst-board-1", m, nil)
	if err == nil {
		t.Fatal("Expected error for unmapped user")
	}
	if err.Error() != "unmapped user assignment: unmapped-user" {
		t.Errorf("Wrong error message: %v", err)
	}
}

func TestTranslateTicketUnmappedWatch(t *testing.T) {
	srcTicket := &fbclient.Ticket{
		ID:           "src-ticket-1",
		Name:         "Test Ticket",
		BinID:        "src-bin-1",
		TicketTypeID: "src-type-1",
		WatchIDs:     []string{"unmapped-watcher"},
	}

	m := &mapping.Mapping{
		Bins: map[string]string{
			"src-bin-1": "dst-bin-1",
		},
		TicketTypes: map[string]string{
			"src-type-1": "dst-type-1",
		},
		Users: map[string]string{},
	}

	_, err := copier.TranslateTicket(srcTicket, "dst-ticket-1", "dst-board-1", m, nil)
	if err == nil {
		t.Fatal("Expected error for unmapped watch user")
	}
	if err.Error() != "unmapped user watch: unmapped-watcher" {
		t.Errorf("Wrong error message: %v", err)
	}
}

func TestTranslateTicketMissingBinMapping(t *testing.T) {
	srcTicket := &fbclient.Ticket{
		ID:           "src-ticket-1",
		Name:         "Test Ticket",
		BinID:        "src-bin-1",
		TicketTypeID: "src-type-1",
	}

	m := &mapping.Mapping{
		Bins:        make(map[string]string),
		TicketTypes: map[string]string{
			"src-type-1": "dst-type-1",
		},
		Users: map[string]string{},
	}

	_, err := copier.TranslateTicket(srcTicket, "dst-ticket-1", "dst-board-1", m, nil)
	if err == nil {
		t.Fatal("Expected error for missing bin mapping")
	}
}

func TestTranslateTicketMissingTicketTypeMapping(t *testing.T) {
	srcTicket := &fbclient.Ticket{
		ID:           "src-ticket-1",
		Name:         "Test Ticket",
		BinID:        "src-bin-1",
		TicketTypeID: "src-type-1",
	}

	m := &mapping.Mapping{
		Bins:        map[string]string{
			"src-bin-1": "dst-bin-1",
		},
		TicketTypes: make(map[string]string),
		Users:       map[string]string{},
	}

	_, err := copier.TranslateTicket(srcTicket, "dst-ticket-1", "dst-board-1", m, nil)
	if err == nil {
		t.Fatal("Expected error for missing ticket type mapping")
	}
}

func TestTranslateTicketWithChecklists(t *testing.T) {
	// Mock server for ID allocation
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/ids" {
			w.Write([]byte(`["id-cl-1", "id-item-1", "id-item-2"]`))
		}
	}))
	defer server.Close()

	client := fbclient.NewClient(server.URL, "key")

	srcTicket := &fbclient.Ticket{
		ID:           "src-ticket-1",
		Name:         "Test Ticket",
		BinID:        "src-bin-1",
		TicketTypeID: "src-type-1",
		Checklists: map[string]fbclient.Checklist{
			"src-cl-1": {
				Name:  "Setup",
				Order: 1.0,
				Items: map[string]fbclient.ChecklistItem{
					"src-item-1": {
						Name:    "Step 1",
						Order:   1.0,
						Checked: true,
					},
				},
			},
		},
	}

	m := &mapping.Mapping{
		Bins: map[string]string{
			"src-bin-1": "dst-bin-1",
		},
		TicketTypes: map[string]string{
			"src-type-1": "dst-type-1",
		},
		Users: map[string]string{},
	}

	dstTicket, err := copier.TranslateTicket(srcTicket, "dst-ticket-1", "dst-board-1", m, client)
	if err != nil {
		t.Fatalf("TranslateTicket failed: %v", err)
	}

	if len(dstTicket.Checklists) != 1 {
		t.Errorf("Expected 1 checklist, got %d", len(dstTicket.Checklists))
	}

	for clID, cl := range dstTicket.Checklists {
		if cl.Name != "Setup" {
			t.Errorf("Checklist name mismatch: %s", cl.Name)
		}
		if len(cl.Items) != 1 {
			t.Errorf("Expected 1 item, got %d", len(cl.Items))
		}
		for itemID, item := range cl.Items {
			if item.Name != "Step 1" {
				t.Errorf("Item name mismatch: %s", item.Name)
			}
			if !item.Checked {
				t.Errorf("Item checked status mismatch")
			}
			// Verify IDs are regenerated (not the source IDs)
			if clID == "src-cl-1" {
				t.Errorf("Checklist ID not regenerated")
			}
			if itemID == "src-item-1" {
				t.Errorf("Item ID not regenerated")
			}
		}
	}
}

func TestTranslateTicketMultipleAssignees(t *testing.T) {
	srcTicket := &fbclient.Ticket{
		ID:           "src-ticket-1",
		Name:         "Test Ticket",
		BinID:        "src-bin-1",
		TicketTypeID: "src-type-1",
		AssignedIDs:  []string{"src-user-1", "src-user-2"},
	}

	m := &mapping.Mapping{
		Bins: map[string]string{
			"src-bin-1": "dst-bin-1",
		},
		TicketTypes: map[string]string{
			"src-type-1": "dst-type-1",
		},
		Users: map[string]string{
			"src-user-1": "dst-user-1",
			"src-user-2": "dst-user-2",
		},
	}

	dstTicket, err := copier.TranslateTicket(srcTicket, "dst-ticket-1", "dst-board-1", m, nil)
	if err != nil {
		t.Fatalf("TranslateTicket failed: %v", err)
	}

	if len(dstTicket.AssignedIDs) != 2 {
		t.Errorf("Expected 2 assignees, got %d", len(dstTicket.AssignedIDs))
	}
	if dstTicket.AssignedIDs[0] != "dst-user-1" || dstTicket.AssignedIDs[1] != "dst-user-2" {
		t.Errorf("AssignedIDs mismatch: %v", dstTicket.AssignedIDs)
	}
}

func TestTranslateTicketDatesPreserved(t *testing.T) {
	srcTicket := &fbclient.Ticket{
		ID:               "src-ticket-1",
		Name:             "Test Ticket",
		BinID:            "src-bin-1",
		TicketTypeID:     "src-type-1",
		PlannedStartDate: "2024-01-15",
		DueDate:          "2024-02-15",
	}

	m := &mapping.Mapping{
		Bins: map[string]string{
			"src-bin-1": "dst-bin-1",
		},
		TicketTypes: map[string]string{
			"src-type-1": "dst-type-1",
		},
		Users: map[string]string{},
	}

	dstTicket, err := copier.TranslateTicket(srcTicket, "dst-ticket-1", "dst-board-1", m, nil)
	if err != nil {
		t.Fatalf("TranslateTicket failed: %v", err)
	}

	if dstTicket.PlannedStartDate != "2024-01-15" {
		t.Errorf("PlannedStartDate mismatch: %s", dstTicket.PlannedStartDate)
	}
	if dstTicket.DueDate != "2024-02-15" {
		t.Errorf("DueDate mismatch: %s", dstTicket.DueDate)
	}
}
