package client

import (
	"context"
	"fmt"

	"github.com/codecflow/fabric/stash"
	"github.com/codecflow/fabric/stash/proto"
)

// FUSESource represents a filesystem source.
type FUSESource struct {
	Path string
}

// SourceOptions provides options for source caching operations.
type SourceOptions struct {
	RoutingKey string
	Lock       bool
}

// StoreContentFromFUSE caches content from the local filesystem.
func (c *Client) StoreContentFromFUSE(source FUSESource, opts SourceOptions) (string, error) {
	ctx, cancel := context.WithTimeout(c.ctx, storeContentRequestTimeout)
	defer cancel()

	if opts.RoutingKey == "" {
		opts.RoutingKey = source.Path
	}

	client, _, err := c.getGRPCClient(&ClientRequest{
		rt:        ClientRequestTypeStorage,
		hash:      source.Path,
		key:       opts.RoutingKey,
		hostIndex: 0,
	})
	if err != nil {
		return "", err
	}

	if opts.Lock {
		resp, err := client.StoreContentFromSourceWithLock(ctx, &proto.StoreContentFromSourceRequest{
			Source: &proto.CacheSource{
				Path: source.Path,
			},
		})
		if err != nil {
			return "", err
		}

		if resp.FailedToAcquireLock {
			return "", stash.ErrUnableToAcquireLock
		}

		if !resp.Ok {
			return "", stash.ErrUnableToPopulateContent
		}

		return resp.Hash, nil
	}

	resp, err := client.StoreContentFromSource(ctx, &proto.StoreContentFromSourceRequest{
		Source: &proto.CacheSource{
			Path: source.Path,
		},
	})
	if err != nil {
		return "", err
	}

	if !resp.Ok {
		return "", stash.ErrUnableToPopulateContent
	}

	return resp.Hash, nil
}

// S3Source represents an S3 object source.
type S3Source struct {
	Path        string
	BucketName  string
	Region      string
	EndpointURL string
	AccessKey   string
	SecretKey   string
}

// StoreContentFromS3 caches content from S3.
func (c *Client) StoreContentFromS3(source S3Source, opts SourceOptions) (string, error) {
	ctx, cancel := context.WithTimeout(c.ctx, storeContentRequestTimeout)
	defer cancel()

	if opts.RoutingKey == "" {
		opts.RoutingKey = fmt.Sprintf("%s/%s/%s/%s",
			source.EndpointURL, source.Region, source.BucketName, source.Path)
	}

	client, _, err := c.getGRPCClient(&ClientRequest{
		rt:        ClientRequestTypeStorage,
		key:       opts.RoutingKey,
		hostIndex: 0,
	})
	if err != nil {
		return "", err
	}

	if opts.Lock {
		resp, err := client.StoreContentFromSourceWithLock(ctx, &proto.StoreContentFromSourceRequest{
			Source: &proto.CacheSource{
				Path:        source.Path,
				BucketName:  source.BucketName,
				Region:      source.Region,
				EndpointUrl: source.EndpointURL,
				AccessKey:   source.AccessKey,
				SecretKey:   source.SecretKey,
			},
		})
		if err != nil {
			return "", err
		}

		if resp.FailedToAcquireLock {
			return "", stash.ErrUnableToAcquireLock
		}

		if !resp.Ok {
			return "", stash.ErrUnableToPopulateContent
		}

		return resp.Hash, nil
	}

	resp, err := client.StoreContentFromSource(ctx, &proto.StoreContentFromSourceRequest{
		Source: &proto.CacheSource{
			Path:        source.Path,
			BucketName:  source.BucketName,
			Region:      source.Region,
			EndpointUrl: source.EndpointURL,
			AccessKey:   source.AccessKey,
			SecretKey:   source.SecretKey,
		},
	})
	if err != nil {
		return "", err
	}

	if !resp.Ok {
		return "", stash.ErrUnableToPopulateContent
	}

	return resp.Hash, nil
}
