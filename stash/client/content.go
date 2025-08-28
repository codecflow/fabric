package client

import (
	"context"
	"io"
	"time"

	"github.com/codecflow/fabric/stash"
	"github.com/codecflow/fabric/stash/proto"
)

// ContentOptions provides options for content operations.
type ContentOptions struct {
	RoutingKey string
}

// GetContent retrieves content by hash with retry logic.
func (c *Client) GetContent(hash string, offset int64, length int64, opts ContentOptions) ([]byte, error) {
	ctx, cancel := context.WithTimeout(c.ctx, getContentRequestTimeout)
	defer cancel()

	maxAttempts := 1
	if length > c.minRetryLengthBytes {
		maxAttempts = int(c.maxGetContentAttempts)
	}

	if opts.RoutingKey == "" {
		opts.RoutingKey = hash
	}

	for attempt := 0; attempt < maxAttempts; attempt++ {
		client, _, err := c.getGRPCClient(&ClientRequest{
			rt:        ClientRequestTypeRetrieval,
			hash:      hash,
			key:       opts.RoutingKey,
			hostIndex: attempt,
		})
		if err != nil {
			return nil, err
		}

		start := time.Now()
		getContentResponse, err := client.GetContent(ctx, &proto.GetContentRequest{
			Hash:   hash,
			Offset: offset,
			Length: length,
		})
		if err != nil || !getContentResponse.Ok {
			c.mu.Lock()
			delete(c.localHostCache, hash)
			c.mu.Unlock()
			continue
		}

		c.logger.Debug("get content", "hash", hash, "elapsed", time.Since(start))
		return getContentResponse.Content, nil
	}

	return nil, stash.ErrContentNotFound
}

// GetContentStream streams content by hash.
func (c *Client) GetContentStream(hash string, offset int64, length int64, opts ContentOptions) (chan []byte, error) {
	ctx, cancel := context.WithTimeout(c.ctx, getContentStreamRequestTimeout)
	contentChan := make(chan []byte)

	if opts.RoutingKey == "" {
		opts.RoutingKey = hash
	}

	go func() {
		defer close(contentChan)
		defer cancel()

		maxAttempts := 1
		if length > c.minRetryLengthBytes {
			maxAttempts = int(c.maxGetContentAttempts)
		}

		for attempt := 0; attempt < maxAttempts; attempt++ {
			client, _, err := c.getGRPCClient(&ClientRequest{
				rt:        ClientRequestTypeRetrieval,
				hash:      hash,
				key:       opts.RoutingKey,
				hostIndex: attempt,
			})
			if err != nil {
				return
			}

			stream, err := client.GetContentStream(ctx, &proto.GetContentRequest{
				Hash:   hash,
				Offset: offset,
				Length: length,
			})
			if err != nil {
				c.mu.Lock()
				delete(c.localHostCache, hash)
				c.mu.Unlock()
				continue
			}

			for {
				resp, err := stream.Recv()
				if err == io.EOF {
					return
				}

				if err != nil || !resp.Ok {
					c.mu.Lock()
					delete(c.localHostCache, hash)
					c.mu.Unlock()
					break
				}

				contentChan <- resp.Content
			}
		}
	}()

	return contentChan, nil
}

// StoreContent stores content via streaming upload.
func (c *Client) StoreContent(chunks chan []byte, hash string, opts ContentOptions) (string, error) {
	ctx, cancel := context.WithTimeout(c.ctx, storeContentRequestTimeout)
	defer cancel()

	if opts.RoutingKey == "" {
		opts.RoutingKey = hash
	}

	client, _, err := c.getGRPCClient(&ClientRequest{
		rt:        ClientRequestTypeStorage,
		hash:      hash,
		key:       opts.RoutingKey,
		hostIndex: 0,
	})
	if err != nil {
		return "", err
	}

	stream, err := client.StoreContent(ctx)
	if err != nil {
		return "", err
	}

	start := time.Now()
	for chunk := range chunks {
		req := &proto.StoreContentRequest{Content: chunk}
		if err := stream.Send(req); err != nil {
			return "", err
		}
	}

	resp, err := stream.CloseAndRecv()
	if err != nil {
		return "", err
	}

	c.logger.Debug("store content", "hash", resp.Hash, "elapsed", time.Since(start))
	return resp.Hash, nil
}

// IsCachedNearby checks if content is cached on any of the N closest hosts.
func (c *Client) IsCachedNearby(hash string, routingKey string) (bool, error) {
	hostsToCheck := c.clientConfig.NTopHosts

	for hostIndex := 0; hostIndex < hostsToCheck; hostIndex++ {
		client, _, err := c.getGRPCClient(&ClientRequest{
			rt:        ClientRequestTypeRetrieval,
			hash:      hash,
			key:       routingKey,
			hostIndex: hostIndex,
		})
		if err != nil {
			return false, err
		}

		resp, err := client.HasContent(c.ctx, &proto.HasContentRequest{Hash: hash})
		if err != nil {
			return false, err
		}

		if resp.Exists {
			return true, nil
		}
	}

	return false, nil
}

// IsPathCachedNearby checks if a filesystem path is cached nearby.
func (c *Client) IsPathCachedNearby(ctx context.Context, path string) bool {
	// Generate filesystem node ID
	nodeID := stash.GenerateFsID(path)

	metadata, err := c.coordinator.GetFsNode(ctx, nodeID)
	if err != nil {
		c.logger.Error("error getting fs node", "path", path, "error", err)
		return false
	}

	exists, err := c.IsCachedNearby(metadata.Hash, path)
	if err != nil {
		c.logger.Error("error checking if content is cached nearby", "hash", metadata.Hash, "error", err)
		return false
	}

	return exists
}
