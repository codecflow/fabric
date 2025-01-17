// server.go
package server

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"time"
)

type Config struct {
	PodNamespace string
	PodDomain    string
	APIKey       string
}

func (s *Server) Connect(w http.ResponseWriter, r *http.Request) {
	key := r.URL.Query().Get("key")
	if !validateAPIKey(key) {
		http.Error(w, "Invalid API key", http.StatusUnauthorized)
		return
	}

	id := r.URL.Query().Get("id")
	protocol := r.URL.Query().Get("protocol")

	if id == "" || protocol == "" {
		http.Error(w, "Missing id or protocol", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	pod, err := s.client.Find(ctx, id)
	if err != nil {
		http.Error(w, "Error checking pod", http.StatusInternalServerError)
		return
	}

	if pod == nil {
		http.Error(w, "Pod not found", http.StatusNotFound)
		return
	}

	port := s.getProtocolPort(protocol)
	if port == 0 {
		http.Error(w, "Invalid protocol", http.StatusBadRequest)
		return
	}

	address := fmt.Sprintf("%s.%s.%s.svc.cluster.local", pod.Name, "entrypoint", "default")
	address = fmt.Sprintf("%s:%d", address, port)

	fmt.Println("Connecting to", address)
	connection, err := net.DialTimeout("tcp", address, 1*time.Minute)
	if err != nil {
		http.Error(w, "Failed to connect to pod", http.StatusInternalServerError)
		return
	}
	defer connection.Close()

	hj, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "Hijacking not supported", http.StatusInternalServerError)
		return
	}

	conn, _, err := hj.Hijack()
	if err != nil {
		http.Error(w, "Hijacking failed", http.StatusInternalServerError)
		return
	}
	defer conn.Close()

	errChan := make(chan error, 2)

	go func() {
		_, err := io.Copy(connection, conn)
		errChan <- err
	}()

	go func() {
		_, err := io.Copy(conn, connection)
		errChan <- err
	}()

	<-errChan
}

func validateAPIKey(_ string) bool {
	return true
}

func (s *Server) getProtocolPort(protocol string) int {
	switch protocol {
	case "vnc":
		return 5901
	case "rtmp":
		return 1935
	case "cdp":
		return 9222
	default:
		return 0
	}
}
