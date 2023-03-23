package main

import (
	"database/sql"
	"fmt"
	"html/template"
	"log"
	"time"

	"github.com/song940/feedparser-go/feed"
)

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
		CREATE TABLE IF NOT EXISTS subscriptions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
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
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			subscription_id INTEGER,
			FOREIGN KEY (subscription_id) REFERENCES subscriptions (id),
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

type Post struct {
	Id        int
	Title     string
	Content   template.HTML
	Link      string
	CreatedAt time.Time
}

// CreateFeed adds a new subscription to the database.
func (reader *Reader) CreateFeed(name, home, link string) (string, error) {
	var id string
	err := reader.db.QueryRow(`
		INSERT INTO subscriptions (name, home, link) VALUES (?, ?, ?) RETURNING id
	`, name, home, link).Scan(&id)
	return id, err
}

// CreatePost adds a new post to the database.
func (reader *Reader) CreatePost(subscriptionID, title, content, link, pubDate string) error {
	createdAt, _ := time.Parse(time.RFC1123Z, pubDate)
	_, err := reader.db.Exec(`
		INSERT INTO posts (title, content, link, created_at, subscription_id) VALUES (?, ?, ?, ?, ?)
	`, title, content, link, createdAt, subscriptionID)
	return err
}

// GetEntriesByCriteria retrieves entries (subscriptions or posts) based on the provided filter.
func (reader *Reader) GetFeedsByFilter(filter string, value interface{}) ([]H, error) {
	var query string
	switch filter {
	case "id":
		query = `
		SELECT id, name, home, link, created_at
		FROM subscriptions
		WHERE id = ? 
		ORDER BY created_at DESC
	`
	default:
		query = `
		SELECT id, name, home, link, created_at
		FROM subscriptions
		ORDER BY created_at DESC
	`
	}
	var rows *sql.Rows
	var err error
	if filter != "" {
		rows, err = reader.db.Query(query, value)
	} else {
		rows, err = reader.db.Query(query)
	}

	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var entries []H
	for rows.Next() {
		var id, name, home, link, createdAt string
		err := rows.Scan(&id, &name, &home, &link, &createdAt)
		if err != nil {
			return nil, err
		}
		entry := H{
			"id":         id,
			"name":       name,
			"home":       home,
			"link":       link,
			"created_at": createdAt,
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

// GetFeeds retrieves all subscriptions from the database.
func (reader *Reader) GetFeeds() ([]H, error) {
	return reader.GetFeedsByFilter("", nil)
}

// GetFeed retrieves a specific subscription from the database.
func (reader *Reader) GetFeed(id string) (H, error) {
	entries, err := reader.GetFeedsByFilter("id", id)
	if err != nil {
		return nil, err
	}
	if len(entries) == 0 {
		return nil, fmt.Errorf("subscription not found")
	}
	return entries[0], nil
}

// GetPostsByFilter retrieves posts based on the provided filter.
func (reader *Reader) GetPostsByFilter(filter string, value interface{}) ([]Post, error) {
	var query string
	switch filter {
	case "id":
		query = `
			SELECT id, title, content, link, created_at
			FROM posts
			WHERE id = ? 
			ORDER BY created_at DESC
		`
	case "subscription_id":
		query = `
			SELECT id, title, content, link, created_at
			FROM posts
			WHERE subscription_id = ? 
			ORDER BY created_at DESC
		`
	default:
		query = `
			SELECT id, title, content, link, created_at
			FROM posts
			ORDER BY created_at DESC
		`
	}
	var rows *sql.Rows
	var err error
	if filter != "" {
		rows, err = reader.db.Query(query, value)
	} else {
		rows, err = reader.db.Query(query)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var posts []Post
	for rows.Next() {
		var post Post
		err := rows.Scan(&post.Id, &post.Title, &post.Content, &post.Link, &post.CreatedAt)
		if err != nil {
			return nil, err
		}
		posts = append(posts, post)
	}
	return posts, nil
}

// GetPost retrieves a specific post from the database.
func (reader *Reader) GetPost(id string) (Post, error) {
	posts, err := reader.GetPostsByFilter("id", id)
	if err != nil {
		return Post{}, err
	}
	if len(posts) == 0 {
		return Post{}, fmt.Errorf("post not found")
	}
	return posts[0], nil
}

// GetPosts retrieves all posts from the database.
func (reader *Reader) GetPosts() ([]Post, error) {
	return reader.GetPostsByFilter("", nil)
}

// GetPostsBySubscriptionId retrieves posts for a specific subscription.
func (reader *Reader) GetPostsBySubscriptionId(id string) ([]Post, error) {
	return reader.GetPostsByFilter("subscription_id", id)
}

// updateSubscriptionPosts fetches new articles for a subscription and saves them to the database.
func (reader *Reader) updateSubscriptionPosts(id string) error {
	subscrition, err := reader.GetFeed(id)
	if err != nil {
		return err
	}
	link := subscrition["link"].(string)
	log.Println("Updating posts for subscription", id, link)
	rss, err := feed.FetchRss(link)
	if err != nil {
		return err
	}
	for _, article := range rss.Items {
		reader.CreatePost(id, article.Title, article.Description, article.Link, article.PubDate)
	}
	return nil
}

// updatePostsPeriodically periodically updates posts for all subscriptions.
func (reader *Reader) updatePostsPeriodically() {
	for range reader.tick.C {
		// Get all subscriptions and update posts for each
		subscriptions, err := reader.GetFeeds()
		if err != nil {
			log.Println("Error getting subscriptions:", err)
			continue
		}

		for _, subscription := range subscriptions {
			id := subscription["id"].(string)
			// Assuming you have a function to fetch and save posts for each subscription
			err := reader.updateSubscriptionPosts(id)
			if err != nil {
				log.Printf("Error updating posts for subscription %s: %v\n", id, err)
			}
		}
	}
}