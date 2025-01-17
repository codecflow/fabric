// server.go
package server

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"time"

	"github.com/sirupsen/logrus"
)

type Config struct {
	PodNamespace string
	PodDomain    string
	APIKey       string
}

func (s *Server) Connect(w http.ResponseWriter, r *http.Request) {
	reqID := fmt.Sprintf("%d", time.Now().UnixNano())
	logger := s.logger.WithFields(logrus.Fields{
		"req_id": reqID,
		"remote": r.RemoteAddr,
	})

	// Get and validate parameters
	id := r.URL.Query().Get("id")
	protocol := r.URL.Query().Get("protocol")

	logger = logger.WithFields(logrus.Fields{
		"pod_id":   id,
		"protocol": protocol,
	})
	logger.Info("New connection request")

	if id == "" || protocol == "" {
		logger.Warn("Missing required parameters")
		http.Error(w, "Missing id or protocol", http.StatusBadRequest)
		return
	}

	// Verify pod exists
	ctx := r.Context()
	pod, err := s.client.Find(ctx, id)
	if err != nil {
		logger.WithError(err).Error("Failed to find pod")
		http.Error(w, "Error checking pod", http.StatusInternalServerError)
		return
	}

	if pod == nil {
		logger.Warn("Pod not found")
		http.Error(w, "Pod not found", http.StatusNotFound)
		return
	}

	// Get protocol port
	port := s.getProtocolPort(protocol)
	if port == 0 {
		logger.Warn("Invalid protocol")
		http.Error(w, "Invalid protocol", http.StatusBadRequest)
		return
	}

	// Construct DNS name
	dnsName := fmt.Sprintf("%s.entrypoint.default.svc.cluster.local", pod.Name)
	logger = logger.WithField("dns_name", dnsName)
	logger.Info("Resolving DNS")

	// Perform DNS lookup
	ips, err := net.LookupHost(dnsName)
	if err != nil {
		logger.WithError(err).Error("DNS lookup failed")
		http.Error(w, "Failed to resolve pod address", http.StatusInternalServerError)
		return
	}

	if len(ips) == 0 {
		logger.Error("DNS lookup returned no IPs")
		http.Error(w, "No IPs found for pod", http.StatusInternalServerError)
		return
	}

	logger.WithField("resolved_ips", ips).Info("DNS lookup successful")

	// Connect using the first resolved IP
	targetAddr := fmt.Sprintf("%s:%d", ips[0], port)
	logger = logger.WithField("target_addr", targetAddr)
	logger.Info("Attempting connection")

	// Connect to pod
	connection, err := net.DialTimeout("tcp", targetAddr, 1*time.Minute)
	if err != nil {
		logger.WithError(err).Error("Failed to connect to pod")
		http.Error(w, "Failed to connect to pod", http.StatusInternalServerError)
		return
	}
	defer connection.Close()
	logger.Info("Connected to pod successfully")

	// Hijack connection
	hj, ok := w.(http.Hijacker)
	if !ok {
		logger.Error("Hijacking not supported")
		http.Error(w, "Hijacking not supported", http.StatusInternalServerError)
		return
	}

	conn, _, err := hj.Hijack()
	if err != nil {
		logger.WithError(err).Error("Hijacking failed")
		http.Error(w, "Hijacking failed", http.StatusInternalServerError)
		return
	}
	defer conn.Close()
	logger.Info("Connection hijacked successfully")

	// For CDP protocol, send HTTP 200 OK
	if protocol == "cdp" {
		if _, err := conn.Write([]byte("HTTP/1.1 200 OK\r\n\r\n")); err != nil {
			logger.WithError(err).Error("Failed to write CDP response")
			return
		}
		logger.Debug("Sent CDP upgrade response")
	}

	// Bidirectional copy
	errChan := make(chan error, 2)

	go func() {
		_, err := io.Copy(connection, conn)
		errChan <- err
		logger.WithError(err).Debug("Client -> Pod copy ended")
	}()

	go func() {
		_, err := io.Copy(conn, connection)
		errChan <- err
		logger.WithError(err).Debug("Pod -> Client copy ended")
	}()

	// Wait for either copy to finish
	err = <-errChan
	if err != nil {
		logger.WithError(err).Info("Connection closed with error")
	} else {
		logger.Info("Connection closed normally")
	}
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
