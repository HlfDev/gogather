package main

import (
	"log"
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

	if cfg.AppleAppID != "" {
		apple := scraper.NewAppleScraper(cfg.AppleAppID, cfg.AppleRegion)
		reviews, err := apple.FetchReviews()
		if err != nil {
			log.Printf("[apple] fetch error: %v", err)
		} else {
			log.Printf("[apple] fetched %d reviews", len(reviews))
			all = append(all, reviews...)
		}
	}

	if cfg.PlayStorePackageName != "" {
		play := scraper.NewPlayStoreScraper(cfg.PlayStorePackageName, cfg.PlayStoreLang, cfg.PlayStoreCountry)
		reviews, err := play.FetchReviews()
		if err != nil {
			log.Printf("[playstore] fetch error: %v", err)
		} else {
			log.Printf("[playstore] fetched %d reviews", len(reviews))
			all = append(all, reviews...)
		}
	}

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
