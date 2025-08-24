package image

import (
	"context"
	"fmt"
	"os/exec"
)

// LocalRegistry uses local image storage (Docker/Buildah)
type LocalRegistry struct{}

// NewLocalRegistry creates a registry using local storage
func NewLocalRegistry() *LocalRegistry {
	return &LocalRegistry{}
}

// Push is a no-op for local registry (images already local)
func (r *LocalRegistry) Push(ctx context.Context, imageID, tag string) (string, error) {
	// For local registry, the image is already available
	return imageID, nil
}

// Pull pulls an image to local storage
func (r *LocalRegistry) Pull(ctx context.Context, imageRef string) error {
	// Try docker first
	if _, err := exec.LookPath("docker"); err == nil {
		cmd := exec.CommandContext(ctx, "docker", "pull", imageRef)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("docker pull failed: %w", err)
		}
		return nil
	}

	// Try buildah
	if _, err := exec.LookPath("buildah"); err == nil {
		cmd := exec.CommandContext(ctx, "buildah", "pull", imageRef)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("buildah pull failed: %w", err)
		}
		return nil
	}

	return fmt.Errorf("no image tool found for pull")
}

// Delete removes an image from local storage
func (r *LocalRegistry) Delete(ctx context.Context, imageRef string) error {
	// Try docker first
	if _, err := exec.LookPath("docker"); err == nil {
		cmd := exec.CommandContext(ctx, "docker", "rmi", imageRef)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("docker rmi failed: %w", err)
		}
		return nil
	}

	// Try buildah
	if _, err := exec.LookPath("buildah"); err == nil {
		cmd := exec.CommandContext(ctx, "buildah", "rmi", imageRef)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("buildah rmi failed: %w", err)
		}
		return nil
	}

	return fmt.Errorf("no image tool found for delete")
}

// DockerRegistry uses Docker registry
type DockerRegistry struct {
	registry string
}

// NewDockerRegistry creates a registry client for a Docker registry
func NewDockerRegistry(registry string) *DockerRegistry {
	return &DockerRegistry{
		registry: registry,
	}
}

// Push pushes an image to Docker registry
func (r *DockerRegistry) Push(ctx context.Context, imageID, tag string) (string, error) {
	// Tag for remote registry
	remoteTag := fmt.Sprintf("%s/%s", r.registry, tag)
	
	// Tag the image
	tagCmd := exec.CommandContext(ctx, "docker", "tag", imageID, remoteTag)
	if err := tagCmd.Run(); err != nil {
		return "", fmt.Errorf("failed to tag image: %w", err)
	}

	// Push to registry
	pushCmd := exec.CommandContext(ctx, "docker", "push", remoteTag)
	if err := pushCmd.Run(); err != nil {
		return "", fmt.Errorf("failed to push image: %w", err)
	}

	return remoteTag, nil
}

// Pull pulls an image from Docker registry
func (r *DockerRegistry) Pull(ctx context.Context, imageRef string) error {
	cmd := exec.CommandContext(ctx, "docker", "pull", imageRef)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to pull image: %w", err)
	}
	return nil
}

// Delete removes an image from Docker registry (requires API call)
func (r *DockerRegistry) Delete(ctx context.Context, imageRef string) error {
	// Note: This requires registry API access, not just docker CLI
	// For now, just remove local copy
	cmd := exec.CommandContext(ctx, "docker", "rmi", imageRef)
	return cmd.Run()
}
