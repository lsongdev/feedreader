package reader

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/song940/feedparser-go/feed"
	"github.com/song940/feedparser-go/opml"
	"github.com/song940/feedreader/templates"
	"gopkg.in/yaml.v2"
)

type H map[string]interface{}

type User struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type Config struct {
	Dir        string `json:"-" yaml:"-"`
	Title      string `json:"title" yaml:"title"`
	Listen     string `json:"listen" yaml:"listen"`
	Users      []User `json:"users" yaml:"users"`
	Stylesheet string `json:"stylesheet" yaml:"stylesheet"`
}

func NewConfig() *Config {
	return &Config{
		Title:  "Reader",
		Listen: "0.0.0.0:8080",
		Users:  []User{{"admin", "admin123"}},
	}
}

func (conf *Config) Load() error {
	filename := filepath.Join(conf.Dir, "config.yaml")
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		return err
	}
	data, err := os.ReadFile(filename)
	if err != nil {
		return err
	}
	return yaml.Unmarshal(data, &conf)
}

// Render renders an HTML template with the provided data.
func (reader *Reader) Render(w http.ResponseWriter, templateName string, data H) {
	if data == nil {
		data = H{}
	}
	data["AppName"] = reader.config.Title
	data["Stylesheet"] = template.CSS(reader.config.Stylesheet)
	// tmpl, err := template.ParseFiles("templates/layout.html", "templates/"+templateName+".html")
	// Parse templates from embedded file system
	tmpl, err := template.New("").ParseFS(templates.Files, "layout.html", templateName+".html")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	// Execute "index.html" within the layout and write to response
	err = tmpl.ExecuteTemplate(w, "layout", data)
	if err != nil {
		log.Println(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (reader *Reader) Error(w http.ResponseWriter, err error) {
	reader.Render(w, "error", H{
		"error": err.Error(),
	})
}

// IndexView handles requests to the home page.
func (reader *Reader) IndexView(w http.ResponseWriter, r *http.Request) {
	reader.PostView(w, r)
}

func (reader *Reader) CheckAuth(w http.ResponseWriter, r *http.Request) bool {
	username, password, ok := r.BasicAuth()
	if ok {
		for _, user := range reader.config.Users {
			if username == user.Username && password == user.Password {
				return true
			}
		}
	}
	w.Header().Set("WWW-Authenticate", "Basic realm=Restricted")
	w.WriteHeader(http.StatusUnauthorized)
	reader.Error(w, fmt.Errorf("Unauthorized"))
	return false
}

// NewView handles requests to the new subscription page.
func (reader *Reader) NewView(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {
		if !reader.CheckAuth(w, r) {
			return
		}
		feedType := r.FormValue("type")
		name := r.FormValue("name")
		home := r.FormValue("home")
		link := r.FormValue("link")
		id, err := reader.CreateFeed(feedType, name, home, link)
		if err != nil {
			reader.Error(w, err)
			return
		}
		http.Redirect(w, r, "/", http.StatusFound)
		go reader.updateSubscriptionPosts(id)
		return
	}

	var feedType, name, home, link string
	url := r.URL.Query().Get("url")
	link = url
	if link != "" {
		if feedType == "" {
			rss, err := feed.FetchRss(link)
			if err == nil {
				feedType = "rss"
				name = rss.Title
				home = rss.Link
			}
		}
		if feedType == "" {
			atom, err := feed.FetchAtom(link)
			if err == nil {
				feedType = "atom"
				name = atom.Title.Data
				home = atom.Links[0].Href
			}
		}
	}
	reader.Render(w, "new", H{
		"type": feedType,
		"name": name,
		"home": home,
		"link": link,
		"url":  url,
	})
}

// FeedView handles requests to the feed page.
func (reader *Reader) FeedView(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		feeds, err := reader.GetFeeds(nil)
		if err != nil {
			reader.Error(w, err)
			return
		}
		reader.Render(w, "feeds", H{
			"subscriptions": feeds,
		})
		return
	}
	feedId, err := strconv.Atoi(id)
	if err != nil {
		reader.Error(w, err)
		return
	}
	feed, err := reader.GetFeed(feedId)
	if err != nil {
		reader.Error(w, err)
		return
	}
	posts, err := reader.GetPostsByFeedId(id)
	if err != nil {
		reader.Error(w, err)
		return
	}
	reader.Render(w, "posts", H{
		"subscription": feed,
		"posts":        posts,
	})
}

// PostView handles requests to view a specific post.
func (reader *Reader) PostView(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		posts, err := reader.GetPosts(nil)
		if err != nil {
			reader.Error(w, err)
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
		reader.Error(w, err)
		return
	}
	reader.Render(w, "post", H{
		"post": post,
		"body": template.HTML(post.Content),
	})
}

func (reader *Reader) ImportView(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		reader.Render(w, "import", nil)
		return
	}
	if !reader.CheckAuth(w, r) {
		return
	}
	r.ParseMultipartForm(32 << 20)
	f, _, err := r.FormFile("file")
	if err != nil {
		reader.Error(w, err)
		return
	}
	defer f.Close()
	data, err := io.ReadAll(f)
	if err != nil {
		reader.Error(w, err)
		return
	}
	err = reader.ImportOPML(data)
	if err != nil {
		reader.Error(w, err)
		return
	}
	http.Redirect(w, r, "/feeds", http.StatusFound)
}

func (reader *Reader) RssXml(w http.ResponseWriter, r *http.Request) {
	posts, err := reader.GetPosts(nil)
	if err != nil {
		reader.Error(w, err)
		return
	}
	w.Header().Set("Content-Type", "text/xml; charset=utf-8")
	rss := feed.RssFeed{
		Title: "Reader",
	}
	for _, post := range posts {
		rss.Items = append(rss.Items, feed.RssItem{
			Title:       post.Title,
			Description: post.Content,
			Link:        post.Link,
			PubDate:     post.CreatedAt.Format(time.RFC1123Z),
		})
	}
	err = xml.NewEncoder(w).Encode(rss)
	if err != nil {
		reader.Error(w, err)
		return
	}
}

func (reader *Reader) AomXml(w http.ResponseWriter, r *http.Request) {
	posts, err := reader.GetPosts(nil)
	if err != nil {
		reader.Error(w, err)
		return
	}
	w.Header().Set("Content-Type", "text/xml; charset=utf-8")
	atom := feed.AtomFeed{
		Title:   feed.AtomText{Data: "Reader"},
		Updated: time.Now().Format(time.RFC3339),
		Generator: feed.AtomGenerator{
			Name:    "Reader",
			Version: "1.0.0",
			URI:     "https://github.com/song940/feedreader",
		},
	}
	for _, post := range posts {
		atom.Entries = append(atom.Entries, feed.AtomEntry{
			ID:      fmt.Sprintf("%d", post.Id),
			Title:   feed.AtomText{Data: post.Title},
			Content: feed.AtomText{Data: post.Content, Type: "html"},
			Links:   []feed.AtomLink{{Href: post.Link}},
			Updated: post.CreatedAt.Format(time.RFC3339),
		})
	}
	w.Header().Set("Content-Type", "text/xml; charset=utf-8")
	err = xml.NewEncoder(w).Encode(atom)
	if err != nil {
		reader.Error(w, err)
		return
	}
}

func (reader *Reader) OpmlXml(w http.ResponseWriter, r *http.Request) {
	subscriptions, err := reader.GetFeeds(nil)
	if err != nil {
		reader.Error(w, err)
		return
	}
	out := &opml.OPML{
		Title:     reader.config.Title,
		CreatedAt: time.Now(),
	}
	for _, subscription := range subscriptions {
		out.Outlines = append(out.Outlines, opml.Outline{
			Type:    subscription.Type,
			Title:   subscription.Name,
			Text:    subscription.Name,
			HTMLURL: subscription.Home,
			XMLURL:  subscription.Link,
		})
	}
	w.Header().Set("Content-Type", "text/xml; charset=utf-8")
	err = xml.NewEncoder(w).Encode(out)
	if err != nil {
		reader.Error(w, err)
		return
	}
}

func (reader *Reader) FeedsJson(w http.ResponseWriter, r *http.Request) {
	feeds, err := reader.GetFeeds(nil)
	if err != nil {
		reader.Error(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	err = json.NewEncoder(w).Encode(feeds)
	if err != nil {
		reader.Error(w, err)
		return
	}
}

func (reader *Reader) PostsJson(w http.ResponseWriter, r *http.Request) {
	posts, err := reader.GetPosts(nil)
	if err != nil {
		reader.Error(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	err = json.NewEncoder(w).Encode(posts)
	if err != nil {
		reader.Error(w, err)
		return
	}
}
