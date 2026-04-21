package copier

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"copycards/internal/copier"
	"copycards/internal/fbclient"
	"copycards/internal/mapping"
)

func TestCopyBoardDryRun(t *testing.T) {
	// Mock src server
	srcServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/boards/board1":
			w.Write([]byte(`{"_id":"board1","name":"Board","bins":[{"_id":"bin1","name":"Backlog"}]}`))
		case "/tickets":
			w.Write([]byte(`[{"_id":"ticket1","name":"Task","bin_id":"bin1","ticketType_id":"type1","order":1}]`))
		case "/ticket-types":
			w.Write([]byte(`[{"_id":"type1","name":"Story"}]`))
		case "/custom-fields":
			w.Write([]byte(`[]`))
		case "/users":
			w.Write([]byte(`[]`))
		}
	}))
	defer srcServer.Close()

	// Mock dst server
	dstServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/boards/board2":
			w.Write([]byte(`{"_id":"board2","name":"Board","bins":[{"_id":"bin2","name":"Backlog"}]}`))
		case "/ticket-types":
			w.Write([]byte(`[{"_id":"type2","name":"Story"}]`))
		case "/custom-fields":
			w.Write([]byte(`[]`))
		case "/users":
			w.Write([]byte(`[]`))
		}
	}))
	defer dstServer.Close()

	srcClient := fbclient.NewClient(srcServer.URL, "key", 1)
	dstClient := fbclient.NewClient(dstServer.URL, "key", 1)

	m := &mapping.Mapping{
		Users:        make(map[string]string),
		TicketTypes:  make(map[string]string),
		CustomFields: make(map[string]string),
		Bins:         make(map[string]string),
		Tickets:      make(map[string]string),
		Comments:     make(map[string]string),
		Attachments:  make(map[string]string),
		UserGroups:   make(map[string]string),
	}

	opts := copier.CopyBoardOptions{
		DryRun:      true,
		Concurrency: 1,
	}

	err := copier.CopyBoard(srcClient, dstClient, "board1", "board2", m, opts)
	if err != nil {
		t.Fatalf("CopyBoard dry-run failed: %v", err)
	}
}
