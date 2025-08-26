package server

import (
	"context"
	"encoding/json"

	"github.com/codecflow/fabric/stash/coordinator"
	"github.com/codecflow/fabric/stash/proto"
)

// Server implements the gRPC Stash service for coordinator operations.
type Server struct {
	proto.UnimplementedStashServer
	coordinator coordinator.Client
}

// New creates a new coordinator gRPC server.
func New(coord coordinator.Client) *Server {
	return &Server{
		coordinator: coord,
	}
}

// AddHostToIndex adds a host to the locality index.
func (s *Server) AddHostToIndex(ctx context.Context, req *proto.AddHostToIndexRequest) (*proto.AddHostToIndexResponse, error) {
	host := hostFromProto(req.Host)
	err := s.coordinator.AddHostToIndex(ctx, req.Locality, host)
	return &proto.AddHostToIndexResponse{Ok: err == nil}, err
}

// SetHostKeepAlive updates the host's keepalive timestamp.
func (s *Server) SetHostKeepAlive(ctx context.Context, req *proto.SetHostKeepAliveRequest) (*proto.SetHostKeepAliveResponse, error) {
	host := hostFromProto(req.Host)
	err := s.coordinator.SetHostKeepAlive(ctx, req.Locality, host)
	return &proto.SetHostKeepAliveResponse{Ok: err == nil}, err
}

// GetAvailableHosts retrieves all active hosts in a locality.
func (s *Server) GetAvailableHosts(ctx context.Context, req *proto.GetAvailableHostsRequest) (*proto.GetAvailableHostsResponse, error) {
	hosts, err := s.coordinator.GetAvailableHosts(ctx, req.Locality)
	if err != nil {
		return &proto.GetAvailableHostsResponse{Ok: false}, err
	}

	protoHosts := make([]*proto.Host, len(hosts))
	for i, host := range hosts {
		protoHosts[i] = hostToProto(host)
	}

	return &proto.GetAvailableHostsResponse{
		Hosts: protoHosts,
		Ok:    true,
	}, nil
}

// GetRegionConfig returns the server configuration for a locality.
func (s *Server) GetRegionConfig(ctx context.Context, req *proto.GetRegionConfigRequest) (*proto.GetRegionConfigResponse, error) {
	config, err := s.coordinator.GetRegionConfig(ctx, req.Locality)
	if err != nil {
		return &proto.GetRegionConfigResponse{Ok: false}, err
	}

	configJSON, err := json.Marshal(config)
	if err != nil {
		return &proto.GetRegionConfigResponse{Ok: false}, err
	}

	return &proto.GetRegionConfigResponse{
		ConfigJson: string(configJSON),
		Ok:         true,
	}, nil
}

// SetClientLock acquires a client lock for content.
func (s *Server) SetClientLock(ctx context.Context, req *proto.SetClientLockRequest) (*proto.SetClientLockResponse, error) {
	err := s.coordinator.SetClientLock(ctx, req.Hash, req.HostId)
	return &proto.SetClientLockResponse{Ok: err == nil}, err
}

// RemoveClientLock releases a client lock.
func (s *Server) RemoveClientLock(ctx context.Context, req *proto.RemoveClientLockRequest) (*proto.RemoveClientLockResponse, error) {
	err := s.coordinator.RemoveClientLock(ctx, req.Hash, req.HostId)
	return &proto.RemoveClientLockResponse{Ok: err == nil}, err
}

// SetStoreFromContentLock acquires a lock for caching content from a source.
func (s *Server) SetStoreFromContentLock(ctx context.Context, req *proto.SetStoreFromContentLockRequest) (*proto.SetStoreFromContentLockResponse, error) {
	err := s.coordinator.SetStoreFromContentLock(ctx, req.Locality, req.SourcePath)
	return &proto.SetStoreFromContentLockResponse{Ok: err == nil}, err
}

// RemoveStoreFromContentLock releases a content source lock.
func (s *Server) RemoveStoreFromContentLock(ctx context.Context, req *proto.RemoveStoreFromContentLockRequest) (*proto.RemoveStoreFromContentLockResponse, error) {
	err := s.coordinator.RemoveStoreFromContentLock(ctx, req.Locality, req.SourcePath)
	return &proto.RemoveStoreFromContentLockResponse{Ok: err == nil}, err
}

// RefreshStoreFromContentLock extends the TTL of a content source lock.
func (s *Server) RefreshStoreFromContentLock(ctx context.Context, req *proto.RefreshStoreFromContentLockRequest) (*proto.RefreshStoreFromContentLockResponse, error) {
	err := s.coordinator.RefreshStoreFromContentLock(ctx, req.Locality, req.SourcePath)
	return &proto.RefreshStoreFromContentLockResponse{Ok: err == nil}, err
}

// GetFsNode retrieves filesystem node metadata.
func (s *Server) GetFsNode(ctx context.Context, req *proto.GetFsNodeRequest) (*proto.GetFsNodeResponse, error) {
	node, err := s.coordinator.GetFsNode(ctx, req.Id)
	if err != nil {
		return &proto.GetFsNodeResponse{Ok: false}, err
	}

	return &proto.GetFsNodeResponse{
		Metadata: fsMetadataToProto(node),
		Ok:       true,
	}, nil
}

// SetFsNode stores filesystem node metadata.
func (s *Server) SetFsNode(ctx context.Context, req *proto.SetFsNodeRequest) (*proto.SetFsNodeResponse, error) {
	node := fsMetadataFromProto(req.Metadata)
	err := s.coordinator.SetFsNode(ctx, req.Id, node)
	return &proto.SetFsNodeResponse{Ok: err == nil}, err
}

// RemoveFsNode deletes a filesystem node.
func (s *Server) RemoveFsNode(ctx context.Context, req *proto.RemoveFsNodeRequest) (*proto.RemoveFsNodeResponse, error) {
	err := s.coordinator.RemoveFsNode(ctx, req.Id)
	return &proto.RemoveFsNodeResponse{Ok: err == nil}, err
}

// GetFsNodeChildren retrieves all child nodes of a directory.
func (s *Server) GetFsNodeChildren(ctx context.Context, req *proto.GetFsNodeChildrenRequest) (*proto.GetFsNodeChildrenResponse, error) {
	children, err := s.coordinator.GetFsNodeChildren(ctx, req.Id)
	if err != nil {
		return &proto.GetFsNodeChildrenResponse{Ok: false}, err
	}

	protoChildren := make([]*proto.FsMetadata, len(children))
	for i, child := range children {
		protoChildren[i] = fsMetadataToProto(child)
	}

	return &proto.GetFsNodeChildrenResponse{
		Children: protoChildren,
		Ok:       true,
	}, nil
}

// AddFsNodeChild adds a child to a directory.
func (s *Server) AddFsNodeChild(ctx context.Context, req *proto.AddFsNodeChildRequest) (*proto.AddFsNodeChildResponse, error) {
	err := s.coordinator.AddFsNodeChild(ctx, req.ParentId, req.ChildId)
	return &proto.AddFsNodeChildResponse{Ok: err == nil}, err
}

// RemoveFsNodeChild removes a child from a directory.
func (s *Server) RemoveFsNodeChild(ctx context.Context, req *proto.RemoveFsNodeChildRequest) (*proto.RemoveFsNodeChildResponse, error) {
	err := s.coordinator.RemoveFsNodeChild(ctx, req.ParentId, req.ChildId)
	return &proto.RemoveFsNodeChildResponse{Ok: err == nil}, err
}
