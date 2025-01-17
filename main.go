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
	"strings"
	"time"
)

func createHandler(w http.ResponseWriter, r *http.Request) {
	// Add CORS headers
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-Stream-ID")

	// Handle preflight request
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	log.Println("Received request to create resource")

	namespace := "default"
	streamID := r.Header.Get("X-Stream-ID")
	if streamID == "" {
		http.Error(w, "Stream ID is required", http.StatusBadRequest)
		return
	}

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
		// Add CORS headers
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-Stream-ID")

		// Handle preflight request
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		streamID := r.Header.Get("X-Stream-ID")
		if streamID == "" {
			http.Error(w, "Stream ID not provided", http.StatusBadRequest)
			return
		}

		targetURL := fmt.Sprintf("http://streamer-%s.entrypoint.default.svc.cluster.local:9222", streamID)
		target, err := url.Parse(targetURL)
		if err != nil {
			log.Printf("Failed to parse target URL: %v", err)
			http.Error(w, "Invalid target URL", http.StatusInternalServerError)
			return
		}

		proxy := httputil.NewSingleHostReverseProxy(target)

		// Modify the director to handle paths correctly
		originalDirector := proxy.Director
		proxy.Director = func(req *http.Request) {
			originalDirector(req)

			// Remove the '/proxy' prefix from the path
			req.URL.Path = strings.TrimPrefix(req.URL.Path, "/proxy")
			if req.URL.Path == "" {
				req.URL.Path = "/"
			}

			req.URL.Host = target.Host
			req.URL.Scheme = target.Scheme

			log.Printf("Proxying request to: %s%s", targetURL, req.URL.Path)
		}

		// Add error handler
		proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
			log.Printf("Proxy error: %v", err)
			http.Error(w, fmt.Sprintf("Proxy error: %v", err), http.StatusBadGateway)
		}

		proxy.ServeHTTP(w, r)
	}
}

func main() {
	// Create a custom server with timeouts
	server := &http.Server{
		Addr:         ":9000",
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	http.HandleFunc("/create", createHandler)
	http.HandleFunc("/proxy/", proxyHandler()) // Note the trailing slash

	log.Println("Starting server on :9000")
	log.Fatal(server.ListenAndServe())
}
