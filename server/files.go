package server

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/sirupsen/logrus"
	core "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/remotecommand"
)

type FileUploadResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
	Error   string `json:"error,omitempty"`
}

type FileDownloadRequest struct {
	ID   string `json:"id"`
	Path string `json:"path"`
}

// Upload handles file uploads to a machine
func (s *Server) Upload(w http.ResponseWriter, r *http.Request) {
	if !Method(w, r, http.MethodPost) {
		return
	}

	// Get machine ID from query params
	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "Missing machine ID", http.StatusBadRequest)
		return
	}

	// Get destination path from query params
	destPath := r.URL.Query().Get("path")
	if destPath == "" {
		destPath = "/data" // Default to /data directory
	}

	// Ensure the path is absolute
	if !strings.HasPrefix(destPath, "/") {
		destPath = "/" + destPath
	}

	// Limit file size to 100MB
	r.Body = http.MaxBytesReader(w, r.Body, 100*1024*1024)

	// Parse the multipart form
	err := r.ParseMultipartForm(100 * 1024 * 1024)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to parse form: %v", err), http.StatusBadRequest)
		return
	}

	// Get the file from the form
	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get file: %v", err), http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Find the pod
	ctx := r.Context()
	pod, err := s.client.Find(ctx, id)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to find machine: %v", err), http.StatusNotFound)
		return
	}

	// Create a buffer to store the tar archive
	var buf bytes.Buffer

	// Create a gzip writer
	gw := gzip.NewWriter(&buf)
	defer gw.Close()

	// Create a tar writer
	tw := tar.NewWriter(gw)
	defer tw.Close()

	// Read the file content
	fileContent, err := io.ReadAll(file)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to read file: %v", err), http.StatusInternalServerError)
		return
	}

	// Create a tar header
	header.Filename = filepath.Base(header.Filename)
	tarHeader := &tar.Header{
		Name: header.Filename,
		Mode: 0644,
		Size: int64(len(fileContent)),
	}

	// Write the header
	if err := tw.WriteHeader(tarHeader); err != nil {
		http.Error(w, fmt.Sprintf("Failed to write tar header: %v", err), http.StatusInternalServerError)
		return
	}

	// Write the file content
	if _, err := tw.Write(fileContent); err != nil {
		http.Error(w, fmt.Sprintf("Failed to write file to tar: %v", err), http.StatusInternalServerError)
		return
	}

	// Close the tar writer to flush the data
	if err := tw.Close(); err != nil {
		http.Error(w, fmt.Sprintf("Failed to close tar writer: %v", err), http.StatusInternalServerError)
		return
	}

	// Close the gzip writer to flush the data
	if err := gw.Close(); err != nil {
		http.Error(w, fmt.Sprintf("Failed to close gzip writer: %v", err), http.StatusInternalServerError)
		return
	}

	// Create the destination directory if it doesn't exist
	mkdirCmd := []string{"mkdir", "-p", destPath}
	execReq := s.client.GetClientset().CoreV1().RESTClient().Post().
		Resource("pods").
		Name(pod.Name).
		Namespace(s.client.GetNamespace()).
		SubResource("exec").
		VersionedParams(&core.PodExecOptions{
			Command:   mkdirCmd,
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

	var mkdirStdout, mkdirStderr bytes.Buffer
	err = exec.Stream(remotecommand.StreamOptions{
		Stdout: &mkdirStdout,
		Stderr: &mkdirStderr,
	})

	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to create directory: %v, stderr: %s", err, mkdirStderr.String()), http.StatusInternalServerError)
		return
	}

	// Create the command to extract the tar file
	destFile := filepath.Join(destPath, header.Filename)
	tarCmd := []string{"tar", "-xzf", "-", "-C", destPath}

	// Create the exec request
	execReq = s.client.GetClientset().CoreV1().RESTClient().Post().
		Resource("pods").
		Name(pod.Name).
		Namespace(s.client.GetNamespace()).
		SubResource("exec").
		VersionedParams(&core.PodExecOptions{
			Command:   tarCmd,
			Container: "machine",
			Stdin:     true,
			Stdout:    true,
			Stderr:    true,
			TTY:       false,
		}, scheme.ParameterCodec)

	// Create the executor
	exec, err = remotecommand.NewSPDYExecutor(s.client.GetConfig(), "POST", execReq.URL())
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to create executor: %v", err), http.StatusInternalServerError)
		return
	}

	// Execute the command
	var stdout, stderr bytes.Buffer
	err = exec.Stream(remotecommand.StreamOptions{
		Stdin:  &buf,
		Stdout: &stdout,
		Stderr: &stderr,
	})

	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to extract file: %v, stderr: %s", err, stderr.String()), http.StatusInternalServerError)
		return
	}

	// Log the upload
	s.logger.WithFields(logrus.Fields{
		"machine_id": id,
		"file":       header.Filename,
		"size":       header.Size,
		"dest_path":  destPath,
	}).Info("File uploaded")

	// Return success response
	resp := FileUploadResponse{
		Success: true,
		Message: fmt.Sprintf("File %s uploaded to %s", header.Filename, destFile),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// Download handles file downloads from a machine
func (s *Server) Download(w http.ResponseWriter, r *http.Request) {
	if !Method(w, r, http.MethodGet) {
		return
	}

	// Get machine ID from query params
	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "Missing machine ID", http.StatusBadRequest)
		return
	}

	// Get file path from query params
	path := r.URL.Query().Get("path")
	if path == "" {
		http.Error(w, "Missing file path", http.StatusBadRequest)
		return
	}

	// Find the pod
	ctx := r.Context()
	pod, err := s.client.Find(ctx, id)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to find machine: %v", err), http.StatusNotFound)
		return
	}

	// Create a buffer to store the tar archive
	var buf bytes.Buffer

	// Create the command to create a tar archive of the file
	tarCmd := []string{"tar", "-czf", "-", path}

	// Create the exec request
	execReq := s.client.GetClientset().CoreV1().RESTClient().Post().
		Resource("pods").
		Name(pod.Name).
		Namespace(s.client.GetNamespace()).
		SubResource("exec").
		VersionedParams(&core.PodExecOptions{
			Command:   tarCmd,
			Container: "machine",
			Stdin:     false,
			Stdout:    true,
			Stderr:    true,
			TTY:       false,
		}, scheme.ParameterCodec)

	// Create the executor
	exec, err := remotecommand.NewSPDYExecutor(s.client.GetConfig(), "POST", execReq.URL())
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to create executor: %v", err), http.StatusInternalServerError)
		return
	}

	// Execute the command
	var stderr bytes.Buffer
	err = exec.Stream(remotecommand.StreamOptions{
		Stdout: &buf,
		Stderr: &stderr,
	})

	if err != nil {
		errMsg := stderr.String()
		if strings.Contains(errMsg, "No such file or directory") {
			http.Error(w, fmt.Sprintf("File not found: %s", path), http.StatusNotFound)
		} else {
			http.Error(w, fmt.Sprintf("Failed to create tar archive: %v, stderr: %s", err, errMsg), http.StatusInternalServerError)
		}
		return
	}

	// Check if the tar archive is empty
	if buf.Len() == 0 {
		http.Error(w, fmt.Sprintf("File not found or empty: %s", path), http.StatusNotFound)
		return
	}

	// Set the content type and filename
	filename := filepath.Base(path)
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	w.Header().Set("Content-Type", "application/octet-stream")

	// Extract the file from the tar archive
	gzr, err := gzip.NewReader(&buf)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to create gzip reader: %v", err), http.StatusInternalServerError)
		return
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to read tar header: %v", err), http.StatusInternalServerError)
			return
		}

		// Skip directories
		if header.Typeflag == tar.TypeDir {
			continue
		}

		// Copy the file content to the response
		_, err = io.Copy(w, tr)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to copy file content: %v", err), http.StatusInternalServerError)
			return
		}

		// We only need the first file
		break
	}

	// Log the download
	s.logger.WithFields(logrus.Fields{
		"machine_id": id,
		"file":       path,
	}).Info("File downloaded")
}
