package fuse

import (
	"fmt"
	"os"
	"time"

	"github.com/codecflow/fabric/mirage"
	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
)

// MountOptions configures FUSE mount behavior
type MountOptions struct {
	ReadOnly      bool     // Mount as read-only (default: true)
	AllowOther    bool     // Allow other users to access
	Debug         bool     // Enable debug logging
	DirectMount   bool     // Use direct mount (macOS)
	Options       []string // Additional FUSE options
	MaxReadAhead  int      // Max read-ahead in KB (default: 128)
	MaxBackground int      // Max background requests (default: 512)
}

// Mount mounts a mirage FileSystem at the specified mount point via FUSE
func Mount(filesystem mirage.FileSystem, mountPoint string, opts MountOptions) (*fuse.Server, error) {
	// Validate parameters
	if filesystem == nil {
		return nil, fmt.Errorf("filesystem cannot be nil")
	}
	if mountPoint == "" {
		return nil, fmt.Errorf("mount point cannot be empty")
	}

	// Create mount point if it doesn't exist
	if err := os.MkdirAll(mountPoint, 0755); err != nil {
		return nil, fmt.Errorf("failed to create mount point: %w", err)
	}

	// Get root node from filesystem
	rootNode, err := filesystem.Node("/")
	if err != nil {
		return nil, fmt.Errorf("failed to get root node: %w", err)
	}

	// Create FUSE root wrapper
	root := &node{
		filesystem: filesystem,
		meta:       rootNode,
	}

	// Configure timeouts
	attrTimeout := 5 * time.Second
	entryTimeout := 5 * time.Second

	fsOpts := &fs.Options{
		AttrTimeout:  &attrTimeout,
		EntryTimeout: &entryTimeout,
	}

	// Set defaults
	maxReadAhead := opts.MaxReadAhead
	if maxReadAhead <= 0 {
		maxReadAhead = 128
	}

	maxBackground := opts.MaxBackground
	if maxBackground <= 0 {
		maxBackground = 512
	}

	// Build FUSE mount options
	mountOpts := &fuse.MountOptions{
		MaxBackground: maxBackground,
		MaxReadAhead:  maxReadAhead * 1024,
		Options:       opts.Options,
		DirectMount:   opts.DirectMount,
		Debug:         opts.Debug,
	}

	if opts.AllowOther {
		mountOpts.Options = append(mountOpts.Options, "allow_other")
	}

	if opts.ReadOnly {
		mountOpts.Options = append(mountOpts.Options, "ro")
	}

	// Create FUSE server
	server, err := fuse.NewServer(fs.NewNodeFS(root, fsOpts), mountPoint, mountOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to create FUSE server: %w", err)
	}

	// Start serving
	go server.Serve()

	// Wait for mount to complete
	if err := server.WaitMount(); err != nil {
		return nil, fmt.Errorf("failed to mount: %w", err)
	}

	return server, nil
}

// Unmount unmounts a FUSE filesystem
// Note: The server's Unmount() method should be called instead when possible
func Unmount(server *fuse.Server) error {
	if server == nil {
		return fmt.Errorf("server cannot be nil")
	}
	return server.Unmount()
}
