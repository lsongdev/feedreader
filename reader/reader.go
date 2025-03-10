package reader

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path"
	"strings"
	"time"

	_ "github.com/glebarez/go-sqlite"
	"github.com/lsongdev/feedreader/feed"
)

type Category struct {
	Id   int    `json:"id"`
	Name string `json:"name"`
}

type Feed struct {
	Id        int       `json:"id"`
	Type      string    `json:"type"`
	Name      string    `json:"name"`
	Home      string    `json:"home"`
	Link      string    `json:"link"`
	Category  *Category `json:"category"`
	CreatedAt time.Time `json:"created_at"`
}

type Post struct {
	Feed

	Id        int       `json:"id"`
	Title     string    `json:"title"`
	Content   string    `json:"content"`
	Link      string    `json:"link"`
	IsSaved   bool      `json:"is_saved"`
	IsRead    bool      `json:"is_read"`
	PubDate   time.Time `json:"pub_date"`
	CreatedAt time.Time `json:"created_at"`
}

// Reader represents the main application struct.
type Reader struct {
	db     *sql.DB
	tick   *time.Ticker
	config *Config
}

// New initializes a new instance of the Reader application.
func NewReader(config *Config) (reader *Reader, err error) {
	// Open a database connection
	file := path.Join(config.Dir, "reader.db")
	if _, err := os.Stat(file); os.IsNotExist(err) {
		os.MkdirAll(config.Dir, 0755)
	}
	db, err := sql.Open("sqlite", file)
	if err != nil {
		return
	}
	if _, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS categories (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			UNIQUE (name)
		)
	`); err != nil {
		return
	}
	// Create subscriptions table
	if _, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS feeds (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			type TEXT,
			name TEXT,
			home TEXT,
			link TEXT,
			category_id INTEGER default 1,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			UNIQUE (name, home, link),
			FOREIGN KEY (category_id) REFERENCES categories (id)
		)
	`); err != nil {
		return
	}
	// Create posts table
	if _, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS posts (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			entry_id TEXT,
			title TEXT,
			content TEXT,
			link TEXT,
			pub_date DATETIME,
			is_read BOOLEAN DEFAULT 0,
			is_saved BOOLEAN DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			feed_id INTEGER,
			FOREIGN KEY (feed_id) REFERENCES feeds (id),
			UNIQUE (entry_id, feed_id)
		)
	`); err != nil {
		return
	}
	// Initialize a ticker with a specified interval for periodic updates
	tick := time.NewTicker(time.Minute * 1)
	reader = &Reader{
		config: config, db: db, tick: tick,
	}
	reader.CreateCategory("Default")
	go reader.updatePostsPeriodically()
	return
}

func (reader *Reader) CreateCategory(name string) (id int, err error) {
	err = reader.db.QueryRow(`
		INSERT INTO categories (name) VALUES (?) RETURNING id
	`, name).Scan(&id)
	return
}

func (reader *Reader) GetCategories() (categories []*Category, err error) {
	rows, err := reader.db.Query("SELECT id, name FROM categories")
	if err != nil {
		return
	}
	defer rows.Close()
	for rows.Next() {
		var category Category
		err := rows.Scan(&category.Id, &category.Name)
		if err != nil {
			return nil, err
		}
		categories = append(categories, &category)
	}
	return
}

func (reader *Reader) UpdateCategory(id int, name string) (err error) {
	_, err = reader.db.Exec("UPDATE categories SET name = ? WHERE id = ?", name, id)
	return
}

func (reader *Reader) DeleteCategory(id int) (err error) {
	_, err = reader.db.Exec("DELETE FROM categories WHERE id = ?", id)
	return
}

// CreateFeed adds a new subscription to the database.
func (reader *Reader) CreateFeed(feedType, name, home, link string, category_id int) (id int, err error) {
	err = reader.db.QueryRow(`
		INSERT INTO feeds (type, name, home, link, category_id) VALUES (?, ?, ?, ?, ?) RETURNING id
	`, feedType, name, home, link, category_id).Scan(&id)
	return id, err
}

// CreatePost adds a new post to the database.
func (reader *Reader) CreatePost(feedId, entryId, title, content, link string, pubDate time.Time) error {
	_, err := reader.db.Exec(`
		INSERT INTO posts (entry_id, title, content, link, pub_date, feed_id) VALUES (?, ?, ?, ?, ?, ?)
	`, entryId, title, content, link, pubDate, feedId)
	return err
}

// GetEntriesByCriteria retrieves entries (subscriptions or posts) based on the provided filter.
func (reader *Reader) GetFeeds(conditions []string) (entries []*Feed, err error) {
	var filter string
	conditions = append(conditions, "f.category_id = g.id")
	if len(conditions) > 0 {
		filter = strings.Join(conditions, " AND ")
	}
	rows, err := reader.db.Query(fmt.Sprintf(`
		SELECT f.id, f.type, f.name, f.home, f.link, f.created_at, g.id, g.name
		FROM feeds f, categories g
		WHERE  %s
		ORDER BY f.created_at DESC`, filter))
	if err != nil {
		return
	}
	defer rows.Close()
	for rows.Next() {
		var feed Feed
		feed.Category = &Category{}
		err := rows.Scan(&feed.Id, &feed.Type, &feed.Name, &feed.Home, &feed.Link, &feed.CreatedAt, &feed.Category.Id, &feed.Category.Name)
		if err != nil {
			return nil, err
		}
		entries = append(entries, &feed)
	}
	return
}

// GetFeed retrieves a specific subscription from the database.
func (reader *Reader) GetFeed(id string) (feed *Feed, err error) {
	entries, err := reader.GetFeeds([]string{fmt.Sprintf("f.id = %s", id)})
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

func (reader *Reader) DeleteFeed(id string) (err error) {
	_, err = reader.db.Exec("DELETE FROM feeds WHERE id = ?", id)
	return
}

func (reader *Reader) GetPosts(conditions []string, limit *Pagination) (posts []Post, err error) {
	conditions = append(conditions, "p.feed_id = s.id")
	conditions = append(conditions, "s.category_id = g.id")
	sql := `SELECT p.id, p.title, p.content, p.link, p.is_read, p.is_saved, p.pub_date, p.created_at, 
                s.id, s.name, s.home, g.id, g.name 
                FROM posts p, feeds s, categories g`

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = fmt.Sprintf(" WHERE %s", strings.Join(conditions, " AND "))
	}

	sql = fmt.Sprintf("%s %s ORDER BY p.pub_date DESC", sql, whereClause)
	if limit != nil {
		sql = sql + limit.SQL()
		err = reader.db.QueryRow(fmt.Sprintf("SELECT COUNT(*) FROM posts p, feeds s, categories g %s", whereClause)).Scan(&limit.Total)
		if err != nil {
			return
		}
	}
	rows, err := reader.db.Query(sql)
	if err != nil {
		return
	}
	defer rows.Close()
	for rows.Next() {
		var post Post
		post.Feed = Feed{}
		post.Feed.Category = &Category{}
		err = rows.Scan(
			&post.Id, &post.Title, &post.Content, &post.Link,
			&post.IsRead, &post.IsSaved,
			&post.PubDate, &post.CreatedAt,
			&post.Feed.Id, &post.Feed.Name, &post.Feed.Home,
			&post.Feed.Category.Id, &post.Feed.Category.Name)
		if err != nil {
			return
		}
		posts = append(posts, post)
	}
	return
}

// GetPost retrieves a specific post from the database.
func (reader *Reader) GetPost(id string) (post Post, err error) {
	posts, err := reader.GetPosts([]string{fmt.Sprintf("p.id = %s", id)}, nil)
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
func (reader *Reader) GetPostsByFeedId(id string, limit *Pagination) ([]Post, error) {
	return reader.GetPosts([]string{fmt.Sprintf("p.feed_id = %s", id)}, limit)
}

func (reader *Reader) UpdatePost(id string, updates []string) error {
	sql := fmt.Sprintf(`UPDATE posts SET %s WHERE id = ?`, strings.Join(updates, ", "))
	_, err := reader.db.Exec(sql, id)
	return err
}

// updateFeedPosts fetches new articles for a subscription and saves them to the database.
func (reader *Reader) updateFeedPosts(feedId string) (err error) {
	subscription, err := reader.GetFeed(feedId)
	if err != nil {
		return
	}

	log.Println("Updating posts for subscription", subscription.Id, subscription.Link)

	// Use the new FetchFeed function which automatically detects feed type
	feedData, err := feed.FetchFeed(subscription.Link)
	if err != nil {
		return err
	}

	// Process all feed items
	for _, item := range feedData.Items {
		// Create post in database
		reader.CreatePost(
			feedId,
			item.ID,
			item.Title,
			item.Description,
			item.Link,
			item.PubDate,
		)
	}

	return nil
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
			err := reader.updateFeedPosts(fmt.Sprint(subscription.Id))
			if err != nil {
				log.Printf("Error updating posts for subscription %d: %v\n", subscription.Id, err)
			}
		}
	}
}

func (reader *Reader) ImportOPML(data []byte) (err error) {
	res, err := feed.ParseOPML(data)
	if err != nil {
		return
	}
	for _, outline := range res.Outlines {
		_, err = reader.CreateFeed(outline.Type, outline.Text, outline.HTMLURL, outline.XMLURL, 1)
		if err != nil {
			return
		}
	}
	return
}
