package storage

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/codecflow/fabric/weaver/internal/config"
)

// IrohClient manages Iroh storage operations
type IrohClient struct {
	config *config.IrohConfig
}

// IrohObject represents an object stored in Iroh
type IrohObject struct {
	CID       string    `json:"cid"`
	Size      int64     `json:"size"`
	CreatedAt time.Time `json:"created_at"`
	Tags      []string  `json:"tags"`
}

// New creates a new Iroh client
func NewIrohClient(cfg *config.IrohConfig) (*IrohClient, error) {
	if !cfg.Enabled {
		return &IrohClient{config: cfg}, nil
	}

	// In a real implementation, this would initialize the Iroh client
	log.Printf("Initializing Iroh client with node: %s", cfg.NodeURL)

	return &IrohClient{
		config: cfg,
	}, nil
}

// Upload uploads a file or directory to Iroh
func (i *IrohClient) Upload(ctx context.Context, localPath string, tags []string) (*IrohObject, error) {
	if !i.config.Enabled {
		return nil, fmt.Errorf("Iroh storage is disabled") // nolint:staticcheck
	}

	log.Printf("Uploading to Iroh: %s", localPath)

	// Check if path exists
	info, err := os.Stat(localPath)
	if err != nil {
		return nil, fmt.Errorf("failed to stat path: %w", err)
	}

	// Calculate size
	var size int64
	if info.IsDir() {
		size, err = i.calculateDirSize(localPath)
		if err != nil {
			return nil, fmt.Errorf("failed to calculate directory size: %w", err)
		}
	} else {
		size = info.Size()
	}

	// In a real implementation, this would use the Iroh API to upload
	cid := i.generateCID(localPath, size)

	object := &IrohObject{
		CID:       cid,
		Size:      size,
		CreatedAt: time.Now(),
		Tags:      tags,
	}

	log.Printf("Uploaded to Iroh: %s (CID: %s, Size: %d bytes)", localPath, cid, size)
	return object, nil
}

// Download downloads a file or directory from Iroh
func (i *IrohClient) Download(ctx context.Context, cid string, localPath string) error {
	if !i.config.Enabled {
		return fmt.Errorf("Iroh storage is disabled") // nolint:staticcheck
	}

	log.Printf("Downloading from Iroh: %s -> %s", cid, localPath)

	// Ensure parent directory exists
	if err := os.MkdirAll(filepath.Dir(localPath), 0o750); err != nil {
		return fmt.Errorf("failed to create parent directory: %w", err)
	}

	// In a real implementation, this would use the Iroh API to download
	// For now, create a placeholder file/directory
	if err := i.createPlaceholder(localPath); err != nil {
		return fmt.Errorf("failed to create placeholder: %w", err)
	}

	log.Printf("Downloaded from Iroh: %s", cid)
	return nil
}

// Delete removes an object from Iroh
func (i *IrohClient) Delete(ctx context.Context, cid string) error {
	if !i.config.Enabled {
		return fmt.Errorf("Iroh storage is disabled") // nolint:staticcheck
	}

	log.Printf("Deleting from Iroh: %s", cid)

	// In a real implementation, this would use the Iroh API to delete
	// For now, just log the operation

	log.Printf("Deleted from Iroh: %s", cid)
	return nil
}

// List lists objects in Iroh with optional tag filtering
func (i *IrohClient) List(ctx context.Context, tags []string) ([]*IrohObject, error) {
	if !i.config.Enabled {
		return []*IrohObject{}, nil
	}

	log.Printf("Listing Iroh objects with tags: %v", tags)

	// In a real implementation, this would query the Iroh API
	// For now, return some placeholder objects
	objects := []*IrohObject{
		{
			CID:       "bafkreiabcdef1234567890",
			Size:      1024,
			CreatedAt: time.Now().Add(-time.Hour),
			Tags:      []string{"snapshot", "workload-123"},
		},
		{
			CID:       "bafkreifedcba0987654321",
			Size:      2048,
			CreatedAt: time.Now().Add(-2 * time.Hour),
			Tags:      []string{"backup", "workload-456"},
		},
	}

	// Filter by tags if provided
	if len(tags) > 0 {
		filtered := []*IrohObject{}
		for _, obj := range objects {
			if i.hasMatchingTags(obj.Tags, tags) {
				filtered = append(filtered, obj)
			}
		}
		objects = filtered
	}

	log.Printf("Found %d Iroh objects", len(objects))
	return objects, nil
}

// GetInfo gets information about an object in Iroh
func (i *IrohClient) GetInfo(ctx context.Context, cid string) (*IrohObject, error) {
	if !i.config.Enabled {
		return nil, fmt.Errorf("Iroh storage is disabled") // nolint:staticcheck
	}

	log.Printf("Getting Iroh object info: %s", cid)

	// In a real implementation, this would query the Iroh API
	object := &IrohObject{
		CID:       cid,
		Size:      1024,
		CreatedAt: time.Now().Add(-time.Hour),
		Tags:      []string{"example"},
	}

	return object, nil
}

// Pin pins an object in Iroh to prevent garbage collection
func (i *IrohClient) Pin(ctx context.Context, cid string) error {
	if !i.config.Enabled {
		return fmt.Errorf("Iroh storage is disabled") // nolint:staticcheck
	}

	log.Printf("Pinning Iroh object: %s", cid)

	// In a real implementation, this would use the Iroh API to pin
	return nil
}

// Unpin unpins an object in Iroh
func (i *IrohClient) Unpin(ctx context.Context, cid string) error {
	if !i.config.Enabled {
		return fmt.Errorf("Iroh storage is disabled") // nolint:staticcheck
	}

	log.Printf("Unpinning Iroh object: %s", cid)

	// In a real implementation, this would use the Iroh API to unpin
	return nil
}

// Share creates a shareable link for an object
func (i *IrohClient) Share(ctx context.Context, cid string, expiry time.Duration) (string, error) {
	if !i.config.Enabled {
		return "", fmt.Errorf("Iroh storage is disabled") // nolint:staticcheck
	}

	log.Printf("Creating share link for Iroh object: %s (expires in %v)", cid, expiry)

	// In a real implementation, this would create a shareable link
	shareURL := fmt.Sprintf("%s/share/%s?expires=%d", i.config.NodeURL, cid, time.Now().Add(expiry).Unix())

	return shareURL, nil
}

// calculateDirSize calculates the total size of a directory
func (i *IrohClient) calculateDirSize(dirPath string) (int64, error) {
	var size int64

	err := filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			size += info.Size()
		}
		return nil
	})

	return size, err
}

// generateCID generates a placeholder CID for testing
func (i *IrohClient) generateCID(path string, size int64) string {
	// In a real implementation, this would be generated by Iroh
	hash := fmt.Sprintf("%x", path+fmt.Sprintf("%d", size))
	if len(hash) > 20 {
		hash = hash[:20]
	}
	return fmt.Sprintf("bafkrei%s", hash)
}

// createPlaceholder creates a placeholder file or directory for testing
func (i *IrohClient) createPlaceholder(localPath string) error {
	// Create a simple placeholder file
	return os.WriteFile(localPath, []byte("placeholder content"), 0o600)
}

// hasMatchingTags checks if an object has any of the specified tags
func (i *IrohClient) hasMatchingTags(objectTags, filterTags []string) bool {
	for _, filterTag := range filterTags {
		for _, objectTag := range objectTags {
			if objectTag == filterTag {
				return true
			}
		}
	}
	return false
}

// GetStats returns storage statistics
func (i *IrohClient) GetStats(ctx context.Context) (map[string]interface{}, error) {
	if !i.config.Enabled {
		return map[string]interface{}{
			"enabled": false,
		}, nil
	}

	// In a real implementation, this would query Iroh for actual stats
	return map[string]interface{}{
		"enabled":      true,
		"nodeURL":      i.config.NodeURL,
		"totalObjects": 42,
		"totalSize":    "1.2GB",
		"timestamp":    time.Now().Format(time.RFC3339),
	}, nil
}
