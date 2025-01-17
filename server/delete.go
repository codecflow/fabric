package server

import "net/http"

func (s *Server) Delete(w http.ResponseWriter, r *http.Request) {
	if !Method(w, r, http.MethodDelete) {
		return
	}

	id := r.URL.Query().Get("id")

	if id == "" {
		http.Error(w, "Missing id parameter", http.StatusBadRequest)
		return
	}

	ctx := r.Context()

	err := s.client.Delete(ctx, id)

	if err != nil {
		http.Error(w, "Failed to delete resource", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}
