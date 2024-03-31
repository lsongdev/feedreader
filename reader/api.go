package reader

import (
	"crypto/md5"
	"fmt"
	"log"
	"strconv"
	"strings"

	"github.com/song940/fever-go/fever"
)

func (user *User) FeverAuthKey() string {
	// md5(user.username +":"+ user.password)
	str := fmt.Sprintf("%s:%s", user.Username, user.Password)
	return fmt.Sprintf("%X", md5.Sum([]byte(str)))
}

// FeverAuthenticate implements fever.Handler.
func (r *Reader) FeverAuthenticate(apiKey string) bool {
	log.Println("Authenticating", apiKey)
	for _, user := range r.config.Users {
		if strings.ToUpper(apiKey) == user.FeverAuthKey() {
			return true
		}
	}
	return false
}

func (r *Reader) FeverGroups() (response fever.GroupsResponse) {
	categories, err := r.GetCategories()
	if err != nil {
		log.Fatalf("Failed to get categories: %v", err)
		return
	}
	response.Groups = make([]fever.Group, 0, len(categories))
	for _, group := range categories {
		response.Groups = append(response.Groups, fever.Group{
			ID:    int64(group.Id),
			Title: group.Name,
		})
	}
	feeds, err := r.GetFeeds(nil)
	if err != nil {
		log.Fatalf("Failed to get feeds: %v", err)
		return
	}
	var feedGroups = make(map[int][]string)
	for _, feed := range feeds {
		feedGroups[feed.Category.Id] = append(feedGroups[feed.Category.Id], strconv.Itoa(feed.Id))
	}
	for groupId, feedIds := range feedGroups {
		response.FeedsGroups = append(response.FeedsGroups, fever.FeedsGroups{
			GroupID: int64(groupId),
			FeedIDs: strings.Join(feedIds, ","),
		})
	}
	return
}

func (r *Reader) FeverFeeds() (response fever.FeedsResponse) {
	feeds, err := r.GetFeeds(nil)
	if err != nil {
		log.Fatalf("Failed to get subscriptions: %v", err)
		return
	}
	var groups map[int][]string = make(map[int][]string)
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
		groups[feed.Category.Id] = append(groups[feed.Category.Id], strconv.Itoa(feed.Id))
	}
	for id, feedIds := range groups {
		response.FeedsGroups = append(response.FeedsGroups, fever.FeedsGroups{
			GroupID: int64(id), FeedIDs: strings.Join(feedIds, ","),
		})
	}
	return response
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
		response.Items = append(response.Items, fever.Item{
			ID:        int64(post.Id),
			FeedID:    int64(post.Feed.Id),
			Author:    post.Feed.Name,
			Title:     post.Title,
			HTML:      post.Content,
			URL:       post.Link,
			IsRead:    b2i(post.IsRead),
			IsSaved:   b2i(post.IsSaved),
			CreatedAt: post.PubDate.Unix(),
		})
	}
	return response
}

func (r *Reader) FeverUnreadItemIds() (response fever.UnreadResponse) {
	postIds := make([]string, 0)
	rows, err := r.db.Query("SELECT id FROM posts WHERE is_read = 0")
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
	return response
}

func (r *Reader) FeverSavedItemIds() (response fever.SavedResponse) {
	postIds := make([]string, 0)
	rows, err := r.db.Query("SELECT id FROM posts WHERE is_saved = 1")
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
			updates = append(updates, "is_read = 1")
		case "unread":
			updates = append(updates, "is_read = 0")
		case "saved":
			updates = append(updates, "is_saved = 1")
		case "unsaved":
			updates = append(updates, "is_saved = 0")
		}
		err := r.UpdatePost(req.Id, updates)
		if err != nil {
			log.Fatalf("Failed to update post: %v", err)
			return
		}
	}
	return
}
