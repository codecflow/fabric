package coordinator

import (
	"context"

	"github.com/codecflow/fabric/stash"
	"github.com/codecflow/fabric/stash/config"
	"github.com/codecflow/fabric/stash/metadata"
)

// Local implements the Client interface with direct metadata access.
// Used by coordinator servers to manage metadata locally.
type Local struct {
	metadata     *metadata.Metadata
	serverConfig *config.Server
}

// NewLocal creates a new local coordinator client.
func NewLocal(meta *metadata.Metadata, serverConfig *config.Server) Client {
	return &Local{
		metadata:     meta,
		serverConfig: serverConfig,
	}
}

// AddHostToIndex adds a host to the locality index.
func (c *Local) AddHostToIndex(ctx context.Context, locality string, host *stash.Host) error {
	return c.metadata.AddHostToIndex(ctx, locality, host)
}

// SetHostKeepAlive updates the host's keepalive timestamp.
func (c *Local) SetHostKeepAlive(ctx context.Context, locality string, host *stash.Host) error {
	return c.metadata.SetHostKeepAlive(ctx, locality, host)
}

// GetAvailableHosts retrieves all active hosts in a locality.
func (c *Local) GetAvailableHosts(ctx context.Context, locality string) ([]*stash.Host, error) {
	return c.metadata.GetAvailableHosts(ctx, locality, func(host *stash.Host) {
		c.metadata.RemoveHostFromIndex(ctx, locality, host)
	})
}

// GetRegionConfig returns the server configuration for a locality.
// If a region-specific config exists, returns that; otherwise returns the default.
func (c *Local) GetRegionConfig(ctx context.Context, locality string) (*config.Server, error) {
	if regionConfig, ok := c.serverConfig.Regions[locality]; ok {
		return &regionConfig.ServerConfig, nil
	}
	return c.serverConfig, nil
}

// SetClientLock acquires a client lock for content.
func (c *Local) SetClientLock(ctx context.Context, hash string, hostId string) error {
	return c.metadata.SetClientLock(ctx, hostId, hash)
}

// RemoveClientLock releases a client lock.
func (c *Local) RemoveClientLock(ctx context.Context, hash string, hostId string) error {
	return c.metadata.RemoveClientLock(ctx, hostId, hash)
}

// SetStoreFromContentLock acquires a lock for caching content from a source.
func (c *Local) SetStoreFromContentLock(ctx context.Context, locality string, sourcePath string) error {
	return c.metadata.SetStoreFromContentLock(ctx, locality, sourcePath)
}

// RemoveStoreFromContentLock releases a content source lock.
func (c *Local) RemoveStoreFromContentLock(ctx context.Context, locality string, sourcePath string) error {
	return c.metadata.RemoveStoreFromContentLock(ctx, locality, sourcePath)
}

// RefreshStoreFromContentLock extends the TTL of a content source lock.
func (c *Local) RefreshStoreFromContentLock(ctx context.Context, locality string, sourcePath string) error {
	return c.metadata.RefreshStoreFromContentLock(ctx, locality, sourcePath)
}

// GetFsNode retrieves filesystem node metadata.
func (c *Local) GetFsNode(ctx context.Context, id string) (*metadata.FsMetadata, error) {
	return c.metadata.GetFsNode(ctx, id)
}

// SetFsNode stores filesystem node metadata.
func (c *Local) SetFsNode(ctx context.Context, id string, node *metadata.FsMetadata) error {
	node.ID = id // Ensure ID matches
	return c.metadata.SetFsNode(ctx, node)
}

// RemoveFsNode deletes a filesystem node.
func (c *Local) RemoveFsNode(ctx context.Context, id string) error {
	return c.metadata.RemoveFsNode(ctx, id)
}

// GetFsNodeChildren retrieves all child nodes of a directory.
func (c *Local) GetFsNodeChildren(ctx context.Context, id string) ([]*metadata.FsMetadata, error) {
	return c.metadata.GetFsNodeChildren(ctx, id)
}

// AddFsNodeChild adds a child to a directory.
func (c *Local) AddFsNodeChild(ctx context.Context, parentId, childId string) error {
	return c.metadata.AddFsNodeChild(ctx, parentId, childId)
}

// RemoveFsNodeChild removes a child from a directory.
func (c *Local) RemoveFsNodeChild(ctx context.Context, parentId, childId string) error {
	return c.metadata.RemoveFsNodeChild(ctx, parentId, childId)
}
