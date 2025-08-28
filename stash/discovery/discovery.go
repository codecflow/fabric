package discovery

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/codecflow/fabric/stash"
	"github.com/codecflow/fabric/stash/config"
	"github.com/codecflow/fabric/stash/coordinator"
	"github.com/codecflow/fabric/stash/network"
	"github.com/codecflow/fabric/stash/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
)

const (
	stashVersion = "0.1.0"
)

// Client discovers and monitors available cache hosts.
type Client struct {
	cfg         *config.Global
	hostMap     *stash.HostMap
	coordinator coordinator.Client
	locality    string
	mu          sync.Mutex
	logger      *slog.Logger
}

// NewClient creates a new discovery client.
func NewClient(cfg *config.Global, hostMap *stash.HostMap, coord coordinator.Client, locality string, logger *slog.Logger) *Client {
	if logger == nil {
		logger = slog.Default()
	}

	return &Client{
		cfg:         cfg,
		hostMap:     hostMap,
		coordinator: coord,
		locality:    locality,
		logger:      logger,
	}
}

// Start begins the host discovery process.
func (d *Client) Start(ctx context.Context) error {
	// Initial discovery
	hosts, err := d.discoverHosts(ctx)
	if err == nil {
		d.updateHostMap(hosts)
	}

	ticker := time.NewTicker(time.Duration(d.cfg.DiscoveryIntervalS) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			hosts, err := d.discoverHosts(ctx)
			if err != nil {
				d.logger.Warn("host discovery failed", "error", err)
				continue
			}

			d.updateHostMap(hosts)
		case <-ctx.Done():
			return nil
		}
	}
}

// updateHostMap adds newly discovered hosts to the host map.
func (d *Client) updateHostMap(newHosts []*stash.Host) {
	for _, h := range newHosts {
		d.hostMap.Set(h)
	}
}

// discoverHosts fetches available hosts from the coordinator and validates them.
func (d *Client) discoverHosts(ctx context.Context) ([]*stash.Host, error) {
	hosts, err := d.coordinator.GetAvailableHosts(ctx, d.locality)
	if err != nil {
		return nil, err
	}

	var wg sync.WaitGroup
	filteredHosts := []*stash.Host{}
	mu := sync.Mutex{}

	for _, host := range hosts {
		// Don't try to get the state on peers we're already aware of
		if d.hostMap.Get(host.ID) != nil {
			continue
		}

		wg.Add(1)
		go func(h *stash.Host) {
			defer wg.Done()

			hostState, err := d.GetHostState(ctx, h)
			if err != nil {
				d.logger.Debug("host state check failed", "host", h.ID, "error", err)
				return
			}

			mu.Lock()
			filteredHosts = append(filteredHosts, hostState)
			mu.Unlock()

			d.logger.Debug("added host to map", "host", hostState.ID, "address", hostState.Address.Private)
		}(host)
	}

	wg.Wait()
	return filteredHosts, nil
}

// GetHostState attempts to connect to the gRPC service and verifies its availability.
func (d *Client) GetHostState(ctx context.Context, host *stash.Host) (*stash.Host, error) {
	addr := host.Address.Public
	if host.Address.Private != "" {
		addr = host.Address.Private
	}

	transportCredentials := grpc.WithTransportCredentials(insecure.NewCredentials())
	if network.IsTLSEnabled(addr) {
		h2creds := credentials.NewTLS(&tls.Config{NextProtos: []string{"h2"}})
		transportCredentials = grpc.WithTransportCredentials(h2creds)
	}

	dialOpts := []grpc.DialOption{
		transportCredentials,
		grpc.WithDefaultCallOptions(
			grpc.MaxCallRecvMsgSize(d.cfg.GRPCMessageSizeBytes),
			grpc.MaxCallSendMsgSize(d.cfg.GRPCMessageSizeBytes),
		),
	}

	dialCtx, cancel := context.WithTimeout(ctx, time.Duration(d.cfg.GRPCDialTimeoutS)*time.Second)
	defer cancel()

	conn, err := grpc.DialContext(dialCtx, addr, dialOpts...)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	client := proto.NewStashClient(conn)

	resp, err := client.GetState(dialCtx, &proto.GetStateRequest{})
	if err != nil {
		return nil, err
	}

	// Update host with latest state
	host.RTT = 0 // Could measure actual RTT here
	host.Capacity.Used = int64(resp.GetCapacityUsagePct())

	if resp.GetVersion() != stashVersion {
		return nil, fmt.Errorf("version mismatch: %s != %s", resp.GetVersion(), stashVersion)
	}

	return host, nil
}
