package copier

import (
	"strings"
	"testing"
	"time"
)

func TestAnnotateDescription_AddsMarker(t *testing.T) {
	in := "Plain description body."
	out := annotateDescription(in)
	if !strings.Contains(out, sanitizeMarker) {
		t.Fatalf("expected marker in output, got: %q", out)
	}
	if !strings.Contains(out, "copycards") {
		t.Fatalf("expected 'copycards' in marker, got: %q", out)
	}
	today := time.Now().UTC().Format("2006-01-02")
	if !strings.Contains(out, today) {
		t.Fatalf("expected today's date %q in marker, got: %q", today, out)
	}
	if !strings.HasPrefix(out, in) {
		t.Fatalf("original description must remain as prefix, got: %q", out)
	}
}

func TestAnnotateDescription_Idempotent(t *testing.T) {
	in := "Body."
	once := annotateDescription(in)
	twice := annotateDescription(once)
	if once != twice {
		t.Fatalf("annotation must be idempotent.\nfirst:  %q\nsecond: %q", once, twice)
	}
}

func TestSanitizeDescription_StringInput(t *testing.T) {
	result, changes := sanitizeDescription("text with select keyword")
	if len(changes) == 0 {
		t.Fatalf("expected changes, got none")
	}
	s, ok := result.(string)
	if !ok {
		t.Fatalf("expected string result, got %T", result)
	}
	if !strings.Contains(s, "s"+zwsp+"elect") {
		t.Fatalf("expected ZWSP-inserted 'select', got: %q", s)
	}
}

func TestSanitizeDescription_NonStringPassThrough(t *testing.T) {
	obj := map[string]interface{}{"type": "doc", "content": "anything"}
	result, changes := sanitizeDescription(obj)
	if len(changes) != 0 {
		t.Fatalf("expected no changes for non-string desc, got: %v", changes)
	}
	if _, ok := result.(map[string]interface{}); !ok {
		t.Fatalf("expected map pass-through, got: %T", result)
	}
}

func TestSanitizeDescription_StringWithoutTriggers(t *testing.T) {
	result, changes := sanitizeDescription("nothing fancy here")
	if len(changes) != 0 {
		t.Fatalf("expected no changes, got: %v", changes)
	}
	if result != "nothing fancy here" {
		t.Fatalf("expected unchanged string, got: %v", result)
	}
}
