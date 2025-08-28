package client

import (
	"context"
	"crypto/tls"
	"time"

	"github.com/codecflow/fabric/stash"
	"github.com/codecflow/fabric/stash/network"
	"github.com/codecflow/fabric/stash/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
)

const (
	stashVersion = "0.1.0"
)

// addHost adds a new host and establishes a gRPC connection.
func (c *Client) addHost(host *stash.Host) error {
	addr := host.Address.Public
	if host.Address.Private != "" {
		addr = host.Address.Private
	}

	transportCredentials := grpc.WithTransportCredentials(insecure.NewCredentials())
	if network.IsTLSEnabled(addr) {
		h2creds := credentials.NewTLS(&tls.Config{NextProtos: []string{"h2"}})
		transportCredentials = grpc.WithTransportCredentials(h2creds)
	}

	// Optimized client dial options matching server configuration
	initialWindowSize := c.globalConfig.GRPCInitialWindowSize
	if initialWindowSize == 0 {
		initialWindowSize = 4 * 1024 * 1024
	}

	initialConnWindowSize := c.globalConfig.GRPCInitialConnWindowSize
	if initialConnWindowSize == 0 {
		initialConnWindowSize = 32 * 1024 * 1024
	}

	writeBufferSize := c.globalConfig.GRPCWriteBufferSize
	if writeBufferSize == 0 {
		writeBufferSize = 256 * 1024
	}

	readBufferSize := c.globalConfig.GRPCReadBufferSize
	if readBufferSize == 0 {
		readBufferSize = 256 * 1024
	}

	var dialOpts = []grpc.DialOption{
		transportCredentials,
		grpc.WithContextDialer(network.DialWithTimeout),
		grpc.WithDefaultCallOptions(
			grpc.MaxCallRecvMsgSize(c.globalConfig.GRPCMessageSizeBytes),
			grpc.MaxCallSendMsgSize(c.globalConfig.GRPCMessageSizeBytes),
		),
		grpc.WithInitialWindowSize(int32(initialWindowSize)),
		grpc.WithInitialConnWindowSize(int32(initialConnWindowSize)),
		grpc.WithWriteBufferSize(writeBufferSize),
		grpc.WithReadBufferSize(readBufferSize),
	}

	if c.clientConfig.Token != "" {
		dialOpts = append(dialOpts, grpc.WithUnaryInterceptor(grpcAuthInterceptor(c.clientConfig.Token)))
	}

	conn, err := grpc.Dial(addr, dialOpts...)
	if err != nil {
		return err
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	c.grpcClients[host.ID] = proto.NewStashClient(conn)
	c.hasher.Add(host)

	go c.monitorHost(host)
	return nil
}

// monitorHost monitors the health of a host.
func (c *Client) monitorHost(host *stash.Host) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			err := func() error {
				client, exists := c.grpcClients[host.ID]
				if !exists {
					return stash.ErrHostNotFound
				}

				resp, err := client.GetState(c.ctx, &proto.GetStateRequest{})
				if err != nil {
					return stash.ErrInvalidHostVersion
				}

				if resp.GetVersion() != stashVersion {
					return stash.ErrInvalidHostVersion
				}

				return nil
			}()

			// We were unable to reach the host for some reason, remove it as a possible client
			if err != nil {
				c.mu.Lock()
				defer c.mu.Unlock()

				c.hostMap.Remove(host)
				c.hasher.Remove(host)

				delete(c.grpcClients, host.Address.Public)

				return
			}

		case <-c.ctx.Done():
			return
		}
	}
}

// grpcAuthInterceptor adds authentication token to gRPC requests.
func grpcAuthInterceptor(token string) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		newCtx := metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+token)
		return invoker(newCtx, method, req, reply, cc, opts...)
	}
}
