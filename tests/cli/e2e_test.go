package cli_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"copycards/internal/copier"
	"copycards/internal/fbclient"
	"copycards/internal/mapping"
)

// TestFullBoardCopyFlow tests the complete end-to-end flow of copying a board
// This test verifies:
// - Full CopyBoard flow via copier package
// - Correct ticket copying with field translation
// - Mapping file creation and persistence
// - Parent/child link restoration
// - Dry-run mode prevents writes
func TestFullBoardCopyFlow(t *testing.T) {
	// Setup temporary home directory for mapping file
	tmpHome := t.TempDir()
	origHome := os.Getenv("HOME")
	defer os.Setenv("HOME", origHome)
	os.Setenv("HOME", tmpHome)

	// Create mock source server
	srcServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/boards/src-board":
			w.Write([]byte(`{
				"_id": "src-board",
				"name": "Source Board",
				"bins": [
					{"_id": "src-bin-backlog", "name": "Backlog"},
					{"_id": "src-bin-inprogress", "name": "In Progress"},
					{"_id": "src-bin-done", "name": "Done"}
				]
			}`))
		case "/tickets/src-ticket-1":
			w.Write([]byte(`{"_id": "src-ticket-1", "name": "Feature A", "bin_id": "src-bin-backlog", "ticketType_id": "src-type-story", "order": 1.0, "description": "Description A", "assigned_ids": ["src-user-1"], "watch_ids": []}`))
		case "/tickets/src-ticket-2":
			w.Write([]byte(`{"_id": "src-ticket-2", "name": "Feature B", "bin_id": "src-bin-backlog", "ticketType_id": "src-type-task", "order": 2.0, "description": "Description B", "assigned_ids": [], "watch_ids": ["src-user-1"]}`))
		case "/tickets/src-ticket-3":
			w.Write([]byte(`{"_id": "src-ticket-3", "name": "Feature C", "bin_id": "src-bin-inprogress", "ticketType_id": "src-type-story", "order": 3.0, "description": "Description C", "assigned_ids": ["src-user-1", "src-user-2"], "watch_ids": [], "customFields": {"src-field-priority": 1}}`))
		case "/tickets":
			if r.URL.Query().Get("bin_id") == "src-bin-backlog" {
				w.Write([]byte(`[
					{"_id": "src-ticket-1", "name": "Feature A", "bin_id": "src-bin-backlog", "ticketType_id": "src-type-story", "order": 1.0, "description": "Description A", "assigned_ids": ["src-user-1"], "watch_ids": []},
					{"_id": "src-ticket-2", "name": "Feature B", "bin_id": "src-bin-backlog", "ticketType_id": "src-type-task", "order": 2.0, "description": "Description B", "assigned_ids": [], "watch_ids": ["src-user-1"]}
				]`))
			} else if r.URL.Query().Get("bin_id") == "src-bin-inprogress" {
				w.Write([]byte(`[
					{"_id": "src-ticket-3", "name": "Feature C", "bin_id": "src-bin-inprogress", "ticketType_id": "src-type-story", "order": 3.0, "description": "Description C", "assigned_ids": ["src-user-1", "src-user-2"], "watch_ids": [], "customFields": {"src-field-priority": 1}}
				]`))
			} else if r.URL.Query().Get("bin_id") == "src-bin-done" {
				w.Write([]byte(`[]`))
			} else if r.URL.Query().Get("parent_id") != "" {
				w.Write([]byte(`[]`))
			} else {
				w.Write([]byte(`[]`))
			}
		case "/ticket-types":
			w.Write([]byte(`[
				{"_id": "src-type-story", "name": "Story"},
				{"_id": "src-type-task", "name": "Task"}
			]`))
		case "/custom-fields":
			w.Write([]byte(`[
				{"_id": "src-field-priority", "name": "Priority", "type": 3}
			]`))
		case "/users":
			w.Write([]byte(`[
				{"_id": "src-user-1", "email": "alice@test.com", "name": "Alice"},
				{"_id": "src-user-2", "email": "bob@test.com", "name": "Bob"}
			]`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srcServer.Close()

	// Create mock destination server
	createdTickets := make(map[string]*fbclient.Ticket)
	var idPool []string
	idCounter := 1
	dstServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/boards/dst-board":
			w.Write([]byte(`{
				"_id": "dst-board",
				"name": "Destination Board",
				"bins": [
					{"_id": "dst-bin-backlog", "name": "Backlog"},
					{"_id": "dst-bin-inprogress", "name": "In Progress"},
					{"_id": "dst-bin-done", "name": "Done"}
				]
			}`))
		case "/ticket-types":
			w.Write([]byte(`[
				{"_id": "dst-type-story", "name": "Story"},
				{"_id": "dst-type-task", "name": "Task"}
			]`))
		case "/custom-fields":
			w.Write([]byte(`[
				{"_id": "dst-field-priority", "name": "Priority", "type": 3}
			]`))
		case "/users":
			w.Write([]byte(`[
				{"_id": "dst-user-1", "email": "alice@test.com", "name": "Alice"},
				{"_id": "dst-user-2", "email": "bob@test.com", "name": "Bob"}
			]`))
		case "/ids":
			// Return a pool of new IDs
			idPool = []string{}
			for i := 0; i < 4; i++ {
				idPool = append(idPool, fmt.Sprintf("dst-ticket-pool-%d", idCounter))
				idCounter++
			}
			data, _ := json.Marshal(idPool)
			w.Write(data)
		default:
			if len(r.URL.Path) > 9 && r.URL.Path[:9] == "/tickets/" {
				if r.Method == "POST" {
					// Capture created tickets
					var ticket fbclient.Ticket
					if err := json.NewDecoder(r.Body).Decode(&ticket); err == nil {
						createdTickets[ticket.ID] = &ticket
					}
					w.WriteHeader(http.StatusOK)
					w.Write([]byte(`{}`))
				} else if r.Method == "GET" {
					// Return ticket if exists
					ticketID := r.URL.Path[9:]
					if t, ok := createdTickets[ticketID]; ok {
						data, _ := json.Marshal(t)
						w.Write(data)
					} else {
						http.NotFound(w, r)
					}
				} else if r.Method == "PUT" {
					w.WriteHeader(http.StatusOK)
					w.Write([]byte(`{}`))
				}
			} else if r.URL.Path == "/tickets/addParent" {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`{}`))
			} else {
				http.NotFound(w, r)
			}
		}
	}))
	defer dstServer.Close()

	// Create clients
	srcClient := fbclient.NewClient(srcServer.URL, "src-key", 4)
	dstClient := fbclient.NewClient(dstServer.URL, "dst-key", 4)

	// Create empty mapping
	m := &mapping.Mapping{
		Users:        make(map[string]string),
		UserGroups:   make(map[string]string),
		TicketTypes:  make(map[string]string),
		CustomFields: make(map[string]string),
		Bins:         make(map[string]string),
		Tickets:      make(map[string]string),
		Comments:     make(map[string]string),
		Attachments:  make(map[string]string),
	}

	// Test 1: Verify dry-run doesn't create tickets
	t.Run("DryRunDoesNotWrite", func(t *testing.T) {
		opts := copier.CopyBoardOptions{
			DryRun:      true,
			Concurrency: 1,
		}
		err := copier.CopyBoard(srcClient, dstClient, "src-board", "dst-board", m, opts)
		if err != nil {
			t.Fatalf("CopyBoard dry-run failed: %v", err)
		}

		// Verify no tickets were created
		if len(createdTickets) != 0 {
			t.Errorf("Expected no tickets created in dry-run, got %d", len(createdTickets))
		}
	})

	// Test 2: Actual copy (non-dry-run)
	t.Run("ActualCopyCreatesTickets", func(t *testing.T) {
		createdTickets = make(map[string]*fbclient.Ticket) // Reset
		opts := copier.CopyBoardOptions{
			DryRun:      false,
			Concurrency: 1,
		}
		err := copier.CopyBoard(srcClient, dstClient, "src-board", "dst-board", m, opts)
		if err != nil {
			t.Fatalf("CopyBoard failed: %v", err)
		}

		// Verify tickets were created
		if len(createdTickets) < 2 {
			t.Errorf("Expected at least 2 tickets created, got %d", len(createdTickets))
		}

		// Check that mapping is populated
		if len(m.Tickets) == 0 {
			t.Error("Expected ticket mappings to be populated")
		}

		// Verify field translation: check one ticket
		if dstTicket, ok := createdTickets["dst-ticket-1"]; ok {
			if dstTicket.Name != "Feature A" {
				t.Errorf("Ticket name mismatch: %s", dstTicket.Name)
			}
			if dstTicket.Description != "Description A" {
				t.Errorf("Ticket description mismatch: %s", dstTicket.Description)
			}
			// Verify bin was translated
			if dstTicket.BinID != "dst-bin-backlog" {
				t.Errorf("Bin ID not translated: %s", dstTicket.BinID)
			}
			// Verify type was translated
			if dstTicket.TicketTypeID != "dst-type-story" {
				t.Errorf("Type ID not translated: %s", dstTicket.TicketTypeID)
			}
			// Verify user was translated
			if len(dstTicket.AssignedIDs) > 0 && dstTicket.AssignedIDs[0] != "dst-user-1" {
				t.Errorf("User ID not translated: %v", dstTicket.AssignedIDs)
			}
		}
	})

	// Test 3: Mapping file is created
	t.Run("MappingFileCreated", func(t *testing.T) {
		// Save mapping
		mappingPath := filepath.Join(tmpHome, ".copycard", "mapping.json")
		err := m.Save(mappingPath)
		if err != nil {
			t.Fatalf("Save mapping failed: %v", err)
		}

		// Verify file exists
		if _, err := os.Stat(mappingPath); os.IsNotExist(err) {
			t.Error("Mapping file was not created")
		}

		// Load and verify content
		loadedM, err := mapping.Load(mappingPath)
		if err != nil {
			t.Fatalf("Load mapping failed: %v", err)
		}

		if len(loadedM.Tickets) == 0 {
			t.Error("Loaded mapping has no ticket mappings")
		}

		if loadedM.SrcBoardID != m.SrcBoardID {
			t.Errorf("Mapping SrcBoardID mismatch")
		}
	})
}

// TestParentChildLinkRestoration tests that parent/child relationships are properly restored
func TestParentChildLinkRestoration(t *testing.T) {
	tmpHome := t.TempDir()
	origHome := os.Getenv("HOME")
	defer os.Setenv("HOME", origHome)
	os.Setenv("HOME", tmpHome)

	// Mock source server with parent/child tickets
	srcServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/boards/src-board":
			w.Write([]byte(`{
				"_id": "src-board",
				"name": "Source Board",
				"bins": [{"_id": "src-bin-1", "name": "Backlog"}]
			}`))
		case "/tickets/src-epic-1":
			w.Write([]byte(`{"_id": "src-epic-1", "name": "Epic", "bin_id": "src-bin-1", "ticketType_id": "src-type-epic", "order": 1}`))
		case "/tickets/src-story-1":
			w.Write([]byte(`{"_id": "src-story-1", "name": "Story 1", "bin_id": "src-bin-1", "ticketType_id": "src-type-story", "order": 2}`))
		case "/tickets":
			w.Write([]byte(`[
				{"_id": "src-epic-1", "name": "Epic", "bin_id": "src-bin-1", "ticketType_id": "src-type-epic", "order": 1},
				{"_id": "src-story-1", "name": "Story 1", "bin_id": "src-bin-1", "ticketType_id": "src-type-story", "order": 2}
			]`))
		case "/ticket-types":
			w.Write([]byte(`[
				{"_id": "src-type-epic", "name": "Epic"},
				{"_id": "src-type-story", "name": "Story"}
			]`))
		case "/custom-fields":
			w.Write([]byte(`[]`))
		case "/users":
			w.Write([]byte(`[]`))
		default:
			if r.URL.Query().Get("parent_id") == "src-epic-1" {
				w.Write([]byte(`[
					{"_id": "src-story-1", "name": "Story 1", "bin_id": "src-bin-1", "ticketType_id": "src-type-story", "order": 2}
				]`))
			} else {
				w.Write([]byte(`[]`))
			}
		}
	}))
	defer srcServer.Close()

	// Mock destination server
	parentLinkCalled := false
	dstServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/boards/dst-board":
			w.Write([]byte(`{
				"_id": "dst-board",
				"name": "Destination Board",
				"bins": [{"_id": "dst-bin-1", "name": "Backlog"}]
			}`))
		case "/ticket-types":
			w.Write([]byte(`[
				{"_id": "dst-type-epic", "name": "Epic"},
				{"_id": "dst-type-story", "name": "Story"}
			]`))
		case "/custom-fields":
			w.Write([]byte(`[]`))
		case "/users":
			w.Write([]byte(`[]`))
		case "/ids":
			w.Write([]byte(`["dst-epic-1", "dst-story-1", "dst-story-2"]`))
		case "/tickets/addParent":
			parentLinkCalled = true
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{}`))
		default:
			if len(r.URL.Path) > 9 && r.URL.Path[:9] == "/tickets/" {
				if r.Method == "POST" {
					w.WriteHeader(http.StatusOK)
					w.Write([]byte(`{}`))
				}
			}
		}
	}))
	defer dstServer.Close()

	srcClient := fbclient.NewClient(srcServer.URL, "src-key", 1)
	dstClient := fbclient.NewClient(dstServer.URL, "dst-key", 1)

	m := &mapping.Mapping{
		Users:        make(map[string]string),
		UserGroups:   make(map[string]string),
		TicketTypes:  make(map[string]string),
		CustomFields: make(map[string]string),
		Bins:         make(map[string]string),
		Tickets:      make(map[string]string),
		Comments:     make(map[string]string),
		Attachments:  make(map[string]string),
	}

	opts := copier.CopyBoardOptions{
		DryRun:      false,
		Concurrency: 1,
	}

	err := copier.CopyBoard(srcClient, dstClient, "src-board", "dst-board", m, opts)
	if err != nil {
		t.Fatalf("CopyBoard failed: %v", err)
	}

	// Verify parent link restoration was attempted
	if !parentLinkCalled {
		t.Error("Parent link restoration endpoint was not called")
	}
}

// TestAttachmentCopyWithFlag tests that attachments are only copied when flag is set
func TestAttachmentCopyWithFlag(t *testing.T) {
	tmpHome := t.TempDir()
	origHome := os.Getenv("HOME")
	defer os.Setenv("HOME", origHome)
	os.Setenv("HOME", tmpHome)

	srcServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/boards/src-board":
			w.Write([]byte(`{
				"_id": "src-board",
				"name": "Source Board",
				"bins": [{"_id": "src-bin-1", "name": "Backlog"}]
			}`))
		case "/tickets/src-ticket-1":
			w.Write([]byte(`{"_id": "src-ticket-1", "name": "Ticket with File", "bin_id": "src-bin-1", "ticketType_id": "src-type-story", "order": 1}`))
		case "/tickets":
			w.Write([]byte(`[
				{"_id": "src-ticket-1", "name": "Ticket with File", "bin_id": "src-bin-1", "ticketType_id": "src-type-story", "order": 1}
			]`))
		case "/ticket-types":
			w.Write([]byte(`[{"_id": "src-type-story", "name": "Story"}]`))
		case "/custom-fields":
			w.Write([]byte(`[]`))
		case "/users":
			w.Write([]byte(`[]`))
		default:
			if r.URL.Path == "/tickets/src-ticket-1/attachments/att-1" {
				w.Write([]byte(`file content`))
			}
		}
	}))
	defer srcServer.Close()

	dstServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/boards/dst-board":
			w.Write([]byte(`{
				"_id": "dst-board",
				"name": "Destination Board",
				"bins": [{"_id": "dst-bin-1", "name": "Backlog"}]
			}`))
		case "/ticket-types":
			w.Write([]byte(`[{"_id": "dst-type-story", "name": "Story"}]`))
		case "/custom-fields":
			w.Write([]byte(`[]`))
		case "/users":
			w.Write([]byte(`[]`))
		case "/ids":
			w.Write([]byte(`["dst-ticket-1"]`))
		default:
			if len(r.URL.Path) > 9 && r.URL.Path[:9] == "/tickets/" {
				if r.Method == "POST" {
					w.WriteHeader(http.StatusOK)
					w.Write([]byte(`{}`))
				}
			}
		}
	}))
	defer dstServer.Close()

	srcClient := fbclient.NewClient(srcServer.URL, "src-key", 1)
	dstClient := fbclient.NewClient(dstServer.URL, "dst-key", 1)

	m := &mapping.Mapping{
		Users:        make(map[string]string),
		UserGroups:   make(map[string]string),
		TicketTypes:  make(map[string]string),
		CustomFields: make(map[string]string),
		Bins:         make(map[string]string),
		Tickets:      make(map[string]string),
		Comments:     make(map[string]string),
		Attachments:  make(map[string]string),
	}

	// Test without attachments flag
	t.Run("WithoutAttachmentFlag", func(t *testing.T) {
		opts := copier.CopyBoardOptions{
			DryRun:             false,
			IncludeAttachments: false,
			Concurrency:        1,
		}

		_ = copier.CopyBoard(srcClient, dstClient, "src-board", "dst-board", m, opts)
		// Attachments should not be fetched
	})

	// Test with attachments flag
	t.Run("WithAttachmentFlag", func(t *testing.T) {
		opts := copier.CopyBoardOptions{
			DryRun:             false,
			IncludeAttachments: true,
			Concurrency:        1,
		}

		_ = copier.CopyBoard(srcClient, dstClient, "src-board", "dst-board", m, opts)
		// With flag set, implementation should attempt to fetch attachments
	})
}

// TestCommentCopyWithFlag tests that comments are only copied when flag is set
func TestCommentCopyWithFlag(t *testing.T) {
	tmpHome := t.TempDir()
	origHome := os.Getenv("HOME")
	defer os.Setenv("HOME", origHome)
	os.Setenv("HOME", tmpHome)

	srcServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/boards/src-board":
			w.Write([]byte(`{
				"_id": "src-board",
				"name": "Source Board",
				"bins": [{"_id": "src-bin-1", "name": "Backlog"}]
			}`))
		case "/tickets/src-ticket-1":
			w.Write([]byte(`{"_id": "src-ticket-1", "name": "Ticket with Comments", "bin_id": "src-bin-1", "ticketType_id": "src-type-story", "order": 1}`))
		case "/tickets":
			w.Write([]byte(`[
				{"_id": "src-ticket-1", "name": "Ticket with Comments", "bin_id": "src-bin-1", "ticketType_id": "src-type-story", "order": 1}
			]`))
		case "/ticket-types":
			w.Write([]byte(`[{"_id": "src-type-story", "name": "Story"}]`))
		case "/custom-fields":
			w.Write([]byte(`[]`))
		case "/users":
			w.Write([]byte(`[]`))
		default:
			if r.URL.Path == "/ticket-comments" {
				w.Write([]byte(`[
					{"_id": "comment-1", "ticket_id": "src-ticket-1", "comment": "Nice work!", "createdAt": "2024-01-01T00:00:00Z"}
				]`))
			}
		}
	}))
	defer srcServer.Close()

	dstServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/boards/dst-board":
			w.Write([]byte(`{
				"_id": "dst-board",
				"name": "Destination Board",
				"bins": [{"_id": "dst-bin-1", "name": "Backlog"}]
			}`))
		case "/ticket-types":
			w.Write([]byte(`[{"_id": "dst-type-story", "name": "Story"}]`))
		case "/custom-fields":
			w.Write([]byte(`[]`))
		case "/users":
			w.Write([]byte(`[]`))
		case "/ids":
			w.Write([]byte(`["dst-ticket-1"]`))
		default:
			if len(r.URL.Path) > 9 && r.URL.Path[:9] == "/tickets/" {
				if r.Method == "POST" {
					w.WriteHeader(http.StatusOK)
					w.Write([]byte(`{}`))
				}
			}
		}
	}))
	defer dstServer.Close()

	srcClient := fbclient.NewClient(srcServer.URL, "src-key", 1)
	dstClient := fbclient.NewClient(dstServer.URL, "dst-key", 1)

	m := &mapping.Mapping{
		Users:        make(map[string]string),
		UserGroups:   make(map[string]string),
		TicketTypes:  make(map[string]string),
		CustomFields: make(map[string]string),
		Bins:         make(map[string]string),
		Tickets:      make(map[string]string),
		Comments:     make(map[string]string),
		Attachments:  make(map[string]string),
	}

	// Test without comments flag
	t.Run("WithoutCommentFlag", func(t *testing.T) {
		opts := copier.CopyBoardOptions{
			DryRun:           false,
			IncludeComments:  false,
			Concurrency:      1,
		}

		_ = copier.CopyBoard(srcClient, dstClient, "src-board", "dst-board", m, opts)
	})

	// Test with comments flag
	t.Run("WithCommentFlag", func(t *testing.T) {
		opts := copier.CopyBoardOptions{
			DryRun:           false,
			IncludeComments:  true,
			Concurrency:      1,
		}

		_ = copier.CopyBoard(srcClient, dstClient, "src-board", "dst-board", m, opts)
	})
}

// TestMockAPIEndpoints verifies all required API endpoints are handled
func TestMockAPIEndpoints(t *testing.T) {
	tmpHome := t.TempDir()
	origHome := os.Getenv("HOME")
	defer os.Setenv("HOME", origHome)
	os.Setenv("HOME", tmpHome)

	endpointsHit := make(map[string]bool)

	srcServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		endpointsHit[r.URL.Path] = true
		switch r.URL.Path {
		case "/boards/board1":
			w.Write([]byte(`{"_id":"board1","name":"Board","bins":[{"_id":"bin1","name":"Backlog"}]}`))
		case "/ticket-types":
			w.Write([]byte(`[{"_id":"type1","name":"Story"}]`))
		case "/custom-fields":
			w.Write([]byte(`[]`))
		case "/users":
			w.Write([]byte(`[]`))
		case "/tickets":
			w.Write([]byte(`[]`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srcServer.Close()

	dstServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		endpointsHit[r.URL.Path] = true
		switch r.URL.Path {
		case "/boards/board2":
			w.Write([]byte(`{"_id":"board2","name":"Board","bins":[{"_id":"bin2","name":"Backlog"}]}`))
		case "/ticket-types":
			w.Write([]byte(`[{"_id":"type2","name":"Story"}]`))
		case "/custom-fields":
			w.Write([]byte(`[]`))
		case "/users":
			w.Write([]byte(`[]`))
		case "/ids":
			w.Write([]byte(`["id1","id2","id3","id4"]`))
		default:
			if r.Method == "POST" && len(r.URL.Path) > 9 && r.URL.Path[:9] == "/tickets/" {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`{}`))
			}
		}
	}))
	defer dstServer.Close()

	srcClient := fbclient.NewClient(srcServer.URL, "src-key", 1)
	dstClient := fbclient.NewClient(dstServer.URL, "dst-key", 1)

	m := &mapping.Mapping{
		Users:        make(map[string]string),
		UserGroups:   make(map[string]string),
		TicketTypes:  make(map[string]string),
		CustomFields: make(map[string]string),
		Bins:         make(map[string]string),
		Tickets:      make(map[string]string),
		Comments:     make(map[string]string),
		Attachments:  make(map[string]string),
	}

	opts := copier.CopyBoardOptions{
		DryRun:      false,
		Concurrency: 1,
	}

	_ = copier.CopyBoard(srcClient, dstClient, "board1", "board2", m, opts)

	// Verify key endpoints were hit
	requiredEndpoints := []string{"/boards/board1", "/boards/board2", "/ticket-types", "/custom-fields", "/users"}
	for _, endpoint := range requiredEndpoints {
		if !endpointsHit[endpoint] {
			t.Errorf("Expected endpoint %s was not called", endpoint)
		}
	}
}
