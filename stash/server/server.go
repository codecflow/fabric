package server

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/codecflow/fabric/stash"
	"github.com/codecflow/fabric/stash/cache"
	"github.com/codecflow/fabric/stash/config"
	"github.com/codecflow/fabric/stash/coordinator"
	"github.com/codecflow/fabric/stash/proto"
	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	getContentStreamChunkSize = 4 * 1024 * 1024 // 4MB
	hostKeepAliveIntervalS    = 30
	stashVersion              = "0.1.0"
)

// Server is the cache service server.
type Server struct {
	proto.UnimplementedStashServer
	ctx         context.Context
	hostID      string
	locality    string
	publicAddr  string
	privateAddr string
	storage     *cache.Storage
	cfg         *config.BlobCacheConfig
	coordinator coordinator.Client
	logger      *slog.Logger
}

// NewServer creates a new cache service server.
func NewServer(ctx context.Context, cfg *config.BlobCacheConfig, locality string, logger *slog.Logger) (*Server, error) {
	if logger == nil {
		logger = slog.Default()
	}

	// Create coordinator client
	var coord coordinator.Client
	var err error

	switch cfg.Server.Mode {
	case config.ServerModeCoordinator:
		// For coordinator mode, we need metadata - will implement later
		return nil, fmt.Errorf("coordinator mode requires metadata initialization")
	default:
		coord, err = coordinator.NewRemote(cfg.Global.CoordinatorHost, cfg.Client.Token, &cfg.Global)
		if err != nil {
			return nil, fmt.Errorf("failed to create coordinator client: %w", err)
		}

		// Get region-specific config if available
		regionConfig, err := coord.GetRegionConfig(ctx, locality)
		if err != nil {
			logger.Info("no region-specific config found, using default", "locality", locality)
		} else {
			cfg.Server = *regionConfig
		}
	}

	// Create disk cache directory
	if err := os.MkdirAll(cfg.Server.DiskCacheDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create cache directory: %w", err)
	}

	// Get or create host ID
	hostID := getHostID(cfg.Server.DiskCacheDir)
	logger.Info("server started", "host_id", hostID, "mode", cfg.Server.Mode)

	// Get network addresses
	publicAddr := getPublicAddr(cfg.Global.ServerPort)
	privateAddr := getPrivateAddr(cfg.Global.ServerPort)

	currentHost := &stash.Host{
		ID: hostID,
		Address: stash.Address{
			Public:  publicAddr,
			Private: privateAddr,
		},
	}

	// Create cache storage
	storage, err := cache.NewStorage(ctx, currentHost, locality, cfg, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create storage: %w", err)
	}

	s := &Server{
		ctx:         ctx,
		hostID:      hostID,
		locality:    locality,
		publicAddr:  publicAddr,
		privateAddr: privateAddr,
		storage:     storage,
		cfg:         cfg,
		coordinator: coord,
		logger:      logger,
	}

	// Start host keepalive
	go s.hostKeepAlive()

	return s, nil
}

// Start starts the gRPC server.
func (s *Server) Start(port uint) error {
	addr := fmt.Sprintf(":%d", port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to listen: %w", err)
	}

	// Configure gRPC server options
	maxMessageSize := s.cfg.Global.GRPCMessageSizeBytes
	if maxMessageSize == 0 {
		maxMessageSize = 100 * 1024 * 1024 // 100MB default
	}

	opts := []grpc.ServerOption{
		grpc.MaxRecvMsgSize(maxMessageSize),
		grpc.MaxSendMsgSize(maxMessageSize),
	}

	if s.cfg.Global.GRPCInitialWindowSize > 0 {
		opts = append(opts, grpc.InitialWindowSize(int32(s.cfg.Global.GRPCInitialWindowSize)))
	}
	if s.cfg.Global.GRPCInitialConnWindowSize > 0 {
		opts = append(opts, grpc.InitialConnWindowSize(int32(s.cfg.Global.GRPCInitialConnWindowSize)))
	}
	if s.cfg.Global.GRPCWriteBufferSize > 0 {
		opts = append(opts, grpc.WriteBufferSize(s.cfg.Global.GRPCWriteBufferSize))
	}
	if s.cfg.Global.GRPCReadBufferSize > 0 {
		opts = append(opts, grpc.ReadBufferSize(s.cfg.Global.GRPCReadBufferSize))
	}
	if s.cfg.Global.GRPCMaxConcurrentStreams > 0 {
		opts = append(opts, grpc.MaxConcurrentStreams(uint32(s.cfg.Global.GRPCMaxConcurrentStreams)))
	}
	if s.cfg.Global.GRPCNumStreamWorkers > 0 {
		opts = append(opts, grpc.NumStreamWorkers(uint32(s.cfg.Global.GRPCNumStreamWorkers)))
	}

	grpcServer := grpc.NewServer(opts...)
	proto.RegisterStashServer(grpcServer, s)

	s.logger.Info("server listening", "address", addr)

	go grpcServer.Serve(listener)

	// Wait for termination signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	sig := <-sigChan
	s.logger.Info("shutting down", "signal", sig)

	grpcServer.GracefulStop()
	s.storage.Cleanup()

	return nil
}

// GetContent retrieves content from the cache.
func (s *Server) GetContent(ctx context.Context, req *proto.GetContentRequest) (*proto.GetContentResponse, error) {
	dst := make([]byte, req.Length)
	n, err := s.storage.Get(req.Hash, req.Offset, req.Length, dst)
	if err != nil {
		s.logger.Debug("get content failed", "hash", req.Hash, "error", err)
		return &proto.GetContentResponse{Ok: false}, nil
	}

	s.logger.Debug("get content", "hash", req.Hash, "bytes", n)
	return &proto.GetContentResponse{Ok: true, Content: dst[:n]}, nil
}

// HasContent checks if content exists in the cache.
func (s *Server) HasContent(ctx context.Context, req *proto.HasContentRequest) (*proto.HasContentResponse, error) {
	exists := s.storage.Exists(req.Hash)
	return &proto.HasContentResponse{Ok: true, Exists: exists}, nil
}

// GetContentStream streams content from the cache.
func (s *Server) GetContentStream(req *proto.GetContentRequest, stream proto.Stash_GetContentStreamServer) error {
	offset := req.Offset
	remaining := req.Length

	s.logger.Info("get content stream", "hash", req.Hash, "offset", offset, "length", req.Length)

	dst := make([]byte, getContentStreamChunkSize)
	for remaining > 0 {
		chunkSize := getContentStreamChunkSize
		if remaining < int64(chunkSize) {
			chunkSize = int(remaining)
		}

		n, err := s.storage.Get(req.Hash, offset, int64(chunkSize), dst)
		if err != nil {
			s.logger.Debug("stream read failed", "hash", req.Hash, "error", err)
			return status.Errorf(codes.NotFound, "content not found: %v", err)
		}
		if n == 0 {
			break
		}

		if err := stream.Send(&proto.GetContentResponse{
			Ok:      true,
			Content: dst[:n],
		}); err != nil {
			return status.Errorf(codes.Internal, "failed to send chunk: %v", err)
		}

		if n < int64(chunkSize) {
			break
		}

		offset += n
		remaining -= n
	}

	return nil
}

// StoreContent stores content in the cache.
func (s *Server) StoreContent(stream proto.Stash_StoreContentServer) error {
	var buffer bytes.Buffer

	s.logger.Info("store content")

	for {
		req, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return status.Errorf(codes.Unknown, "receive error: %v", err)
		}

		if _, err := buffer.Write(req.Content); err != nil {
			return status.Errorf(codes.Internal, "buffer write failed: %v", err)
		}
	}

	content := buffer.Bytes()
	hashBytes := sha256.Sum256(content)
	hash := hex.EncodeToString(hashBytes[:])

	if err := s.storage.Add(stream.Context(), hash, content); err != nil {
		s.logger.Error("store failed", "hash", hash, "error", err)
		return status.Errorf(codes.Internal, "store failed: %v", err)
	}

	s.logger.Info("store content", "hash", hash, "size", len(content))
	return stream.SendAndClose(&proto.StoreContentResponse{Ok: true, Hash: hash})
}

// StoreContentFromSource stores content from a source path.
func (s *Server) StoreContentFromSource(ctx context.Context, req *proto.StoreContentFromSourceRequest) (*proto.StoreContentFromSourceResponse, error) {
	path := filepath.Join("/", req.Source.Path)
	s.logger.Info("store from source", "path", path)

	if _, err := os.Stat(path); os.IsNotExist(err) {
		return &proto.StoreContentFromSourceResponse{Ok: false, ErrorMsg: "source not found"}, nil
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return &proto.StoreContentFromSourceResponse{Ok: false, ErrorMsg: err.Error()}, nil
	}

	hashBytes := sha256.Sum256(content)
	hash := hex.EncodeToString(hashBytes[:])

	if err := s.storage.Add(ctx, hash, content); err != nil {
		return &proto.StoreContentFromSourceResponse{Ok: false, ErrorMsg: err.Error()}, nil
	}

	s.logger.Info("stored from source", "hash", hash, "size", len(content))
	return &proto.StoreContentFromSourceResponse{Ok: true, Hash: hash}, nil
}

// StoreContentFromSourceWithLock stores content with distributed locking.
func (s *Server) StoreContentFromSourceWithLock(ctx context.Context, req *proto.StoreContentFromSourceRequest) (*proto.StoreContentFromSourceWithLockResponse, error) {
	sourcePath := req.Source.Path

	if err := s.coordinator.SetStoreFromContentLock(ctx, s.locality, sourcePath); err != nil {
		return &proto.StoreContentFromSourceWithLockResponse{
			Ok:                  false,
			FailedToAcquireLock: true,
			ErrorMsg:            err.Error(),
		}, nil
	}

	storeCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Refresh lock periodically
	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-storeCtx.Done():
				return
			case <-ticker.C:
				s.coordinator.RefreshStoreFromContentLock(ctx, s.locality, sourcePath)
			}
		}
	}()

	resp, err := s.StoreContentFromSource(storeCtx, req)
	if err != nil {
		return &proto.StoreContentFromSourceWithLockResponse{Ok: false, ErrorMsg: err.Error()}, nil
	}

	if err := s.coordinator.RemoveStoreFromContentLock(ctx, s.locality, sourcePath); err != nil {
		s.logger.Error("failed to remove lock", "error", err)
	}

	return &proto.StoreContentFromSourceWithLockResponse{Ok: true, Hash: resp.Hash}, nil
}

// GetState returns server state information.
func (s *Server) GetState(ctx context.Context, req *proto.GetStateRequest) (*proto.GetStateResponse, error) {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	usagePct := float32(0)
	if s.storage != nil {
		memUsageMB := float64(memStats.Alloc) / (1024 * 1024)
		usagePct = float32(memUsageMB / float64(s.cfg.Server.MaxCachePct))
	}

	return &proto.GetStateResponse{
		Ok:               true,
		Version:          stashVersion,
		CapacityUsagePct: usagePct,
		PrivateIpAddr:    strings.Split(s.privateAddr, ":")[0],
	}, nil
}

// hostKeepAlive maintains host registration with coordinator.
func (s *Server) hostKeepAlive() {
	host := &stash.Host{
		ID: s.hostID,
		Address: stash.Address{
			Public:  s.publicAddr,
			Private: s.privateAddr,
		},
	}

	// Initial registration
	if err := s.coordinator.AddHostToIndex(s.ctx, s.locality, host); err != nil {
		s.logger.Warn("failed to add host to index", "error", err)
	}
	if err := s.coordinator.SetHostKeepAlive(s.ctx, s.locality, host); err != nil {
		s.logger.Warn("failed to set keepalive", "error", err)
	}

	ticker := time.NewTicker(hostKeepAliveIntervalS * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			diskUsage, _, usagePct, _ := s.storage.GetDiskCacheMetrics()

			// Update capacity
			host.Capacity.Used = diskUsage
			host.Capacity.Total = int64(usagePct)

			s.coordinator.AddHostToIndex(s.ctx, s.locality, host)
			s.coordinator.SetHostKeepAlive(s.ctx, s.locality, host)

			s.logger.Debug("keepalive", "host_id", s.hostID, "disk_mb", diskUsage)
		}
	}
}

// Helper functions

func getHostID(diskCacheDir string) string {
	filePath := filepath.Join(diskCacheDir, "HOST_ID")

	if content, err := os.ReadFile(filePath); err == nil {
		return strings.TrimSpace(string(content))
	}

	hostID := fmt.Sprintf("stash-%s", uuid.New().String()[:6])
	os.WriteFile(filePath, []byte(hostID), 0644)
	return hostID
}

func getPublicAddr(port uint) string {
	// Try to get public IP, fallback to localhost
	// In production, this should use actual public IP discovery
	return fmt.Sprintf("0.0.0.0:%d", port)
}

func getPrivateAddr(port uint) string {
	// Try to get private IP, fallback to localhost
	// In production, this should use actual private IP discovery
	return fmt.Sprintf("127.0.0.1:%d", port)
}
