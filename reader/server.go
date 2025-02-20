package reader

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/lsongdev/feedreader/feed"
	"github.com/lsongdev/feedreader/templates"
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

type Pagination struct {
	Page  int
	Size  int
	Total int64
}

func (limit Pagination) Offset() int {
	return (limit.Page - 1) * limit.Size
}

func (limit Pagination) SQL() string {
	return fmt.Sprintf(" LIMIT %d OFFSET %d", limit.Size, limit.Offset())
}

func (limit Pagination) Prev() int {
	return limit.Page - 1
}

func (limit Pagination) Next() int {
	return limit.Page + 1
}

func (limit Pagination) HasMore() bool {
	return limit.Total > int64(limit.Page*limit.Size)
}

func (limit Pagination) PageCount() int {
	return int(limit.Total/int64(limit.Size)) + 1
}

func NewLimitFromQuery(query url.Values) *Pagination {
	limit := &Pagination{Page: 1, Size: 100}
	if query.Has("page") {
		limit.Page, _ = strconv.Atoi(query.Get("page"))
	}
	if query.Has("size") {
		limit.Size, _ = strconv.Atoi(query.Get("size"))
	}
	return limit
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
		category := r.FormValue("category")
		categoryId, _ := strconv.Atoi(category)
		id, err := reader.CreateFeed(feedType, name, home, link, categoryId)
		if err != nil {
			reader.Error(w, err)
			return
		}
		http.Redirect(w, r, "/", http.StatusFound)
		go reader.updateFeedPosts(fmt.Sprint(id))
		return
	}

	categories, err := reader.GetCategories()
	if err != nil {
		reader.Error(w, err)
		return
	}

	var feedType, name, home, link string
	url := r.URL.Query().Get("url")
	link = url
	feedData, err := feed.FetchFeed(link)
	if err == nil {
		feedType = string(feedData.Type)
		name = feedData.Title
		home = feedData.Link
	}
	reader.Render(w, "new", H{
		"categories": categories,
		"type":       feedType,
		"name":       name,
		"home":       home,
		"link":       link,
		"url":        url,
	})
}

func (reader *Reader) FeedsView(w http.ResponseWriter, r *http.Request) {
	var conditions []string
	if r.URL.Query().Has("category") {
		categoryId := r.URL.Query().Get("category")
		conditions = append(conditions, fmt.Sprintf("g.id = %s", categoryId))
	}
	feeds, err := reader.GetFeeds(conditions)
	if err != nil {
		reader.Error(w, err)
		return
	}
	categories, err := reader.GetCategories()
	if err != nil {
		reader.Error(w, err)
		return
	}
	reader.Render(w, "feeds", H{
		"feeds":      feeds,
		"categories": categories,
	})
}

func (reader *Reader) PostsView(w http.ResponseWriter, r *http.Request) {
	feedId := r.URL.Query().Get("id")
	feed, err := reader.GetFeed(feedId)
	if err != nil {
		reader.Error(w, err)
		return
	}
	limit := NewLimitFromQuery(r.URL.Query())
	posts, err := reader.GetPostsByFeedId(feedId, limit)
	if err != nil {
		reader.Error(w, err)
		return
	}
	reader.Render(w, "posts", H{
		"feed":       feed,
		"posts":      posts,
		"pagination": limit,
	})
}

func (reader *Reader) DeleteFeedView(w http.ResponseWriter, r *http.Request) {
	feedId := r.URL.Query().Get("id")
	err := reader.DeleteFeed(feedId)
	if err != nil {
		reader.Error(w, err)
		return
	}
	w.WriteHeader(http.StatusOK)
}

// FeedView handles requests to the feed page.
func (reader *Reader) FeedView(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		if r.URL.Query().Has("id") {
			reader.PostsView(w, r)
		} else {
			reader.FeedsView(w, r)
		}
	case "DELETE":
		reader.DeleteFeedView(w, r)
	}

}

// PostView handles requests to view a specific post.
func (reader *Reader) PostView(w http.ResponseWriter, r *http.Request) {
	if r.URL.Query().Has("id") {
		id := r.URL.Query().Get("id")
		post, err := reader.GetPost(id)
		if err != nil {
			reader.Error(w, err)
			return
		}
		reader.Render(w, "post", H{
			"post": post,
			"body": template.HTML(post.Content),
		})
		return
	}
	var conditions []string
	if r.URL.Query().Has("unread") {
		conditions = append(conditions, "is_read = 0")
	}
	if r.URL.Query().Has("readed") {
		conditions = append(conditions, "is_read = 1")
	}
	if r.URL.Query().Has("saved") {
		conditions = append(conditions, "is_saved = 1")
	}
	if r.URL.Query().Has("category") {
		categoryId := r.URL.Query().Get("category")
		conditions = append(conditions, fmt.Sprintf("g.id = %s", categoryId))
	}
	limit := NewLimitFromQuery(r.URL.Query())
	posts, err := reader.GetPosts(conditions, limit)
	if err != nil {
		reader.Error(w, err)
		return
	}
	// Render the template with the data
	reader.Render(w, "posts", H{
		"posts":      posts,
		"pagination": limit,
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
	posts, err := reader.GetPosts(nil, nil)
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
	posts, err := reader.GetPosts(nil, nil)
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
	out := &feed.OPML{
		Title:     reader.config.Title,
		CreatedAt: time.Now(),
	}
	for _, subscription := range subscriptions {
		out.Outlines = append(out.Outlines, feed.Outline{
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
	posts, err := reader.GetPosts(nil, nil)
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

func (reader *Reader) CategoryView(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		return
	}
	r.ParseForm()
	if !reader.CheckAuth(w, r) {
		return
	}
	categoryName := r.FormValue("name")
	categoryId, err := reader.CreateCategory(categoryName)
	if err != nil {
		reader.Error(w, err)
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/posts?category=%d", categoryId), http.StatusFound)
}

func (reader *Reader) RefreshView(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		go reader.updatePostsPeriodically()
		http.Redirect(w, r, "/posts", http.StatusFound)
	} else {
		reader.updateFeedPosts(id)
		http.Redirect(w, r, fmt.Sprintf("/feeds?id=%s", id), http.StatusFound)
	}
}
