package scraper

import (
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

type PlayStoreScraper struct {
	PackageName string
	Lang        string
	Country     string
	client      *http.Client
}

func NewPlayStoreScraper(packageName, lang, country string) *PlayStoreScraper {
	return &PlayStoreScraper{
		PackageName: packageName,
		Lang:        lang,
		Country:     country,
		client:      &http.Client{Timeout: 15 * time.Second},
	}
}

var (
	// Matches the data payload inside AF_initDataCallback for key 'ds:11'.
	// Format: AF_initDataCallback({key: 'ds:11', hash: '...', data:[...],...})
	reDs11 = regexp.MustCompile(`AF_initDataCallback\(\{key: 'ds:11'.*?data:([\s\S]+?),\s*sideChannel:`)

	// og:title contains the app name: "AppName – Apps on Google Play"
	reOgTitle = regexp.MustCompile(`property="og:title"\s+content="([^"]+)"`)
)

// FetchReviews fetches the latest reviews by scraping the Play Store app page.
// Reviews are embedded in the page HTML as part of the AF_initDataCallback
// payload for key 'ds:11'. Google returns up to 20 "most relevant" reviews
// per page; results are sorted by date in the caller (main.go).
func (s *PlayStoreScraper) FetchReviews() ([]Review, error) {
	html, err := s.fetchHTML("")
	if err != nil {
		return nil, err
	}
	appName := extractAppName(html, s.PackageName)
	return parsePlayStoreHTML(html, s.PackageName, appName)
}

// fetchPlayStoreAppName fetches the display name of a Play Store app from its HTML page.
// It is shared by both PlayStoreScraper and PlayStoreAPIScraper.
func fetchPlayStoreAppName(packageName, lang, country string, client *http.Client) string {
	pageURL := fmt.Sprintf(
		"https://play.google.com/store/apps/details?id=%s&hl=%s&gl=%s",
		packageName, lang, country,
	)
	req, err := http.NewRequest("GET", pageURL, nil)
	if err != nil {
		return packageName
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept-Language", lang)

	resp, err := client.Do(req)
	if err != nil {
		return packageName
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return packageName
	}
	return extractAppName(string(body), packageName)
}

// fetchHTML fetches the Play Store app page, optionally with extra query params.
func (s *PlayStoreScraper) fetchHTML(extraParams string) (string, error) {
	pageURL := fmt.Sprintf(
		"https://play.google.com/store/apps/details?id=%s&hl=%s&gl=%s%s",
		s.PackageName, s.Lang, s.Country, extraParams,
	)

	req, err := http.NewRequest("GET", pageURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept-Language", s.Lang)

	resp, err := s.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("http get: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status %d for %s", resp.StatusCode, s.PackageName)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

// extractAppName reads the app name from the og:title meta tag.
// Typical format: "Dafiti: Shopping no seu Bolso – Apps on Google Play"
func extractAppName(html, fallback string) string {
	if m := reOgTitle.FindStringSubmatch(html); len(m) > 1 {
		name := m[1]
		// Strip trailing store suffix (both " - " and " – " separators)
		for _, sep := range []string{" – ", " - "} {
			if idx := strings.LastIndex(name, sep); idx > 0 {
				name = name[:idx]
				break
			}
		}
		if name = strings.TrimSpace(name); name != "" {
			return name
		}
	}
	return fallback
}
