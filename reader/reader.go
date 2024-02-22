package reader

import (
	"database/sql"
	"fmt"
	"log"
	"strings"
	"time"

	_ "github.com/glebarez/go-sqlite"
	"github.com/song940/feedparser-go/feed"
)

type Feed struct {
	Id        int
	Type      string
	Name      string
	Home      string
	Link      string
	CreatedAt time.Time
}

type Post struct {
	Feed

	Id        int
	Title     string
	Content   string
	Link      string
	Starred   bool
	Readed    bool
	CreatedAt time.Time
}

// Reader represents the main application struct.
type Reader struct {
	db   *sql.DB
	tick *time.Ticker
}

// New initializes a new instance of the Reader application.
func NewReader() (reader *Reader, err error) {
	// Open a database connection
	file := "./reader.db"

	db, err := sql.Open("sqlite", file)
	if err != nil {
		return nil, err
	}

	// Create subscriptions table
	if _, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS feeds (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			type TEXT,
			name TEXT,
			home TEXT,
			link TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			UNIQUE (name, home, link)
		)
	`); err != nil {
		return nil, err
	}

	// Create posts table
	if _, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS posts (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			title TEXT,
			content TEXT,
			link TEXT,
			readed BOOLEAN DEFAULT 0,
			starred BOOLEAN DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			feed_id INTEGER,
			FOREIGN KEY (feed_id) REFERENCES feeds (id),
			UNIQUE (link)
		)
	`); err != nil {
		return nil, err
	}

	// Initialize a ticker with a specified interval for periodic updates
	tick := time.NewTicker(1 * time.Minute) // Update every 1 minute, adjust as needed

	reader = &Reader{db: db, tick: tick}
	go reader.updatePostsPeriodically()
	return
}

// CreateFeed adds a new subscription to the database.
func (reader *Reader) CreateFeed(feedType, name, home, link string) (id int, err error) {
	err = reader.db.QueryRow(`
		INSERT INTO feeds (type, name, home, link) VALUES (?, ?, ?, ?) RETURNING id
	`, feedType, name, home, link).Scan(&id)
	return id, err
}

// CreatePost adds a new post to the database.
func (reader *Reader) CreatePost(feedId int, title, content, link, pubDate string) error {
	createdAt, _ := time.Parse(time.RFC1123Z, pubDate)
	_, err := reader.db.Exec(`
		INSERT INTO posts (title, content, link, created_at, feed_id) VALUES (?, ?, ?, ?, ?)
	`, title, content, link, createdAt, feedId)
	return err
}

// GetEntriesByCriteria retrieves entries (subscriptions or posts) based on the provided filter.
func (reader *Reader) GetFeeds(conditions []string) ([]*Feed, error) {
	var filter string
	if len(conditions) > 0 {
		filter = "WHERE " + strings.Join(conditions, " AND ")
	}
	rows, err := reader.db.Query(fmt.Sprintf(`
	SELECT id, type, name, home, link, created_at
	FROM feeds %s
	ORDER BY created_at DESC`, filter))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var entries []*Feed
	for rows.Next() {
		var feed Feed
		err := rows.Scan(&feed.Id, &feed.Type, &feed.Name, &feed.Home, &feed.Link, &feed.CreatedAt)
		if err != nil {
			return nil, err
		}
		entries = append(entries, &feed)
	}
	return entries, nil
}

// GetFeed retrieves a specific subscription from the database.
func (reader *Reader) GetFeed(id int) (feed *Feed, err error) {
	entries, err := reader.GetFeeds([]string{fmt.Sprintf("id = %d", id)})
	if err != nil {
		return
	}
	if len(entries) == 0 {
		err = fmt.Errorf("feed not found")
		return
	}
	feed = entries[0]
	return
}

// GetPostsByFilter retrieves posts based on the provided filter.
func (reader *Reader) GetPosts(conditions []string) (posts []*Post, err error) {
	conditions = append(conditions, "p.feed_id = s.id")
	sql := fmt.Sprintf(`
		SELECT p.id, p.title, p.content, p.link, p.readed, p.starred, p.created_at, s.id, s.name, s.home
		FROM posts p, feeds s
		WHERE %s
		ORDER BY p.created_at DESC
	`, strings.Join(conditions, " AND "))
	rows, err := reader.db.Query(sql)
	if err != nil {
		return
	}
	defer rows.Close()
	for rows.Next() {
		var post Post
		post.Feed = Feed{}
		err := rows.Scan(
			&post.Id, &post.Title, &post.Content, &post.Link,
			&post.Readed, &post.Starred,
			&post.CreatedAt,
			&post.Feed.Id, &post.Feed.Name, &post.Feed.Home)
		if err != nil {
			return nil, err
		}
		posts = append(posts, &post)
	}
	return
}

// GetPost retrieves a specific post from the database.
func (reader *Reader) GetPost(id string) (post *Post, err error) {
	posts, err := reader.GetPosts([]string{fmt.Sprintf("p.id = %s", id)})
	if err != nil {
		return
	}
	if len(posts) == 0 {
		return post, fmt.Errorf("post not found")
	}
	post = posts[0]
	return
}

// GetPostsBySubscriptionId retrieves posts for a specific subscription.
func (reader *Reader) GetPostsByFeedId(id string) ([]*Post, error) {
	return reader.GetPosts([]string{fmt.Sprintf("p.feed_id = %s", id)})
}

func (reader *Reader) UpdatePost(id int, updates []string) error {
	sql := fmt.Sprintf(`UPDATE posts SET %s WHERE id = ?`, strings.Join(updates, ", "))
	_, err := reader.db.Exec(sql, id)
	return err
}

// updateSubscriptionPosts fetches new articles for a subscription and saves them to the database.
func (reader *Reader) updateSubscriptionPosts(id int) (err error) {
	subscrition, err := reader.GetFeed(id)
	if err != nil {
		return
	}
	log.Println("Updating posts for subscription", subscrition.Id, subscrition.Link)
	switch subscrition.Type {
	case "atom":
		atom, err := feed.FetchAtom(subscrition.Link)
		if err != nil {
			return err
		}
		for _, entry := range atom.Entries {
			reader.CreatePost(id, entry.Title.Data, entry.Content.Data, entry.Links[0].Href, entry.Updated)
		}
	case "rss":
		rss, err := feed.FetchRss(subscrition.Link)
		if err != nil {
			return err
		}
		for _, article := range rss.Items {
			reader.CreatePost(id, article.Title, article.Description, article.Link, article.PubDate)
		}
		return nil
	default:
		return fmt.Errorf("unknown subscription type: %s", subscrition.Type)
	}
	return
}

// updatePostsPeriodically periodically updates posts for all subscriptions.
func (reader *Reader) updatePostsPeriodically() {
	for range reader.tick.C {
		// Get all subscriptions and update posts for each
		subscriptions, err := reader.GetFeeds(nil)
		if err != nil {
			log.Println("Error getting subscriptions:", err)
			continue
		}

		for _, subscription := range subscriptions {
			// Assuming you have a function to fetch and save posts for each subscription
			err := reader.updateSubscriptionPosts(subscription.Id)
			if err != nil {
				log.Printf("Error updating posts for subscription %d: %v\n", subscription.Id, err)
			}
		}
	}
}
