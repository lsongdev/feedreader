package main

import (
	"database/sql"
	"html/template"
	"log"
	"net/http"
	"os"

	_ "github.com/mattn/go-sqlite3"
	"github.com/song940/feedreader/opml"
	"github.com/song940/feedreader/parsers"
)

type H map[string]interface{}

type ReaderItem struct {
	parsers.RssItem
	Id uint64 `db:"id"`
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
      title TEXT NOT NULL,
      link TEXT UNIQUE NOT NULL,
      description TEXT NOT NULL,
      pubdate INTEGER NOT NULL
		);`
	_, err = s.db.Exec(sql)
	return
}

func (s *Storage) insertItem(item *parsers.RssItem) (err error) {
	sql := `insert into feeds (title, link, description, pubdate) values (?, ?, ?, ?)`
	_, err = s.db.Exec(sql, item.Title, item.Link, item.Description, item.PubDate)
	return
}

func (s *Storage) getItems() (items []*ReaderItem, err error) {
	sql := `select * from feeds`
	rows, err := s.db.Query(sql)
	if err != nil {
		return
	}
	defer rows.Close()
	for rows.Next() {
		item := &ReaderItem{}
		err = rows.Scan(&item.Id, &item.Title, &item.Link, &item.Description, &item.PubDate)
		if err != nil {
			return
		}
		items = append(items, item)
	}
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
		rss, err := parsers.FetchRss(outline.XMLURL)
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
	items, err := reader.store.getItems()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	reader.Reader(w, "index", H{
		"items": items,
	})
}
