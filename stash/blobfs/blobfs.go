package blobfs

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/codecflow/fabric/stash"
	"github.com/codecflow/fabric/stash/client"
	"github.com/codecflow/fabric/stash/config"
	"github.com/codecflow/fabric/stash/coordinator"
	"github.com/codecflow/fabric/stash/metadata"
	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
	"github.com/moby/sys/mountinfo"
)

// SystemOpts configures the BlobFS filesystem.
type SystemOpts struct {
	Verbose     bool
	Coordinator coordinator.Client
	Config      *config.Client
	Client      *client.Client
	Logger      *slog.Logger
}

// BlobFS is the FUSE filesystem implementation.
type BlobFS struct {
	ctx         context.Context
	root        *Node
	verbose     bool
	coordinator coordinator.Client
	client      *client.Client
	config      *config.Client
	logger      *slog.Logger
}

// Mount mounts the BlobFS filesystem at the configured mount point.
func Mount(ctx context.Context, opts SystemOpts) (func() error, <-chan error, *fuse.Server, error) {
	mountPoint := opts.Config.BlobFs.MountPoint

	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}

	opts.Logger.Info("mounting blobfs", "path", mountPoint)

	// Create mount point if it doesn't exist
	if _, err := os.Stat(mountPoint); os.IsNotExist(err) {
		if err := os.MkdirAll(mountPoint, 0755); err != nil {
			return nil, nil, nil, fmt.Errorf("failed to create mount point: %w", err)
		}
		opts.Logger.Info("mount point created")
	} else if isFuseMount(mountPoint) {
		if err := forceUnmount(mountPoint); err != nil {
			return nil, nil, nil, fmt.Errorf("failed to unmount existing FUSE mount: %w", err)
		}
	}

	blobfs, err := NewFileSystem(ctx, opts)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("could not create filesystem: %w", err)
	}

	root, _ := blobfs.Root()

	// Configure timeouts
	attrTimeout := time.Second * 5
	entryTimeout := time.Second * 5
	negativeTimeout := time.Second * 2

	fsOptions := &fs.Options{
		AttrTimeout:     &attrTimeout,
		EntryTimeout:    &entryTimeout,
		NegativeTimeout: &negativeTimeout,
	}

	// Configure limits
	maxWriteKB := opts.Config.BlobFs.MaxWriteKB
	if maxWriteKB <= 0 {
		maxWriteKB = 1024
	}

	maxReadAheadKB := opts.Config.BlobFs.MaxReadAheadKB
	if maxReadAheadKB <= 0 {
		maxReadAheadKB = 128
	}

	maxBackgroundTasks := opts.Config.BlobFs.MaxBackgroundTasks
	if maxBackgroundTasks <= 0 {
		maxBackgroundTasks = 512
	}

	options := append([]string{}, opts.Config.BlobFs.Options...)

	server, err := fuse.NewServer(fs.NewNodeFS(root, fsOptions), mountPoint, &fuse.MountOptions{
		MaxBackground:        maxBackgroundTasks,
		DisableXAttrs:        true,
		EnableSymlinkCaching: true,
		SyncRead:             false,
		RememberInodes:       true,
		MaxReadAhead:         maxReadAheadKB * 1024,
		MaxWrite:             maxWriteKB * 1024,
		Options:              options,
		DirectMount:          opts.Config.BlobFs.DirectMount,
	})
	if err != nil {
		return nil, nil, nil, fmt.Errorf("could not create server: %w", err)
	}

	serverError := make(chan error, 1)
	startServer := func() error {
		go func() {
			go server.Serve()

			if err := server.WaitMount(); err != nil {
				serverError <- err
				return
			}

			server.Wait()
			close(serverError)
		}()

		return nil
	}

	return startServer, serverError, server, nil
}

// UpdateReadAheadKB updates the kernel readahead value for the mount.
func UpdateReadAheadKB(mountPoint string, valueKB int) error {
	mounts, err := mountinfo.GetMounts(nil)
	if err != nil {
		return fmt.Errorf("failed to get mount info: %w", err)
	}

	var deviceID string
	for _, mount := range mounts {
		if mount.Mountpoint == mountPoint {
			deviceID = fmt.Sprintf("%d:%d", mount.Major, mount.Minor)
			break
		}
	}

	if deviceID == "" {
		return fmt.Errorf("mount point %s not found", mountPoint)
	}

	readAheadPath := fmt.Sprintf("/sys/class/bdi/%s/read_ahead_kb", deviceID)
	cmd := exec.Command("sh", "-c", fmt.Sprintf("echo %d > %s", valueKB, readAheadPath))
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to update read_ahead_kb at %s: %w", readAheadPath, err)
	}

	return nil
}

// NewFileSystem initializes a new BlobFS with root metadata.
func NewFileSystem(ctx context.Context, opts SystemOpts) (*BlobFS, error) {
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}

	bfs := &BlobFS{
		ctx:         ctx,
		verbose:     opts.Verbose,
		config:      opts.Config,
		client:      opts.Client,
		coordinator: opts.Coordinator,
		logger:      opts.Logger,
	}

	rootID := stash.GenerateFsID("/")
	rootPath := "/"

	dirMeta, err := opts.Coordinator.GetFsNode(ctx, rootID)
	if err != nil || dirMeta == nil {
		opts.Logger.Info("root node metadata not found, creating")

		dirMeta = &metadata.FsMetadata{
			PID:  "",
			ID:   rootID,
			Path: rootPath,
			Ino:  1,
			Mode: fuse.S_IFDIR | 0755,
		}

		if err := opts.Coordinator.SetFsNode(ctx, rootID, dirMeta); err != nil {
			return nil, fmt.Errorf("unable to create blobfs root: %w", err)
		}
	}

	attr := fuse.Attr{
		Ino:  1,
		Mode: dirMeta.Mode,
	}

	rootNode := &Node{
		filesystem: bfs,
		attr:       attr,
		bfsNode: &BlobFsNode{
			Path: dirMeta.Path,
			ID:   dirMeta.ID,
			PID:  dirMeta.PID,
			Attr: attr,
		},
	}

	bfs.root = rootNode
	return bfs, nil
}

// Root returns the root node of the filesystem.
func (bfs *BlobFS) Root() (fs.InodeEmbedder, error) {
	if bfs.root == nil {
		return nil, fmt.Errorf("root not initialized")
	}
	return bfs.root, nil
}

// isFuseMount checks if a path is a FUSE mount.
func isFuseMount(mountPoint string) bool {
	cmd := exec.Command("findmnt", "-n", "-o", "FSTYPE", mountPoint)
	output, err := cmd.Output()
	if err != nil {
		return false
	}
	return strings.Contains(string(output), "fuse")
}

// forceUnmount forcefully unmounts a FUSE filesystem.
func forceUnmount(mountPoint string) error {
	cmd := exec.Command("fusermount", "-uz", mountPoint)
	if _, err := cmd.CombinedOutput(); err != nil {
		return err
	}
	return nil
}
