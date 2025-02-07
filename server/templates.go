package server

import (
	"encoding/json"
	"net/http"
	"sync"
)

type Template struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Image       string            `json:"image"`
	OS          string            `json:"os"`
	Version     string            `json:"version"`
	Resources   Resources         `json:"resources"`
	Env         map[string]string `json:"env"`
}

type Resources struct {
	CPU    string `json:"cpu"`
	Memory string `json:"memory"`
	Disk   string `json:"disk"`
}

type TemplateStore struct {
	templates map[string]Template
	mu        sync.RWMutex
}

func NewTemplateStore() *TemplateStore {
	store := &TemplateStore{
		templates: make(map[string]Template),
	}

	// Add default templates
	store.templates["windows"] = Template{
		ID:          "windows",
		Name:        "Windows",
		Description: "Windows 11 desktop environment",
		Image:       "ghcr.io/codecflow/windows:11",
		OS:          "windows",
		Version:     "11",
		Resources: Resources{
			CPU:    "2",
			Memory: "4Gi",
			Disk:   "10Gi",
		},
	}

	store.templates["macos"] = Template{
		ID:          "macos",
		Name:        "macOS",
		Description: "macOS desktop environment",
		Image:       "ghcr.io/codecflow/macos:latest",
		OS:          "macos",
		Version:     "latest",
		Resources: Resources{
			CPU:    "2",
			Memory: "4Gi",
			Disk:   "10Gi",
		},
	}

	store.templates["ubuntu"] = Template{
		ID:          "ubuntu",
		Name:        "Ubuntu",
		Description: "Ubuntu Linux desktop environment",
		Image:       "ghcr.io/codecflow/ubuntu:22.04",
		OS:          "linux",
		Version:     "22.04",
		Resources: Resources{
			CPU:    "1",
			Memory: "2Gi",
			Disk:   "5Gi",
		},
	}

	return store
}

func (s *TemplateStore) Get(id string) (Template, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	template, ok := s.templates[id]
	return template, ok
}

func (s *TemplateStore) List() []Template {
	s.mu.RLock()
	defer s.mu.RUnlock()
	templates := make([]Template, 0, len(s.templates))
	for _, template := range s.templates {
		templates = append(templates, template)
	}
	return templates
}

func (s *TemplateStore) Add(template Template) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.templates[template.ID] = template
}

func (s *TemplateStore) Delete(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.templates, id)
}

func (s *Server) ListTemplates(w http.ResponseWriter, r *http.Request) {
	if !Method(w, r, http.MethodGet) {
		return
	}

	templates := s.templates.List()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(templates)
}

func (s *Server) GetTemplate(w http.ResponseWriter, r *http.Request) {
	if !Method(w, r, http.MethodGet) {
		return
	}

	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "Missing template ID", http.StatusBadRequest)
		return
	}

	template, ok := s.templates.Get(id)
	if !ok {
		http.Error(w, "Template not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(template)
}
