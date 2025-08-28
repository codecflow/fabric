package source

import (
	"errors"
	"log/slog"
	"syscall"

	"github.com/codecflow/fabric/stash/config"
)

const (
	ModeJuiceFS    string = "juicefs"
	ModeMountPoint string = "mountpoint"
)

// Source represents an external storage backend that can be mounted.
type Source interface {
	Mount(localPath string) error
	Format(fsName string) error
	Unmount(localPath string) error
}

// IsMounted checks if a path is a mount point by checking if its inode is 1.
func IsMounted(mountPoint string) bool {
	var stat syscall.Stat_t
	err := syscall.Stat(mountPoint, &stat)
	if err != nil {
		return false
	}
	return stat.Ino == 1
}

// New creates a new Source based on the configuration.
func New(cfg config.Source, logger *slog.Logger) (Source, error) {
	if logger == nil {
		logger = slog.Default()
	}

	switch cfg.Mode {
	case ModeJuiceFS:
		s, err := NewJuiceFS(cfg.JuiceFS, logger)
		if err != nil {
			return nil, err
		}

		// Format filesystem (no-op if already formatted)
		if err := s.Format(cfg.FilesystemName); err != nil {
			logger.Error("unable to format filesystem", "error", err)
			return nil, err
		}

		// Mount filesystem
		if err := s.Mount(cfg.FilesystemPath); err != nil {
			logger.Error("unable to mount filesystem", "error", err)
			return nil, err
		}

		return s, nil

	case ModeMountPoint:
		s, err := NewMountPoint(cfg.MountPoint, logger)
		if err != nil {
			return nil, err
		}

		// Mount filesystem
		if err := s.Mount(cfg.FilesystemPath); err != nil {
			logger.Error("unable to mount filesystem", "error", err)
			return nil, err
		}

		return s, nil
	}

	return nil, errors.New("invalid source mode")
}
