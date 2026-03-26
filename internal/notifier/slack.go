package notifier

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/hlfdev/gogather/internal/scraper"
)

type SlackNotifier struct {
	webhookURL string
	client     *http.Client
}

func NewSlack(webhookURL string) *SlackNotifier {
	return &SlackNotifier{
		webhookURL: webhookURL,
		client:     &http.Client{Timeout: 10 * time.Second},
	}
}

// slackMessage uses legacy attachments to get the colored left border.
type slackMessage struct {
	Attachments []slackAttachment `json:"attachments"`
}

type slackAttachment struct {
	Color  string       `json:"color"`
	Blocks []slackBlock `json:"blocks"`
}

type slackBlock struct {
	Type     string         `json:"type"`
	Text     *slackText     `json:"text,omitempty"`
	Elements []slackElement `json:"elements,omitempty"`
}

type slackText struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type slackElement struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

func (s *SlackNotifier) Send(review scraper.Review) error {
	data, err := json.Marshal(buildMessage(review))
	if err != nil {
		return err
	}
	resp, err := s.client.Post(s.webhookURL, "application/json", bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("slack send: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("slack responded with status %d", resp.StatusCode)
	}
	return nil
}

func buildMessage(r scraper.Review) slackMessage {
	return slackMessage{
		Attachments: []slackAttachment{{
			Color:  ratingColor(r.Rating),
			Blocks: buildBlocks(r),
		}},
	}
}

// Layout:
//
//	[colored bar]
//	*App Name*   ★★★★☆
//	:store_icon: Store Name
//
//	*Title* (if present)
//	> Review body text
//	> continued...
//
//	👤 Author  ·  📅 Date  ·  📱 vX.Y.Z
func buildBlocks(r scraper.Review) []slackBlock {
	storeIcon, storeName := storeInfo(r.Source)

	// ── header ──────────────────────────────────────────────────
	header := fmt.Sprintf("*%s*   %s\n%s  %s",
		escape(r.AppName),
		renderStars(r.Rating),
		storeIcon,
		storeName,
	)

	// ── body ────────────────────────────────────────────────────
	var bodyLines []string
	if r.Title != "" {
		bodyLines = append(bodyLines, fmt.Sprintf("*%s*", escape(r.Title)))
	}
	bodyText := strings.TrimSpace(r.Body)
	if len(bodyText) > 2800 {
		bodyText = bodyText[:2800] + "…"
	}
	for _, line := range strings.Split(bodyText, "\n") {
		bodyLines = append(bodyLines, "> "+escape(line))
	}

	// ── footer ──────────────────────────────────────────────────
	var footerParts []string
	if r.Author != "" {
		footerParts = append(footerParts, "👤 "+r.Author)
	}
	if !r.Date.IsZero() {
		footerParts = append(footerParts, "📅 "+r.Date.Format("02/01/2006"))
	}
	if r.Version != "" {
		footerParts = append(footerParts, "📱 v"+r.Version)
	}

	blocks := []slackBlock{
		{Type: "section", Text: &slackText{Type: "mrkdwn", Text: header}},
		{Type: "section", Text: &slackText{Type: "mrkdwn", Text: strings.Join(bodyLines, "\n")}},
	}
	if len(footerParts) > 0 {
		blocks = append(blocks, slackBlock{
			Type:     "context",
			Elements: []slackElement{{Type: "mrkdwn", Text: strings.Join(footerParts, "   ·   ")}},
		})
	}
	return blocks
}

func storeInfo(source scraper.Source) (icon, name string) {
	if source == scraper.SourcePlayStore {
		return ":google_play:", "Google Play"
	}
	return ":applestore:", "App Store"
}

func renderStars(rating int) string {
	if rating < 0 {
		rating = 0
	}
	if rating > 5 {
		rating = 5
	}
	return strings.Repeat("★", rating) + strings.Repeat("☆", 5-rating)
}

func ratingColor(rating int) string {
	switch {
	case rating >= 4:
		return "#2eb886" // green
	case rating == 3:
		return "#ecb22e" // yellow
	default:
		return "#e01e5a" // red
	}
}

func escape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return s
}
