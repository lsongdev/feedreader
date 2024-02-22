package main

import (
	"net/http"

	"github.com/song940/feedreader/reader"
	"github.com/song940/fever-go/fever"
)

func main() {
	reader, err := reader.NewReader()
	if err != nil {
		panic(err)
	}
	api := fever.New(reader)
	http.HandleFunc("/", reader.IndexView)
	http.HandleFunc("/new", reader.NewView)
	http.HandleFunc("/posts", reader.PostView)
	http.HandleFunc("/feeds", reader.FeedView)
	http.HandleFunc("/rss.xml", reader.RssXML)
	http.HandleFunc("/atom.xml", reader.AomXML)
	http.HandleFunc("/opml.xml", reader.OpmlXML)
	http.Handle("/fever/", api)
	err = http.ListenAndServe(":8080", nil)
	if err != nil {
		panic(err)
	}
}
