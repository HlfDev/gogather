package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	SlackWebhookURL string
	PollInterval    time.Duration

	// Apple App Store
	AppleAppID  string
	AppleRegion string // e.g. "br", "us"

	// Google Play Store
	PlayStorePackageName string
	PlayStoreLang        string // e.g. "pt", "en"
	PlayStoreCountry     string // e.g. "br", "us"
}

func Load() *Config {
	intervalSec := getEnvInt("POLL_INTERVAL_SECONDS", 3600)

	return &Config{
		SlackWebhookURL:      requireEnv("SLACK_WEBHOOK_URL"),
		PollInterval:         time.Duration(intervalSec) * time.Second,
		AppleAppID:  os.Getenv("APPLE_APP_ID"),
		AppleRegion: getEnvDefault("APPLE_REGION", "br"),
		PlayStorePackageName: os.Getenv("PLAY_STORE_PACKAGE"),
		PlayStoreLang:        getEnvDefault("PLAY_STORE_LANG", "pt"),
		PlayStoreCountry:     getEnvDefault("PLAY_STORE_COUNTRY", "br"),
	}
}

func requireEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		panic("missing required env var: " + key)
	}
	return v
}

func getEnvDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getEnvInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}
