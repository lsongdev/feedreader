package feed

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type FeedItem struct {
	ID          string
	Title       string
	Link        string
	Description string
	PubDate     time.Time
}

type Feed struct {
	Type  string
	Title string
	Link  string
	Items []*FeedItem
}

// parseTime attempts to parse a time string using various common formats
func parseTime(str string) (t time.Time, err error) {
	layouts := []string{
		time.RFC3339,
		time.RFC1123,
		time.RFC1123Z,
		"Mon, 02 Jan 2006 15:04:05 GMT",
		"Monday, 02 Jan 2006 15:04:05 -07:00",
		"2006-01-02T15:04:05Z",            // Common ISO format without timezone offset
		"2006-01-02T15:04:05",             // ISO format without Z
		"Mon, 02 Jan 2006 15:04:05 -0700", // RFC822Z
		"02 Jan 2006 15:04:05 -0700",      // Common format without day of week
		"2006-01-02",                      // Just date
	}

	// Clean the string first
	str = strings.TrimSpace(str)

	// Try each layout
	for _, layout := range layouts {
		t, err := time.Parse(layout, str)
		if err == nil {
			return t, nil
		}
	}

	// If we couldn't parse, return current time and error
	err = fmt.Errorf("could not parse time: %s", str)
	return time.Now(), err
}

// ParseFeed auto-detects the feed type (RSS or Atom) and returns a generic Feed.
func ParseFeed(data []byte) (feed *Feed, err error) {
	// Try to parse as RSS first
	rssFeed, rssErr := ParseRss(data)
	if rssErr == nil && rssFeed != nil {
		// It's an RSS feed
		feed = &Feed{
			Type:  "rss",
			Link:  rssFeed.Link,
			Title: rssFeed.Title,
			Items: make([]*FeedItem, 0, len(rssFeed.Items)),
		}

		// Convert RSS items to generic feed items
		for _, item := range rssFeed.Items {
			// Parse the publication date
			pubDate, parseErr := parseTime(item.PubDate)
			if parseErr != nil {
				// Use current time if parsing fails, but continue
				pubDate = time.Now()
			}

			feedItem := &FeedItem{
				ID:          item.ID(),
				Title:       cleanContent(item.Title),
				Link:        item.Link,
				Description: cleanContent(item.GetContent()),
				PubDate:     pubDate,
			}
			feed.Items = append(feed.Items, feedItem)
		}
		return
	}

	// Try to parse as Atom
	atomFeed, atomErr := ParseAtom(data)
	if atomErr == nil && atomFeed != nil {
		// It's an Atom feed
		feed = &Feed{
			Type:  "atom",
			Link:  atomFeed.Links[0].Href,
			Title: atomFeed.Title.Data,
			Items: make([]*FeedItem, 0, len(atomFeed.Entries)),
		}

		// Convert Atom entries to generic feed items
		for _, entry := range atomFeed.Entries {
			var link string
			// Find the first link or the link with rel="alternate"
			for _, l := range entry.Links {
				if link == "" || l.Rel == "alternate" {
					link = l.Href
				}
				if l.Rel == "alternate" {
					break
				}
			}

			// Get publication date (prefer Published, fallback to Updated)
			pubDateStr := entry.Published
			if pubDateStr == "" {
				pubDateStr = entry.Updated
			}

			// Parse the publication date
			pubDate, parseErr := parseTime(pubDateStr)
			if parseErr != nil {
				// Use current time if parsing fails, but continue
				pubDate = time.Now()
			}

			feedItem := &FeedItem{
				ID:          entry.ID,
				Title:       cleanContent(entry.Title.Data),
				Link:        link,
				Description: cleanContent(entry.GetContent()),
				PubDate:     pubDate,
			}
			feed.Items = append(feed.Items, feedItem)
		}
		return
	}

	// Neither RSS nor Atom parsing succeeded
	if rssErr != nil && atomErr != nil {
		return nil, errors.New("failed to parse feed: not a valid RSS or Atom format")
	}

	return nil, errors.New("unknown feed format")
}

// FetchFeed downloads a feed from the given URL and parses it.
func FetchFeed(url string) (feed *Feed, err error) {
	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	// Send GET request
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Check status code
	if resp.StatusCode != http.StatusOK {
		return nil, errors.New("failed to fetch feed: " + resp.Status)
	}

	// Read response body
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// Clean data - trim BOM if present
	data = bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF})

	// Parse the feed
	return ParseFeed(data)
}
