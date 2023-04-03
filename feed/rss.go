package feed

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"log"
	"net/http"
)

type RssItem struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	Description string `xml:"description"`
	PubDate     string `xml:"pubDate"`
}

type RssFeed struct {
	Title string    `xml:"channel>title"`
	Link  string    `xml:"channel>link"`
	Items []RssItem `xml:"channel>item"`
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

func FetchRss(url string) (feed *RssFeed, err error) {
	res, err := http.Get(url)
	if err != nil {
		log.Fatal(err)
		return
	}
	if res.StatusCode != 200 {
		err = fmt.Errorf("status code: %d", res.StatusCode)
		return
	}
	data, err := io.ReadAll(res.Body)
	if err != nil {
		log.Fatal(err)
	}
	feed, err = ParseRss(data)
	return
}
