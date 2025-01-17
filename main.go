package main

import (
	"captain/k8s"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
)

func createHandler(w http.ResponseWriter, r *http.Request) {
	// Add CORS headers
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	// Handle preflight request
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	log.Println("Received request to create resource")

	namespace := "default"
	streamID := r.Header.Get("X-Stream-ID")
	client, err := k8s.NewClient()
	if err != nil {
		log.Printf("Failed to create Kubernetes client: %v", err)
		http.Error(w, "Failed to create Kubernetes client", http.StatusInternalServerError)
		return
	}

	ctx := context.Background()

	_, err = client.Create(ctx, namespace, streamID)
	if err != nil {
		log.Printf("Failed to create resource in namespace %s: %v", namespace, err)
		http.Error(w, "Failed to create resource", http.StatusInternalServerError)
		return
	}

	log.Printf("Successfully created resource in namespace %s", namespace)

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "success"})
}

func proxyHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		streamID := r.Header.Get("X-Stream-ID")
		if streamID == "" {
			http.Error(w, "Stream ID not provided", http.StatusBadRequest)
			return
		}

		targetURL := fmt.Sprintf("http://streamer-%s.entrypoint.default.svc.cluster.local:9222", streamID)
		target, err := url.Parse(targetURL)
		if err != nil {
			http.Error(w, "Invalid target URL", http.StatusInternalServerError)
			return
		}

		proxy := httputil.NewSingleHostReverseProxy(target)
		proxy.ServeHTTP(w, r)
	}
}

func main() {
	http.HandleFunc("/create", createHandler)
	http.HandleFunc("/proxy", proxyHandler())

	log.Println("Starting server on :9000")
	log.Fatal(http.ListenAndServe(":9000", nil))
}
