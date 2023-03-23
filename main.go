package main

import (
	"bytes"
	"database/sql"
	"encoding/xml"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"

	"github.com/mattn/go-sqlite3"
	_ "github.com/mattn/go-sqlite3"
)

type Outline struct {
	Text   string `xml:"text,attr"`
	XMLURL string `xml:"xmlUrl,attr"`
}

type OPML struct {
	Outlines []Outline `xml:"body>outline"`
}

func ParseOPML(data []byte) (opml *OPML, err error) {
	err = xml.Unmarshal(data, &opml)
	if err != nil {
		return
	}
	return
}

type Item struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	Description string `xml:"description"`
	PubDate     string `xml:"pubDate"`
}

type Feed struct {
	Title       string `xml:"channel>title"`
	Link        string `xml:"channel>link"`
	Description string `xml:"channel>description"`
	Items       []Item `xml:"channel>item"`
}

func FetchFeed(url string) (feed Feed, err error) {
	res, err := http.Get(url)
	if err != nil {
		log.Fatal(err)
		return
	}
	if res.StatusCode != 200 {
		err = fmt.Errorf("status code: %d", res.StatusCode)
		return
	}
	data, err := io.ReadAll(res.Body)
	if err != nil {
		log.Fatal(err)
	}
	data = bytes.Map(func(r rune) rune {
		if r == '\u0008' {
			return -1
		}
		return r
	}, data)
	err = xml.Unmarshal(data, &feed)
	return
}

func insertItem(db *sql.DB, item *Item) error {
	stmt, err := db.Prepare(`
			INSERT INTO feeds (title, link, description, pubdate)
			VALUES (?, ?, ?, ?)
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	_, err = stmt.Exec(item.Title, item.Link, item.Description, item.PubDate)
	if err != nil {
		if sqliteErr, ok := err.(sqlite3.Error); ok {
			if sqliteErr.Code == sqlite3.ErrConstraint {
				log.Println("Record is exists", item.Title)
				// Ignore unique constraint errors
				return nil
			}
		}
		return err
	}
	return nil
}

func initDatabase(db *sql.DB) error {
	sql := `
		CREATE TABLE IF NOT EXISTS feeds (
				id INTEGER PRIMARY KEY,
				title TEXT NOT NULL,
				link TEXT UNIQUE NOT NULL,
				description TEXT NOT NULL,
				pubdate INTEGER NOT NULL
		);`
	_, err := db.Exec(sql)
	if err != nil {
		return err
	}
	return nil
}

func main() {
	db, err := sql.Open("sqlite3", "rss.db")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	err = initDatabase(db)
	if err != nil {
		log.Fatal(err)
	}
	f, err := os.ReadFile("./subscriptions.opml")
	if err != nil {
		log.Fatal(err)
	}
	feeds, err := ParseOPML(f)
	if err != nil {
		log.Fatal(err)
	}
	for _, outline := range feeds.Outlines {
		rss, err := FetchFeed(outline.XMLURL)
		if err != nil {
			log.Fatal(err)
		}
		log.Println(rss.Title)
		for _, item := range rss.Items {
			err = insertItem(db, &item)
			if err != nil {
				log.Fatal(err)
			}
			// log.Println("Inserted", item.Title)
		}
	}
}
