package main

import (
	"log"
	"sort"
	"time"

	"github.com/hlfdev/gogather/config"
	"github.com/hlfdev/gogather/internal/notifier"
	"github.com/hlfdev/gogather/internal/scraper"
	"github.com/hlfdev/gogather/internal/store"
)

func main() {
	cfg := config.Load()

	seen, err := store.New("seen_reviews.json")
	if err != nil {
		log.Fatalf("failed to load seen store: %v", err)
	}

	slack := notifier.NewSlack(cfg.SlackWebhookURL)

	if cfg.PollInterval == 0 {
		// Single-run mode: used by GitHub Actions / cron jobs.
		poll(cfg, seen, slack)
		return
	}

	log.Printf("gogather started — polling every %s", cfg.PollInterval)
	for {
		poll(cfg, seen, slack)
		time.Sleep(cfg.PollInterval)
	}
}

func poll(cfg *config.Config, seen *store.SeenStore, slack *notifier.SlackNotifier) {
	var all []scraper.Review

	if cfg.AppleEnabled && cfg.AppleAppID != "" {
		apple := scraper.NewAppleScraper(cfg.AppleAppID, cfg.AppleRegion)
		reviews, err := apple.FetchReviews()
		if err != nil {
			log.Printf("[apple] fetch error: %v", err)
		} else {
			log.Printf("[apple] fetched %d reviews", len(reviews))
			all = append(all, reviews...)
		}
	}

	if cfg.PlayStoreEnabled && cfg.PlayStorePackageName != "" {
		reviews, err := fetchPlayStoreReviews(cfg)
		if err != nil {
			log.Printf("[playstore] fetch error: %v", err)
		} else {
			log.Printf("[playstore] fetched %d reviews", len(reviews))
			all = append(all, reviews...)
		}
	}

	// Drop reviews older than MaxReviewAge (avoids old "most-relevant" results
	// from the Play Store HTML scraper polluting the Slack channel).
	if cfg.MaxReviewAge > 0 {
		cutoff := time.Now().Add(-cfg.MaxReviewAge)
		filtered := all[:0]
		skipped := 0
		for _, r := range all {
			if !r.Date.IsZero() && r.Date.Before(cutoff) {
				skipped++
				continue
			}
			filtered = append(filtered, r)
		}
		if skipped > 0 {
			log.Printf("filtered %d reviews older than %s", skipped, cfg.MaxReviewAge.String())
		}
		all = filtered
	}

	// Send oldest reviews first so Slack channel reads chronologically.
	sort.Slice(all, func(i, j int) bool {
		return all[i].Date.Before(all[j].Date)
	})

	newCount := 0
	for _, r := range all {
		if r.ID == "" || seen.IsSeen(r.ID) {
			continue
		}
		if err := slack.Send(r); err != nil {
			log.Printf("[slack] failed to send review %s: %v", r.ID, err)
			continue
		}
		if err := seen.MarkSeen(r.ID); err != nil {
			log.Printf("[store] failed to mark review %s as seen: %v", r.ID, err)
		}
		newCount++
	}

	log.Printf("poll done — %d new reviews sent to Slack", newCount)
}

// fetchPlayStoreReviews fetches reviews using the Play Store internal batchexecute RPC.
// No API key required — reviews are sorted newest-first.
func fetchPlayStoreReviews(cfg *config.Config) ([]scraper.Review, error) {
	s := scraper.NewPlayStoreScraper(cfg.PlayStorePackageName, cfg.PlayStoreLang, cfg.PlayStoreCountry)
	return s.FetchReviews()
}
