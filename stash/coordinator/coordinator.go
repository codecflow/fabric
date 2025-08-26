package coordinator

import (
	"context"

	"github.com/codecflow/fabric/stash"
	"github.com/codecflow/fabric/stash/config"
	"github.com/codecflow/fabric/stash/metadata"
)

// Client defines the interface for coordinator operations.
// It provides two implementations:
//   - Local: Direct access to metadata (used by coordinator server)
//   - Remote: gRPC client to coordinator (used by slave servers/clients)
type Client interface {
	// Host operations
	AddHostToIndex(ctx context.Context, locality string, host *stash.Host) error
	SetHostKeepAlive(ctx context.Context, locality string, host *stash.Host) error
	GetAvailableHosts(ctx context.Context, locality string) ([]*stash.Host, error)
	GetRegionConfig(ctx context.Context, locality string) (*config.Server, error)

	// Client locking
	SetClientLock(ctx context.Context, hash string, hostId string) error
	RemoveClientLock(ctx context.Context, hash string, hostId string) error

	// Content locking
	SetStoreFromContentLock(ctx context.Context, locality string, sourcePath string) error
	RemoveStoreFromContentLock(ctx context.Context, locality string, sourcePath string) error
	RefreshStoreFromContentLock(ctx context.Context, locality string, sourcePath string) error

	// Filesystem operations
	GetFsNode(ctx context.Context, id string) (*metadata.FsMetadata, error)
	SetFsNode(ctx context.Context, id string, node *metadata.FsMetadata) error
	RemoveFsNode(ctx context.Context, id string) error
	GetFsNodeChildren(ctx context.Context, id string) ([]*metadata.FsMetadata, error)
	AddFsNodeChild(ctx context.Context, parentId, childId string) error
	RemoveFsNodeChild(ctx context.Context, parentId, childId string) error
}
