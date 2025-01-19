package server

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"

	core "k8s.io/api/core/v1"
)

type LogsRequest struct {
	ID        string `json:"id"`
	Container string `json:"container,omitempty"`
	Tail      int    `json:"tail,omitempty"`
	Follow    bool   `json:"follow,omitempty"`
}

type LogsResponse struct {
	Logs  string `json:"logs"`
	Error string `json:"error,omitempty"`
}

func (s *Server) Logs(w http.ResponseWriter, r *http.Request) {
	if !Method(w, r, http.MethodGet) {
		return
	}

	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "Missing machine ID", http.StatusBadRequest)
		return
	}

	container := r.URL.Query().Get("container")
	if container == "" {
		container = "machine"
	}

	tailStr := r.URL.Query().Get("tail")
	tail := int64(100)
	if tailStr != "" {
		t, err := strconv.ParseInt(tailStr, 10, 64)
		if err == nil {
			tail = t
		}
	}

	followStr := r.URL.Query().Get("follow")
	follow := false
	if followStr == "true" {
		follow = true
	}

	ctx := r.Context()
	pod, err := s.client.Find(ctx, id)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to find machine: %v", err), http.StatusNotFound)
		return
	}

	opts := &core.PodLogOptions{
		Container: container,
		TailLines: &tail,
		Follow:    follow,
	}

	req := s.client.GetClientset().CoreV1().Pods(s.client.GetNamespace()).GetLogs(pod.Name, opts)
	logs, err := req.Stream(ctx)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get logs: %v", err), http.StatusInternalServerError)
		return
	}
	defer logs.Close()

	if follow {
		// For streaming logs
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("Transfer-Encoding", "chunked")

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "Streaming not supported", http.StatusInternalServerError)
			return
		}

		buffer := make([]byte, 4096)
		for {
			n, err := logs.Read(buffer)
			if err != nil {
				if err != io.EOF {
					fmt.Fprintf(w, "data: {\"error\": \"%s\"}\n\n", err.Error())
				}
				break
			}

			fmt.Fprintf(w, "data: %s\n\n", string(buffer[:n]))
			flusher.Flush()
		}
	} else {
		// For non-streaming logs
		logBytes, err := io.ReadAll(logs)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to read logs: %v", err), http.StatusInternalServerError)
			return
		}

		resp := LogsResponse{
			Logs: string(logBytes),
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}
