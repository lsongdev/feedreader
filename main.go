package main

import (
	"log"
	"net/http"

	"github.com/song940/feedreader/reader"
	"github.com/song940/fever-go/fever"
)

type debugHandler struct {
	Fever *fever.Fever
}

func (h debugHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	log.Println("Fever:", r.Method, r.URL, r.Form)
	h.Fever.ServeHTTP(w, r)
}

func main() {
	reader, err := reader.NewReader()
	if err != nil {
		panic(err)
	}
	api := fever.New(reader)

	debug := debugHandler{Fever: api}
	http.HandleFunc("/", reader.IndexView)
	http.HandleFunc("/new", reader.NewView)
	http.HandleFunc("/posts", reader.PostView)
	http.HandleFunc("/feeds", reader.FeedView)
	http.HandleFunc("/import", reader.ImportView)
	http.HandleFunc("/rss.xml", reader.RssXML)
	http.HandleFunc("/atom.xml", reader.AomXML)
	http.HandleFunc("/opml.xml", reader.OpmlXML)
	http.Handle("/fever/", debug)
	err = http.ListenAndServe(":8080", nil)
	if err != nil {
		panic(err)
	}
}
