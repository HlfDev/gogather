package scraper

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

const (
	batchExecuteURL = "https://play.google.com/_/PlayStoreUi/data/batchexecute"
	reviewsPerFetch = 100
)

// reOgTitle matches the og:title meta tag: "AppName – Apps on Google Play"
var reOgTitle = regexp.MustCompile(`property="og:title"\s+content="([^"]+)"`)

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

// FetchReviews fetches recent reviews via the Play Store internal batchexecute RPC.
// Reviews are sorted newest-first (sort=2). No API key required.
func (s *PlayStoreScraper) FetchReviews() ([]Review, error) {
	appName := fetchPlayStoreAppName(s.PackageName, s.Lang, s.Country, s.client)

	apiURL := fmt.Sprintf("%s?hl=%s&gl=%s", batchExecuteURL, url.QueryEscape(s.Lang), url.QueryEscape(s.Country))
	body := s.buildFreqBody("")

	req, err := http.NewRequest("POST", apiURL, strings.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded;charset=UTF-8")
	req.Header.Set("Origin", "https://play.google.com")
	req.Header.Set("Referer", "https://play.google.com/")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http post batchexecute: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d for %s", resp.StatusCode, s.PackageName)
	}

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return parseBatchExecuteResponse(raw, s.PackageName, appName)
}

// buildFreqBody constructs the f.req POST body for the batchexecute RPC.
// sort=2 = newest first. pageToken is empty for the first page.
func (s *PlayStoreScraper) buildFreqBody(pageToken string) string {
	var innerPayload string
	if pageToken == "" {
		innerPayload = fmt.Sprintf(
			`[null,[2,2,[%d],null,[null,null,null,null,null,null,null,null,null]],["%s",7]]`,
			reviewsPerFetch, s.PackageName,
		)
	} else {
		innerPayload = fmt.Sprintf(
			`[null,[2,2,[%d,null,"%s"],null,[null,null,null,null,null,null,null,null,null]],["%s",7]]`,
			reviewsPerFetch, pageToken, s.PackageName,
		)
	}

	// JSON-encode the inner payload so it becomes a proper JSON string value.
	encodedInner, _ := json.Marshal(innerPayload)
	freqJSON := fmt.Sprintf(`[[["oCPfdb",%s,null,"generic"]]]`, encodedInner)

	return "f.req=" + url.QueryEscape(freqJSON) + "\n"
}

// fetchPlayStoreAppName fetches the display name from the og:title meta tag.
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

// extractAppName reads the app name from the og:title meta tag.
// Typical format: "Dafiti: Shopping no seu Bolso – Apps on Google Play"
func extractAppName(html, fallback string) string {
	if m := reOgTitle.FindStringSubmatch(html); len(m) > 1 {
		name := m[1]
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
