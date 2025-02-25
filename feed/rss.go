package feed

import (
	"bytes"
	"encoding/xml"
	"regexp"
	"strings"
)

type RssGuid struct {
	Value       string `xml:",chardata"`
	IsPermaLink bool   `xml:"isPermaLink,attr,omitempty"`
}

type RssItem struct {
	Guid           RssGuid `xml:"guid"`
	Title          string  `xml:"title"`
	Link           string  `xml:"link"`
	Description    string  `xml:"description"`
	PubDate        string  `xml:"pubDate"`
	ContentEncoded string  `xml:"encoded,omitempty"`
}

func (item *RssItem) ID() string {
	if item.Guid.Value != "" {
		return item.Guid.Value
	}
	return item.Link
}

func cleanContent(content string) string {
	// Remove complete CDATA sections, keeping the content inside
	cdataRegex := regexp.MustCompile(`<!\[CDATA\[(.*?)\]\]>`)
	content = cdataRegex.ReplaceAllString(content, "$1")

	// In case there are any standalone CDATA markers left, remove them too
	content = strings.ReplaceAll(content, "<![CDATA[", "")
	content = strings.ReplaceAll(content, "]]>", "")

	// Remove any XML comments
	commentRegex := regexp.MustCompile(`<!--.*?-->`)
	content = commentRegex.ReplaceAllString(content, "")

	// Clean up any extra whitespace
	content = strings.TrimSpace(content)

	return content
}

func (item *RssItem) GetContent() string {
	if item.ContentEncoded != "" {
		return cleanContent(item.ContentEncoded)
	}
	return item.Description
}

type RssFeed struct {
	XMLName     xml.Name  `xml:"rss"`
	Title       string    `xml:"channel>title"`
	Description string    `xml:"channel>description"`
	Link        string    `xml:"channel>link"`
	Items       []RssItem `xml:"channel>item"`
}

func ParseRss(data []byte) (feed *RssFeed, err error) {
	data = bytes.Map(func(r rune) rune {
		if r == '\u0008' {
			return -1
		}
		return r
	}, data)
	err = xml.Unmarshal(data, &feed)
	return
}
