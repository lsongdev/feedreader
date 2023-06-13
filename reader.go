package main

import (
	"html/template"
	"net/http"
	"os"

	_ "github.com/mattn/go-sqlite3"
	"github.com/song940/feedreader/opml"
)

type H map[string]interface{}

type Reader struct {
	opml *opml.OPML
}

func New() (reader *Reader, err error) {
	f, err := os.ReadFile("./subscriptions.opml")
	if err != nil {
		return
	}
	o, err := opml.ParseOPML(f)
	if err != nil {
		return
	}
	reader = &Reader{
		opml: o,
	}
	return
}

func (r *Reader) Render(w http.ResponseWriter, name string, data H) {
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
	reader.Render(w, "index", H{
		"Title": reader.opml.Title,
		"Feeds": reader.opml.Outlines,
	})
}

func (reader *Reader) SubscribeView(w http.ResponseWriter, r *http.Request) {

}
