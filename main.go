package main

import (
	"captain/server"
	"context"
	"log"
	"net/http"
	"time"
)

func main() {
	server, err := server.NewServer("streamer", "default", "entrypoint", "ghcr.io/codecflow/conductor:1.0.0")
	if err != nil {
		log.Fatalf("Failed to create server: %v", err)
	}

	handler := server.LoggingMiddleware(server.AuthMiddleware(server))

	httpServer := &http.Server{
		Addr:         ":9000",
		Handler:      handler,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go server.StartHealthChecker(ctx)

	log.Println("Starting server on :9000")
	log.Fatal(httpServer.ListenAndServe())
}
