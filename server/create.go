package server

import (
	"captain/k8s"
	"fmt"
	"net/http"

	"github.com/sirupsen/logrus"
)

type CreateRequest struct {
	ID       string            `json:"id"`
	Image    string            `json:"image,omitempty"`
	Template string            `json:"template,omitempty"`
	Tools    []string          `json:"tools,omitempty"`
	Env      map[string]string `json:"env,omitempty"`
}

func (s *Server) Create(w http.ResponseWriter, r *http.Request) {
	if !Method(w, r, http.MethodPost) {
		return
	}

	if r.Header.Get("Content-Type") != "application/json" {
		http.Error(w, "Content-Type must be application/json", http.StatusUnsupportedMediaType)
		return
	}

	var req CreateRequest

	if err := JSON(r, &req); err != nil {
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	// Check if a template is specified
	image := req.Image
	env := req.Env
	if req.Template != "" {
		template, ok := s.templates.Get(req.Template)
		if !ok {
			http.Error(w, fmt.Sprintf("Template %s not found", req.Template), http.StatusBadRequest)
			return
		}

		// Use template image if no specific image is provided
		if image == "" {
			image = template.Image
		}

		// Merge template environment variables with request environment variables
		if env == nil {
			env = make(map[string]string)
		}
		for k, v := range template.Env {
			if _, exists := env[k]; !exists {
				env[k] = v
			}
		}
	}

	if image == "" {
		http.Error(w, "Image or template must be specified", http.StatusBadRequest)
		return
	}

	s.logger.WithFields(logrus.Fields{
		"id":       req.ID,
		"image":    image,
		"template": req.Template,
	}).Info("Creating machine")

	ctx := r.Context()

	_, err := s.client.Create(ctx, k8s.Machine{
		ID:    req.ID,
		Image: image,
		Tools: req.Tools,
		Env:   env,
	})

	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to create resource: %v", err), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}
