package source

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"

	"github.com/codecflow/fabric/stash/config"
)

// MountPoint implements the Source interface for AWS MountPoint S3.
type MountPoint struct {
	mountCmd *exec.Cmd
	config   config.MountPoint
	logger   *slog.Logger
}

// NewMountPoint creates a new MountPoint source.
func NewMountPoint(cfg config.MountPoint, logger *slog.Logger) (Source, error) {
	if logger == nil {
		logger = slog.Default()
	}

	return &MountPoint{
		config: cfg,
		logger: logger,
	}, nil
}

// Mount mounts the S3 bucket using mount-s3.
func (s *MountPoint) Mount(localPath string) error {
	// Force unmount previous mounts (mount-s3 doesn't always clean up gracefully)
	s.Unmount(localPath)

	// Ensure mount point exists
	if err := os.MkdirAll(localPath, 0755); err != nil {
		return fmt.Errorf("failed to create mount point: %w", err)
	}

	s.mountCmd = exec.Command(
		"mount-s3",
		s.config.BucketName,
		localPath,
		"--region",
		s.config.Region,
		"--endpoint-url",
		s.config.EndpointURL,
		"--read-only",
	)

	if s.config.ForcePathStyle {
		s.mountCmd.Args = append(s.mountCmd.Args, "--force-path-style")
	}

	if s.config.AccessKey != "" || s.config.SecretKey != "" {
		s.mountCmd.Env = append(os.Environ(),
			fmt.Sprintf("AWS_ACCESS_KEY_ID=%s", s.config.AccessKey),
			fmt.Sprintf("AWS_SECRET_ACCESS_KEY=%s", s.config.SecretKey),
		)
	}

	go func() {
		output, err := s.mountCmd.CombinedOutput()
		if err != nil {
			s.logger.Error("error executing mount-s3", "error", err, "output", string(output))
		}
	}()

	s.logger.Info("mountpoint filesystem being mounted", "path", localPath)
	return nil
}

// Format is a no-op for MountPoint (S3 buckets don't need formatting).
func (s *MountPoint) Format(fsName string) error {
	return nil
}

// Unmount unmounts the S3 filesystem.
func (s *MountPoint) Unmount(localPath string) error {
	cmd := exec.Command("umount", localPath)

	output, err := cmd.CombinedOutput()
	if err != nil {
		// Don't return error if already unmounted
		return nil
	}

	s.logger.Info("mountpoint filesystem unmounted", "path", localPath, "output", string(output))
	return nil
}
