package client

import (
	"time"

	"github.com/codecflow/fabric/stash"
	"github.com/codecflow/fabric/stash/proto"
)

// ClientRequestType defines the type of client request.
type ClientRequestType int

const (
	ClientRequestTypeStorage   ClientRequestType = iota // Store content
	ClientRequestTypeRetrieval                          // Retrieve content
)

// ClientRequest represents a request to get a gRPC client.
type ClientRequest struct {
	rt        ClientRequestType
	hash      string
	key       string
	hostIndex int
}

// getGRPCClient returns the appropriate gRPC client for a request.
func (c *Client) getGRPCClient(request *ClientRequest) (proto.StashClient, *stash.Host, error) {
	var host *stash.Host = nil

	switch request.rt {
	case ClientRequestTypeStorage:
		hosts := c.hasher.GetN(c.clientConfig.NTopHosts, request.key)
		hostIndex := min(int64(request.hostIndex), int64(len(hosts)-1))

		if len(hosts) > 0 {
			host = hosts[hostIndex]
		}

	case ClientRequestTypeRetrieval:
		c.mu.RLock()
		cachedHost, hostFound := c.localHostCache[request.hash]
		c.mu.RUnlock()

		if hostFound {
			host = cachedHost.host
		} else {
			c.mu.Lock()

			hosts := c.hasher.GetN(c.clientConfig.NTopHosts, request.key)
			hostIndex := min(int64(request.hostIndex), int64(len(hosts)-1))

			if len(hosts) == 0 {
				host = nil
			} else {
				host = hosts[hostIndex]
				c.localHostCache[request.hash] = &localClientCache{
					host:      host,
					timestamp: time.Now(),
				}
			}

			c.mu.Unlock()
		}
	}

	if host == nil {
		return nil, nil, stash.ErrHostNotFound
	}

	client, exists := c.grpcClients[host.ID]
	if !exists {
		c.mu.Lock()
		delete(c.localHostCache, request.hash)
		c.mu.Unlock()

		return nil, nil, stash.ErrHostNotFound
	}

	return client, host, nil
}

// manageLocalClientCache periodically cleans up stale entries.
func (c *Client) manageLocalClientCache(interval time.Duration, ttl time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			now := time.Now()
			stale := make([]string, 0)

			c.mu.RLock()
			for hash, entry := range c.localHostCache {
				if now.Sub(entry.timestamp) > ttl {
					stale = append(stale, hash)
				}
			}
			c.mu.RUnlock()

			c.mu.Lock()
			for _, hash := range stale {
				delete(c.localHostCache, hash)
			}
			c.mu.Unlock()

		case <-c.ctx.Done():
			return
		}
	}
}

func min(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}
