package main

import (
	"flag"
	"net/http"
	"os"
	"path"

	"github.com/song940/feedreader/reader"
	"github.com/song940/fever-go/fever"
)

func main() {
	config := reader.NewConfig()
	conf, err := os.UserConfigDir()
	if err != nil {
		conf = "/etc"
	}
	dir := path.Join(conf, "feedreader")
	flag.StringVar(&config.Dir, "d", dir, "config directory")
	flag.Parse()
	config.Load()

	server, err := reader.NewReader(config)
	if err != nil {
		panic(err)
	}
	api := fever.New(server)
	router := http.NewServeMux()
	router.HandleFunc("/", server.IndexView)
	router.HandleFunc("/new", server.NewView)
	router.HandleFunc("/posts", server.PostView)
	router.HandleFunc("/feeds", server.FeedView)
	router.HandleFunc("/import", server.ImportView)
	router.HandleFunc("/rss.xml", server.RssXml)
	router.HandleFunc("/atom.xml", server.AomXml)
	router.HandleFunc("/opml.xml", server.OpmlXml)
	router.HandleFunc("/feeds.json", server.FeedsJson)
	router.HandleFunc("/posts.json", server.PostsJson)
	router.Handle("/fever/", api)
	err = http.ListenAndServe(config.Listen, router)
	if err != nil {
		panic(err)
	}
}
