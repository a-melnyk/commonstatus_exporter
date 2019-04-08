package main

import (
	"log"
	"net/http"
	"time"
)

func main() {

	err := http.ListenAndServe(
		":8081",
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(15 * time.Second)
			http.ServeFile(w, r, "/valid_metrics.txt")
		}),
	)
	log.Fatal(err)
}
