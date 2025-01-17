package main

import (
	"log"
	"net/http"
	"time"
)

func main() {
	server, err := NewServer()
	if err != nil {
		log.Fatalf("Failed to create server: %v", err)
	}

	httpServer := &http.Server{
		Addr:         ":9000",
		Handler:      server,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	log.Println("Starting server on :9000")
	log.Fatal(httpServer.ListenAndServe())
}
