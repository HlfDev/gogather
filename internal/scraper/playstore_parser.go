package scraper

// parseBatchExecuteResponse decodes a Play Store batchexecute RPC response.
//
// The response body starts with )]}'\n\n followed by a JSON array. After
// stripping that prefix, the outer structure is:
//
//	outer[0] = ["wrb.fr", "oCPfdb", "<inner_json_string>", ...]
//
// The inner JSON string (outer[0][2]) contains:
//
//	inner[0] = []review  (array of review arrays)
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
	"strings"
	"time"
)

func parseBatchExecuteResponse(raw []byte, packageName, appName string) ([]Review, error) {
	// Strip the )]}'\n\n prefix emitted by the batchexecute endpoint.
	body := string(raw)
	idx := strings.Index(body, "\n\n")
	if idx < 0 {
		return nil, fmt.Errorf("batchexecute response: missing \\n\\n separator")
	}
	body = body[idx+2:]

	// Parse the outer JSON array.
	var outer []json.RawMessage
	if err := json.Unmarshal([]byte(body), &outer); err != nil {
		return nil, fmt.Errorf("outer parse: %w", err)
	}
	if len(outer) == 0 {
		return nil, nil
	}

	// outer[0] = ["wrb.fr", "oCPfdb", "<inner_json_string>", ...]
	var firstEntry []json.RawMessage
	if err := json.Unmarshal(outer[0], &firstEntry); err != nil {
		return nil, fmt.Errorf("first entry parse: %w", err)
	}
	if len(firstEntry) < 3 {
		return nil, fmt.Errorf("unexpected first entry length %d", len(firstEntry))
	}

	// firstEntry[2] is a JSON-encoded string — parse it to get the real payload.
	var innerStr string
	if err := json.Unmarshal(firstEntry[2], &innerStr); err != nil {
		return nil, fmt.Errorf("inner string parse: %w", err)
	}

	var inner []interface{}
	if err := json.Unmarshal([]byte(innerStr), &inner); err != nil {
		return nil, fmt.Errorf("inner parse: %w", err)
	}
	if len(inner) == 0 {
		return nil, nil
	}

	reviewsSlice, ok := inner[0].([]interface{})
	if !ok {
		return nil, nil
	}

	var reviews []Review
	for _, rv := range reviewsSlice {
		r, err := parsePlayStoreReview(rv, packageName, appName)
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
