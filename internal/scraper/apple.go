package scraper

// Apple App Store uses the public RSS/JSON feed:
// https://itunes.apple.com/{region}/rss/customerreviews/page=1/id={appID}/sortby=mostrecent/json

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
)

type AppleScraper struct {
	AppID          string
	Region         string
	cachedAppName  string
	client         *http.Client
}

func NewAppleScraper(appID, region string) *AppleScraper {
	return &AppleScraper{
		AppID:  appID,
		Region: region,
		client: &http.Client{Timeout: 15 * time.Second},
	}
}

type appleRSSResponse struct {
	Feed struct {
		Entry []appleEntry `json:"entry"`
	} `json:"feed"`
}

type appleEntry struct {
	ID struct {
		Label string `json:"label"`
	} `json:"id"`
	Author struct {
		Name struct {
			Label string `json:"label"`
		} `json:"name"`
	} `json:"author"`
	ImRating struct {
		Label string `json:"label"`
	} `json:"im:rating"`
	ImVersion struct {
		Label string `json:"label"`
	} `json:"im:version"`
	Title struct {
		Label string `json:"label"`
	} `json:"title"`
	Content struct {
		Label string `json:"label"`
	} `json:"content"`
	Updated struct {
		Label string `json:"label"`
	} `json:"updated"`
}

// FetchReviews fetches up to 100 most recent reviews (pages 1 and 2).
// The app name is resolved once via the iTunes Lookup API and cached.
func (s *AppleScraper) FetchReviews() ([]Review, error) {
	if s.cachedAppName == "" {
		name, err := s.lookupAppName()
		if err == nil {
			s.cachedAppName = name
		}
	}

	var all []Review
	for page := 1; page <= 2; page++ {
		reviews, err := s.fetchPage(page)
		if err != nil {
			return nil, err
		}
		all = append(all, reviews...)
	}
	return all, nil
}

// lookupAppName fetches the app name from the iTunes Lookup API.
// https://itunes.apple.com/lookup?id={appID}&country={region}
func (s *AppleScraper) lookupAppName() (string, error) {
	url := fmt.Sprintf("https://itunes.apple.com/lookup?id=%s&country=%s", s.AppID, s.Region)
	resp, err := s.client.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var result struct {
		Results []struct {
			TrackName string `json:"trackName"`
		} `json:"results"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	if len(result.Results) == 0 || result.Results[0].TrackName == "" {
		return "", fmt.Errorf("trackName not found")
	}
	return result.Results[0].TrackName, nil
}

func (s *AppleScraper) fetchPage(page int) ([]Review, error) {
	url := fmt.Sprintf(
		"https://itunes.apple.com/%s/rss/customerreviews/page=%d/id=%s/sortby=mostrecent/json",
		s.Region, page, s.AppID,
	)

	resp, err := s.client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("apple fetch: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("apple read body: %w", err)
	}

	var rss appleRSSResponse
	if err := json.Unmarshal(body, &rss); err != nil {
		return nil, fmt.Errorf("apple parse: %w", err)
	}

	appName := s.cachedAppName

	var reviews []Review
	for _, e := range rss.Feed.Entry {
		rating, _ := strconv.Atoi(e.ImRating.Label)
		// Entries with no rating are app metadata entries, not reviews — skip them.
		if rating == 0 && e.ImRating.Label == "" {
			continue
		}
		date, _ := time.Parse(time.RFC3339, e.Updated.Label)

		reviews = append(reviews, Review{
			ID:      e.ID.Label,
			Source:  SourceApple,
			AppName: appName,
			Author:  e.Author.Name.Label,
			Rating:  rating,
			Title:   e.Title.Label,
			Body:    e.Content.Label,
			Date:    date,
			Version: e.ImVersion.Label,
		})
	}
	return reviews, nil
}
