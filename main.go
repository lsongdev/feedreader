package main

import (
	"net/http"

	_ "github.com/glebarez/go-sqlite"
)

func main() {
	reader, err := NewReader()
	if err != nil {
		panic(err)
	}

	http.HandleFunc("/", reader.IndexView)
	http.HandleFunc("/new", reader.NewView)
	http.HandleFunc("/posts", reader.PostView)
	http.HandleFunc("/subscriptions", reader.SubscriptionsView)
	http.HandleFunc("/subscriptions.xml", reader.SubscriptionsXML)
	err = http.ListenAndServe(":8080", nil)
	if err != nil {
		panic(err)
	}
}
