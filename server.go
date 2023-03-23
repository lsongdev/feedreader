package main

import (
	"embed"
	"html/template"
	"net/http"
	"strconv"

	"github.com/song940/feedparser-go/feed"
)

//go:embed templates/*.html
var templatefiles embed.FS

type H map[string]interface{}

// Render renders an HTML template with the provided data.
func (reader Reader) Render(w http.ResponseWriter, templateName string, data H) {
	// tmpl, err := template.ParseFiles("templates/layout.html", "templates/"+templateName+".html")
	// Parse templates from embedded file system
	tmpl, err := template.New("").ParseFS(templatefiles, "templates/layout.html", "templates/"+templateName+".html")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Execute "index.html" within the layout and write to response
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	err = tmpl.ExecuteTemplate(w, "layout", data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// FeedView handles requests to the feed page.
func (reader *Reader) FeedView(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		subscriptions, err := reader.GetFeeds()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		reader.Render(w, "feeds", H{
			"subscriptions": subscriptions,
		})
		return
	}
	idInt, err := strconv.Atoi(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	subscription, err := reader.GetFeed(idInt)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	posts, err := reader.GetPostsBySubscriptionId(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	reader.Render(w, "posts", H{
		"subscription": subscription,
		"posts":        posts,
	})
}

// PostView handles requests to view a specific post.
func (reader *Reader) PostView(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		posts, err := reader.GetPosts()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		// Render the template with the data
		reader.Render(w, "posts", H{
			"posts": posts,
		})
		return
	}
	post, err := reader.GetPost(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	reader.Render(w, "post", H{
		"post": post,
	})
}

// IndexView handles requests to the home page.
func (reader *Reader) IndexView(w http.ResponseWriter, r *http.Request) {
	reader.PostView(w, r)
}

// NewView handles requests to the new subscription page.
func (reader *Reader) NewView(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {
		name := r.FormValue("name")
		home := r.FormValue("home")
		link := r.FormValue("link")
		id, err := reader.CreateFeed(name, home, link)
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
