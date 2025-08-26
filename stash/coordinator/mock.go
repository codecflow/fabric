package coordinator

import (
	"context"
	"errors"
	"sync"

	"github.com/codecflow/fabric/stash"
	"github.com/codecflow/fabric/stash/config"
	"github.com/codecflow/fabric/stash/metadata"
)

// Mock is a simple in-memory coordinator for testing.
// Does not require Redis or any external dependencies.
type Mock struct {
	hosts        map[string]map[string]*stash.Host // locality -> hostId -> host
	fsNodes      map[string]*metadata.FsMetadata   // id -> metadata
	fsChildren   map[string][]string               // parent id -> child ids
	locks        map[string]bool                   // lock keys
	serverConfig *config.Server                    // server configuration
	mu           sync.RWMutex
}

// NewMock creates a new mock coordinator client.
func NewMock(serverConfig *config.Server) Client {
	return &Mock{
		hosts:        make(map[string]map[string]*stash.Host),
		fsNodes:      make(map[string]*metadata.FsMetadata),
		fsChildren:   make(map[string][]string),
		locks:        make(map[string]bool),
		serverConfig: serverConfig,
	}
}

// AddHostToIndex adds a host to the locality index.
func (m *Mock) AddHostToIndex(ctx context.Context, locality string, host *stash.Host) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.hosts[locality]; !exists {
		m.hosts[locality] = make(map[string]*stash.Host)
	}
	m.hosts[locality][host.ID] = host
	return nil
}

// SetHostKeepAlive updates the host's keepalive timestamp.
func (m *Mock) SetHostKeepAlive(ctx context.Context, locality string, host *stash.Host) error {
	return m.AddHostToIndex(ctx, locality, host)
}

// GetAvailableHosts retrieves all active hosts in a locality.
func (m *Mock) GetAvailableHosts(ctx context.Context, locality string) ([]*stash.Host, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	hosts := []*stash.Host{}
	if localityHosts, exists := m.hosts[locality]; exists {
		for _, host := range localityHosts {
			hosts = append(hosts, host)
		}
	}
	return hosts, nil
}

// GetRegionConfig returns the server configuration for a locality.
func (m *Mock) GetRegionConfig(ctx context.Context, locality string) (*config.Server, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.serverConfig == nil {
		return nil, errors.New("server config not set")
	}

	if regionConfig, ok := m.serverConfig.Regions[locality]; ok {
		return &regionConfig.ServerConfig, nil
	}
	return m.serverConfig, nil
}

// SetClientLock acquires a client lock for content.
func (m *Mock) SetClientLock(ctx context.Context, hash string, hostId string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := "client-lock:" + hash
	if m.locks[key] {
		return errors.New("lock already held")
	}
	m.locks[key] = true
	return nil
}

// RemoveClientLock releases a client lock.
func (m *Mock) RemoveClientLock(ctx context.Context, hash string, hostId string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := "client-lock:" + hash
	delete(m.locks, key)
	return nil
}

// SetStoreFromContentLock acquires a lock for caching content from a source.
func (m *Mock) SetStoreFromContentLock(ctx context.Context, locality string, sourcePath string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := "store-lock:" + locality + ":" + sourcePath
	if m.locks[key] {
		return errors.New("lock already held")
	}
	m.locks[key] = true
	return nil
}

// RemoveStoreFromContentLock releases a content source lock.
func (m *Mock) RemoveStoreFromContentLock(ctx context.Context, locality string, sourcePath string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := "store-lock:" + locality + ":" + sourcePath
	delete(m.locks, key)
	return nil
}

// RefreshStoreFromContentLock extends the TTL of a content source lock.
func (m *Mock) RefreshStoreFromContentLock(ctx context.Context, locality string, sourcePath string) error {
	// No-op for mock - locks don't expire
	return nil
}

// GetFsNode retrieves filesystem node metadata.
func (m *Mock) GetFsNode(ctx context.Context, id string) (*metadata.FsMetadata, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	node, exists := m.fsNodes[id]
	if !exists {
		return nil, errors.New("fs node not found")
	}
	return node, nil
}

// SetFsNode stores filesystem node metadata.
func (m *Mock) SetFsNode(ctx context.Context, id string, node *metadata.FsMetadata) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	node.ID = id // Ensure ID matches
	m.fsNodes[id] = node
	return nil
}

// RemoveFsNode deletes a filesystem node.
func (m *Mock) RemoveFsNode(ctx context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.fsNodes, id)
	return nil
}

// GetFsNodeChildren retrieves all child nodes of a directory.
func (m *Mock) GetFsNodeChildren(ctx context.Context, id string) ([]*metadata.FsMetadata, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	childIds, exists := m.fsChildren[id]
	if !exists {
		return []*metadata.FsMetadata{}, nil
	}

	children := []*metadata.FsMetadata{}
	for _, childId := range childIds {
		if node, exists := m.fsNodes[childId]; exists {
			children = append(children, node)
		}
	}
	return children, nil
}

// AddFsNodeChild adds a child to a directory.
func (m *Mock) AddFsNodeChild(ctx context.Context, parentId, childId string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.fsChildren[parentId]; !exists {
		m.fsChildren[parentId] = []string{}
	}

	// Don't add duplicates
	for _, existingId := range m.fsChildren[parentId] {
		if existingId == childId {
			return nil
		}
	}

	m.fsChildren[parentId] = append(m.fsChildren[parentId], childId)
	return nil
}

// RemoveFsNodeChild removes a child from a directory.
func (m *Mock) RemoveFsNodeChild(ctx context.Context, parentId, childId string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if children, exists := m.fsChildren[parentId]; exists {
		newChildren := []string{}
		for _, id := range children {
			if id != childId {
				newChildren = append(newChildren, id)
			}
		}
		m.fsChildren[parentId] = newChildren
	}
	return nil
}
