package main

import (
	"database/sql"
	"html/template"
	"log"
	"net/http"
	"os"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/song940/feedreader/feed"
	"github.com/song940/feedreader/opml"
)

type H map[string]interface{}

type ReaderFeed struct {
	feed.RssFeed
	Id        uint64
	CreatedAt time.Time
}

type ReaderItem struct {
	feed.RssItem
	Id        uint64 `db:"id"`
	CreatedAt time.Time
}

type Storage struct {
	db *sql.DB
}

func NewStorage() (store *Storage, err error) {
	db, err := sql.Open("sqlite3", "feedreader.db")
	if err != nil {
		return
	}
	store = &Storage{db}
	return
}

func (s *Storage) Init() (err error) {
	sql := `
		create table if not exists feeds (
      id INTEGER PRIMARY KEY,
			title text not null,
			link text not null,
			created_at timestamp default CURRENT_TIMESTAMP
		);
		create table if not exists entries (
			id INTEGER PRIMARY KEY,
			feed_id INTEGER not null,
			title text not null,
			link text not null,
			content text not null,
			pubdate timestamp not null,
			created_at timestamp default CURRENT_TIMESTAMP,
	    FOREIGN KEY (feed_id) REFERENCES feeds(id)
		);
		`
	_, err = s.db.Exec(sql)
	return
}

func (s *Storage) insertItem(item *feed.RssItem) (err error) {
	sql := `insert into feeds (title, link, description, pubdate) values (?, ?, ?, ?)`
	_, err = s.db.Exec(sql, item.Title, item.Link, item.Description, item.PubDate)
	return
}

func (s *Storage) addFeed(feed *feed.RssFeed) (out *ReaderFeed, err error) {
	sql := `insert into feeds (title, link) values (?, ?) returning id`
	err = s.db.QueryRow(sql, feed.Title, feed.Link, feed.Link).Scan(&out.Id)
	return
}

type Reader struct {
	store *Storage
}

func New() (reader *Reader, err error) {
	store, err := NewStorage()
	if err != nil {
		return
	}
	store.Init()
	reader = &Reader{store}
	f, err := os.ReadFile("./subscriptions.opml")
	if err != nil {
		log.Fatal(err)
	}
	feeds, err := opml.ParseOPML(f)
	if err != nil {
		log.Fatal(err)
	}
	go reader.Update(feeds)
	return
}

func (r *Reader) Update(feeds *opml.OPML) {
	for _, outline := range feeds.Outlines {
		rss, err := feed.FetchRss(outline.XMLURL)
		if err != nil {
			log.Fatal(err)
		}
		log.Println(rss.Title)
		for _, item := range rss.Items {
			r.store.insertItem(&item)
			if err != nil {
				log.Fatal(err)
			}
		}
	}
}

func (r *Reader) Reader(w http.ResponseWriter, name string, data H) {
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

func (reader *Reader) IndexView(w http.ResponseWriter, r *http.Request) {
	items, err := reader.store
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	reader.Reader(w, "index", H{
		"items": items,
	})
}

func (reader *Reader) SubscribeView(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		reader.Reader(w, "new", H{})
		return
	}
	if r.Method == http.MethodPost {
		r.ParseForm()
		feedLink := r.FormValue("link")
		feed, err := feed.FetchRss(feedLink)
		if err != nil {
			log.Fatal(err)
		}
		reader.store.addFeed(feed)
	}
}
