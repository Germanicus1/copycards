package copier

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"
)

// sqlKeywords are words that AWS WAF's SQL injection rule sets commonly flag.
// When they appear densely in natural English text (as in knowledge-base
// descriptions), the rules still fire even though no injection is happening.
// Inserting a zero-width space between the first two letters of each match
// breaks the pattern at the byte level while remaining invisible to readers.
var sqlKeywords = []string{
	"select", "from", "where", "union", "drop", "insert", "update",
	"delete", "table", "having", "join", "alter", "exec",
}

// zwsp is the Unicode ZERO WIDTH SPACE, U+200B.
const zwsp = "​"

// sanitizeMarker is how we detect that a description has already been
// annotated by a previous sanitize run, so annotateDescription stays
// idempotent.
const sanitizeMarker = "[copycards: description sanitized"

var (
	// pathTraversalLong matches ".../" (three or more dots followed by slash).
	pathTraversalLong = regexp.MustCompile(`\.{3,}/`)
	// pathTraversalShort matches "../" (exactly two dots followed by slash).
	// Must run after the long form so longer matches aren't split.
	pathTraversalShort = regexp.MustCompile(`\.\.\/`)
	// sqlKeywordPattern matches any of sqlKeywords as a whole word,
	// case-insensitive.
	sqlKeywordPattern = buildSQLPattern()
)

func buildSQLPattern() *regexp.Regexp {
	parts := make([]string, len(sqlKeywords))
	for i, kw := range sqlKeywords {
		parts[i] = regexp.QuoteMeta(kw)
	}
	return regexp.MustCompile(`(?i)\b(` + strings.Join(parts, "|") + `)\b`)
}

// SanitizeForWAF applies byte-level transformations that bypass common
// CloudFront/AWS WAF managed rule sets while preserving visible meaning.
// Returns the transformed string and a list of human-readable change
// descriptions (empty list means nothing matched).
func SanitizeForWAF(s string) (string, []string) {
	if s == "" {
		return s, nil
	}

	var changes []string
	out := s

	// ".../" must come before "../" so overlapping matches are handled
	// correctly (".../" would otherwise be consumed by the shorter rule first
	// and leave a stray dot behind).
	if n := len(pathTraversalLong.FindAllString(out, -1)); n > 0 {
		out = pathTraversalLong.ReplaceAllStringFunc(out, func(match string) string {
			// Preserve every dot and put a space before the slash.
			return strings.TrimSuffix(match, "/") + " /"
		})
		changes = append(changes, fmt.Sprintf("broke '.../' path-traversal pattern (%d occurrence%s)", n, plural(n)))
	}

	if n := len(pathTraversalShort.FindAllString(out, -1)); n > 0 {
		out = pathTraversalShort.ReplaceAllString(out, ".. /")
		changes = append(changes, fmt.Sprintf("broke '../' path-traversal pattern (%d occurrence%s)", n, plural(n)))
	}

	// Collect unique SQL keywords that actually matched, then mutate.
	matchedKeywords := map[string]struct{}{}
	for _, m := range sqlKeywordPattern.FindAllString(out, -1) {
		matchedKeywords[strings.ToLower(m)] = struct{}{}
	}
	if len(matchedKeywords) > 0 {
		out = sqlKeywordPattern.ReplaceAllStringFunc(out, func(match string) string {
			if len(match) < 2 {
				return match
			}
			return match[:1] + zwsp + match[1:]
		})
		changes = append(changes, "zero-width spaces inserted into SQL-like keywords: "+formatKeywords(matchedKeywords))
	}

	return out, changes
}

// sanitizeDescription handles Ticket.Description's interface{} type.
// If desc is a string it is sanitized; any other concrete type is returned
// unchanged with no recorded changes.
func sanitizeDescription(desc interface{}) (interface{}, []string) {
	s, ok := desc.(string)
	if !ok {
		return desc, nil
	}
	out, changes := SanitizeForWAF(s)
	if len(changes) == 0 {
		return desc, nil
	}
	return out, changes
}

// annotateDescription appends an audit note to desc. The note is deliberately
// short and does not list specific trigger keywords, so it can't itself trip
// the WAF we're trying to bypass. Idempotent — a description that already
// carries the marker is returned unchanged.
func annotateDescription(desc string) string {
	if strings.Contains(desc, sanitizeMarker) {
		return desc
	}
	note := fmt.Sprintf(
		"\n\n---\n[copycards: description sanitized on %s to bypass WAF filters. Minor byte-level changes to the text above; visual appearance preserved.]",
		time.Now().UTC().Format("2006-01-02"),
	)
	return desc + note
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

func formatKeywords(set map[string]struct{}) string {
	keys := make([]string, 0, len(set))
	for k := range set {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return strings.Join(keys, ", ")
}
