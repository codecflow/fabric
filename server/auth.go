package server

import (
	"net/http"
	"strings"
)

// AuthConfig holds authentication configuration
type AuthConfig struct {
	APIKeys map[string]APIKey
}

// APIKey represents an API key with associated permissions
type APIKey struct {
	Key         string
	Description string
	Permissions []string
}

// NewAuthConfig creates a new authentication configuration
func NewAuthConfig() *AuthConfig {
	return &AuthConfig{
		APIKeys: make(map[string]APIKey),
	}
}

// AddAPIKey adds an API key to the configuration
func (c *AuthConfig) AddAPIKey(key, description string, permissions []string) {
	c.APIKeys[key] = APIKey{
		Key:         key,
		Description: description,
		Permissions: permissions,
	}
}

// AuthMiddleware creates a middleware that checks for valid API keys
func (s *Server) AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip auth for health and version endpoints
		if strings.HasPrefix(r.URL.Path, "/health") || strings.HasPrefix(r.URL.Path, "/version") {
			next.ServeHTTP(w, r)
			return
		}

		// Get API key from header or query parameter
		apiKey := r.Header.Get("X-API-Key")
		if apiKey == "" {
			apiKey = r.URL.Query().Get("api_key")
		}

		// Check if API key is valid
		if apiKey == "" {
			http.Error(w, "API key is required", http.StatusUnauthorized)
			return
		}

		// Check if API key exists
		_, ok := s.auth.APIKeys[apiKey]
		if !ok {
			http.Error(w, "Invalid API key", http.StatusUnauthorized)
			return
		}

		// TODO: Check permissions based on endpoint and method

		// API key is valid, proceed to next handler
		next.ServeHTTP(w, r)
	})
}
