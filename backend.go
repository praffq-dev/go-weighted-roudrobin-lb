package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
)

func main() {
	port := flag.Int("port", 8081, "Port to listen on")
	flag.Parse()

	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("Health check for server running on port %d", *port)
		w.WriteHeader(http.StatusOK)
	})

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "Hello from server on port %d!", *port)
	})

	log.Printf("Listening on port %d", *port)
	err := http.ListenAndServe(fmt.Sprintf(":%d", *port), nil)
	if err != nil {
		log.Fatal(err)
	}
}
