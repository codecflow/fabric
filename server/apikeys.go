package server

import (
	"encoding/json"
	"math/rand"
	"net/http"
	"time"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

type APIKeyRequest struct {
	Description string   `json:"description"`
	Permissions []string `json:"permissions"`
}

type APIKeyResponse struct {
	Key         string   `json:"key"`
	Description string   `json:"description"`
	Permissions []string `json:"permissions"`
}

func (s *Server) CreateAPIKey(w http.ResponseWriter, r *http.Request) {
	if !Method(w, r, http.MethodPost) {
		return
	}

	var req APIKeyRequest
	if err := JSON(r, &req); err != nil {
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	key := generateAPIKey()
	s.auth.AddAPIKey(key, req.Description, req.Permissions)

	resp := APIKeyResponse{
		Key:         key,
		Description: req.Description,
		Permissions: req.Permissions,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (s *Server) ListAPIKeys(w http.ResponseWriter, r *http.Request) {
	if !Method(w, r, http.MethodGet) {
		return
	}

	keys := make([]APIKeyResponse, 0, len(s.auth.APIKeys))
	for _, key := range s.auth.APIKeys {
		keys = append(keys, APIKeyResponse{
			Key:         key.Key,
			Description: key.Description,
			Permissions: key.Permissions,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(keys)
}

func (s *Server) DeleteAPIKey(w http.ResponseWriter, r *http.Request) {
	if !Method(w, r, http.MethodDelete) {
		return
	}

	key := r.URL.Query().Get("key")
	if key == "" {
		http.Error(w, "Missing key parameter", http.StatusBadRequest)
		return
	}

	if _, ok := s.auth.APIKeys[key]; !ok {
		http.Error(w, "API key not found", http.StatusNotFound)
		return
	}

	delete(s.auth.APIKeys, key)
	w.WriteHeader(http.StatusOK)
}

func generateAPIKey() string {
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, 32)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}
