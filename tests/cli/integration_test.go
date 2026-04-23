package cli_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"copycards/internal/cli"
	"copycards/internal/mapping"
)

// buildMockSrcServer creates a mock src Flowboards API server
func buildMockSrcServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/boards":
			w.Write([]byte(`[{"_id":"board1","name":"Main Board","bins":["bin1"]},{"_id":"board2","name":"Secondary","bins":[]}]`))
		case "/boards/board1":
			w.Write([]byte(`{"_id":"board1","name":"Main Board","bins":["bin1"]}`))
		case "/bins":
			w.Write([]byte(`[{"_id":"bin1","name":"Backlog"}]`))
		case "/ticket-types":
			w.Write([]byte(`[{"_id":"type1","name":"Story"}]`))
		case "/custom-fields":
			w.Write([]byte(`[]`))
		case "/users":
			w.Write([]byte(`[{"_id":"user1","email":"alice@test.com","name":"Alice"}]`))
		default:
			if r.URL.Path[:8] == "/tickets" {
				w.Write([]byte(`[{"_id":"ticket1","name":"Task A","bin_id":"bin1","ticketType_id":"type1","order":1}]`))
			} else {
				http.NotFound(w, r)
			}
		}
	}))
}

// buildMockDstServer creates a mock dst Flowboards API server
func buildMockDstServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/boards":
			w.Write([]byte(`[{"_id":"dst-board1","name":"Dst Board","bins":["dst-bin1"]}]`))
		case "/boards/dst-board1":
			w.Write([]byte(`{"_id":"dst-board1","name":"Dst Board","bins":["dst-bin1"]}`))
		case "/bins":
			w.Write([]byte(`[{"_id":"dst-bin1","name":"Backlog"}]`))
		case "/ticket-types":
			w.Write([]byte(`[{"_id":"dst-type1","name":"Story"}]`))
		case "/custom-fields":
			w.Write([]byte(`[]`))
		case "/users":
			w.Write([]byte(`[{"_id":"dst-user1","email":"alice@test.com","name":"Alice"}]`))
		case "/ids":
			w.Write([]byte(`["new-id-1","new-id-2","new-id-3","new-id-4"]`))
		default:
			if len(r.URL.Path) > 8 && r.URL.Path[:8] == "/tickets" {
				if r.Method == "POST" {
					w.WriteHeader(http.StatusOK)
					w.Write([]byte(`{}`))
				} else {
					w.Write([]byte(`[]`))
				}
			} else {
				http.NotFound(w, r)
			}
		}
	}))
}

func TestListBoardsWithMockServer(t *testing.T) {
	srcServer := buildMockSrcServer(t)
	defer srcServer.Close()
	dstServer := buildMockDstServer(t)
	defer dstServer.Close()

	home := withTempHome(t)
	writeTestConfig(t, home, srcServer.URL, "")

	if err := cli.ListBoards("src"); err != nil {
		t.Fatalf("ListBoards failed: %v", err)
	}
}

func TestVerifyBoardsWithMockServer(t *testing.T) {
	srcServer := buildMockSrcServer(t)
	defer srcServer.Close()
	dstServer := buildMockDstServer(t)
	defer dstServer.Close()

	home := withTempHome(t)
	writeTestConfig(t, home, srcServer.URL, dstServer.URL)

	if err := cli.VerifyBoards("src", "dst", "board1", "dst-board1"); err != nil {
		t.Fatalf("VerifyBoards failed: %v", err)
	}
}

func TestShowMappingEmpty(t *testing.T) {
	withTempHome(t)

	if err := cli.ShowMapping("", "", ""); err != nil {
		t.Fatalf("ShowMapping failed: %v", err)
	}
}

func TestShowMappingWithContent(t *testing.T) {
	home := withTempHome(t)
	copycardDir := filepath.Join(home, ".copycard")
	if err := os.MkdirAll(copycardDir, 0o755); err != nil {
		t.Fatalf("mkdir .copycard: %v", err)
	}

	m := &mapping.Mapping{
		From:         "src",
		To:           "dst",
		SrcBoardID:   "board1",
		DstBoardID:   "dst-board1",
		Tickets:      map[string]string{"t1": "t2", "t3": "t4"},
		Users:        map[string]string{"u1": "u2"},
		Bins:         map[string]string{"b1": "b2"},
		TicketTypes:  map[string]string{"tt1": "tt2"},
		CustomFields: map[string]string{},
		Comments:     map[string]string{},
		Attachments:  map[string]string{},
		UserGroups:   map[string]string{},
	}
	if err := m.Save(filepath.Join(copycardDir, "mapping.json")); err != nil {
		t.Fatalf("save mapping: %v", err)
	}

	if err := cli.ShowMapping("", "", ""); err != nil {
		t.Fatalf("ShowMapping failed: %v", err)
	}
}

func TestDiffBoardsWithMockServer(t *testing.T) {
	srcServer := buildMockSrcServer(t)
	defer srcServer.Close()

	// DiffBoards uses both orgs but only hits the src endpoint for tickets;
	// point both at srcServer so there's nothing to stub on a second host.
	home := withTempHome(t)
	writeTestConfig(t, home, srcServer.URL, srcServer.URL)

	if err := cli.DiffBoards("src", "dst", "board1", "dst-board1"); err != nil {
		t.Fatalf("DiffBoards failed: %v", err)
	}
}
