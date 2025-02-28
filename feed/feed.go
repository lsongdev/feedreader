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

// FeedType 表示订阅源类型
type FeedType string

const (
	TypeRSS  FeedType = "rss"
	TypeAtom FeedType = "atom"
)

// FeedItem 代表一个订阅项目
type FeedItem struct {
	ID          string
	Title       string
	Link        string
	Description string
	PubDate     time.Time
}

// Feed 代表一个订阅源
type Feed struct {
	Type  FeedType
	Title string
	Link  string
	Items []*FeedItem
}

// 定义常用的时间格式
var timeLayouts = []string{
	time.RFC3339,
	time.RFC1123,
	time.RFC1123Z,
	"Mon, 02 Jan 2006 15:04:05 GMT",
	"Monday, 02 Jan 2006 15:04:05 -07:00",
	"2006-01-02T15:04:05Z",            // 无时区偏移的常见ISO格式
	"2006-01-02T15:04:05",             // 无Z的ISO格式
	"Mon, 02 Jan 2006 15:04:05 -0700", // RFC822Z
	"02 Jan 2006 15:04:05 -0700",      // 无星期的常见格式
	"2006-01-02",                      // 仅日期
}

// parseTime 尝试使用各种常见格式解析时间字符串
func parseTime(str string) (time.Time, error) {
	str = strings.TrimSpace(str)

	// 如果字符串为空，直接返回当前时间
	if str == "" {
		return time.Now(), errors.New("empty time string")
	}

	// 尝试每种布局
	for _, layout := range timeLayouts {
		t, err := time.Parse(layout, str)
		if err == nil {
			return t, nil
		}
	}

	// 如果无法解析，返回当前时间和错误
	return time.Now(), fmt.Errorf("could not parse time: %s", str)
}

// ParseFeed 自动检测订阅源类型（RSS或Atom）并返回通用Feed
func ParseFeed(data []byte) (*Feed, error) {
	// 清理数据 - 如果存在BOM则去除
	data = bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF})

	// 尝试解析为RSS
	if feed, err := parseAsRSS(data); err == nil && feed != nil {
		return feed, nil
	}

	// 尝试解析为Atom
	if feed, err := parseAsAtom(data); err == nil && feed != nil {
		return feed, nil
	}

	return nil, errors.New("failed to parse feed: not a valid RSS or Atom format")
}

// parseAsRSS 尝试将数据解析为RSS格式
func parseAsRSS(data []byte) (*Feed, error) {
	rssFeed, err := ParseRss(data)
	if err != nil || rssFeed == nil {
		return nil, err
	}

	feed := &Feed{
		Type:  TypeRSS,
		Link:  rssFeed.Link,
		Title: rssFeed.Title,
		Items: make([]*FeedItem, 0, len(rssFeed.Items)),
	}

	// 将RSS项目转换为通用订阅项目
	for _, item := range rssFeed.Items {
		pubDate, _ := parseTime(item.PubDate) // 忽略错误，使用返回的时间

		feedItem := &FeedItem{
			ID:          item.ID(),
			Title:       cleanContent(item.Title),
			Link:        item.Link,
			Description: cleanContent(item.GetContent()),
			PubDate:     pubDate,
		}
		feed.Items = append(feed.Items, feedItem)
	}

	return feed, nil
}

// parseAsAtom 尝试将数据解析为Atom格式
func parseAsAtom(data []byte) (*Feed, error) {
	atomFeed, err := ParseAtom(data)
	if err != nil || atomFeed == nil {
		return nil, err
	}

	// 确保links不为空
	if len(atomFeed.Links) == 0 {
		return nil, errors.New("atom feed has no links")
	}

	feed := &Feed{
		Type:  TypeAtom,
		Link:  atomFeed.Links[0].Href,
		Title: atomFeed.Title.Data,
		Items: make([]*FeedItem, 0, len(atomFeed.Entries)),
	}

	// 将Atom条目转换为通用订阅项目
	for _, entry := range atomFeed.Entries {
		// 查找链接
		link := findBestLink(entry.Links)

		// 获取发布日期（优先使用Published，备用Updated）
		pubDateStr := entry.Published
		if pubDateStr == "" {
			pubDateStr = entry.Updated
		}

		pubDate, _ := parseTime(pubDateStr) // 忽略错误，使用返回的时间

		feedItem := &FeedItem{
			ID:          entry.ID,
			Title:       cleanContent(entry.Title.Data),
			Link:        link,
			Description: cleanContent(entry.GetContent()),
			PubDate:     pubDate,
		}
		feed.Items = append(feed.Items, feedItem)
	}

	return feed, nil
}

// findBestLink 从链接列表中找出最佳链接
func findBestLink(links []AtomLink) string {
	if len(links) == 0 {
		return ""
	}

	// 首先尝试找到rel="alternate"的链接
	for _, l := range links {
		if l.Rel == "alternate" {
			return l.Href
		}
	}

	// 如果没有，返回第一个链接
	return links[0].Href
}

// FetchFeed 从给定URL下载订阅源并解析它
func FetchFeed(url string) (*Feed, error) {
	// 创建带超时的HTTP客户端
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	// 发送GET请求
	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch feed: %w", err)
	}
	defer resp.Body.Close()

	// 检查状态码
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch feed: HTTP %s", resp.Status)
	}

	// 读取响应体
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// 解析订阅源
	return ParseFeed(data)
}
