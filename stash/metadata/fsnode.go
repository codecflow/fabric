package metadata

import (
	"context"
	"errors"
)

// FsMetadata represents filesystem node metadata for BlobFS (FUSE).
// This structure is used to store inode information required by the FUSE kernel interface.
type FsMetadata struct {
	PID       string `redis:"pid"`       // Parent ID
	ID        string `redis:"id"`        // Node ID
	Name      string `redis:"name"`      // Filename
	Path      string `redis:"path"`      // Full path
	Hash      string `redis:"hash"`      // Content hash (for files)
	Ino       uint64 `redis:"ino"`       // Inode number
	Size      uint64 `redis:"size"`      // File size in bytes
	Blocks    uint64 `redis:"blocks"`    // Number of 512B blocks allocated
	Atime     uint64 `redis:"atime"`     // Last access time (seconds since epoch)
	Mtime     uint64 `redis:"mtime"`     // Last modification time
	Ctime     uint64 `redis:"ctime"`     // Last status change time
	Atimensec uint32 `redis:"atimensec"` // Nanosecond component of atime
	Mtimensec uint32 `redis:"mtimensec"` // Nanosecond component of mtime
	Ctimensec uint32 `redis:"ctimensec"` // Nanosecond component of ctime
	Mode      uint32 `redis:"mode"`      // File mode (permissions + type)
	Nlink     uint32 `redis:"nlink"`     // Number of hard links
	Rdev      uint32 `redis:"rdev"`      // Device ID (for special files)
	Blksize   uint32 `redis:"blksize"`   // Preferred I/O block size
	Padding   uint32 `redis:"padding"`   // Reserved for alignment
	Uid       uint32 `redis:"uid"`       // Owner user ID
	Gid       uint32 `redis:"gid"`       // Owner group ID
	Gen       uint64 `redis:"gen"`       // Generation number (increments on update)
}

// GetFsNode retrieves filesystem node metadata by ID.
func (m *Metadata) GetFsNode(ctx context.Context, id string) (*FsMetadata, error) {
	key := Keys.FsNode(id)
	var node FsMetadata
	if err := m.backend.GetObject(ctx, key, &node); err != nil {
		return nil, err
	}
	return &node, nil
}

// SetFsNode stores filesystem node metadata.
// If the node already exists, increments the generation number for cache invalidation.
func (m *Metadata) SetFsNode(ctx context.Context, node *FsMetadata) error {
	if node.ID == "" {
		return errors.New("node ID is required")
	}

	// Check if node exists and increment Gen if so
	existing, err := m.GetFsNode(ctx, node.ID)
	if err == nil && existing != nil {
		node.Gen = existing.Gen + 1
	}

	key := Keys.FsNode(node.ID)
	return m.backend.SetObject(ctx, key, node)
}

// RemoveFsNode deletes a filesystem node's metadata.
func (m *Metadata) RemoveFsNode(ctx context.Context, id string) error {
	key := Keys.FsNode(id)
	return m.backend.Del(ctx, key)
}

// GetFsNodeChildren retrieves full metadata for all child nodes in a directory.
// Skips any nodes that fail to load.
func (m *Metadata) GetFsNodeChildren(ctx context.Context, id string) ([]*FsMetadata, error) {
	key := Keys.FsNodeChildren(id)
	nodeIds, err := m.backend.SMembers(ctx, key)
	if err != nil {
		return nil, err
	}

	entries := []*FsMetadata{}
	for _, nodeId := range nodeIds {
		node, err := m.GetFsNode(ctx, nodeId)
		if err != nil {
			continue // Skip nodes that fail to load
		}
		entries = append(entries, node)
	}

	return entries, nil
}

// AddFsNodeChild adds a child node ID to a parent directory.
func (m *Metadata) AddFsNodeChild(ctx context.Context, parentId, childId string) error {
	key := Keys.FsNodeChildren(parentId)
	return m.backend.SAdd(ctx, key, childId)
}

// RemoveFsNodeChild removes a child node ID from a parent directory.
func (m *Metadata) RemoveFsNodeChild(ctx context.Context, parentId, childId string) error {
	key := Keys.FsNodeChildren(parentId)
	return m.backend.SRem(ctx, key, childId)
}
