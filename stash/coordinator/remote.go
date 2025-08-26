package coordinator

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"time"

	"github.com/codecflow/fabric/stash"
	"github.com/codecflow/fabric/stash/config"
	"github.com/codecflow/fabric/stash/metadata"
	"github.com/codecflow/fabric/stash/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
)

// Remote implements the Client interface via gRPC to a coordinator server.
// Used by slave servers and clients to communicate with the coordinator.
type Remote struct {
	coordinatorHost string
	client          proto.StashClient
	conn            *grpc.ClientConn
}

// NewRemote creates a new remote coordinator client.
func NewRemote(coordinatorHost string, token string, cfg *config.Global) (Client, error) {
	// Determine transport credentials
	transportCreds := grpc.WithTransportCredentials(insecure.NewCredentials())
	if isTLSEnabled(coordinatorHost) {
		h2creds := credentials.NewTLS(&tls.Config{NextProtos: []string{"h2"}})
		transportCreds = grpc.WithTransportCredentials(h2creds)
	}

	dialOpts := []grpc.DialOption{
		transportCreds,
		grpc.WithDefaultCallOptions(
			grpc.MaxCallRecvMsgSize(cfg.GRPCMessageSizeBytes),
			grpc.MaxCallSendMsgSize(cfg.GRPCMessageSizeBytes),
		),
	}

	// Add auth interceptor if token provided
	if token != "" {
		dialOpts = append(dialOpts, grpc.WithUnaryInterceptor(authInterceptor(token)))
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(cfg.GRPCDialTimeoutS)*time.Second)
	defer cancel()

	conn, err := grpc.DialContext(ctx, coordinatorHost, dialOpts...)
	if err != nil {
		return nil, err
	}

	return &Remote{
		coordinatorHost: coordinatorHost,
		client:          proto.NewStashClient(conn),
		conn:            conn,
	}, nil
}

// Close closes the gRPC connection.
func (r *Remote) Close() error {
	return r.conn.Close()
}

// AddHostToIndex adds a host to the locality index.
func (r *Remote) AddHostToIndex(ctx context.Context, locality string, host *stash.Host) error {
	resp, err := r.client.AddHostToIndex(ctx, &proto.AddHostToIndexRequest{
		Locality: locality,
		Host:     hostToProto(host),
	})
	if err != nil {
		return err
	}
	if !resp.Ok {
		return errors.New("failed to add host to index")
	}
	return nil
}

// SetHostKeepAlive updates the host's keepalive timestamp.
func (r *Remote) SetHostKeepAlive(ctx context.Context, locality string, host *stash.Host) error {
	resp, err := r.client.SetHostKeepAlive(ctx, &proto.SetHostKeepAliveRequest{
		Locality: locality,
		Host:     hostToProto(host),
	})
	if err != nil {
		return err
	}
	if !resp.Ok {
		return errors.New("failed to set host keep alive")
	}
	return nil
}

// GetAvailableHosts retrieves all active hosts in a locality.
func (r *Remote) GetAvailableHosts(ctx context.Context, locality string) ([]*stash.Host, error) {
	resp, err := r.client.GetAvailableHosts(ctx, &proto.GetAvailableHostsRequest{
		Locality: locality,
	})
	if err != nil {
		return nil, err
	}
	if !resp.Ok {
		return nil, errors.New("failed to get available hosts")
	}

	hosts := make([]*stash.Host, len(resp.Hosts))
	for i, protoHost := range resp.Hosts {
		hosts[i] = hostFromProto(protoHost)
	}
	return hosts, nil
}

// GetRegionConfig returns the server configuration for a locality.
func (r *Remote) GetRegionConfig(ctx context.Context, locality string) (*config.Server, error) {
	resp, err := r.client.GetRegionConfig(ctx, &proto.GetRegionConfigRequest{
		Locality: locality,
	})
	if err != nil {
		return nil, err
	}
	if !resp.Ok {
		return nil, errors.New("failed to get region config")
	}

	var serverConfig config.Server
	if err := json.Unmarshal([]byte(resp.ConfigJson), &serverConfig); err != nil {
		return nil, err
	}
	return &serverConfig, nil
}

// SetClientLock acquires a client lock for content.
func (r *Remote) SetClientLock(ctx context.Context, hash string, hostId string) error {
	resp, err := r.client.SetClientLock(ctx, &proto.SetClientLockRequest{
		Hash:   hash,
		HostId: hostId,
	})
	if err != nil {
		return err
	}
	if !resp.Ok {
		return errors.New("failed to set client lock")
	}
	return nil
}

// RemoveClientLock releases a client lock.
func (r *Remote) RemoveClientLock(ctx context.Context, hash string, hostId string) error {
	resp, err := r.client.RemoveClientLock(ctx, &proto.RemoveClientLockRequest{
		Hash:   hash,
		HostId: hostId,
	})
	if err != nil {
		return err
	}
	if !resp.Ok {
		return errors.New("failed to remove client lock")
	}
	return nil
}

// SetStoreFromContentLock acquires a lock for caching content from a source.
func (r *Remote) SetStoreFromContentLock(ctx context.Context, locality string, sourcePath string) error {
	resp, err := r.client.SetStoreFromContentLock(ctx, &proto.SetStoreFromContentLockRequest{
		Locality:   locality,
		SourcePath: sourcePath,
	})
	if err != nil {
		return err
	}
	if !resp.Ok {
		return errors.New("failed to set store from content lock")
	}
	return nil
}

// RemoveStoreFromContentLock releases a content source lock.
func (r *Remote) RemoveStoreFromContentLock(ctx context.Context, locality string, sourcePath string) error {
	resp, err := r.client.RemoveStoreFromContentLock(ctx, &proto.RemoveStoreFromContentLockRequest{
		Locality:   locality,
		SourcePath: sourcePath,
	})
	if err != nil {
		return err
	}
	if !resp.Ok {
		return errors.New("failed to remove store from content lock")
	}
	return nil
}

// RefreshStoreFromContentLock extends the TTL of a content source lock.
func (r *Remote) RefreshStoreFromContentLock(ctx context.Context, locality string, sourcePath string) error {
	resp, err := r.client.RefreshStoreFromContentLock(ctx, &proto.RefreshStoreFromContentLockRequest{
		Locality:   locality,
		SourcePath: sourcePath,
	})
	if err != nil {
		return err
	}
	if !resp.Ok {
		return errors.New("failed to refresh store from content lock")
	}
	return nil
}

// GetFsNode retrieves filesystem node metadata.
func (r *Remote) GetFsNode(ctx context.Context, id string) (*metadata.FsMetadata, error) {
	resp, err := r.client.GetFsNode(ctx, &proto.GetFsNodeRequest{Id: id})
	if err != nil {
		return nil, err
	}
	if !resp.Ok {
		return nil, errors.New("failed to get fs node")
	}
	return fsMetadataFromProto(resp.Metadata), nil
}

// SetFsNode stores filesystem node metadata.
func (r *Remote) SetFsNode(ctx context.Context, id string, node *metadata.FsMetadata) error {
	resp, err := r.client.SetFsNode(ctx, &proto.SetFsNodeRequest{
		Id:       id,
		Metadata: fsMetadataToProto(node),
	})
	if err != nil {
		return err
	}
	if !resp.Ok {
		return errors.New("failed to set fs node")
	}
	return nil
}

// RemoveFsNode deletes a filesystem node.
func (r *Remote) RemoveFsNode(ctx context.Context, id string) error {
	resp, err := r.client.RemoveFsNode(ctx, &proto.RemoveFsNodeRequest{Id: id})
	if err != nil {
		return err
	}
	if !resp.Ok {
		return errors.New("failed to remove fs node")
	}
	return nil
}

// GetFsNodeChildren retrieves all child nodes of a directory.
func (r *Remote) GetFsNodeChildren(ctx context.Context, id string) ([]*metadata.FsMetadata, error) {
	resp, err := r.client.GetFsNodeChildren(ctx, &proto.GetFsNodeChildrenRequest{Id: id})
	if err != nil {
		return nil, err
	}
	if !resp.Ok {
		return nil, errors.New("failed to get fs node children")
	}

	children := make([]*metadata.FsMetadata, len(resp.Children))
	for i, protoChild := range resp.Children {
		children[i] = fsMetadataFromProto(protoChild)
	}
	return children, nil
}

// AddFsNodeChild adds a child to a directory.
func (r *Remote) AddFsNodeChild(ctx context.Context, parentId, childId string) error {
	resp, err := r.client.AddFsNodeChild(ctx, &proto.AddFsNodeChildRequest{
		ParentId: parentId,
		ChildId:  childId,
	})
	if err != nil {
		return err
	}
	if !resp.Ok {
		return errors.New("failed to add fs node child")
	}
	return nil
}

// RemoveFsNodeChild removes a child from a directory.
func (r *Remote) RemoveFsNodeChild(ctx context.Context, parentId, childId string) error {
	resp, err := r.client.RemoveFsNodeChild(ctx, &proto.RemoveFsNodeChildRequest{
		ParentId: parentId,
		ChildId:  childId,
	})
	if err != nil {
		return err
	}
	if !resp.Ok {
		return errors.New("failed to remove fs node child")
	}
	return nil
}

// Helper functions

func isTLSEnabled(addr string) bool {
	return len(addr) > 5 && addr[:5] == "https"
}

func authInterceptor(token string) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		ctx = context.WithValue(ctx, "token", token)
		return invoker(ctx, method, req, reply, cc, opts...)
	}
}

func hostToProto(h *stash.Host) *proto.Host {
	return &proto.Host{
		Id: h.ID,
		Address: &proto.Address{
			Public:  h.Address.Public,
			Private: h.Address.Private,
		},
		Capacity: &proto.Capacity{
			Used:  h.Capacity.Used,
			Total: h.Capacity.Total,
		},
	}
}

func hostFromProto(p *proto.Host) *stash.Host {
	return &stash.Host{
		ID: p.Id,
		Address: stash.Address{
			Public:  p.Address.Public,
			Private: p.Address.Private,
		},
		Capacity: stash.Capacity{
			Used:  p.Capacity.Used,
			Total: p.Capacity.Total,
		},
	}
}

func fsMetadataToProto(m *metadata.FsMetadata) *proto.FsMetadata {
	return &proto.FsMetadata{
		Pid:       m.PID,
		Id:        m.ID,
		Name:      m.Name,
		Path:      m.Path,
		Hash:      m.Hash,
		Ino:       m.Ino,
		Size:      m.Size,
		Blocks:    m.Blocks,
		Atime:     m.Atime,
		Mtime:     m.Mtime,
		Ctime:     m.Ctime,
		Atimensec: m.Atimensec,
		Mtimensec: m.Mtimensec,
		Ctimensec: m.Ctimensec,
		Mode:      m.Mode,
		Nlink:     m.Nlink,
		Rdev:      m.Rdev,
		Blksize:   m.Blksize,
		Padding:   m.Padding,
		Uid:       m.Uid,
		Gid:       m.Gid,
		Gen:       m.Gen,
	}
}

func fsMetadataFromProto(p *proto.FsMetadata) *metadata.FsMetadata {
	return &metadata.FsMetadata{
		PID:       p.Pid,
		ID:        p.Id,
		Name:      p.Name,
		Path:      p.Path,
		Hash:      p.Hash,
		Ino:       p.Ino,
		Size:      p.Size,
		Blocks:    p.Blocks,
		Atime:     p.Atime,
		Mtime:     p.Mtime,
		Ctime:     p.Ctime,
		Atimensec: p.Atimensec,
		Mtimensec: p.Mtimensec,
		Ctimensec: p.Ctimensec,
		Mode:      p.Mode,
		Nlink:     p.Nlink,
		Rdev:      p.Rdev,
		Blksize:   p.Blksize,
		Padding:   p.Padding,
		Uid:       p.Uid,
		Gid:       p.Gid,
		Gen:       p.Gen,
	}
}
