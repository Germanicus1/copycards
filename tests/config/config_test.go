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
