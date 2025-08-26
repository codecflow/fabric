package server

import (
	"github.com/codecflow/fabric/stash"
	"github.com/codecflow/fabric/stash/metadata"
	"github.com/codecflow/fabric/stash/proto"
)

// hostToProto converts a stash.Host to proto.Host.
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

// hostFromProto converts a proto.Host to stash.Host.
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

// fsMetadataToProto converts metadata.FsMetadata to proto.FsMetadata.
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

// fsMetadataFromProto converts proto.FsMetadata to metadata.FsMetadata.
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
