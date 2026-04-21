package config

import (
	"encoding/json"
	"fmt"
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
