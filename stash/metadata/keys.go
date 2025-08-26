package metadata

import (
	"fmt"
	"strings"
)

// Keys provides methods to generate Redis keys for the stash metadata store.
var Keys = &keys{}

type keys struct{}

// HostIndex returns the Redis key for the set of host IDs in a locality.
// Example: "stash:host_index:us-east-1"
func (k *keys) HostIndex(locality string) string {
	return fmt.Sprintf("stash:host_index:%s", locality)
}

// HostKeepAlive returns the Redis key for a host's keepalive entry.
// This key has a TTL and contains the host's JSON data.
// Example: "stash:host:keepalive:us-east-1:stash-host-abc123"
func (k *keys) HostKeepAlive(locality, hostId string) string {
	return fmt.Sprintf("stash:host:keepalive:%s:%s", locality, hostId)
}

// FsNode returns the Redis key for a filesystem node's metadata.
// The value is a Redis hash containing all file/directory attributes.
// Example: "stash:fs:node:abc123def456"
func (k *keys) FsNode(id string) string {
	return fmt.Sprintf("stash:fs:node:%s", id)
}

// FsNodeChildren returns the Redis key for a filesystem node's children set.
// The value is a Redis set containing child node IDs.
// Example: "stash:fs:node:abc123def456:children"
func (k *keys) FsNodeChildren(id string) string {
	return fmt.Sprintf("stash:fs:node:%s:children", id)
}

// ClientLock returns the Redis key for a client's lock on specific content.
// Used to prevent concurrent operations on the same content hash.
// Example: "stash:client_lock:host-abc123:hash-def456"
func (k *keys) ClientLock(hostId, hash string) string {
	return fmt.Sprintf("stash:client_lock:%s:%s", hostId, hash)
}

// StoreFromContentLock returns the Redis key for locking content source operations.
// Prevents multiple hosts from simultaneously caching the same source file.
// Forward slashes in paths are replaced with underscores.
// Example: "stash:store_from_content_lock:us-east-1:_mnt_data_file.bin"
func (k *keys) StoreFromContentLock(locality, sourcePath string) string {
	sourcePath = strings.ReplaceAll(sourcePath, "/", "_")
	return fmt.Sprintf("stash:store_from_content_lock:%s:%s", locality, sourcePath)
}
