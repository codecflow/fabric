package main

import (
	"log"
	"net/http"
	"time"

	"captain/server"
)

func main() {
	server, err := server.NewServer("streamer", "default", "entrypoint", "ghcr.io/codecflow/conductor:1.0.0")
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
