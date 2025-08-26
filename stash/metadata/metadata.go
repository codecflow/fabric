package metadata

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/codecflow/fabric/stash"
)

// Config defines configuration for metadata operations.
type Config struct {
	HostKeepAliveTimeout    time.Duration
	StoreFromContentLockTTL time.Duration
	ClientLockTTL           time.Duration
}

// DefaultConfig returns default metadata configuration.
func DefaultConfig() Config {
	return Config{
		HostKeepAliveTimeout:    60 * time.Second,
		StoreFromContentLockTTL: 5 * time.Second,
		ClientLockTTL:           300 * time.Second,
	}
}

// Metadata provides high-level operations for managing cluster metadata.
type Metadata struct {
	backend Backend
	config  Config
}

// New creates a new Metadata instance with the given backend and config.
func New(backend Backend, config Config) *Metadata {
	return &Metadata{
		backend: backend,
		config:  config,
	}
}

// AddHostToIndex adds a host ID to the locality index.
func (m *Metadata) AddHostToIndex(ctx context.Context, locality string, host *stash.Host) error {
	key := Keys.HostIndex(locality)
	return m.backend.SAdd(ctx, key, host.ID)
}

// RemoveHostFromIndex removes a host ID from the locality index.
func (m *Metadata) RemoveHostFromIndex(ctx context.Context, locality string, host *stash.Host) error {
	key := Keys.HostIndex(locality)
	return m.backend.SRem(ctx, key, host.ID)
}

// SetHostKeepAlive stores host data with a TTL to indicate it's alive.
func (m *Metadata) SetHostKeepAlive(ctx context.Context, locality string, host *stash.Host) error {
	hostBytes, err := json.Marshal(host)
	if err != nil {
		return err
	}

	key := Keys.HostKeepAlive(locality, host.ID)
	return m.backend.Set(ctx, key, hostBytes, m.config.HostKeepAliveTimeout)
}

// GetAvailableHosts retrieves all active hosts in a locality.
// If a host's keepalive has expired, it's removed from the index and the callback is invoked.
func (m *Metadata) GetAvailableHosts(ctx context.Context, locality string, removeHostCallback func(host *stash.Host)) ([]*stash.Host, error) {
	indexKey := Keys.HostIndex(locality)
	hostIds, err := m.backend.SMembers(ctx, indexKey)
	if err != nil {
		return nil, err
	}

	hosts := []*stash.Host{}
	for _, hostId := range hostIds {
		keepaliveKey := Keys.HostKeepAlive(locality, hostId)
		hostBytes, err := m.backend.Get(ctx, keepaliveKey)
		if err != nil {
			// If the keepalive key doesn't exist (expired), remove from index
			if errors.Is(err, ErrKeyNotFound) {
				m.RemoveHostFromIndex(ctx, locality, &stash.Host{ID: hostId})
				if removeHostCallback != nil {
					removeHostCallback(&stash.Host{ID: hostId})
				}
			}
			continue
		}

		host := &stash.Host{}
		if err = json.Unmarshal(hostBytes, host); err != nil {
			continue
		}

		hosts = append(hosts, host)
	}

	if len(hosts) == 0 {
		return nil, errors.New("no available hosts")
	}

	return hosts, nil
}

// GetHostIndex returns all host IDs in a locality (without checking keepalive).
func (m *Metadata) GetHostIndex(ctx context.Context, locality string) ([]string, error) {
	key := Keys.HostIndex(locality)
	return m.backend.SMembers(ctx, key)
}

// SetClientLock acquires a distributed lock for a client operation on specific content.
func (m *Metadata) SetClientLock(ctx context.Context, clientId, hash string) error {
	key := Keys.ClientLock(clientId, hash)
	unlock, err := m.backend.Lock(ctx, key, m.config.ClientLockTTL, 0)
	if err != nil {
		return err
	}
	// Store unlock function for later release (caller manages this)
	_ = unlock
	return nil
}

// RemoveClientLock releases a distributed lock.
func (m *Metadata) RemoveClientLock(ctx context.Context, clientId, hash string) error {
	key := Keys.ClientLock(clientId, hash)
	// For now, we delete the key directly
	// In a full implementation, we'd track unlock functions
	return m.backend.Del(ctx, key)
}

// SetStoreFromContentLock acquires a lock for caching content from a source.
func (m *Metadata) SetStoreFromContentLock(ctx context.Context, locality, sourcePath string) error {
	key := Keys.StoreFromContentLock(locality, sourcePath)
	unlock, err := m.backend.Lock(ctx, key, m.config.StoreFromContentLockTTL, 0)
	if err != nil {
		return err
	}
	_ = unlock
	return nil
}

// RefreshStoreFromContentLock extends the TTL of a content lock.
func (m *Metadata) RefreshStoreFromContentLock(ctx context.Context, locality, sourcePath string) error {
	key := Keys.StoreFromContentLock(locality, sourcePath)
	return m.backend.RefreshLock(ctx, key, m.config.StoreFromContentLockTTL)
}

// RemoveStoreFromContentLock releases a content lock.
func (m *Metadata) RemoveStoreFromContentLock(ctx context.Context, locality, sourcePath string) error {
	key := Keys.StoreFromContentLock(locality, sourcePath)
	return m.backend.Del(ctx, key)
}

// Close closes the underlying backend connection.
func (m *Metadata) Close() error {
	return m.backend.Close()
}
