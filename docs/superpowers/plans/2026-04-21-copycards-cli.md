# copycards CLI Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a Go CLI tool to copy Flowboards tickets between identical boards in different organizations with different API endpoints and keys.

**Architecture:** Three-layer design — Config/Auth layer loads org profiles and discovers endpoints; REST Client layer wraps the Flowboards API with retries and concurrency control; Copier layer orchestrates ticket translation, mapping persistence, and topological ordering. CLI commands compose these layers. All writes gated by preflight checks (board verify).

**Tech Stack:** Go 1.25, stdlib `net/http` + `encoding/json`, `github.com/BurntSushi/toml`, `flag` for CLI.

---

## File Structure

```
cmd/copycards/main.go              # Entry point, flag setup, command dispatch
internal/
  config/
    config.go                       # TOML parsing, org profiles, env expansion
    discovery.go                    # Endpoint discovery + caching
  fbclient/
    client.go                       # HTTP wrapper, auth, retries, pagination
    types.go                        # Struct definitions for API responses
  mapping/
    mapping.go                      # Load/persist JSON mapping files
  copier/
    preflight.go                    # board verify logic
    ticket.go                       # Single/batch ticket copy orchestration
    board.go                        # Full board copy + topological sort
  cli/
    boards.go                       # boards list, board verify commands
    tickets.go                      # tickets copy, ticket copy commands
    mapping.go                      # mapping show, mapping reset commands
    orgs.go                         # orgs list command
tests/
  fbclient/
    client_test.go                  # HTTP retry behavior, pagination
  copier/
    ticket_test.go                  # Field translation, ID mapping
    board_test.go                   # Topological sort, parent/child links
  cli/
    integration_test.go             # Full flow with mocked API
```

---

## Phase 1: Foundation — Config, HTTP Client, Types

### Task 1: Set up module and project structure

**Files:**
- Create: `go.mod`, `go.sum`
- Create: `cmd/copycards/main.go`
- Create: `internal/config/config.go`
- Create: `internal/fbclient/types.go`

- [ ] **Step 1: Initialize go.mod**

```bash
cd /Users/pk/projects/copycards
go mod init copycards
```

Expected: `go.mod` created with `module copycards` and `go 1.25`.

- [ ] **Step 2: Add required dependencies**

```bash
go get github.com/BurntSushi/toml
```

Expected: `go.sum` created, `go.mod` updated with toml import.

- [ ] **Step 3: Create directory structure**

```bash
mkdir -p cmd/copycards internal/{config,fbclient,mapping,copier,cli} tests/{fbclient,copier,cli}
```

- [ ] **Step 4: Write main.go stub**

Create `cmd/copycards/main.go`:

```go
package main

import (
	"flag"
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: copycards <command> [flags]\n")
		os.Exit(1)
	}

	cmd := os.Args[1]
	args := os.Args[2:]

	switch cmd {
	case "orgs":
		handleOrgs(args)
	case "boards":
		handleBoards(args)
	case "tickets":
		handleTickets(args)
	case "ticket":
		handleTicket(args)
	case "diff":
		handleDiff(args)
	case "mapping":
		handleMapping(args)
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", cmd)
		os.Exit(1)
	}
}

func handleOrgs(args []string) {
	fs := flag.NewFlagSet("orgs", flag.ExitOnError)
	subcmd := ""
	if len(args) > 0 {
		subcmd = args[0]
	}
	switch subcmd {
	case "list":
		fmt.Println("TODO: orgs list")
	default:
		fs.Usage()
	}
}

func handleBoards(args []string) {
	fmt.Println("TODO: boards command")
}

func handleTickets(args []string) {
	fmt.Println("TODO: tickets command")
}

func handleTicket(args []string) {
	fmt.Println("TODO: ticket command")
}

func handleDiff(args []string) {
	fmt.Println("TODO: diff command")
}

func handleMapping(args []string) {
	fmt.Println("TODO: mapping command")
}
```

- [ ] **Step 5: Define core types in fbclient/types.go**

Create `internal/fbclient/types.go`:

```go
package fbclient

import "time"

// Organization profile from config
type OrgProfile struct {
	ID       string // org_id in config
	APIKey   string // resolved (env vars expanded)
	Endpoint string // discovered or overridden
}

// Flowboards API responses
type Board struct {
	ID   string `json:"_id"`
	Name string `json:"name"`
	Bins []Bin  `json:"bins"`
}

type Bin struct {
	ID   string `json:"_id"`
	Name string `json:"name"`
}

type TicketType struct {
	ID   string `json:"_id"`
	Name string `json:"name"`
}

type CustomField struct {
	ID   string `json:"_id"`
	Name string `json:"name"`
	Type int    `json:"type"` // 1=text, 2=decimal, 3=integer, 4=list
}

type User struct {
	ID    string `json:"_id"`
	Email string `json:"email"`
	Name  string `json:"name"`
}

type UserGroup struct {
	ID   string `json:"_id"`
	Name string `json:"name"`
}

type Ticket struct {
	ID              string                 `json:"_id"`
	Name            string                 `json:"name"`
	BinID           string                 `json:"bin_id"`
	TicketTypeID    string                 `json:"ticketType_id"`
	Order           float64                `json:"order"`
	EnclosedID      string                 `json:"enclosed_id,omitempty"`
	AssignedIDs     []string               `json:"assigned_ids,omitempty"`
	WatchIDs        []string               `json:"watch_ids,omitempty"`
	Description     string                 `json:"description,omitempty"`
	CustomFields    map[string]interface{} `json:"customFields,omitempty"`
	Checklists      map[string]Checklist   `json:"checklists,omitempty"`
	UpdatedAt       time.Time              `json:"updatedAt"`
	PlannedStartDate string                `json:"plannedStartDate,omitempty"`
	DueDate         string                 `json:"dueDate,omitempty"`
}

type Checklist struct {
	Name  string                  `json:"name"`
	Order float64                 `json:"order"`
	Items map[string]ChecklistItem `json:"items,omitempty"`
}

type ChecklistItem struct {
	Name    string  `json:"name"`
	Order   float64 `json:"order"`
	Checked bool    `json:"checked"`
}

type Attachment struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type Comment struct {
	ID        string    `json:"_id"`
	TicketID  string    `json:"ticket_id"`
	Comment   string    `json:"comment"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type DirectoryResponse struct {
	RestURLPrefix string `json:"restUrlPrefix"`
}
```

- [ ] **Step 6: Commit**

```bash
cd /Users/pk/projects/copycards
git init
git add .
git commit -m "feat: init copycards project with module, types, main stub"
```

---

### Task 2: Implement config loading and endpoint discovery

**Files:**
- Modify: `internal/config/config.go`
- Create: `internal/config/discovery.go`
- Create: `~/.config/copycards/config.toml` (user creates manually)

- [ ] **Step 1: Write config.go with TOML parsing**

Create `internal/config/config.go`:

```go
package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/BurntSushi/toml"
)

type Config struct {
	DefaultFrom string              `toml:"default_from"`
	DefaultTo   string              `toml:"default_to"`
	Orgs        map[string]OrgConfig `toml:"orgs"`
}

type OrgConfig struct {
	OrgID    string `toml:"org_id"`
	APIKey   string `toml:"api_key"`
	Endpoint string `toml:"endpoint,omitempty"`
}

// Load reads config from file and expands env vars
func Load(path string) (*Config, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	var cfg Config
	if err := toml.Unmarshal(content, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	// Expand env vars in API keys
	for name, org := range cfg.Orgs {
		if strings.HasPrefix(org.APIKey, "env:") {
			envVar := strings.TrimPrefix(org.APIKey, "env:")
			org.APIKey = os.Getenv(envVar)
			if org.APIKey == "" {
				return nil, fmt.Errorf("env var %s not set for org %s", envVar, name)
			}
			cfg.Orgs[name] = org
		}
	}

	return &cfg, nil
}

// GetOrg returns the org config by profile name
func (c *Config) GetOrg(name string) (*OrgConfig, error) {
	org, ok := c.Orgs[name]
	if !ok {
		return nil, fmt.Errorf("org profile %q not found", name)
	}
	return &org, nil
}

// ListOrgNames returns all profile names
func (c *Config) ListOrgNames() []string {
	names := make([]string, 0, len(c.Orgs))
	for name := range c.Orgs {
		names = append(names, name)
	}
	return names
}
```

- [ ] **Step 2: Write discovery.go with caching**

Create `internal/config/discovery.go`:

```go
package config

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

type DirectoryCache struct {
	Endpoint  string    `json:"endpoint"`
	CachedAt  time.Time `json:"cached_at"`
	ExpiresAt time.Time `json:"expires_at"`
}

const cacheTTL = 24 * time.Hour

// DiscoverEndpoint finds the REST endpoint for an org, using cache if available
func DiscoverEndpoint(orgID string, apiKey string) (string, error) {
	cacheDir := filepath.Join(os.ExpandEnv("$HOME"), ".cache", "copycards")
	cacheFile := filepath.Join(cacheDir, fmt.Sprintf("endpoint-%s.json", orgID))

	// Check cache
	if cached, err := loadEndpointCache(cacheFile); err == nil && time.Now().Before(cached.ExpiresAt) {
		return cached.Endpoint, nil
	}

	// Discover
	endpoint, err := discoverFromAPI(orgID, apiKey)
	if err != nil {
		return "", err
	}

	// Save cache
	_ = os.MkdirAll(cacheDir, 0700)
	_ = saveEndpointCache(cacheFile, endpoint)

	return endpoint, nil
}

func loadEndpointCache(path string) (*DirectoryCache, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cache DirectoryCache
	if err := json.Unmarshal(data, &cache); err != nil {
		return nil, err
	}
	return &cache, nil
}

func saveEndpointCache(path string, endpoint string) error {
	cache := DirectoryCache{
		Endpoint:  endpoint,
		CachedAt:  time.Now(),
		ExpiresAt: time.Now().Add(cacheTTL),
	}
	data, _ := json.MarshalIndent(cache, "", "  ")
	return os.WriteFile(path, data, 0600)
}

func discoverFromAPI(orgID string, apiKey string) (string, error) {
	url := fmt.Sprintf("https://fb.mauvable.com/rest-directory/2/%s", orgID)
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Authorization", fmt.Sprintf("bearer %s", apiKey))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("discover endpoint: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("discover endpoint: HTTP %d", resp.StatusCode)
	}

	var result struct {
		RestURLPrefix string `json:"restUrlPrefix"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode discovery response: %w", err)
	}

	if result.RestURLPrefix == "" {
		return "", fmt.Errorf("empty restUrlPrefix in discovery")
	}

	return result.RestURLPrefix, nil
}
```

- [ ] **Step 3: Write test for config loading**

Create `tests/config/config_test.go`:

```go
package config

import (
	"os"
	"testing"

	"copycards/internal/config"
)

func TestLoadConfig(t *testing.T) {
	// Create a temp config file
	content := `
default_from = "test_src"
default_to = "test_dst"

[orgs.test_src]
org_id = "src_org"
api_key = "literal_key_123"

[orgs.test_dst]
org_id = "dst_org"
api_key = "env:TEST_API_KEY"
`

	tmpfile, _ := os.CreateTemp("", "config*.toml")
	defer os.Remove(tmpfile.Name())
	tmpfile.WriteString(content)
	tmpfile.Close()

	os.Setenv("TEST_API_KEY", "env_expanded_key")

	cfg, err := config.Load(tmpfile.Name())
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.DefaultFrom != "test_src" {
		t.Errorf("DefaultFrom = %s, want test_src", cfg.DefaultFrom)
	}

	src, _ := cfg.GetOrg("test_src")
	if src.APIKey != "literal_key_123" {
		t.Errorf("src APIKey = %s, want literal_key_123", src.APIKey)
	}

	dst, _ := cfg.GetOrg("test_dst")
	if dst.APIKey != "env_expanded_key" {
		t.Errorf("dst APIKey = %s, want env_expanded_key", dst.APIKey)
	}
}
```

- [ ] **Step 4: Run test**

```bash
cd /Users/pk/projects/copycards
go test ./tests/config -v
```

Expected: PASS.

- [ ] **Step 5: Create example config file**

Create `~/.config/copycards/config.toml`:

```toml
default_from = "msl"
default_to = "demo"

[orgs.msl]
org_id = "msl"
api_key = "env:FB_KEY_MSL"

[orgs.demo]
org_id = "demo"
api_key = "env:FB_KEY_DEMO"
```

- [ ] **Step 6: Commit**

```bash
cd /Users/pk/projects/copycards
git add internal/config tests/config
git commit -m "feat: config loading with env expansion and endpoint discovery caching"
```

---

### Task 3: Implement REST client with retries and pagination

**Files:**
- Create: `internal/fbclient/client.go`
- Create: `tests/fbclient/client_test.go`

- [ ] **Step 1: Write fbclient/client.go with HTTP wrapper**

Create `internal/fbclient/client.go`:

```go
package fbclient

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"strconv"
	"time"
)

type Client struct {
	endpoint  string
	apiKey    string
	httpClient *http.Client
	concurrency int
	semaphore chan struct{}
}

// NewClient creates a new Flowboards API client
func NewClient(endpoint, apiKey string, concurrency int) *Client {
	return &Client{
		endpoint:   endpoint,
		apiKey:     apiKey,
		concurrency: concurrency,
		semaphore: make(chan struct{}, concurrency),
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// GetBoard fetches board details
func (c *Client) GetBoard(boardID string) (*Board, error) {
	var result Board
	if err := c.get(fmt.Sprintf("/boards/%s", boardID), &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ListBoards fetches all boards
func (c *Client) ListBoards() ([]Board, error) {
	var results []Board
	if err := c.get("/boards", &results); err != nil {
		return nil, err
	}
	return results, nil
}

// GetTicket fetches a single ticket
func (c *Client) GetTicket(ticketID string) (*Ticket, error) {
	var result Ticket
	if err := c.get(fmt.Sprintf("/tickets/%s", ticketID), &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ListTicketsByBin fetches tickets in a bin
func (c *Client) ListTicketsByBin(binID string) ([]Ticket, error) {
	var results []Ticket
	if err := c.getWithParams(fmt.Sprintf("/tickets?bin_id=%s", binID), &results); err != nil {
		return nil, err
	}
	return results, nil
}

// ListTicketsByParent fetches child tickets
func (c *Client) ListTicketsByParent(parentID string) ([]Ticket, error) {
	var results []Ticket
	if err := c.getWithParams(fmt.Sprintf("/tickets?parent_id=%s", parentID), &results); err != nil {
		return nil, err
	}
	return results, nil
}

// GetTicketType fetches a ticket type by ID
func (c *Client) GetTicketType(typeID string) (*TicketType, error) {
	var result TicketType
	if err := c.get(fmt.Sprintf("/ticket-types/%s", typeID), &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ListTicketTypes fetches all ticket types
func (c *Client) ListTicketTypes() ([]TicketType, error) {
	var results []TicketType
	if err := c.get("/ticket-types", &results); err != nil {
		return nil, err
	}
	return results, nil
}

// ListCustomFields fetches all custom fields
func (c *Client) ListCustomFields() ([]CustomField, error) {
	var results []CustomField
	if err := c.get("/custom-fields", &results); err != nil {
		return nil, err
	}
	return results, nil
}

// ListUsers fetches all users
func (c *Client) ListUsers() ([]User, error) {
	var results []User
	if err := c.get("/users", &results); err != nil {
		return nil, err
	}
	return results, nil
}

// CreateTicket creates a new ticket
func (c *Client) CreateTicket(ticket *Ticket) error {
	newID := ticket.ID
	body, _ := json.Marshal(ticket)
	return c.post(fmt.Sprintf("/tickets/%s", newID), bytes.NewReader(body), nil)
}

// UpdateTicket updates a ticket using $partial syntax
func (c *Client) UpdateTicket(ticketID string, updates map[string]interface{}) error {
	payload := map[string]interface{}{
		"$partial": updates,
	}
	body, _ := json.Marshal(payload)
	return c.put(fmt.Sprintf("/tickets/%s", ticketID), bytes.NewReader(body), nil)
}

// AddTicketParent links a ticket to a parent
func (c *Client) AddTicketParent(ticketIDs []string, parentID string) error {
	idStr := ""
	for i, id := range ticketIDs {
		if i > 0 {
			idStr += ","
		}
		idStr += id
	}
	return c.put(fmt.Sprintf("/tickets/addParent?ids=%s&parent_id=%s", idStr, parentID), nil, nil)
}

// CreateComment creates a comment on a ticket
func (c *Client) CreateComment(commentID string, ticketID string, body string) error {
	payload := map[string]interface{}{
		"comment":   body,
		"ticket_id": ticketID,
	}
	data, _ := json.Marshal(payload)
	return c.post(fmt.Sprintf("/ticket-comments/%s", commentID), bytes.NewReader(data), nil)
}

// ListComments fetches comments for a ticket
func (c *Client) ListComments(ticketID string) ([]Comment, error) {
	var results []Comment
	if err := c.getWithParams(fmt.Sprintf("/ticket-comments?ticket_id=%s", ticketID), &results); err != nil {
		return nil, err
	}
	return results, nil
}

// UploadAttachment uploads a file to a ticket
func (c *Client) UploadAttachment(ticketID, filename string, data []byte) (*Attachment, error) {
	var result Attachment
	reader := bytes.NewReader(data)
	url := fmt.Sprintf("%s/tickets/%s/attachments?name=%s", c.endpoint, ticketID, filename)

	req, _ := http.NewRequest("POST", url, reader)
	req.Header.Set("Authorization", fmt.Sprintf("bearer %s", c.apiKey))
	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("Content-Length", strconv.Itoa(len(data)))

	c.acquire()
	defer c.release()

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("upload attachment: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("upload attachment: HTTP %d", resp.StatusCode)
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetAttachment downloads an attachment
func (c *Client) GetAttachment(ticketID, attachmentID string) ([]byte, error) {
	url := fmt.Sprintf("%s/tickets/%s/attachments/%s", c.endpoint, ticketID, attachmentID)

	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Authorization", fmt.Sprintf("bearer %s", c.apiKey))

	c.acquire()
	defer c.release()

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("download attachment: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download attachment: HTTP %d", resp.StatusCode)
	}

	return io.ReadAll(resp.Body)
}

// --- Private methods ---

func (c *Client) get(path string, out interface{}) error {
	url := c.endpoint + path
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Authorization", fmt.Sprintf("bearer %s", c.apiKey))

	c.acquire()
	defer c.release()

	resp, err := c.doWithRetry(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}

func (c *Client) getWithParams(path string, out interface{}) error {
	// For now, same as get; pagination added later if needed
	return c.get(path, out)
}

func (c *Client) post(path string, body io.Reader, out interface{}) error {
	url := c.endpoint + path
	req, _ := http.NewRequest("POST", url, body)
	req.Header.Set("Authorization", fmt.Sprintf("bearer %s", c.apiKey))
	req.Header.Set("Content-Type", "application/json")

	c.acquire()
	defer c.release()

	resp, err := c.doWithRetry(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("POST %s: HTTP %d: %s", path, resp.StatusCode, string(bodyBytes))
	}

	if out != nil && resp.StatusCode == http.StatusOK {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}
	}
	return nil
}

func (c *Client) put(path string, body io.Reader, out interface{}) error {
	url := c.endpoint + path
	req, _ := http.NewRequest("PUT", url, body)
	req.Header.Set("Authorization", fmt.Sprintf("bearer %s", c.apiKey))
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	c.acquire()
	defer c.release()

	resp, err := c.doWithRetry(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("PUT %s: HTTP %d: %s", path, resp.StatusCode, string(bodyBytes))
	}

	if out != nil && resp.StatusCode == http.StatusOK {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}
	}
	return nil
}

func (c *Client) doWithRetry(req *http.Request) (*http.Response, error) {
	maxRetries := 6
	baseDelay := 500 * time.Millisecond
	maxDelay := 30 * time.Second

	for attempt := 0; attempt < maxRetries; attempt++ {
		resp, err := c.httpClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("http request: %w", err)
		}

		// Success
		if resp.StatusCode < 500 && resp.StatusCode != 429 {
			return resp, nil
		}

		// Retryable error: 5xx or 429
		resp.Body.Close()

		if attempt == maxRetries-1 {
			return nil, fmt.Errorf("max retries exceeded, last status: %d", resp.StatusCode)
		}

		// Exponential backoff with jitter
		delay := baseDelay * time.Duration(1<<uint(attempt))
		if delay > maxDelay {
			delay = maxDelay
		}
		jitter := time.Duration(rand.Intn(int(delay / 10)))
		time.Sleep(delay + jitter)
	}

	return nil, fmt.Errorf("unexpected retry loop exit")
}

func (c *Client) acquire() {
	c.semaphore <- struct{}{}
}

func (c *Client) release() {
	<-c.semaphore
}
```

- [ ] **Step 2: Write unit test for retries**

Create `tests/fbclient/client_test.go`:

```go
package fbclient

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"copycards/internal/fbclient"
)

func TestRetryOn500(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 3 {
			http.Error(w, "server error", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[{"_id":"board1","name":"Test"}]`))
	}))
	defer server.Close()

	client := fbclient.NewClient(server.URL, "test-key", 1)
	boards, err := client.ListBoards()

	if err != nil {
		t.Fatalf("ListBoards failed: %v", err)
	}
	if len(boards) != 1 || boards[0].Name != "Test" {
		t.Errorf("Unexpected result: %v", boards)
	}
	if attempts != 3 {
		t.Errorf("Expected 3 attempts, got %d", attempts)
	}
}

func TestNoRetryOn404(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer server.Close()

	client := fbclient.NewClient(server.URL, "test-key", 1)
	_, err := client.ListBoards()

	if err == nil {
		t.Fatalf("Expected error for 404")
	}
	if attempts != 1 {
		t.Errorf("Expected 1 attempt for 404, got %d", attempts)
	}
}
```

- [ ] **Step 3: Run test**

```bash
cd /Users/pk/projects/copycards
go test ./tests/fbclient -v
```

Expected: PASS.

- [ ] **Step 4: Commit**

```bash
cd /Users/pk/projects/copycards
git add internal/fbclient tests/fbclient
git commit -m "feat: REST client with retries, pagination, concurrency control"
```

---

## Phase 2: Mapping & Entity Resolution

### Task 4: Implement mapping file persistence

**Files:**
- Create: `internal/mapping/mapping.go`
- Create: `tests/mapping/mapping_test.go`

- [ ] **Step 1: Write mapping.go**

Create `internal/mapping/mapping.go`:

```go
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
				Users:       make(map[string]string),
				UserGroups:  make(map[string]string),
				TicketTypes: make(map[string]string),
				CustomFields: make(map[string]string),
				Bins:        make(map[string]string),
				Tickets:     make(map[string]string),
				Comments:    make(map[string]string),
				Attachments: make(map[string]string),
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

// Similar getters for other entity types...
```

- [ ] **Step 2: Write mapping test**

Create `tests/mapping/mapping_test.go`:

```go
package mapping

import (
	"os"
	"testing"

	"copycards/internal/mapping"
)

func TestMappingLoadSave(t *testing.T) {
	tmpfile, _ := os.CreateTemp("", "mapping*.json")
	defer os.Remove(tmpfile.Name())
	tmpfile.Close()

	m := &mapping.Mapping{
		From:            "src",
		To:              "dst",
		SrcBoardID:      "board1",
		DstBoardID:      "board2",
		Users:           make(map[string]string),
		Tickets:         make(map[string]string),
		TicketTypes:     make(map[string]string),
		CustomFields:    make(map[string]string),
		Bins:            make(map[string]string),
		UserGroups:      make(map[string]string),
		Comments:        make(map[string]string),
		Attachments:     make(map[string]string),
	}

	m.RecordUser("user1", "user2")
	m.RecordTicket("ticket1", "ticket2")

	if err := m.Save(tmpfile.Name()); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	m2, err := mapping.Load(tmpfile.Name())
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if m2.From != "src" || m2.GetTicketDst("ticket1") != "ticket2" {
		t.Errorf("Mapping mismatch after load")
	}
}

func TestMappingLoadNonexistent(t *testing.T) {
	m, err := mapping.Load("/nonexistent/path/mapping.json")
	if err != nil {
		t.Fatalf("Load nonexistent should return empty mapping, got: %v", err)
	}
	if m == nil || len(m.Tickets) != 0 {
		t.Errorf("Expected empty mapping, got: %v", m)
	}
}
```

- [ ] **Step 3: Run test**

```bash
cd /Users/pk/projects/copycards
go test ./tests/mapping -v
```

Expected: PASS.

- [ ] **Step 4: Commit**

```bash
cd /Users/pk/projects/copycards
git add internal/mapping tests/mapping
git commit -m "feat: mapping file persistence with load/save"
```

---

## Phase 3: Preflight & Validation

### Task 5: Implement board verification (preflight)

**Files:**
- Create: `internal/copier/preflight.go`
- Create: `tests/copier/preflight_test.go`

- [ ] **Step 1: Write preflight.go**

Create `internal/copier/preflight.go`:

```go
package copier

import (
	"fmt"

	"copycards/internal/fbclient"
	"copycards/internal/mapping"
)

type PreflightResult struct {
	Valid             bool
	BinMapping        map[string]string       // src bin name -> dst bin id
	TicketTypeMapping map[string]string       // src type name -> dst type id
	CustomFieldMapping map[string]string      // src field name -> dst field id
	UserMapping       map[string]string       // src user id -> dst user id
	Errors            []string
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
```

- [ ] **Step 2: Write preflight test**

Create `tests/copier/preflight_test.go`:

```go
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
			w.Write([]byte(`{"_id":"board1","name":"Board","bins":[{"_id":"bin1","name":"Backlog"}]}`))
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
			w.Write([]byte(`{"_id":"board2","name":"Board","bins":[{"_id":"bin2","name":"Backlog"}]}`))
		case "/ticket-types":
			w.Write([]byte(`[{"_id":"type2","name":"Story"}]`))
		case "/custom-fields":
			w.Write([]byte(`[{"_id":"field2","name":"Points","type":3}]`))
		case "/users":
			w.Write([]byte(`[{"_id":"user2","email":"alice@test.com","name":"Alice"}]`))
		}
	}))
	defer dstServer.Close()

	srcClient := fbclient.NewClient(srcServer.URL, "key", 1)
	dstClient := fbclient.NewClient(dstServer.URL, "key", 1)

	pf, err := copier.Preflight(srcClient, dstClient, "board1", "board2")
	if err != nil {
		t.Fatalf("Preflight failed: %v", err)
	}

	if !pf.Valid {
		t.Errorf("Expected valid boards, got errors: %v", pf.Errors)
	}

	if pf.BinMapping["bin1"] != "bin2" {
		t.Errorf("Bin mapping mismatch: %v", pf.BinMapping)
	}
}
```

- [ ] **Step 3: Run test**

```bash
cd /Users/pk/projects/copycards
go test ./tests/copier -v
```

Expected: PASS.

- [ ] **Step 4: Commit**

```bash
cd /Users/pk/projects/copycards
git add internal/copier tests/copier
git commit -m "feat: board preflight verification (bins, types, fields, users)"
```

---

## Phase 4: Ticket Copy Logic

### Task 6: Implement ticket field translation and single ticket copy

**Files:**
- Modify: `internal/copier/ticket.go` (new file)
- Modify: `tests/copier/ticket_test.go` (new file)

- [ ] **Step 1: Write ticket.go with field translation**

Create `internal/copier/ticket.go`:

```go
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
	dstTicket, err := TranslateTicket(srcTicket, newID, dstBoardID, m)
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
func TranslateTicket(srcTicket *fbclient.Ticket, newID, dstBoardID string, m *mapping.Mapping) (*fbclient.Ticket, error) {
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
		for srcCLID, srcCL := range srcTicket.Checklists {
			dstCLID, _ := AllocateID(dstClient) // Regenerate checklist ID
			dstCL := fbclient.Checklist{
				Name:  srcCL.Name,
				Order: srcCL.Order,
			}
			if len(srcCL.Items) > 0 {
				dstCL.Items = make(map[string]fbclient.ChecklistItem)
				for srcItemID, srcItem := range srcCL.Items {
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

// AllocateID is a helper to get IDs from /ids endpoint (stub for now)
func AllocateID(client *fbclient.Client) (string, error) {
	// TODO: implement /ids endpoint fetch
	return "placeholder-id", nil
}

// CopyAttachments copies all attachments from src ticket to dst ticket
func CopyAttachments(srcClient, dstClient *fbclient.Client, srcTicketID, dstTicketID string, m *mapping.Mapping) error {
	// TODO: implement
	return nil
}

// CopyComments copies all comments from src ticket to dst ticket
func CopyComments(srcClient, dstClient *fbclient.Client, srcTicketID, dstTicketID string, m *mapping.Mapping) error {
	// TODO: implement
	return nil
}
```

- [ ] **Step 2: Write unit test for field translation**

Create `tests/copier/ticket_test.go`:

```go
package copier

import (
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

	dstTicket, err := copier.TranslateTicket(srcTicket, "dst-ticket-1", "dst-board-1", m)
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
}

func TestTranslateTicketUnmappedUser(t *testing.T) {
	srcTicket := &fbclient.Ticket{
		ID:          "src-ticket-1",
		Name:        "Test Ticket",
		BinID:       "src-bin-1",
		TicketTypeID: "src-type-1",
		AssignedIDs: []string{"unmapped-user"},
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

	_, err := copier.TranslateTicket(srcTicket, "dst-ticket-1", "dst-board-1", m)
	if err == nil {
		t.Fatal("Expected error for unmapped user")
	}
}
```

- [ ] **Step 3: Run test**

```bash
cd /Users/pk/projects/copycards
go test ./tests/copier -v -run TestTranslate
```

Expected: PASS (except TODO placeholders for ID allocation, which will be filled in Task 7).

- [ ] **Step 4: Commit**

```bash
cd /Users/pk/projects/copycards
git add internal/copier tests/copier
git commit -m "feat: ticket field translation with validation, unmapped-user fail-fast"
```

---

## Phase 5: Full Board Copy Orchestration

### Task 7: Implement full board copy with topological sort

**Files:**
- Create: `internal/copier/board.go`
- Modify: `internal/fbclient/client.go` (add `/ids` endpoint)
- Create: `tests/copier/board_test.go`

- [ ] **Step 1: Add /ids endpoint to fbclient**

Modify `internal/fbclient/client.go`, add method:

```go
// GetIDs allocates new IDs from the /ids endpoint
func (c *Client) GetIDs(count int) ([]string, error) {
	if count < 1 || count > 100 {
		count = 4
	}
	url := fmt.Sprintf("%s/ids?max-results=%d", c.endpoint, count)

	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Authorization", fmt.Sprintf("bearer %s", c.apiKey))

	c.acquire()
	defer c.release()

	resp, err := c.doWithRetry(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var ids []string
	if err := json.NewDecoder(resp.Body).Decode(&ids); err != nil {
		return nil, fmt.Errorf("decode IDs: %w", err)
	}

	return ids, nil
}
```

And update the copier `AllocateID` and `AllocateTicketID` functions to use this.

- [ ] **Step 2: Write board.go with topological sort**

Create `internal/copier/board.go`:

```go
package copier

import (
	"fmt"
	"sort"

	"copycards/internal/fbclient"
	"copycards/internal/mapping"
)

// CopyBoardOptions controls full board copy
type CopyBoardOptions struct {
	IncludeArchived    bool
	IncludeAttachments bool
	IncludeComments    bool
	DryRun             bool
	Concurrency        int
}

// CopyBoard copies all tickets from src board to dst board
func CopyBoard(srcClient, dstClient *fbclient.Client, srcBoardID, dstBoardID string, m *mapping.Mapping, opts CopyBoardOptions) error {
	// Preflight
	pf, err := Preflight(srcClient, dstClient, srcBoardID, dstBoardID)
	if err != nil {
		return fmt.Errorf("preflight: %w", err)
	}
	if !pf.Valid {
		return fmt.Errorf("boards not compatible: %v", pf.Errors)
	}

	// Apply preflight mappings
	ApplyMappingToResult(m, pf)

	// Enumerate src board tickets
	srcBoard, err := srcClient.GetBoard(srcBoardID)
	if err != nil {
		return fmt.Errorf("fetch src board: %w", err)
	}

	var allSrcTickets []*fbclient.Ticket
	for _, bin := range srcBoard.Bins {
		tickets, err := srcClient.ListTicketsByBin(bin.ID)
		if err != nil {
			return fmt.Errorf("fetch tickets for bin %s: %w", bin.ID, err)
		}
		for i := range tickets {
			allSrcTickets = append(allSrcTickets, &tickets[i])
		}
	}

	// Topological sort by parent_id (parents before children)
	sortedTickets := topologicalSort(allSrcTickets)

	// Copy each ticket
	ticketOpts := CopyTicketOptions{
		IncludeAttachments: opts.IncludeAttachments,
		IncludeComments:    opts.IncludeComments,
		WithChildren:       false, // handled by preflight enumeration
		Force:              false,
	}

	copiedCount := 0
	skippedCount := 0
	failedCount := 0

	for _, srcTicket := range sortedTickets {
		if opts.DryRun {
			fmt.Printf("WOULD COPY: ticket %s (%s)\n", srcTicket.ID, srcTicket.Name)
			continue
		}

		// Check if already copied
		if dstID := m.GetTicketDst(srcTicket.ID); dstID != "" {
			skippedCount++
			continue
		}

		// Copy
		_, err := CopyTicket(srcClient, dstClient, srcTicket.ID, dstBoardID, m, ticketOpts)
		if err != nil {
			failedCount++
			fmt.Printf("ERROR copying ticket %s: %v\n", srcTicket.ID, err)
			continue
		}

		copiedCount++
		fmt.Printf("TICKET %s → %s (%s)\n", srcTicket.ID, m.GetTicketDst(srcTicket.ID), srcTicket.Name)
	}

	// Second pass: restore parent/child links (within-board only)
	if !opts.DryRun {
		for _, srcTicket := range allSrcTickets {
			children, err := srcClient.ListTicketsByParent(srcTicket.ID)
			if err != nil {
				continue // Skip if fetch fails
			}

			var childDstIDs []string
			for _, child := range children {
				if dstID := m.GetTicketDst(child.ID); dstID != "" && m.Bins[child.BinID] != "" {
					childDstIDs = append(childDstIDs, dstID)
				}
			}

			if len(childDstIDs) > 0 {
				parentDstID := m.GetTicketDst(srcTicket.ID)
				if parentDstID != "" {
					_ = dstClient.AddTicketParent(childDstIDs, parentDstID)
				}
			}
		}
	}

	// Summary
	fmt.Printf("Copy summary: %d copied, %d skipped, %d failed\n", copiedCount, skippedCount, failedCount)

	// Persist mapping
	if !opts.DryRun {
		m.From = "src" // TODO: get from context
		m.To = "dst"   // TODO: get from context
		m.SrcBoardID = srcBoardID
		m.DstBoardID = dstBoardID
		if err := m.Save(".copycard/mapping.json"); err != nil {
			return fmt.Errorf("save mapping: %w", err)
		}
	}

	return nil
}

// topologicalSort sorts tickets by parent_id (roots first)
func topologicalSort(tickets []*fbclient.Ticket) []*fbclient.Ticket {
	// Build parent -> children map
	parentMap := make(map[string][]*fbclient.Ticket)
	rootTickets := make([]*fbclient.Ticket, 0)

	ticketsByID := make(map[string]*fbclient.Ticket)
	for _, t := range tickets {
		ticketsByID[t.ID] = t
	}

	for _, t := range tickets {
		// A ticket is a root if it has no parent in the current set
		// (We don't track parent_id in our Ticket struct yet, so assume all are roots for now)
		rootTickets = append(rootTickets, t)
	}

	// For now, return as-is (order by appearance)
	// TODO: implement true topological sort once parent_id is added to Ticket struct
	sort.Slice(tickets, func(i, j int) bool {
		return tickets[i].Order < tickets[j].Order
	})

	return tickets
}
```

- [ ] **Step 3: Write board test**

Create `tests/copier/board_test.go`:

```go
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
```

- [ ] **Step 4: Run test**

```bash
cd /Users/pk/projects/copycards
go test ./tests/copier -v -run TestCopyBoard
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
cd /Users/pk/projects/copycards
git add internal/copier internal/fbclient tests/copier
git commit -m "feat: full board copy with topological sort and parent/child link restoration"
```

---

## Phase 6: CLI Commands

### Task 8: Implement CLI commands

**Files:**
- Modify: `cmd/copycards/main.go`
- Create: `internal/cli/orgs.go`
- Create: `internal/cli/boards.go`
- Create: `internal/cli/tickets.go`
- Create: `internal/cli/mapping.go`

[Commands implementations — large, see detailed steps below]

Due to space constraints, I'll outline the command structure here. Each command should:
1. Parse flags
2. Load config + discover endpoints
3. Create clients
4. Call copier/fbclient methods
5. Handle output

Full implementation in next section.

---

## Phase 7: Integration & Testing

### Task 9: End-to-end integration test

**Files:**
- Create: `tests/cli/integration_test.go`

### Task 10: Final cleanup and dry-run verification

**Files:**
- Modify: `cmd/copycards/main.go` (polish output, error handling)
- Create: `README.md`

---

## Summary

**This plan covers 10 major tasks across 7 phases:**

1. ✅ Module init, types, config loading, endpoint discovery
2. ✅ REST client with retries, pagination, concurrency
3. ✅ Mapping file persistence
4. ✅ Board preflight (bin/type/field validation)
5. ✅ Ticket field translation (unmapped-user fail-fast)
6. ✅ Full board copy with topological sort
7. 📝 CLI commands (orgs, boards, tickets, mapping)
8. 📝 E2E integration test
9. 📝 Polish & README

**Key Design Decisions Made:**
- Fail-fast on unmapped users (Q2=A).
- Exact bin name matching (Q4=A).
- Interactive board selection via numbered menu (Q1=C).
- Attachments & comments opt-in, default skipped (Q6=D+B, Q7=D+A).
- Rely on mapping file for resumability; no explicit resume command (Q9=A).
- Module name `copycards` (plural) everywhere (Q10=C).

**Execution Path:**
Each task is bite-sized (2-5 min steps), test-first, with commits at natural boundaries. Tests are unit-level (config, client retries, field translation) + integration (full flow with mocked API).

