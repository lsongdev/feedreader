package main

import (
	"database/sql"
	"html/template"
	"log"
	"net/http"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/song940/feedparser-go/feed"
)

type H map[string]interface{}

// Reader represents the main application struct.
type Reader struct {
	db   *sql.DB
	tick *time.Ticker
}

// New initializes a new instance of the Reader application.
func New() (reader *Reader, err error) {
	// Open an in-memory SQLite database for demonstration
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		return nil, err
	}

	// Create subscriptions table
	if _, err = db.Exec(`
		CREATE TABLE subscriptions (
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
		CREATE TABLE posts (
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

// Render renders an HTML template with the provided data.
func (reader *Reader) Render(w http.ResponseWriter, name string, data H) {
	t, err := template.ParseFiles("./templates/" + name + ".html")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Add("Content-Type", "text/html; charset=utf-8")
	err = t.Execute(w, data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// CreateSubscription adds a new subscription to the database.
func (reader *Reader) CreateSubscription(name, home, link string) (string, error) {
	var id string
	err := reader.db.QueryRow(`
		INSERT INTO subscriptions (name, home, link) VALUES (?, ?, ?) RETURNING id
	`, name, home, link).Scan(&id)
	return id, err
}

// GetSubscriptions retrieves all subscriptions from the database.
func (reader *Reader) GetSubscriptions() ([]map[string]interface{}, error) {
	rows, err := reader.db.Query("SELECT id, name, home, link, created_at FROM subscriptions ORDER BY created_at DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var subscriptions []map[string]interface{}

	for rows.Next() {
		var id, name, home, link, createdAt string
		err := rows.Scan(&id, &name, &home, &link, &createdAt)
		if err != nil {
			return nil, err
		}
		subscription := map[string]interface{}{
			"id":         id,
			"name":       name,
			"home":       home,
			"link":       link,
			"created_at": createdAt,
		}
		subscriptions = append(subscriptions, subscription)
	}

	return subscriptions, nil
}

// GetSubscription retrieves a specific subscription from the database.
func (reader *Reader) GetSubscription(id string) (map[string]interface{}, error) {
	row := reader.db.QueryRow("SELECT name, home, link, created_at FROM subscriptions WHERE id = ?", id)

	var name, home, link string
	var createdAt string // Change the type based on the actual type in your database

	err := row.Scan(&name, &home, &link, &createdAt)
	if err != nil {
		return nil, err
	}
	subscription := map[string]interface{}{
		"id":         id,
		"name":       name,
		"home":       home,
		"link":       link,
		"created_at": createdAt,
	}
	return subscription, nil
}

// GetSubscriptionPosts retrieves posts for a specific subscription.
func (reader *Reader) GetSubscriptionPosts(id string) ([]map[string]interface{}, error) {
	rows, err := reader.db.Query(`
		SELECT id, title, content, created_at
		FROM posts
		WHERE subscription_id = ?
		ORDER BY created_at DESC
	`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var posts []map[string]interface{}

	for rows.Next() {
		var id int
		var title, content string
		var createdAt string // Change the type based on the actual type in your database

		err := rows.Scan(&id, &title, &content, &createdAt)
		if err != nil {
			return nil, err
		}

		post := map[string]interface{}{
			"id":         id,
			"title":      title,
			"created_at": createdAt,
		}

		posts = append(posts, post)
	}

	return posts, nil
}

// updatePostsPeriodically periodically updates posts for all subscriptions.
func (reader *Reader) updatePostsPeriodically() {
	for range reader.tick.C {
		// Get all subscriptions and update posts for each
		subscriptions, err := reader.GetSubscriptions()
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

// updateSubscriptionPosts fetches new articles for a subscription and saves them to the database.
func (reader *Reader) updateSubscriptionPosts(id string) error {
	subscrition, err := reader.GetSubscription(id)
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

// CreatePost adds a new post to the database.
func (reader *Reader) CreatePost(subscriptionID string, title, content, link, pubDate string) error {
	_, err := reader.db.Exec(`
		INSERT INTO posts (title, content, link, created_at, subscription_id) VALUES (?, ?, ?, ?, ?)
	`, title, content, link, pubDate, subscriptionID)
	return err
}

func (reader *Reader) GetPost(id string) (map[string]interface{}, error) {
	row := reader.db.QueryRow(`
		SELECT title, content, link, created_at
		FROM posts
		WHERE id = ?
	`, id)
	var title, content, link, createdAt string
	err := row.Scan(&title, &content, &link, &createdAt)
	if err != nil {
		return nil, err
	}
	post := map[string]interface{}{
		"title":      title,
		"content":    template.HTML(content),
		"link":       link,
		"created_at": createdAt,
	}
	return post, nil
}

// IndexView handles requests to the home page.
func (reader *Reader) IndexView(w http.ResponseWriter, r *http.Request) {
	subscriptions, err := reader.GetSubscriptions()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	reader.Render(w, "index", H{
		"subscriptions": subscriptions,
	})
}

// NewView handles requests to the new subscription page.
func (reader *Reader) NewView(w http.ResponseWriter, r *http.Request) {

	if r.Method == "POST" {
		name := r.FormValue("name")
		home := r.FormValue("home")
		link := r.FormValue("link")
		id, err := reader.CreateSubscription(name, home, link)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		http.Redirect(w, r, "/", http.StatusFound)
		go reader.updateSubscriptionPosts(id)
		return
	}

	var name, home, link string
	url := r.URL.Query().Get("url")
	link = url
	if link != "" {
		rss, err := feed.FetchRss(link)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		name = rss.Title
		home = rss.Link
	}
	reader.Render(w, "new", H{
		"name": name,
		"home": home,
		"link": link,
		"url":  url,
	})
}

func (reader *Reader) SubscriptionView(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	subscrition, err := reader.GetSubscription(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	posts, err := reader.GetSubscriptionPosts(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	reader.Render(w, "list", H{
		"subscription": subscrition,
		"posts":        posts,
	})
}

func (reader *Reader) PostView(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	post, err := reader.GetPost(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	reader.Render(w, "post", H{
		"post": post,
	})
}

func main() {
	reader, err := New()
	if err != nil {
		panic(err)
	}

	http.HandleFunc("/", reader.IndexView)
	http.HandleFunc("/new", reader.NewView)
	http.HandleFunc("/subscription", reader.SubscriptionView)
	http.HandleFunc("/post", reader.PostView)
	http.ListenAndServe(":8080", nil)
}
