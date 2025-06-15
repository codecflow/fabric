package main

import (
	"fmt"
	"os"

	"github.com/sirupsen/logrus"
)

func main() {
	logger := logrus.New()
	logger.SetFormatter(&logrus.JSONFormatter{})
	logger.SetLevel(logrus.InfoLevel)

	logger.Info("Shuttle node runner starting...")

	// TODO: Implement shuttle functionality
	// - Join Tailscale mesh
	// - Connect to containerd
	// - Listen for workload assignments from NATS
	// - Manage side-car containers (ctrl, stream)

	fmt.Println("Shuttle not implemented yet")
	os.Exit(1)
}
