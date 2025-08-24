package image

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/codecflow/fabric/pkg/types"
)

// Service handles image building and management
type Service struct {
	builder  Builder
	registry Registry
	cache    Cache
}

// NewService creates a new image service
func NewService(builder Builder, registry Registry, cache Cache) *Service {
	return &Service{
		builder:  builder,
		registry: registry,
		cache:    cache,
	}
}

// Builder defines how to build container images
type Builder interface {
	// Build creates a container image from a spec
	Build(ctx context.Context, spec *types.ImageSpec) (*BuildResult, error)
}

// Registry defines where to store built images
type Registry interface {
	// Push uploads an image to the registry
	Push(ctx context.Context, imageID, tag string) (string, error)

	// Pull downloads an image from the registry
	Pull(ctx context.Context, imageRef string) error

	// Delete removes an image from the registry
	Delete(ctx context.Context, imageRef string) error
}

// Cache manages local image cache
type Cache interface {
	// Get retrieves a cached image ID
	Get(ctx context.Context, key string) (string, bool)

	// Set stores an image ID in cache
	Set(ctx context.Context, key string, imageID string) error

	// Delete removes from cache
	Delete(ctx context.Context, key string) error
}

// BuildResult contains the result of building an image
type BuildResult struct {
	ImageID    string
	ImageTag   string
	Digest     string
	SizeBytes  int64
	BuildTime  int64 // milliseconds
	CacheHit   bool
}

// BuildImage builds or retrieves a cached image
func (s *Service) BuildImage(ctx context.Context, spec *types.ImageSpec) (*BuildResult, error) {
	// If image is already specified, just use it
	if spec.Image != "" {
		return &BuildResult{
			ImageID:  spec.Image,
			ImageTag: spec.Tag,
			CacheHit: true,
		}, nil
	}

	// Generate cache key from spec
	cacheKey := s.generateCacheKey(spec)

	// Check cache
	if imageID, found := s.cache.Get(ctx, cacheKey); found {
		return &BuildResult{
			ImageID:  imageID,
			CacheHit: true,
		}, nil
	}

	// Build image
	result, err := s.builder.Build(ctx, spec)
	if err != nil {
		return nil, fmt.Errorf("failed to build image: %w", err)
	}

	// Cache the result
	if err := s.cache.Set(ctx, cacheKey, result.ImageID); err != nil {
		// Log but don't fail
		fmt.Printf("warning: failed to cache image: %v\n", err)
	}

	return result, nil
}

// PushImage pushes an image to the registry
func (s *Service) PushImage(ctx context.Context, imageID, tag string) (string, error) {
	return s.registry.Push(ctx, imageID, tag)
}

// PullImage pulls an image from the registry
func (s *Service) PullImage(ctx context.Context, imageRef string) error {
	return s.registry.Pull(ctx, imageRef)
}

// generateCacheKey creates a unique key for an image spec
func (s *Service) generateCacheKey(spec *types.ImageSpec) string {
	h := sha256.New()
	
	// Hash all relevant fields
	io.WriteString(h, spec.BaseImage)
	io.WriteString(h, spec.PythonVersion)
	
	for _, pkg := range spec.PythonPackages {
		io.WriteString(h, pkg)
	}
	
	for _, cmd := range spec.Commands {
		io.WriteString(h, cmd)
	}
	
	return fmt.Sprintf("%x", h.Sum(nil))
}

// GenerateDockerfile generates a Dockerfile from an ImageSpec
func GenerateDockerfile(spec *types.ImageSpec) string {
	var dockerfile string

	// Base image
	baseImage := spec.BaseImage
	if baseImage == "" {
		if spec.PythonVersion != "" {
			baseImage = fmt.Sprintf("python:%s-slim", spec.PythonVersion)
		} else {
			baseImage = "ubuntu:22.04"
		}
	}
	dockerfile += fmt.Sprintf("FROM %s\n\n", baseImage)

	// Working directory
	dockerfile += "WORKDIR /workspace\n\n"

	// Python packages
	if len(spec.PythonPackages) > 0 {
		dockerfile += "# Install Python packages\n"
		dockerfile += "RUN pip install --no-cache-dir \\\n"
		for i, pkg := range spec.PythonPackages {
			if i < len(spec.PythonPackages)-1 {
				dockerfile += fmt.Sprintf("    %s \\\n", pkg)
			} else {
				dockerfile += fmt.Sprintf("    %s\n", pkg)
			}
		}
		dockerfile += "\n"
	}

	// Custom commands
	if len(spec.Commands) > 0 {
		dockerfile += "# Custom build commands\n"
		for _, cmd := range spec.Commands {
			dockerfile += fmt.Sprintf("RUN %s\n", cmd)
		}
		dockerfile += "\n"
	}

	return dockerfile
}

// WriteDockerfile writes a Dockerfile to disk
func WriteDockerfile(dir string, spec *types.ImageSpec) (string, error) {
	content := GenerateDockerfile(spec)
	
	path := filepath.Join(dir, "Dockerfile")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return "", fmt.Errorf("failed to write Dockerfile: %w", err)
	}
	
	return path, nil
}
