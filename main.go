package main

import (
	"fmt"
	"log"

	"captain/k8s"
)

func main() {
	config, err := k8s.FromLocal()

	if err != nil {
		log.Fatalf("Failed to get config: %v", err)
	}

	clientset, err := k8s.NewForConfig(config)
	if err != nil {
		log.Fatalf("Failed to create clientset: %v", err)
	}

	createdPod, err := k8s.Create(clientset, "default")
	if err != nil {
		log.Fatalf("Failed to create pod: %v", err)
	}

	fmt.Printf("Successfully created pod %s in namespace %s\n", createdPod.Name, createdPod.Namespace)
}
