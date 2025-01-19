package server

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	core "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/remotecommand"
)

type ExecRequest struct {
	ID      string   `json:"id"`
	Command []string `json:"command"`
}

type ExecResponse struct {
	Stdout string `json:"stdout"`
	Stderr string `json:"stderr"`
	Error  string `json:"error,omitempty"`
}

func (s *Server) Exec(w http.ResponseWriter, r *http.Request) {
	if !Method(w, r, http.MethodPost) {
		return
	}

	if r.Header.Get("Content-Type") != "application/json" {
		http.Error(w, "Content-Type must be application/json", http.StatusUnsupportedMediaType)
		return
	}

	var req ExecRequest
	if err := JSON(r, &req); err != nil {
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	if req.ID == "" {
		http.Error(w, "Missing machine ID", http.StatusBadRequest)
		return
	}

	if len(req.Command) == 0 {
		http.Error(w, "Missing command", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	pod, err := s.client.Find(ctx, req.ID)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to find machine: %v", err), http.StatusNotFound)
		return
	}

	execReq := s.client.GetClientset().CoreV1().RESTClient().Post().
		Resource("pods").
		Name(pod.Name).
		Namespace(s.client.GetNamespace()).
		SubResource("exec").
		VersionedParams(&core.PodExecOptions{
			Command:   req.Command,
			Container: "machine",
			Stdin:     false,
			Stdout:    true,
			Stderr:    true,
			TTY:       false,
		}, scheme.ParameterCodec)

	exec, err := remotecommand.NewSPDYExecutor(s.client.GetConfig(), "POST", execReq.URL())
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to create executor: %v", err), http.StatusInternalServerError)
		return
	}

	var stdout, stderr io.Writer
	stdoutBuf := &safeBuffer{}
	stderrBuf := &safeBuffer{}
	stdout = stdoutBuf
	stderr = stderrBuf

	err = exec.Stream(remotecommand.StreamOptions{
		Stdout: stdout,
		Stderr: stderr,
	})

	resp := ExecResponse{
		Stdout: stdoutBuf.String(),
		Stderr: stderrBuf.String(),
	}

	if err != nil {
		resp.Error = err.Error()
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

type safeBuffer struct {
	data []byte
}

func (s *safeBuffer) Write(p []byte) (n int, err error) {
	s.data = append(s.data, p...)
	return len(p), nil
}

func (s *safeBuffer) String() string {
	return string(s.data)
}
