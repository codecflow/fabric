package metadata

import (
	"context"
	"errors"
	"time"
)

// ErrKeyNotFound is returned when a key doesn't exist in the backend.
var ErrKeyNotFound = errors.New("key not found")

// Backend defines the interface for metadata storage.
// Implementations can use Redis, in-memory maps, etcd, or any other K/V store.
//
// The interface provides primitives for:
//   - Sets: Managing collections (e.g., host indexes, directory children)
//   - Key-Value: Simple data with optional TTL (e.g., host keepalives)
//   - Hashes: Structured data (e.g., filesystem node metadata)
//   - Locks: Distributed coordination (e.g., preventing concurrent cache operations)
type Backend interface {
	// Set operations
	// Used for: host indexes, filesystem node children
	SAdd(ctx context.Context, key string, members ...string) error
	SRem(ctx context.Context, key string, members ...string) error
	SMembers(ctx context.Context, key string) ([]string, error)

	// Key-value operations with optional TTL
	// Used for: host keepalive data, region configs
	Set(ctx context.Context, key string, value []byte, ttl time.Duration) error
	Get(ctx context.Context, key string) ([]byte, error)
	Del(ctx context.Context, key string) error

	// Hash operations (structured key-value within a single key)
	// Used for: filesystem node metadata (ino, size, mode, etc.)
	HSet(ctx context.Context, key string, fields map[string]interface{}) error
	HGetAll(ctx context.Context, key string) (map[string]string, error)

	// Distributed locking
	// Lock attempts to acquire a lock and returns an unlock function
	// retries: number of retry attempts (0 = fail immediately)
	Lock(ctx context.Context, key string, ttl time.Duration, retries int) (unlock func() error, err error)

	// RefreshLock extends the TTL of an existing lock
	// Used for: long-running operations that need to hold locks
	RefreshLock(ctx context.Context, key string, ttl time.Duration) error

	// Close releases resources and closes the backend connection
	Close() error
}
