package copier

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"copycards/internal/copier"
	"copycards/internal/fbclient"
)

func TestPreflightIdenticalBoards(t *testing.T) {
	// Mock src server
	srcServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/boards/board1":
			w.Write([]byte(`{"_id":"board1","name":"Board","bins":["bin1"]}`))
		case "/bins":
			w.Write([]byte(`[{"_id":"bin1","name":"Backlog"}]`))
		case "/tickets":
			w.Write([]byte(`[{"_id":"ticket1","name":"Task","bin_id":"bin1","ticketType_id":"type1","order":1,"customFields":{"field1":"3"}}]`))
		case "/ticket-types":
			w.Write([]byte(`[{"_id":"type1","name":"Story"}]`))
		case "/custom-fields":
			w.Write([]byte(`[{"_id":"field1","name":"Points","type":3}]`))
		case "/users":
			w.Write([]byte(`[{"_id":"user1","email":"alice@test.com","name":"Alice"}]`))
		}
	}))
	defer srcServer.Close()

	// Mock dst server
	dstServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/boards/board2":
			w.Write([]byte(`{"_id":"board2","name":"Board","bins":["bin2"]}`))
		case "/bins":
			w.Write([]byte(`[{"_id":"bin2","name":"Backlog"}]`))
		case "/ticket-types":
			w.Write([]byte(`[{"_id":"type2","name":"Story"}]`))
		case "/custom-fields":
			w.Write([]byte(`[{"_id":"field2","name":"Points","type":3}]`))
		case "/users":
			w.Write([]byte(`[{"_id":"user2","email":"alice@test.com","name":"Alice"}]`))
		}
	}))
	defer dstServer.Close()

	srcClient := fbclient.NewClient(srcServer.URL, "key")
	dstClient := fbclient.NewClient(dstServer.URL, "key")

	pf, err := copier.Preflight(srcClient, dstClient, "board1", "board2")
	if err != nil {
		t.Fatalf("Preflight failed: %v", err)
	}

	if !pf.Valid {
		t.Errorf("Expected valid boards, got errors:\n%s", pf.FormatErrors())
	}

	if pf.BinMapping["bin1"] != "bin2" {
		t.Errorf("Bin mapping mismatch: %v", pf.BinMapping)
	}

	if pf.TicketTypeMapping["type1"] != "type2" {
		t.Errorf("TicketType mapping mismatch: %v", pf.TicketTypeMapping)
	}

	if pf.CustomFieldMapping["field1"] != "field2" {
		t.Errorf("CustomField mapping mismatch: %v", pf.CustomFieldMapping)
	}

	if pf.UserMapping["user1"] != "user2" {
		t.Errorf("User mapping mismatch: %v", pf.UserMapping)
	}
}
