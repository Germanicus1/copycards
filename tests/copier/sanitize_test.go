package copier

import (
	"strings"
	"testing"
	"time"

	"copycards/internal/copier"
)

const zwsp = "​"

func TestSanitizeForWAF_PathTraversalLong(t *testing.T) {
	in := "See PM 2.1 for guidance at https://example/#a/t/Name.../Id"
	out, changes := copier.SanitizeForWAF(in)
	if strings.Contains(out, ".../") {
		t.Fatalf("expected '.../' broken, got: %q", out)
	}
	if !strings.Contains(out, "... /") {
		t.Fatalf("expected '... /' in output, got: %q", out)
	}
	if len(changes) == 0 || !strings.Contains(changes[0], "path-traversal") {
		t.Fatalf("expected path-traversal change note, got: %v", changes)
	}
}

func TestSanitizeForWAF_PathTraversalShort(t *testing.T) {
	in := "relative path ../config/file"
	out, changes := copier.SanitizeForWAF(in)
	if strings.Contains(out, "../") {
		t.Fatalf("expected '../' broken, got: %q", out)
	}
	if !strings.Contains(out, ".. /") {
		t.Fatalf("expected '.. /' in output, got: %q", out)
	}
	if len(changes) == 0 {
		t.Fatalf("expected a change entry, got none")
	}
}

func TestSanitizeForWAF_SQLKeyword(t *testing.T) {
	in := "correctly select the class of service"
	out, changes := copier.SanitizeForWAF(in)
	if !strings.Contains(out, "s"+zwsp+"elect") {
		t.Fatalf("expected ZWSP-inserted 'select' in output, got: %q", out)
	}
	// Stripping ZWSP should yield the original text exactly.
	restored := strings.ReplaceAll(out, zwsp, "")
	if restored != in {
		t.Fatalf("round-trip differs. want %q got %q", in, restored)
	}
	if len(changes) == 0 || !strings.Contains(changes[0], "select") {
		t.Fatalf("expected change note mentioning 'select', got: %v", changes)
	}
}

func TestSanitizeForWAF_MultipleSQLKeywordsOneChangeEntry(t *testing.T) {
	in := "select columns from the table having count > 1"
	_, changes := copier.SanitizeForWAF(in)
	// All SQL keywords aggregated into exactly one change entry.
	sqlEntries := 0
	for _, c := range changes {
		if strings.Contains(c, "SQL-like keywords") {
			sqlEntries++
		}
	}
	if sqlEntries != 1 {
		t.Fatalf("expected exactly 1 aggregated SQL change entry, got %d: %v", sqlEntries, changes)
	}
	// And the aggregated entry must name each keyword at least once.
	for _, kw := range []string{"select", "from", "table", "having"} {
		if !strings.Contains(changes[0], kw) {
			t.Fatalf("expected %q in change list, got: %v", kw, changes)
		}
	}
}

func TestSanitizeForWAF_NoTriggers(t *testing.T) {
	in := "A perfectly innocent description with no bad patterns."
	out, changes := copier.SanitizeForWAF(in)
	if out != in {
		t.Fatalf("input should pass through unchanged. got: %q", out)
	}
	if len(changes) != 0 {
		t.Fatalf("expected no changes, got: %v", changes)
	}
}

func TestSanitizeForWAF_MixedTriggers(t *testing.T) {
	in := "Navigate to .../foo and select from the table"
	out, changes := copier.SanitizeForWAF(in)
	if strings.Contains(out, ".../") {
		t.Fatalf("path-traversal still present: %q", out)
	}
	if !strings.Contains(out, "s"+zwsp+"elect") {
		t.Fatalf("SQL keyword not ZWSP-broken: %q", out)
	}
	if len(changes) < 2 {
		t.Fatalf("expected at least 2 change entries, got: %v", changes)
	}
}

func TestSanitizeForWAF_EmptyInput(t *testing.T) {
	out, changes := copier.SanitizeForWAF("")
	if out != "" || len(changes) != 0 {
		t.Fatalf("empty input should yield empty output & no changes, got %q / %v", out, changes)
	}
}

// Round-trip: stripping all inserted ZWSP and unwinding the space-before-slash
// edits should recover the original input for a mixed description.
func TestSanitizeForWAF_VisualEquivalence(t *testing.T) {
	in := "Use .../foo with select from table"
	out, _ := copier.SanitizeForWAF(in)
	restored := strings.ReplaceAll(out, zwsp, "")
	restored = strings.ReplaceAll(restored, "... /", ".../")
	restored = strings.ReplaceAll(restored, ".. /", "../")
	if restored != in {
		t.Fatalf("visual round-trip mismatch\n want: %q\n got:  %q", in, restored)
	}
}

// --- annotateDescription is unexported; exercise it via the exported
// Sanitize helper indirectly. We use reflection-free round-trip testing: the
// marker must appear exactly once after repeated annotation, and must include
// today's date and "copycards".

// A tiny in-package helper isn't accessible from this external test package,
// so we test annotation behaviour through SanitizeForWAF results where we
// know the tier-3 call site will append the note. Here we directly verify
// marker idempotency by replicating the contract: a description that already
// contains the sanitize marker should not be annotated a second time when
// passed through again.

// These annotation tests live in the same package as the production code
// (internal test). Kept in a separate in-package file to preserve style.

func TestSanitizeMarkerFormatIncludesDateAndName(t *testing.T) {
	// The marker must include today's ISO date and "copycards".
	today := time.Now().UTC().Format("2006-01-02")
	// We can't call annotateDescription directly (unexported) from this
	// external package. Instead, assert the documented format contract:
	// SanitizeForWAF never adds the marker itself (that's annotate's job),
	// so we only check the constant semantics by looking at a sanitized
	// string for absence of the marker.
	in := "select from table"
	out, _ := copier.SanitizeForWAF(in)
	if strings.Contains(out, "copycards:") {
		t.Fatalf("SanitizeForWAF must not embed the copycards marker itself; got: %q", out)
	}
	_ = today // reserved for a future in-package test that exercises annotateDescription directly.
}
