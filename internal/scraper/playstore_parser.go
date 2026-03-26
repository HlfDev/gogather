package scraper

// parsePlayStoreHTML extracts reviews from the Play Store app page HTML.
//
// The Play Store embeds review data in an AF_initDataCallback call with key 'ds:11'.
// The data array structure is:
//
//	data[0] = []review      (up to 20 most recent reviews)
//	data[1] = pagination info
//
// Each review array has the following positions:
//
//	rv[0]     string   review ID
//	rv[1][0]  string   author name
//	rv[2]     float64  star rating (1-5)
//	rv[3]     string?  title (often null)
//	rv[4]     string   body text
//	rv[5][0]  float64  unix timestamp (seconds)
//	rv[10]    string?  app version (may be absent or null)

import (
	"encoding/json"
	"fmt"
	"time"
)

func parsePlayStoreHTML(html, packageName, appName string) ([]Review, error) {
	m := reDs11.FindStringSubmatch(html)
	if len(m) < 2 {
		return nil, fmt.Errorf("ds:11 payload not found — app may not exist or Play Store changed its format")
	}

	var data []interface{}
	if err := json.Unmarshal([]byte(m[1]), &data); err != nil {
		return nil, fmt.Errorf("ds:11 parse: %w", err)
	}

	if len(data) == 0 {
		return nil, nil
	}

	reviewsSlice, ok := data[0].([]interface{})
	if !ok {
		return nil, nil
	}

	var reviews []Review
	for _, raw := range reviewsSlice {
		r, err := parsePlayStoreReview(raw, packageName, appName)
		if err != nil {
			continue
		}
		reviews = append(reviews, r)
	}
	return reviews, nil
}

// parsePlayStoreReview maps a single review array to a Review struct.
func parsePlayStoreReview(raw interface{}, packageName, appName string) (Review, error) {
	rv, ok := raw.([]interface{})
	if !ok || len(rv) < 5 {
		return Review{}, fmt.Errorf("unexpected review structure")
	}

	id, _ := rv[0].(string)
	if id == "" {
		return Review{}, fmt.Errorf("missing review ID")
	}

	// Author name: rv[1][0]
	authorName := ""
	if a1, ok := rv[1].([]interface{}); ok && len(a1) > 0 {
		authorName, _ = a1[0].(string)
	}

	rating := 0
	if r, ok := rv[2].(float64); ok {
		rating = int(r)
	}

	title := ""
	if len(rv) > 3 {
		title, _ = rv[3].(string)
	}

	body := ""
	if len(rv) > 4 {
		body, _ = rv[4].(string)
	}

	var date time.Time
	if len(rv) > 5 {
		if dateArr, ok := rv[5].([]interface{}); ok && len(dateArr) > 0 {
			if ts, ok := dateArr[0].(float64); ok && ts > 0 {
				date = time.Unix(int64(ts), 0)
			}
		}
	}

	// Version: rv[10] (direct string, not nested)
	version := ""
	if len(rv) > 10 && rv[10] != nil {
		version, _ = rv[10].(string)
	}

	return Review{
		ID:      id,
		Source:  SourcePlayStore,
		AppName: appName,
		Author:  authorName,
		Rating:  rating,
		Title:   title,
		Body:    body,
		Date:    date,
		Version: version,
	}, nil
}

func preview(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
