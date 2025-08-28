package network

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"time"
)

// GetDefaultInterface returns the name of the default network interface.
func GetDefaultInterface() (string, error) {
	file, err := os.Open("/proc/net/route")
	if err != nil {
		return "", err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) > 1 && fields[1] == "00000000" { // Destination of default route
			return fields[0], nil
		}
	}

	return "", fmt.Errorf("default route not found")
}

// IsTLSEnabled checks if the address uses TLS (port 443).
func IsTLSEnabled(addr string) bool {
	_, port, err := net.SplitHostPort(addr)
	if err != nil {
		return false
	}
	return port == "443"
}

// GetPublicIPAddr retrieves the public IP address of this machine.
func GetPublicIPAddr() (string, error) {
	client := &http.Client{
		Timeout: 5 * time.Second,
	}
	resp, err := client.Get("https://api.ipify.org?format=text")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	ip, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(string(ip)), nil
}

// GetPrivateIPAddr retrieves the private IP address of this machine.
func GetPrivateIPAddr() (string, error) {
	ifaceName, err := GetDefaultInterface()
	if err != nil {
		// Fallback: try to get any non-loopback interface
		return getAnyPrivateIP()
	}

	iface, err := net.InterfaceByName(ifaceName)
	if err != nil {
		return getAnyPrivateIP()
	}

	addrs, err := iface.Addrs()
	if err != nil {
		return getAnyPrivateIP()
	}

	for _, addr := range addrs {
		var ip net.IP
		switch v := addr.(type) {
		case *net.IPNet:
			ip = v.IP
		case *net.IPAddr:
			ip = v.IP
		}

		if ip == nil || ip.IsLoopback() {
			continue
		}

		// Found IPv4 address
		if ip.To4() != nil {
			return ip.String(), nil
		}
	}

	return "", fmt.Errorf("no active network interface found with a private IP address")
}

// getAnyPrivateIP is a fallback that tries to get any private IP.
func getAnyPrivateIP() (string, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return "", err
	}

	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}

			if ip == nil || ip.IsLoopback() {
				continue
			}

			if ip.To4() != nil {
				return ip.String(), nil
			}
		}
	}

	return "", fmt.Errorf("no private IP address found")
}

// DialWithTimeout dials a TCP connection with a timeout.
func DialWithTimeout(ctx context.Context, addr string) (net.Conn, error) {
	timeout := time.Second
	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	conn, err := (&net.Dialer{}).DialContext(timeoutCtx, "tcp", addr)
	if err != nil {
		return nil, err
	}
	return conn, nil
}
