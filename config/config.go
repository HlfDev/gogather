package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	SlackWebhookURL string
	PollInterval    time.Duration
	MaxReviewAge    time.Duration // reviews older than this are ignored (0 = no limit)

	// Apple App Store
	AppleEnabled bool
	AppleAppID   string
	AppleRegion  string // e.g. "br", "us"

	// Google Play Store
	PlayStoreEnabled        bool
	PlayStorePackageName    string
	PlayStoreLang           string // e.g. "pt", "en"
	PlayStoreCountry        string // e.g. "br", "us"
	PlayStoreCredentialsJSON string // service account JSON key (enables Developer API)
}

func Load() *Config {
	intervalSec := getEnvInt("POLL_INTERVAL_SECONDS", 3600)
	maxAgeDays := getEnvInt("MAX_REVIEW_AGE_DAYS", 90)

	return &Config{
		SlackWebhookURL:      requireEnv("SLACK_WEBHOOK_URL"),
		PollInterval:         time.Duration(intervalSec) * time.Second,
		MaxReviewAge:         time.Duration(maxAgeDays) * 24 * time.Hour,
		AppleEnabled:         getEnvBool("APPLE_ENABLED", true),
		AppleAppID:           os.Getenv("APPLE_APP_ID"),
		AppleRegion:          getEnvDefault("APPLE_REGION", "br"),
		PlayStoreEnabled:         getEnvBool("PLAY_STORE_ENABLED", true),
		PlayStorePackageName:     os.Getenv("PLAY_STORE_PACKAGE"),
		PlayStoreLang:            getEnvDefault("PLAY_STORE_LANG", "pt"),
		PlayStoreCountry:         getEnvDefault("PLAY_STORE_COUNTRY", "br"),
		PlayStoreCredentialsJSON: os.Getenv("PLAY_STORE_CREDENTIALS_JSON"),
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

func getEnvBool(key string, def bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	return v == "1" || v == "true" || v == "yes"
}
