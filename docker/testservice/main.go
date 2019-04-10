package main

import (
	"log"
	"net/http"
)

func main() {

	err := http.ListenAndServe(
		":8081",
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.ServeFile(w, r, "/valid_metrics.txt")
		}),
	)
	log.Fatal(err)
}
