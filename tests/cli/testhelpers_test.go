package cli_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// withTempHome creates a fresh temporary directory and points $HOME at it
// for the duration of the test. The value is automatically restored when
// the test ends (via t.Setenv). Returns the temp-home path.
func withTempHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	return home
}

// writeTestConfig writes a minimal config.toml under
// home/.config/copycards/config.toml. If srcURL is non-empty an [orgs.src]
// section is included; same for dstURL. Pass "" to omit either. When both
// URLs are set, default_from/default_to are set to "src"/"dst".
func writeTestConfig(t *testing.T, home, srcURL, dstURL string) {
	t.Helper()
	configDir := filepath.Join(home, ".config", "copycards")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}

	var b strings.Builder
	if srcURL != "" && dstURL != "" {
		b.WriteString("default_from = \"src\"\ndefault_to = \"dst\"\n\n")
	}
	if srcURL != "" {
		fmt.Fprintf(&b, "[orgs.src]\norg_id = \"src-org\"\napi_key = \"src-key\"\nendpoint = %q\n\n", srcURL)
	}
	if dstURL != "" {
		fmt.Fprintf(&b, "[orgs.dst]\norg_id = \"dst-org\"\napi_key = \"dst-key\"\nendpoint = %q\n", dstURL)
	}

	path := filepath.Join(configDir, "config.toml")
	if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
}
