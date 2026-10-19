package main

import (
	"fmt"
	"os"
	"strings"
)

// Config holds everything read from the environment. Nothing here is
// persisted — this process is meant to be stateless except for the
// short-lived in-memory image cache (see store.go).
type Config struct {
	Port             string
	LineChannelID    string // the LINE Login/MINI App channel's Channel ID (the LIFF app lives here) — used as the expected "aud" on ID tokens. Deliberately NOT the same channel as LineChannelToken below — see DEPLOYMENT.md.
	LineChannelToken string // channel access token for the separate Messaging API channel, used to call the push endpoint
	PublicBaseURL    string // e.g. https://complaints.example.com (no trailing slash) — must be the publicly reachable HTTPS origin of THIS service
	MaxUploadMB      int64
	MaxPhotos        int
	StaticDir        string
}

func loadConfig() (*Config, error) {
	cfg := &Config{
		Port:             getEnv("PORT", "8080"),
		LineChannelID:    os.Getenv("LINE_CHANNEL_ID"),
		LineChannelToken: os.Getenv("LINE_CHANNEL_ACCESS_TOKEN"),
		PublicBaseURL:    strings.TrimRight(os.Getenv("PUBLIC_BASE_URL"), "/"),
		MaxUploadMB:      10,
		MaxPhotos:        4,
		StaticDir:        getEnv("STATIC_DIR", "./static"),
	}

	var missing []string
	if cfg.LineChannelID == "" {
		missing = append(missing, "LINE_CHANNEL_ID")
	}
	if cfg.LineChannelToken == "" {
		missing = append(missing, "LINE_CHANNEL_ACCESS_TOKEN")
	}
	if cfg.PublicBaseURL == "" {
		missing = append(missing, "PUBLIC_BASE_URL")
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("missing required environment variable(s): %s", strings.Join(missing, ", "))
	}
	if !strings.HasPrefix(cfg.PublicBaseURL, "https://") {
		return nil, fmt.Errorf("PUBLIC_BASE_URL must start with https:// (LINE will not fetch images over plain http) — got %q", cfg.PublicBaseURL)
	}

	return cfg, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
