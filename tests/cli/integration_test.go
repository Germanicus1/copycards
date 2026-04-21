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

// setupTestConfig creates a temporary config file and sets HOME to use it
func setupTestConfig(t *testing.T, srcURL, dstURL string) string {
	t.Helper()

	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, ".config", "copycards")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}

	configContent := `default_from = "src"
default_to = "dst"

[orgs.src]
org_id = "src-org"
api_key = "src-key"
endpoint = "` + srcURL + `"

[orgs.dst]
org_id = "dst-org"
api_key = "dst-key"
endpoint = "` + dstURL + `"
`
	configPath := filepath.Join(configDir, "config.toml")
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	return configPath
}

// buildMockSrcServer creates a mock src Flowboards API server
func buildMockSrcServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/boards":
			w.Write([]byte(`[{"_id":"board1","name":"Main Board","bins":[{"_id":"bin1","name":"Backlog"}]},{"_id":"board2","name":"Secondary","bins":[]}]`))
		case "/boards/board1":
			w.Write([]byte(`{"_id":"board1","name":"Main Board","bins":[{"_id":"bin1","name":"Backlog"}]}`))
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
			w.Write([]byte(`[{"_id":"dst-board1","name":"Dst Board","bins":[{"_id":"dst-bin1","name":"Backlog"}]}]`))
		case "/boards/dst-board1":
			w.Write([]byte(`{"_id":"dst-board1","name":"Dst Board","bins":[{"_id":"dst-bin1","name":"Backlog"}]}`))
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

	// Test cli.ListBoards requires config file
	// We test by temporarily overriding HOME to point to our temp config dir
	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, ".config", "copycards")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}

	configContent := `default_from = "src"
default_to = "dst"

[orgs.src]
org_id = "src-org"
api_key = "src-key"
endpoint = "` + srcServer.URL + `"
`
	if err := os.WriteFile(filepath.Join(configDir, "config.toml"), []byte(configContent), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	// Temporarily override HOME
	origHome := os.Getenv("HOME")
	defer os.Setenv("HOME", origHome)
	os.Setenv("HOME", tmpDir)

	// Call ListBoards
	err := cli.ListBoards("src")
	if err != nil {
		t.Fatalf("ListBoards failed: %v", err)
	}
}

func TestVerifyBoardsWithMockServer(t *testing.T) {
	srcServer := buildMockSrcServer(t)
	defer srcServer.Close()

	dstServer := buildMockDstServer(t)
	defer dstServer.Close()

	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, ".config", "copycards")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}

	configContent := `default_from = "src"
default_to = "dst"

[orgs.src]
org_id = "src-org"
api_key = "src-key"
endpoint = "` + srcServer.URL + `"

[orgs.dst]
org_id = "dst-org"
api_key = "dst-key"
endpoint = "` + dstServer.URL + `"
`
	if err := os.WriteFile(filepath.Join(configDir, "config.toml"), []byte(configContent), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	origHome := os.Getenv("HOME")
	defer os.Setenv("HOME", origHome)
	os.Setenv("HOME", tmpDir)

	err := cli.VerifyBoards("src", "dst", "board1", "dst-board1")
	if err != nil {
		t.Fatalf("VerifyBoards failed: %v", err)
	}
}

func TestShowMappingEmpty(t *testing.T) {
	tmpDir := t.TempDir()

	origHome := os.Getenv("HOME")
	defer os.Setenv("HOME", origHome)
	os.Setenv("HOME", tmpDir)

	// Should not error even if no mapping exists
	err := cli.ShowMapping("", "", "")
	if err != nil {
		t.Fatalf("ShowMapping failed: %v", err)
	}
}

func TestShowMappingWithContent(t *testing.T) {
	tmpDir := t.TempDir()
	copycardDir := filepath.Join(tmpDir, ".copycard")
	if err := os.MkdirAll(copycardDir, 0755); err != nil {
		t.Fatalf("mkdir .copycard: %v", err)
	}

	// Create a mapping file
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

	mappingPath := filepath.Join(copycardDir, "mapping.json")
	if err := m.Save(mappingPath); err != nil {
		t.Fatalf("save mapping: %v", err)
	}

	origHome := os.Getenv("HOME")
	defer os.Setenv("HOME", origHome)
	os.Setenv("HOME", tmpDir)

	err := cli.ShowMapping("", "", "")
	if err != nil {
		t.Fatalf("ShowMapping failed: %v", err)
	}
}

func TestDiffBoardsWithMockServer(t *testing.T) {
	srcServer := buildMockSrcServer(t)
	defer srcServer.Close()

	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, ".config", "copycards")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}

	configContent := `[orgs.src]
org_id = "src-org"
api_key = "src-key"
endpoint = "` + srcServer.URL + `"

[orgs.dst]
org_id = "dst-org"
api_key = "dst-key"
endpoint = "` + srcServer.URL + `"
`
	if err := os.WriteFile(filepath.Join(configDir, "config.toml"), []byte(configContent), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	origHome := os.Getenv("HOME")
	defer os.Setenv("HOME", origHome)
	os.Setenv("HOME", tmpDir)

	err := cli.DiffBoards("src", "dst", "board1", "dst-board1")
	if err != nil {
		t.Fatalf("DiffBoards failed: %v", err)
	}
}
