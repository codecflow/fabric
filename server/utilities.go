package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
)

func JSON[T any](r *http.Request, dest *T) error {
	if r.Body == nil {
		return errors.New("request body is empty")
	}
	defer r.Body.Close()

	if contentType := r.Header.Get("Content-Type"); !strings.HasPrefix(contentType, "application/json") {
		return errors.New("Content-Type must be application/json")
	}

	decoder := json.NewDecoder(r.Body)
	return decoder.Decode(dest)
}

func Method(w http.ResponseWriter, r *http.Request, method string) bool {
	if r.Method != method {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return false
	}
	return true
}
