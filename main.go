package main

import (
	"flag"
	"net/http"
	"os"
	"path"
	"time"

	"github.com/song940/feedreader/reader"
	"github.com/song940/fever-go/fever"
)

func main() {
	var root, addr string
	userconfig, err := os.UserConfigDir()
	if err != nil {
		panic(err)
	}
	root = path.Join(userconfig, "feedreader")
	flag.StringVar(&root, "d", root, "working directory")
	flag.StringVar(&addr, "l", ":8080", "address to listen")
	flag.Parse()

	server, err := reader.NewReader(&reader.Config{
		Dir:      root,
		Interval: 5 * time.Minute,
	})
	if err != nil {
		panic(err)
	}
	api := fever.New(server)
	http.HandleFunc("/", server.IndexView)
	http.HandleFunc("/new", server.NewView)
	http.HandleFunc("/posts", server.PostView)
	http.HandleFunc("/feeds", server.FeedView)
	http.HandleFunc("/import", server.ImportView)
	http.HandleFunc("/rss.xml", server.RssXML)
	http.HandleFunc("/atom.xml", server.AomXML)
	http.HandleFunc("/opml.xml", server.OpmlXML)
	http.Handle("/fever/", api)
	err = http.ListenAndServe(addr, nil)
	if err != nil {
		panic(err)
	}
}
