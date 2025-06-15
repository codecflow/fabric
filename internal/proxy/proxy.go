package proxy

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync"
	"time"

	"fabric/internal/config"
	"fabric/internal/types"
)

// Server manages HTTP proxy for workload access
type Server struct {
	config   *config.ProxyConfig
	server   *http.Server
	routes   map[string]*Route
	routesMu sync.RWMutex
}

// Route represents a proxy route to a workload
type Route struct {
	WorkloadID string
	Target     *url.URL
	Proxy      *httputil.ReverseProxy
	CreatedAt  time.Time
	LastUsed   time.Time
}

// New creates a new proxy server
func New(cfg *config.ProxyConfig) (*Server, error) {
	return &Server{
		config: cfg,
		routes: make(map[string]*Route),
	}, nil
}

// Start starts the proxy server
func (s *Server) Start(ctx context.Context) error {
	if !s.config.Enabled {
		return nil
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleProxy)
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/routes", s.handleRoutes)

	s.server = &http.Server{
		Addr:    fmt.Sprintf(":%d", s.config.Port),
		Handler: s.withLogging(mux),
	}

	go func() {
		log.Printf("Starting proxy server on port %d", s.config.Port)
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("Proxy server error: %v", err)
		}
	}()

	// Start cleanup routine
	go s.cleanupRoutine(ctx)

	return nil
}

// Stop stops the proxy server
func (s *Server) Stop() error {
	if s.server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return s.server.Shutdown(ctx)
	}
	return nil
}

// AddRoute adds a new proxy route for a workload
func (s *Server) AddRoute(workload *types.Workload, targetURL string) error {
	s.routesMu.Lock()
	defer s.routesMu.Unlock()

	target, err := url.Parse(targetURL)
	if err != nil {
		return fmt.Errorf("invalid target URL: %w", err)
	}

	proxy := httputil.NewSingleHostReverseProxy(target)
	proxy.ErrorHandler = s.errorHandler

	route := &Route{
		WorkloadID: workload.ID,
		Target:     target,
		Proxy:      proxy,
		CreatedAt:  time.Now(),
		LastUsed:   time.Now(),
	}

	routeKey := s.getRouteKey(workload)
	s.routes[routeKey] = route

	log.Printf("Added proxy route: %s -> %s", routeKey, targetURL)
	return nil
}

// RemoveRoute removes a proxy route
func (s *Server) RemoveRoute(workload *types.Workload) {
	s.routesMu.Lock()
	defer s.routesMu.Unlock()

	routeKey := s.getRouteKey(workload)
	delete(s.routes, routeKey)

	log.Printf("Removed proxy route: %s", routeKey)
}

// handleProxy handles incoming proxy requests
func (s *Server) handleProxy(w http.ResponseWriter, r *http.Request) {
	// Extract workload info from request
	routeKey := s.extractRouteKey(r)
	if routeKey == "" {
		http.Error(w, "Invalid request path", http.StatusBadRequest)
		return
	}

	s.routesMu.RLock()
	route, exists := s.routes[routeKey]
	s.routesMu.RUnlock()

	if !exists {
		http.Error(w, "Workload not found", http.StatusNotFound)
		return
	}

	// Update last used time
	s.routesMu.Lock()
	route.LastUsed = time.Now()
	s.routesMu.Unlock()

	// Modify request for proxying
	r.URL.Host = route.Target.Host
	r.URL.Scheme = route.Target.Scheme
	r.Header.Set("X-Forwarded-Host", r.Header.Get("Host"))
	r.Header.Set("X-Forwarded-Proto", "http")

	// Proxy the request
	route.Proxy.ServeHTTP(w, r)
}

// handleHealth handles health check requests
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"healthy","timestamp":"` + time.Now().Format(time.RFC3339) + `"}`))
}

// handleRoutes handles route listing requests
func (s *Server) handleRoutes(w http.ResponseWriter, r *http.Request) {
	s.routesMu.RLock()
	defer s.routesMu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	routes := make([]map[string]interface{}, 0, len(s.routes))
	for key, route := range s.routes {
		routes = append(routes, map[string]interface{}{
			"key":        key,
			"workloadId": route.WorkloadID,
			"target":     route.Target.String(),
			"createdAt":  route.CreatedAt.Format(time.RFC3339),
			"lastUsed":   route.LastUsed.Format(time.RFC3339),
		})
	}

	fmt.Fprintf(w, `{"routes":%v}`, routes)
}

// errorHandler handles proxy errors
func (s *Server) errorHandler(w http.ResponseWriter, r *http.Request, err error) {
	log.Printf("Proxy error for %s: %v", r.URL.Path, err)
	http.Error(w, "Service temporarily unavailable", http.StatusBadGateway)
}

// withLogging adds request logging middleware
func (s *Server) withLogging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %v", r.Method, r.URL.Path, time.Since(start))
	})
}

// getRouteKey generates a route key for a workload
func (s *Server) getRouteKey(workload *types.Workload) string {
	return fmt.Sprintf("%s.%s", workload.Name, workload.Namespace)
}

// extractRouteKey extracts route key from request path
func (s *Server) extractRouteKey(r *http.Request) string {
	path := strings.TrimPrefix(r.URL.Path, "/")
	parts := strings.Split(path, "/")

	if len(parts) < 1 || parts[0] == "" {
		return ""
	}

	// Support both direct access and subdomain-style routing
	if strings.Contains(parts[0], ".") {
		return parts[0]
	}

	// Extract from Host header for subdomain routing
	host := r.Header.Get("Host")
	if host != "" && strings.Contains(host, ".") {
		subdomain := strings.Split(host, ".")[0]
		if subdomain != "www" && subdomain != "" {
			return subdomain
		}
	}

	return parts[0]
}

// cleanupRoutine periodically cleans up unused routes
func (s *Server) cleanupRoutine(ctx context.Context) {
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.cleanupUnusedRoutes()
		}
	}
}

// cleanupUnusedRoutes removes routes that haven't been used recently
func (s *Server) cleanupUnusedRoutes() {
	s.routesMu.Lock()
	defer s.routesMu.Unlock()

	cutoff := time.Now().Add(-24 * time.Hour) // Remove routes unused for 24h

	for key, route := range s.routes {
		if route.LastUsed.Before(cutoff) {
			delete(s.routes, key)
			log.Printf("Cleaned up unused route: %s", key)
		}
	}
}

// GetRouteStats returns statistics about proxy routes
func (s *Server) GetRouteStats() map[string]interface{} {
	s.routesMu.RLock()
	defer s.routesMu.RUnlock()

	return map[string]interface{}{
		"totalRoutes": len(s.routes),
		"timestamp":   time.Now().Format(time.RFC3339),
	}
}
