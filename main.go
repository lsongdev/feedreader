package main

import "net/http"

func main() {
	reader, err := New()
	if err != nil {
		panic(err)
	}

	http.HandleFunc("/", reader.IndexView)
	http.ListenAndServe(":8080", nil)
}
