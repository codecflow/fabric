package server

import (
	"encoding/json"
	"net/http"
)

type VersionResponse struct {
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	BuildTime string `json:"buildTime"`
}

var (
	version   = "1.0.0"
	commit    = "unknown"
	buildTime = "unknown"
)

func (s *Server) Version(w http.ResponseWriter, r *http.Request) {
	if !Method(w, r, http.MethodGet) {
		return
	}

	resp := VersionResponse{
		Version:   version,
		Commit:    commit,
		BuildTime: buildTime,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
