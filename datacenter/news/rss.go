package news

import (
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/corpix/uarand"
)

type rssDocument struct {
	Channel rssChannel `xml:"channel"`
}

type rssChannel struct {
	Title string    `xml:"title"`
	Items []rssItem `xml:"item"`
}

type rssItem struct {
	Title       string   `xml:"title"`
	Link        string   `xml:"link"`
	Description string   `xml:"description"`
	PubDate     string   `xml:"pubDate"`
	Categories  []string `xml:"category"`
}

// QueryRSSFeeds reads a small set of RSS feeds and returns normalized articles.
func (n News) QueryRSSFeeds(ctx context.Context, feeds []RSSFeed, limitPerFeed int) ([]Article, error) {
	if limitPerFeed <= 0 {
		limitPerFeed = 10
	}
	articles := []Article{}
	errTexts := []string{}
	for _, feed := range feeds {
		items, err := n.QueryRSSFeed(ctx, feed, limitPerFeed)
		if err != nil {
			errTexts = append(errTexts, fmt.Sprintf("%s: %s", feed.Name, err.Error()))
			continue
		}
		articles = append(articles, items...)
	}
	articles = dedupeNewsArticles(articles)
	sort.SliceStable(articles, func(i, j int) bool {
		return articles[i].PublishedAt.After(articles[j].PublishedAt)
	})
	if len(errTexts) > 0 {
		return articles, errors.New(strings.Join(errTexts, "; "))
	}
	return articles, nil
}

func (n News) QueryRSSFeed(ctx context.Context, feed RSSFeed, limit int) ([]Article, error) {
	if strings.TrimSpace(feed.URL) == "" {
		return nil, fmt.Errorf("empty rss url")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, feed.URL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", uarand.GetRandom())
	req.Header.Set("Accept", "application/rss+xml, application/xml, text/xml")
	resp, err := n.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("http status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 2*1024*1024))
	if err != nil {
		return nil, err
	}
	return parseRSSArticles(body, feed, limit)
}

func parseRSSArticles(body []byte, feed RSSFeed, limit int) ([]Article, error) {
	doc := rssDocument{}
	if err := xml.Unmarshal(body, &doc); err != nil {
		return nil, err
	}
	source := cleanNewsText(doc.Channel.Title)
	if source == "" {
		source = feed.Name
	}
	articles := []Article{}
	for _, item := range doc.Channel.Items {
		title := cleanNewsText(item.Title)
		link := strings.TrimSpace(item.Link)
		if title == "" || link == "" {
			continue
		}
		articles = append(articles, Article{
			Source:      source,
			FeedName:    feed.Name,
			Region:      feed.Region,
			Topic:       feed.Topic,
			Title:       title,
			Description: cleanNewsText(item.Description),
			Link:        link,
			PublishedAt: parseNewsPublishedAt(item.PubDate),
			Categories:  cleanNewsCategories(item.Categories),
		})
		if limit > 0 && len(articles) >= limit {
			break
		}
	}
	if len(articles) == 0 {
		return nil, fmt.Errorf("rss feed empty")
	}
	return articles, nil
}

func cleanNewsText(text string) string {
	text = html.UnescapeString(text)
	text = strings.ReplaceAll(text, "\n", " ")
	text = strings.ReplaceAll(text, "\r", " ")
	text = strings.TrimSpace(stripSimpleHTMLTags(text))
	for strings.Contains(text, "  ") {
		text = strings.ReplaceAll(text, "  ", " ")
	}
	return text
}

func stripSimpleHTMLTags(text string) string {
	result := strings.Builder{}
	inTag := false
	for _, ch := range text {
		switch ch {
		case '<':
			inTag = true
		case '>':
			inTag = false
		default:
			if !inTag {
				result.WriteRune(ch)
			}
		}
	}
	return result.String()
}

func cleanNewsCategories(items []string) []string {
	result := []string{}
	seen := map[string]bool{}
	for _, item := range items {
		item = cleanNewsText(item)
		if item == "" || seen[item] {
			continue
		}
		seen[item] = true
		result = append(result, item)
	}
	return result
}

func parseNewsPublishedAt(text string) time.Time {
	text = strings.TrimSpace(text)
	if text == "" {
		return time.Time{}
	}
	layouts := []string{
		time.RFC1123Z,
		time.RFC1123,
		time.RFC822Z,
		time.RFC822,
		time.RFC3339,
		"Mon, 02 Jan 2006 15:04:05 -0700",
		"Mon, 02 Jan 2006 15:04:05 MST",
		"2006-01-02 15:04:05",
	}
	for _, layout := range layouts {
		if parsed, err := time.Parse(layout, text); err == nil {
			return parsed
		}
	}
	return time.Time{}
}

func dedupeNewsArticles(articles []Article) []Article {
	result := []Article{}
	seen := map[string]bool{}
	for _, article := range articles {
		key := strings.ToLower(strings.TrimSpace(article.Link))
		if key == "" {
			key = strings.ToLower(strings.TrimSpace(article.Title))
		}
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, article)
	}
	return result
}
