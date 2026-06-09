package news

import "testing"

func TestParseRSSArticlesNormalizesFeedItems(t *testing.T) {
	body := []byte(`<?xml version="1.0" encoding="UTF-8" ?>
<rss version="2.0">
  <channel>
    <title>Example Feed</title>
    <item>
      <title>Fed signals higher rates &amp; tech pressure</title>
      <link>https://example.com/a</link>
      <description><![CDATA[<p>Markets watch policy.</p>]]></description>
      <pubDate>Tue, 09 Jun 2026 04:30:00 GMT</pubDate>
      <category>Markets</category>
    </item>
    <item>
      <title></title>
      <link>https://example.com/empty</link>
    </item>
  </channel>
</rss>`)

	articles, err := parseRSSArticles(body, RSSFeed{Name: "Example", URL: "https://example.com/rss", Region: "global", Topic: "business"}, 10)
	if err != nil {
		t.Fatalf("parseRSSArticles failed: %v", err)
	}
	if len(articles) != 1 {
		t.Fatalf("expected 1 article, got %d", len(articles))
	}
	if articles[0].Title != "Fed signals higher rates & tech pressure" {
		t.Fatalf("unexpected title: %q", articles[0].Title)
	}
	if articles[0].Description != "Markets watch policy." {
		t.Fatalf("unexpected description: %q", articles[0].Description)
	}
	if articles[0].PublishedAt.IsZero() {
		t.Fatalf("expected parsed published time")
	}
	if articles[0].Region != "global" || articles[0].Topic != "business" {
		t.Fatalf("expected feed metadata to be carried, got %+v", articles[0])
	}
}
