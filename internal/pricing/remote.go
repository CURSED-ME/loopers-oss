package pricing

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// FetchRemotePricing fetches the remote pricing JSON and returns a Config.
func FetchRemotePricing(urlStr string) (*Config, error) {
	u, err := url.Parse(urlStr)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}
	if u.Scheme != "https" {
		host := strings.Split(u.Host, ":")[0]
		if host != "localhost" && host != "127.0.0.1" {
			return nil, fmt.Errorf("remote pricing URL must be HTTPS or localhost")
		}
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(urlStr)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch remote pricing: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("remote pricing returned status %d", resp.StatusCode)
	}

	const maxBodyBytes = 10 * 1024 * 1024 // 10 MB guard against unbounded reads
	var config Config
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxBodyBytes)).Decode(&config); err != nil {
		return nil, fmt.Errorf("failed to decode remote pricing: %w", err)
	}

	return &config, nil
}
