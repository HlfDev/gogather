package scraper

import "time"

type Source string

const (
	SourceApple     Source = "Apple App Store"
	SourcePlayStore Source = "Google Play Store"
)

type Review struct {
	ID       string
	Source   Source
	AppName  string
	Author   string
	Rating   int
	Title    string
	Body     string
	Date     time.Time
	Version  string
}
