package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"log"
	"log/slog"
	"os"
	"time"

	"github.com/codecflow/fabric/stash/client"
	"github.com/codecflow/fabric/stash/config"
)

var (
	configPath = flag.String("config", "./config.yaml", "Path to config file")
)

func main() {
	flag.Parse()

	// Set config path environment variable
	os.Setenv("CONFIG_PATH", *configPath)

	// Load configuration
	configManager, err := config.NewManager[config.BlobCacheConfig]()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	cfg := configManager.Get()

	// Initialize logger
	logLevel := slog.LevelInfo
	if cfg.Global.DebugMode {
		logLevel = slog.LevelDebug
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: logLevel,
	}))

	logger.Info("starting e2e basic test", "config", *configPath)

	ctx := context.Background()

	// Create client
	c, err := client.NewClient(ctx, &cfg, logger)
	if err != nil {
		log.Fatalf("Unable to create client: %v", err)
	}
	defer c.Cleanup()

	logger.Info("waiting for hosts...")

	// Wait for hosts to be available
	err = c.WaitForHosts(30 * time.Second)
	if err != nil {
		log.Fatalf("Unable to wait for hosts: %v", err)
	}

	logger.Info("hosts available")

	// Test data
	testContent := "Hello, World! This is a test message from the stash e2e test."
	hashBytes := sha256.Sum256([]byte(testContent))
	expectedHash := hex.EncodeToString(hashBytes[:])

	logger.Info("test data", "content_length", len(testContent), "hash", expectedHash)

	// Test 1: Store content
	logger.Info("test 1: storing content...")

	contentChan := make(chan []byte, 1)
	contentChan <- []byte(testContent)
	close(contentChan)

	computedHash, err := c.StoreContent(contentChan, expectedHash, client.ContentOptions{
		RoutingKey: expectedHash,
	})
	if err != nil {
		log.Fatalf("Unable to store content: %v", err)
	}

	logger.Info("content stored", "computed_hash", computedHash, "expected_hash", expectedHash)

	if computedHash != expectedHash {
		log.Fatalf("Hash mismatch: got %s, expected %s", computedHash, expectedHash)
	}

	// Test 2: Check if cached nearby
	logger.Info("test 2: checking if content is cached nearby...")

	exists, err := c.IsCachedNearby(expectedHash, expectedHash)
	if err != nil {
		log.Fatalf("Unable to check if content is cached nearby: %v", err)
	}

	if !exists {
		log.Fatalf("Content should be cached nearby but is not")
	}

	logger.Info("content is cached nearby", "exists", exists)

	// Test 3: Get content
	logger.Info("test 3: retrieving content...")

	gotContent, err := c.GetContent(expectedHash, 0, int64(len(testContent)), client.ContentOptions{
		RoutingKey: expectedHash,
	})
	if err != nil {
		log.Fatalf("Unable to get content: %v", err)
	}

	logger.Info("content retrieved", "length", len(gotContent))

	if string(gotContent) != testContent {
		log.Fatalf("Content mismatch:\nGot:      %s\nExpected: %s", string(gotContent), testContent)
	}

	// Test 4: Stream content
	logger.Info("test 4: streaming content...")

	streamChan, err := c.GetContentStream(expectedHash, 0, int64(len(testContent)), client.ContentOptions{
		RoutingKey: expectedHash,
	})
	if err != nil {
		log.Fatalf("Unable to get content stream: %v", err)
	}

	var streamedContent []byte
	for chunk := range streamChan {
		streamedContent = append(streamedContent, chunk...)
	}

	if string(streamedContent) != testContent {
		log.Fatalf("Streamed content mismatch:\nGot:      %s\nExpected: %s", string(streamedContent), testContent)
	}

	logger.Info("content streamed successfully", "length", len(streamedContent))

	// Test 5: Get nearby hosts
	logger.Info("test 5: getting nearby hosts...")

	hosts, err := c.GetNearbyHosts()
	if err != nil {
		log.Fatalf("Unable to get nearby hosts: %v", err)
	}

	logger.Info("nearby hosts retrieved", "count", len(hosts))
	for i, host := range hosts {
		logger.Info("host", "index", i, "id", host.ID, "public", host.Address.Public, "private", host.Address.Private)
	}

	// All tests passed
	fmt.Println("\n✅ All tests passed!")
	fmt.Printf("   - Stored content: %s\n", expectedHash)
	fmt.Printf("   - Content length: %d bytes\n", len(testContent))
	fmt.Printf("   - Cached nearby: %v\n", exists)
	fmt.Printf("   - Retrieved successfully: ✓\n")
	fmt.Printf("   - Streamed successfully: ✓\n")
	fmt.Printf("   - Available hosts: %d\n", len(hosts))
}
