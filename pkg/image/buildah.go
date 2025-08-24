package image

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/codecflow/fabric/pkg/types"
)

// BuildahBuilder implements Builder using buildah
type BuildahBuilder struct {
	workDir string
}

// NewBuildahBuilder creates a new buildah builder
func NewBuildahBuilder(workDir string) *BuildahBuilder {
	return &BuildahBuilder{
		workDir: workDir,
	}
}

// Build creates a container image using buildah
func (b *BuildahBuilder) Build(ctx context.Context, spec *types.ImageSpec) (*BuildResult, error) {
	start := time.Now()

	// Create temporary build directory
	buildDir, err := os.MkdirTemp(b.workDir, "build-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create build directory: %w", err)
	}
	defer os.RemoveAll(buildDir)

	// Write Dockerfile
	dockerfilePath, err := WriteDockerfile(buildDir, spec)
	if err != nil {
		return nil, err
	}

	// Generate image tag
	imageTag := fmt.Sprintf("fabric-image:%d", time.Now().Unix())

	// Build with buildah
	cmd := exec.CommandContext(ctx, "buildah", "build",
		"-t", imageTag,
		"-f", dockerfilePath,
		buildDir,
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("buildah build failed: %w", err)
	}

	buildTime := time.Since(start).Milliseconds()

	return &BuildResult{
		ImageID:   imageTag,
		ImageTag:  imageTag,
		BuildTime: buildTime,
		CacheHit:  false,
	}, nil
}

// DockerBuilder implements Builder using Docker
type DockerBuilder struct {
	workDir string
}

// NewDockerBuilder creates a new docker builder
func NewDockerBuilder(workDir string) *DockerBuilder {
	return &DockerBuilder{
		workDir: workDir,
	}
}

// Build creates a container image using docker
func (d *DockerBuilder) Build(ctx context.Context, spec *types.ImageSpec) (*BuildResult, error) {
	start := time.Now()

	// Create temporary build directory
	buildDir, err := os.MkdirTemp(d.workDir, "build-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create build directory: %w", err)
	}
	defer os.RemoveAll(buildDir)

	// Write Dockerfile
	dockerfilePath, err := WriteDockerfile(buildDir, spec)
	if err != nil {
		return nil, err
	}

	// Generate image tag
	imageTag := fmt.Sprintf("fabric-image:%d", time.Now().Unix())

	// Build with docker
	cmd := exec.CommandContext(ctx, "docker", "build",
		"-t", imageTag,
		"-f", dockerfilePath,
		buildDir,
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("docker build failed: %w", err)
	}

	buildTime := time.Since(start).Milliseconds()

	return &BuildResult{
		ImageID:   imageTag,
		ImageTag:  imageTag,
		BuildTime: buildTime,
		CacheHit:  false,
	}, nil
}

// DetectBuilder detects and returns the appropriate builder
func DetectBuilder(workDir string) (Builder, error) {
	// Try buildah first (daemonless)
	if _, err := exec.LookPath("buildah"); err == nil {
		return NewBuildahBuilder(workDir), nil
	}

	// Fall back to docker
	if _, err := exec.LookPath("docker"); err == nil {
		return NewDockerBuilder(workDir), nil
	}

	return nil, fmt.Errorf("no image builder found (tried buildah, docker)")
}
