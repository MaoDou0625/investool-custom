package news

import (
	"net/http"
	"time"
)

// News is a small client for public news feeds used by local risk context.
type News struct {
	HTTPClient *http.Client
}

// RSSFeed describes a public RSS source.
type RSSFeed struct {
	Name   string
	URL    string
	Region string
	Topic  string
}

// Article is a normalized news item from RSS.
type Article struct {
	Source      string
	FeedName    string
	Region      string
	Topic       string
	Title       string
	Description string
	Link        string
	PublishedAt time.Time
	Categories  []string
}

func NewNews() News {
	return News{
		HTTPClient: &http.Client{Timeout: 12 * time.Second},
	}
}
