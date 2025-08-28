package client

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/codecflow/fabric/stash"
	"github.com/codecflow/fabric/stash/config"
	"github.com/codecflow/fabric/stash/coordinator"
	"github.com/codecflow/fabric/stash/discovery"
	"github.com/codecflow/fabric/stash/proto"
	"github.com/codecflow/fabric/stash/rendezvous"
)

const (
	getContentRequestTimeout        = 60 * time.Second
	getContentStreamRequestTimeout  = 600 * time.Second
	storeContentRequestTimeout      = 300 * time.Second
	localClientCacheCleanupInterval = 5 * time.Second
	localClientCacheTTL             = 300 * time.Second
)

// Client is a cache client that can retrieve and store content.
type Client struct {
	ctx                   context.Context
	locality              string
	clientConfig          *config.Client
	globalConfig          *config.Global
	grpcClients           map[string]proto.StashClient
	hostMap               *stash.HostMap
	mu                    sync.RWMutex
	discoveryClient       *discovery.Client
	coordinator           coordinator.Client
	localHostCache        map[string]*localClientCache
	hasher                *rendezvous.Hash[*stash.Host]
	minRetryLengthBytes   int64
	maxGetContentAttempts int64
	logger                *slog.Logger
}

type localClientCache struct {
	host      *stash.Host
	timestamp time.Time
}

// NewClient creates a new cache client.
func NewClient(ctx context.Context, cfg *config.BlobCacheConfig, logger *slog.Logger) (*Client, error) {
	if logger == nil {
		logger = slog.Default()
	}

	coord, err := coordinator.NewRemote(cfg.Global.CoordinatorHost, cfg.Client.Token, &cfg.Global)
	if err != nil {
		return nil, err
	}

	locality := cfg.Global.GetLocality()

	c := &Client{
		ctx:                   ctx,
		locality:              locality,
		clientConfig:          &cfg.Client,
		globalConfig:          &cfg.Global,
		grpcClients:           make(map[string]proto.StashClient),
		localHostCache:        make(map[string]*localClientCache),
		mu:                    sync.RWMutex{},
		coordinator:           coord,
		hasher:                rendezvous.New[*stash.Host](),
		minRetryLengthBytes:   cfg.Client.MinRetryLengthBytes,
		maxGetContentAttempts: max(int64(cfg.Client.MaxGetContentAttempts), 1),
		logger:                logger,
	}

	c.hostMap = stash.NewHostMap(c.addHost)
	c.discoveryClient = discovery.NewClient(&cfg.Global, c.hostMap, coord, locality, logger)

	// Start searching for nearby cache hosts
	go c.discoveryClient.Start(c.ctx)

	// Monitor and cleanup local client cache
	go c.manageLocalClientCache(localClientCacheCleanupInterval, localClientCacheTTL)

	return c, nil
}

// Cleanup shuts down the client gracefully.
func (c *Client) Cleanup() error {
	// Future: unmount blobfs if enabled
	return nil
}

// GetNearbyHosts returns all available hosts in the locality.
func (c *Client) GetNearbyHosts() ([]*stash.Host, error) {
	hosts, err := c.coordinator.GetAvailableHosts(c.ctx, c.locality)
	if err != nil {
		return nil, err
	}
	return hosts, nil
}

// HostsAvailable returns true if any hosts are available.
func (c *Client) HostsAvailable() bool {
	return c.hostMap.Len() > 0
}

// WaitForHosts blocks until at least one host is available or timeout.
func (c *Client) WaitForHosts(timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	if c.HostsAvailable() {
		c.logger.Info("cache available")
		return nil
	}

	c.logger.Info("waiting for hosts to be available")
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			if c.HostsAvailable() {
				c.logger.Info("cache available")
				return nil
			}
			time.Sleep(1 * time.Second)
		}
	}
}

// GetState checks the state of a host.
func (c *Client) GetState() error {
	ctx, cancel := context.WithTimeout(c.ctx, getContentRequestTimeout)
	defer cancel()

	client, _, err := c.getGRPCClient(&ClientRequest{rt: ClientRequestTypeRetrieval, hostIndex: 0})
	if err != nil {
		return err
	}

	_, err = client.GetState(ctx, &proto.GetStateRequest{})
	if err != nil {
		return err
	}

	return nil
}

func max(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
