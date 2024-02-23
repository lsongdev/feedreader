package reader

import (
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/song940/fever-go/fever"
)

// FeverAuthenticate implements fever.Handler.
func (*Reader) FeverAuthenticate(apiKey string) bool {
	return true
}

func (r *Reader) FeverGroups() (response fever.GroupsResponse) {
	feeds, err := r.GetFeeds(nil)
	if err != nil {
		log.Fatalf("Failed to get subscriptions: %v", err)
		return
	}
	feedIds := make([]string, 0, len(feeds))
	for _, feed := range feeds {
		feedIds = append(feedIds, strconv.Itoa(int(feed.Id)))
	}
	response.Groups = []fever.Group{
		{ID: 1, Title: "All"},
	}
	response.FeedsGroups = []fever.FeedsGroups{
		{GroupID: 1, FeedIDs: strings.Join(feedIds, ",")},
	}
	return
}

func (r *Reader) FeverFeeds() (response fever.FeedsResponse) {
	feeds, err := r.GetFeeds(nil)
	if err != nil {
		log.Fatalf("Failed to get subscriptions: %v", err)
		return
	}
	for _, feed := range feeds {
		response.Feeds = append(response.Feeds, fever.Feed{
			ID:          int64(feed.Id),
			FaviconID:   1,
			Title:       feed.Name,
			URL:         feed.Link,
			SiteURL:     feed.Home,
			IsSpark:     1,
			LastUpdated: feed.CreatedAt.Unix(),
		})
	}
	feedIds := make([]string, 0, len(feeds))
	for _, feed := range feeds {
		feedIds = append(feedIds, strconv.Itoa(int(feed.Id)))
	}
	response.FeedsGroups = []fever.FeedsGroups{
		{GroupID: 1, FeedIDs: strings.Join(feedIds, ",")},
	}
	return response
}

func parseTime(str string) (t time.Time, err error) {
	layouts := []string{
		time.RFC3339,
		time.RFC1123Z,
	}
	for _, layout := range layouts {
		t, err := time.Parse(layout, str)
		if err == nil {
			return t, nil
		}
	}
	err = fmt.Errorf("could not parse time: %s", str)
	return
}

func (r *Reader) FeverItems(req *fever.ItemRequest) (response fever.ItemsResponse) {
	conditions := make([]string, 0)
	if req.SinceId != "" {
		conditions = append(conditions, fmt.Sprintf("p.id > %s", req.SinceId))
	}
	if req.WithIDs != "" {
		conditions = append(conditions, fmt.Sprintf("p.id IN (%s)", req.WithIDs))
	}
	posts, err := r.GetPosts(conditions)
	if err != nil {
		log.Fatalf("Failed to get posts: %v", err)
		return
	}
	b2i := func(b bool) int {
		if b {
			return 1
		} else {
			return 0
		}
	}
	response.Total = len(posts)
	response.Items = make([]fever.Item, 0, len(posts))
	for _, post := range posts {
		t, _ := parseTime(post.PubDate)
		response.Items = append(response.Items, fever.Item{
			ID:        int64(post.Id),
			FeedID:    int64(post.Feed.Id),
			Author:    post.Feed.Name,
			Title:     post.Title,
			HTML:      post.Content,
			URL:       post.Link,
			IsRead:    b2i(post.Readed),
			IsSaved:   b2i(post.Starred),
			CreatedAt: t.Unix(),
		})
	}
	return response
}

func (r *Reader) FeverUnreadItemIds() (response fever.UnreadResponse) {
	postIds := make([]string, 0)
	rows, err := r.db.Query("SELECT id FROM posts WHERE readed = 0")
	if err != nil {
		log.Fatalf("Failed to get unread posts: %v", err)
		return
	}
	for rows.Next() {
		var id string
		err := rows.Scan(&id)
		if err != nil {
			log.Fatalf("Failed to get unread posts: %v", err)
			return
		}
		postIds = append(postIds, id)
	}
	response.ItemIDs = strings.Join(postIds, ",")
	log.Println("Unread item ids:", response.ItemIDs)
	return response
}

func (r *Reader) FeverSavedItemIds() (response fever.SavedResponse) {
	postIds := make([]string, 0)
	rows, err := r.db.Query("SELECT id FROM posts WHERE starred = 1")
	if err != nil {
		log.Fatalf("Failed to get saved posts: %v", err)
		return
	}
	for rows.Next() {
		var id string
		err := rows.Scan(&id)
		if err != nil {
			log.Fatalf("Failed to get saved posts: %v", err)
			return
		}
		postIds = append(postIds, id)
	}
	response.ItemIDs = strings.Join(postIds, ",")
	return response
}

func (r *Reader) FeverMark(req *fever.MarkRequest) (response fever.MarkResponse) {
	log.Println("Marking item", req.Type, req.Id, "as", req.As)
	updates := make([]string, 0)
	if req.Type == "item" {
		switch req.As {
		case "read":
			updates = append(updates, "readed = 1")
		case "unread":
			updates = append(updates, "readed = 0")
		case "saved":
			updates = append(updates, "starred = 1")
		case "unsaved":
			updates = append(updates, "starred = 0")
		}
		err := r.UpdatePost(req.Id, updates)
		if err != nil {
			log.Fatalf("Failed to update post: %v", err)
			return
		}
	}
	return
}
