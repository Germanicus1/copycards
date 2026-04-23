package copier

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"copycards/internal/copier"
	"copycards/internal/fbclient"
)

// preflightFixture describes one mock Flowboards server for a preflight test.
// Each field is a JSON body returned when the matching path is hit. Paths
// without a fixture respond with 404.
type preflightFixture struct {
	boardID      string
	boardJSON    string
	binsJSON     string
	ticketsJSON  string
	typesJSON    string
	fieldsJSON   string
	usersJSON    string
}

func newMockServer(t *testing.T, fx preflightFixture) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/boards/" + fx.boardID:
			w.Write([]byte(fx.boardJSON))
		case "/bins":
			w.Write([]byte(fx.binsJSON))
		case "/tickets":
			w.Write([]byte(fx.ticketsJSON))
		case "/ticket-types":
			w.Write([]byte(fx.typesJSON))
		case "/custom-fields":
			w.Write([]byte(fx.fieldsJSON))
		case "/users":
			w.Write([]byte(fx.usersJSON))
		default:
			http.NotFound(w, r)
		}
	}))
}

// identicalPair builds src and dst fixtures that only differ in their IDs —
// names match across the pair, so preflight considers them compatible. Tests
// then mutate one or the other to introduce a specific mismatch.
func identicalPair() (preflightFixture, preflightFixture) {
	src := preflightFixture{
		boardID:     "board1",
		boardJSON:   `{"_id":"board1","name":"Board","bins":["bin1"]}`,
		binsJSON:    `[{"_id":"bin1","name":"Backlog"}]`,
		ticketsJSON: `[{"_id":"ticket1","name":"Task","bin_id":"bin1","ticketType_id":"type1","order":1,"customFields":{"field1":"3"}}]`,
		typesJSON:   `[{"_id":"type1","name":"Story"}]`,
		fieldsJSON:  `[{"_id":"field1","name":"Points","type":3}]`,
		usersJSON:   `[{"_id":"user1","email":"alice@test.com","name":"Alice"}]`,
	}
	dst := preflightFixture{
		boardID:     "board2",
		boardJSON:   `{"_id":"board2","name":"Board","bins":["bin2"]}`,
		binsJSON:    `[{"_id":"bin2","name":"Backlog"}]`,
		ticketsJSON: `[]`,
		typesJSON:   `[{"_id":"type2","name":"Story"}]`,
		fieldsJSON:  `[{"_id":"field2","name":"Points","type":3}]`,
		usersJSON:   `[{"_id":"user2","email":"alice@test.com","name":"Alice"}]`,
	}
	return src, dst
}

func runPreflight(t *testing.T, src, dst preflightFixture) *copier.PreflightResult {
	t.Helper()
	srcSrv := newMockServer(t, src)
	defer srcSrv.Close()
	dstSrv := newMockServer(t, dst)
	defer dstSrv.Close()

	pf, err := copier.Preflight(
		fbclient.NewClient(srcSrv.URL, "key"),
		fbclient.NewClient(dstSrv.URL, "key"),
		src.boardID, dst.boardID,
	)
	if err != nil {
		t.Fatalf("Preflight returned error: %v", err)
	}
	return pf
}

func TestPreflightIdenticalBoards(t *testing.T) {
	src, dst := identicalPair()
	pf := runPreflight(t, src, dst)

	if !pf.Valid {
		t.Fatalf("Expected valid boards, got errors:\n%s", pf.FormatErrors())
	}
	checkMapping := func(name string, got map[string]string, want map[string]string) {
		for k, v := range want {
			if got[k] != v {
				t.Errorf("%s: expected %s→%s, got %v", name, k, v, got)
			}
		}
	}
	checkMapping("Bin", pf.BinMapping, map[string]string{"bin1": "bin2"})
	checkMapping("TicketType", pf.TicketTypeMapping, map[string]string{"type1": "type2"})
	checkMapping("CustomField", pf.CustomFieldMapping, map[string]string{"field1": "field2"})
	checkMapping("User", pf.UserMapping, map[string]string{"user1": "user2"})
}

func TestPreflightMissingBin(t *testing.T) {
	src, dst := identicalPair()
	// dst board has a different-named bin; the src bin "Backlog" has no dst match.
	dst.binsJSON = `[{"_id":"bin2","name":"Review"}]`

	pf := runPreflight(t, src, dst)
	if pf.Valid {
		t.Fatalf("Expected invalid boards, got valid")
	}
	if len(pf.MissingBins) != 1 || pf.MissingBins[0].Name != "Backlog" {
		t.Fatalf("Expected MissingBins=[Backlog], got %v", pf.MissingBins)
	}
	if pf.MissingBins[0].Details != "not found in dst org" {
		t.Errorf("Expected Details 'not found in dst org', got %q", pf.MissingBins[0].Details)
	}
}

func TestPreflightBinExistsInOrgButNotOnBoard(t *testing.T) {
	src, dst := identicalPair()
	// dst org has a bin named Backlog, but the board's bin list doesn't include it.
	// dst board references a different bin (Review).
	dst.boardJSON = `{"_id":"board2","name":"Board","bins":["bin3"]}`
	dst.binsJSON = `[{"_id":"bin2","name":"Backlog"},{"_id":"bin3","name":"Review"}]`

	pf := runPreflight(t, src, dst)
	if pf.Valid {
		t.Fatalf("Expected invalid boards, got valid")
	}
	if len(pf.MissingBins) != 1 || pf.MissingBins[0].Name != "Backlog" {
		t.Fatalf("Expected MissingBins=[Backlog], got %v", pf.MissingBins)
	}
	if pf.MissingBins[0].Details != "exists in dst org but not on this board" {
		t.Errorf("Expected 'exists in dst org but not on this board' details, got %q", pf.MissingBins[0].Details)
	}
}

func TestPreflightMissingTicketType(t *testing.T) {
	src, dst := identicalPair()
	dst.typesJSON = `[{"_id":"type2","name":"Bug"}]` // name differs

	pf := runPreflight(t, src, dst)
	if pf.Valid {
		t.Fatalf("Expected invalid boards, got valid")
	}
	if len(pf.MissingTypes) != 1 || pf.MissingTypes[0].Name != "Story" {
		t.Fatalf("Expected MissingTypes=[Story], got %v", pf.MissingTypes)
	}
}

func TestPreflightUnusedTicketTypeIgnored(t *testing.T) {
	// Src has an unused type that's missing from dst — preflight must ignore
	// it because no ticket references it.
	src, dst := identicalPair()
	src.typesJSON = `[{"_id":"type1","name":"Story"},{"_id":"type99","name":"UnusedEpic"}]`

	pf := runPreflight(t, src, dst)
	if !pf.Valid {
		t.Fatalf("Unused types should not cause incompatibility. Errors:\n%s", pf.FormatErrors())
	}
}

func TestPreflightMissingCustomField(t *testing.T) {
	src, dst := identicalPair()
	dst.fieldsJSON = `[{"_id":"field2","name":"Priority","type":3}]` // name differs

	pf := runPreflight(t, src, dst)
	if pf.Valid {
		t.Fatalf("Expected invalid boards, got valid")
	}
	if len(pf.MissingFields) != 1 || pf.MissingFields[0].Name != "Points" {
		t.Fatalf("Expected MissingFields=[Points], got %v", pf.MissingFields)
	}
}

func TestPreflightCustomFieldTypeMismatch(t *testing.T) {
	// Same-named custom field but different type — preflight's composite key
	// means it counts as missing.
	src, dst := identicalPair()
	dst.fieldsJSON = `[{"_id":"field2","name":"Points","type":1}]` // type 1 vs src type 3

	pf := runPreflight(t, src, dst)
	if pf.Valid {
		t.Fatalf("Expected invalid boards, got valid")
	}
	if len(pf.MissingFields) != 1 {
		t.Fatalf("Expected 1 missing field, got %v", pf.MissingFields)
	}
	if pf.MissingFields[0].Details != fmt.Sprintf("type %d", 3) {
		t.Errorf("Expected 'type 3' details, got %q", pf.MissingFields[0].Details)
	}
}
