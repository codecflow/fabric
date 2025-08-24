package image

import (
	"context"
	"sync"
)

// MemoryCache is a simple in-memory cache
type MemoryCache struct {
	mu    sync.RWMutex
	data  map[string]string
}

// NewMemoryCache creates a new memory cache
func NewMemoryCache() *MemoryCache {
	return &MemoryCache{
		data: make(map[string]string),
	}
}

// Get retrieves a cached image ID
func (c *MemoryCache) Get(ctx context.Context, key string) (string, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	val, ok := c.data[key]
	return val, ok
}

// Set stores an image ID in cache
func (c *MemoryCache) Set(ctx context.Context, key string, imageID string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	c.data[key] = imageID
	return nil
}

// Delete removes from cache
func (c *MemoryCache) Delete(ctx context.Context, key string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	delete(c.data, key)
	return nil
}
