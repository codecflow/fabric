package source

import (
	"fmt"
	"log/slog"
	"os/exec"
	"strconv"
	"time"

	"github.com/codecflow/fabric/stash/config"
)

const juiceFSMountTimeout = 30 * time.Second

// JuiceFS implements the Source interface for JuiceFS filesystems.
type JuiceFS struct {
	mountCmd *exec.Cmd
	config   config.JuiceFS
	logger   *slog.Logger
}

// NewJuiceFS creates a new JuiceFS source.
func NewJuiceFS(cfg config.JuiceFS, logger *slog.Logger) (Source, error) {
	if logger == nil {
		logger = slog.Default()
	}

	return &JuiceFS{
		config: cfg,
		logger: logger,
	}, nil
}

// Mount mounts the JuiceFS filesystem to the specified local path.
func (s *JuiceFS) Mount(localPath string) error {
	s.logger.Info("mounting juicefs filesystem", "path", localPath)

	cacheSize := strconv.FormatInt(s.config.CacheSize, 10)

	prefetch := strconv.FormatInt(s.config.Prefetch, 10)
	if s.config.Prefetch <= 0 {
		prefetch = "1"
	}

	bufferSize := strconv.FormatInt(s.config.BufferSize, 10)
	if s.config.BufferSize <= 0 {
		bufferSize = "300"
	}

	s.mountCmd = exec.Command(
		"juicefs",
		"mount",
		s.config.RedisURI,
		localPath,
		"-d",
		"--bucket", s.config.Bucket,
		"--cache-size", cacheSize,
		"--prefetch", prefetch,
		"--buffer-size", bufferSize,
		"--no-bgjob",
		"--no-usage-report",
	)

	// Start the mount command in the background
	go func() {
		output, err := s.mountCmd.CombinedOutput()
		if err != nil {
			s.logger.Error("error executing juicefs mount", "error", err, "output", string(output))
		}
	}()

	// Wait for mount to complete
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	timeout := time.After(juiceFSMountTimeout)

	done := make(chan bool)
	go func() {
		for {
			select {
			case <-timeout:
				done <- false
				return
			case <-ticker.C:
				if IsMounted(localPath) {
					done <- true
					return
				}
			}
		}
	}()

	if !<-done {
		return fmt.Errorf("failed to mount juicefs filesystem to: %s", localPath)
	}

	s.logger.Info("juicefs filesystem mounted", "path", localPath)
	return nil
}

// Format formats the JuiceFS filesystem.
func (s *JuiceFS) Format(fsName string) error {
	blockSize := strconv.FormatInt(s.config.BlockSize, 10)
	if s.config.BlockSize <= 0 {
		blockSize = "4096"
	}

	cmd := exec.Command(
		"juicefs",
		"format",
		"--storage", "s3",
		"--bucket", s.config.Bucket,
		"--block-size", blockSize,
		s.config.RedisURI,
		fsName,
		"--no-update",
	)

	if s.config.AccessKey != "" || s.config.SecretKey != "" {
		cmd.Args = append(
			cmd.Args,
			"--access-key", s.config.AccessKey,
			"--secret-key", s.config.SecretKey,
		)
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("error executing juicefs format: %v, output: %s", err, string(output))
	}

	return nil
}

// Unmount unmounts the JuiceFS filesystem.
func (s *JuiceFS) Unmount(localPath string) error {
	cmd := exec.Command("juicefs", "umount", "--force", localPath)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("error executing juicefs umount: %v, output: %s", err, string(output))
	}

	s.logger.Info("juicefs filesystem unmounted", "path", localPath)
	return nil
}
