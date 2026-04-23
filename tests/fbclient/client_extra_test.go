package fbclient

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"copycards/internal/fbclient"
)

// Helpers ---------------------------------------------------------------

// simpleJSON returns a server that responds with status 200 + body for the
// exact path, and 404 otherwise. Path matches the request's r.URL.Path.
func simpleJSON(t *testing.T, path, body string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != path {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(body))
	}))
}

// Read-path smoke tests --------------------------------------------------

func TestGetBin(t *testing.T) {
	srv := simpleJSON(t, "/bins/bin1", `{"_id":"bin1","name":"Backlog"}`)
	defer srv.Close()
	c := fbclient.NewClient(srv.URL, "key")

	bin, err := c.GetBin("bin1")
	if err != nil {
		t.Fatalf("GetBin: %v", err)
	}
	if bin.ID != "bin1" || bin.Name != "Backlog" {
		t.Errorf("unexpected bin: %+v", bin)
	}
}

func TestListBinsSinglePage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`[{"_id":"bin1","name":"Backlog"},{"_id":"bin2","name":"Done"}]`))
	}))
	defer srv.Close()
	c := fbclient.NewClient(srv.URL, "key")

	bins, err := c.ListBins()
	if err != nil {
		t.Fatalf("ListBins: %v", err)
	}
	if len(bins) != 2 {
		t.Fatalf("expected 2 bins, got %d", len(bins))
	}
}

// Pagination: server returns a `page-token` header on page 1 and nothing on
// page 2. ListBins must follow the header, preserve order, and stop when the
// header is empty.
func TestListBinsPagination(t *testing.T) {
	pagesServed := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		pagesServed++
		if r.URL.Query().Get("page-token") == "" {
			w.Header().Set("page-token", "PAGE2")
			w.Write([]byte(`[{"_id":"bin1","name":"A"}]`))
			return
		}
		// Page 2
		w.Write([]byte(`[{"_id":"bin2","name":"B"}]`))
	}))
	defer srv.Close()
	c := fbclient.NewClient(srv.URL, "key")

	bins, err := c.ListBins()
	if err != nil {
		t.Fatalf("ListBins: %v", err)
	}
	if len(bins) != 2 {
		t.Fatalf("expected 2 bins across pages, got %d: %+v", len(bins), bins)
	}
	if bins[0].ID != "bin1" || bins[1].ID != "bin2" {
		t.Errorf("order lost: %+v", bins)
	}
	if pagesServed != 2 {
		t.Errorf("expected 2 page requests, got %d", pagesServed)
	}
}

func TestGetTicket(t *testing.T) {
	srv := simpleJSON(t, "/tickets/t1", `{"_id":"t1","name":"Task","bin_id":"b1","order":1}`)
	defer srv.Close()
	c := fbclient.NewClient(srv.URL, "key")

	ticket, err := c.GetTicket("t1")
	if err != nil {
		t.Fatalf("GetTicket: %v", err)
	}
	if ticket.ID != "t1" || ticket.Name != "Task" || ticket.BinID != "b1" {
		t.Errorf("unexpected ticket: %+v", ticket)
	}
}

func TestListTicketsByBin(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("bin_id") != "b1" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`[{"_id":"t1","name":"A","bin_id":"b1","order":1}]`))
	}))
	defer srv.Close()
	c := fbclient.NewClient(srv.URL, "key")

	tickets, err := c.ListTicketsByBin("b1")
	if err != nil {
		t.Fatalf("ListTicketsByBin: %v", err)
	}
	if len(tickets) != 1 || tickets[0].ID != "t1" {
		t.Errorf("unexpected tickets: %+v", tickets)
	}
}

func TestListTicketsByParent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("parent_id") != "p1" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`[{"_id":"c1","name":"child","bin_id":"b1","order":1}]`))
	}))
	defer srv.Close()
	c := fbclient.NewClient(srv.URL, "key")

	children, err := c.ListTicketsByParent("p1")
	if err != nil {
		t.Fatalf("ListTicketsByParent: %v", err)
	}
	if len(children) != 1 || children[0].ID != "c1" {
		t.Errorf("unexpected children: %+v", children)
	}
}

func TestListUsers(t *testing.T) {
	srv := simpleJSON(t, "/users", `[{"_id":"u1","email":"a@b.c","name":"A"}]`)
	defer srv.Close()
	c := fbclient.NewClient(srv.URL, "key")

	users, err := c.ListUsers()
	if err != nil {
		t.Fatalf("ListUsers: %v", err)
	}
	if len(users) != 1 || users[0].Email != "a@b.c" {
		t.Errorf("unexpected users: %+v", users)
	}
}

func TestGetIDs(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// The client requests /ids?max-results=N — verify the N.
		if got := r.URL.Query().Get("max-results"); got != "3" {
			t.Errorf("expected max-results=3, got %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`["id-a","id-b","id-c"]`))
	}))
	defer srv.Close()
	c := fbclient.NewClient(srv.URL, "key")

	ids, err := c.GetIDs(3)
	if err != nil {
		t.Fatalf("GetIDs: %v", err)
	}
	if len(ids) != 3 || ids[0] != "id-a" {
		t.Errorf("unexpected ids: %+v", ids)
	}
}

// Resilience tests ------------------------------------------------------

// CloudFront 403 returns HTML with "Request blocked". After retries the
// client must surface ErrCloudFrontBlocked so callers can branch on it.
func TestCloudFrontBlockedSentinel(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(`<html><body>Request blocked. CloudFront</body></html>`))
	}))
	defer srv.Close()
	c := fbclient.NewClient(srv.URL, "key")

	_, err := c.ListBoards()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, fbclient.ErrCloudFrontBlocked) {
		t.Errorf("expected ErrCloudFrontBlocked, got %v", err)
	}
}

// Plain (non-CloudFront) 403 returns JSON without the "CloudFront"/"Request
// blocked" markers. The classifier should NOT retry (it's not a transient
// WAF block) and the error must NOT carry ErrCloudFrontBlocked.
func TestPlain403NotRetriedNotCloudFront(t *testing.T) {
	attempts := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(`{"error":"permission denied"}`))
	}))
	defer srv.Close()
	c := fbclient.NewClient(srv.URL, "key")

	_, err := c.ListBoards()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if errors.Is(err, fbclient.ErrCloudFrontBlocked) {
		t.Errorf("plain 403 should not surface CloudFront sentinel, got %v", err)
	}
	if attempts > 1 {
		t.Errorf("plain 403 should not be retried, got %d attempts", attempts)
	}
}

// dumpFailedBody is best-effort and unexported. Verify its observable side
// effect: a failing POST writes <ts>-POST-<suffix>.json and matching
// .error.txt under $HOME/.copycard/failed-posts/.
func TestDumpFailedBodyOnPostFailure(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()
	c := fbclient.NewClient(srv.URL, "key")

	err := c.CreateTicket(&fbclient.Ticket{
		ID: "t1", Name: "x", BinID: "b1",
	})
	if err == nil {
		t.Fatal("expected error from failing POST")
	}

	dir := filepath.Join(home, ".copycard", "failed-posts")
	entries, readErr := os.ReadDir(dir)
	if readErr != nil {
		t.Fatalf("reading %s: %v", dir, readErr)
	}
	hasJSON, hasErr := false, false
	for _, e := range entries {
		name := e.Name()
		if strings.Contains(name, "POST-t1") && strings.HasSuffix(name, ".json") {
			hasJSON = true
		}
		if strings.Contains(name, "POST-t1") && strings.HasSuffix(name, ".error.txt") {
			hasErr = true
		}
	}
	if !hasJSON {
		t.Errorf("expected a POST-t1-*.json dump, entries: %v", entries)
	}
	if !hasErr {
		t.Errorf("expected a POST-t1-*.error.txt dump, entries: %v", entries)
	}
}
