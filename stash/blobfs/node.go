package blobfs

import (
	"context"
	"fmt"
	"path"
	"strings"
	"syscall"
	"time"

	"github.com/codecflow/fabric/stash"
	"github.com/codecflow/fabric/stash/client"
	"github.com/codecflow/fabric/stash/metadata"
	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
)

// BlobFsNode represents a node in the BlobFS filesystem.
type BlobFsNode struct {
	Path     string
	ID       string
	PID      string
	Name     string
	Target   string
	Hash     string
	Attr     fuse.Attr
	Prefetch *bool
}

// Node is a FUSE filesystem node.
type Node struct {
	fs.Inode
	filesystem *BlobFS
	bfsNode    *BlobFsNode
	attr       fuse.Attr
}

func (n *Node) log(format string, v ...interface{}) {
	if n.filesystem.verbose {
		n.filesystem.logger.Info(fmt.Sprintf("(%s) %s", n.bfsNode.Path, format), v...)
	}
}

// OnAdd is called when the node is added to the tree.
func (n *Node) OnAdd(ctx context.Context) {
	n.log("OnAdd called")
}

// Getattr returns the attributes of this node.
func (n *Node) Getattr(ctx context.Context, fh fs.FileHandle, out *fuse.AttrOut) syscall.Errno {
	n.log("Getattr called")

	node := n.bfsNode

	out.Ino = node.Attr.Ino
	out.Size = node.Attr.Size
	out.Blocks = node.Attr.Blocks
	out.Atime = node.Attr.Atime
	out.Mtime = node.Attr.Mtime
	out.Ctime = node.Attr.Ctime
	out.Mode = node.Attr.Mode
	out.Nlink = node.Attr.Nlink
	out.Owner = node.Attr.Owner
	out.Atimensec = node.Attr.Atimensec
	out.Mtimensec = node.Attr.Mtimensec
	out.Ctimensec = node.Attr.Ctimensec

	return fs.OK
}

// metaToAttr converts metadata to FUSE attributes.
func metaToAttr(meta *metadata.FsMetadata) fuse.Attr {
	return fuse.Attr{
		Ino:       meta.Ino,
		Size:      meta.Size,
		Blocks:    meta.Blocks,
		Atime:     meta.Atime,
		Mtime:     meta.Mtime,
		Ctime:     meta.Ctime,
		Atimensec: meta.Atimensec,
		Mtimensec: meta.Mtimensec,
		Ctimensec: meta.Ctimensec,
		Mode:      meta.Mode,
		Nlink:     meta.Nlink,
		Owner: fuse.Owner{
			Uid: meta.Uid,
			Gid: meta.Gid,
		},
		Rdev:    meta.Rdev,
		Blksize: meta.Blksize,
		Padding: meta.Padding,
	}
}

// inodeFromFsId creates an inode from a filesystem ID.
func (n *Node) inodeFromFsId(ctx context.Context, fsId string) (*fs.Inode, *fuse.Attr, error) {
	meta, err := n.filesystem.coordinator.GetFsNode(ctx, fsId)
	if err != nil {
		return nil, nil, syscall.ENOENT
	}

	attr := metaToAttr(meta)

	node := n.NewInode(ctx,
		&Node{
			filesystem: n.filesystem,
			bfsNode: &BlobFsNode{
				Path:     meta.Path,
				ID:       meta.ID,
				PID:      meta.PID,
				Name:     meta.Name,
				Hash:     meta.Hash,
				Attr:     attr,
				Target:   "",
				Prefetch: nil,
			},
			attr: attr,
		},
		fs.StableAttr{Mode: meta.Mode, Ino: meta.Ino, Gen: meta.Gen},
	)

	return node, &attr, nil
}

// Lookup looks up a child node by name.
func (n *Node) Lookup(ctx context.Context, name string, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
	fullPath := path.Join(n.bfsNode.Path, name)
	n.log("Lookup called with path: %s", fullPath)

	// Special handling for '%' character - force caching
	if strings.Contains(fullPath, "%") {
		sourcePath := strings.ReplaceAll(fullPath, "%", "/")
		n.log("Storing content from source with path: %s", sourcePath)

		_, err := n.filesystem.client.StoreContentFromFUSE(client.FUSESource{Path: sourcePath}, client.SourceOptions{
			RoutingKey: sourcePath,
			Lock:       true,
		})
		if err != nil {
			return nil, syscall.ENOENT
		}

		node, attr, err := n.inodeFromFsId(ctx, stash.GenerateFsID(sourcePath))
		if err != nil {
			return nil, syscall.ENOENT
		}

		out.Attr = *attr
		return node, fs.OK
	}

	node, attr, err := n.inodeFromFsId(ctx, stash.GenerateFsID(fullPath))
	if err != nil {
		return nil, syscall.ENOENT
	}

	out.Attr = *attr
	return node, fs.OK
}

// Opendir is called when a directory is opened.
func (n *Node) Opendir(ctx context.Context) syscall.Errno {
	n.log("Opendir called")
	return 0
}

// Open is called when a file is opened.
func (n *Node) Open(ctx context.Context, flags uint32) (fh fs.FileHandle, fuseFlags uint32, errno syscall.Errno) {
	n.log("Open called with flags: %v", flags)

	if n.filesystem.config.BlobFs.DirectIO {
		fuseFlags |= fuse.FOPEN_DIRECT_IO
		fuseFlags &= ^uint32(fuse.FOPEN_KEEP_CACHE)
		return nil, fuseFlags, fs.OK
	}

	return nil, 0, fs.OK
}

// Read reads data from the file.
func (n *Node) Read(ctx context.Context, f fs.FileHandle, dest []byte, off int64) (fuse.ReadResult, syscall.Errno) {
	n.log("Read called with offset: %v, length: %v", off, len(dest))

	// Don't try to read 0 byte files
	if n.bfsNode.Attr.Size == 0 {
		return fuse.ReadResultData(dest[:0]), fs.OK
	}

	sourcePath := n.bfsNode.Path

	buffer, err := n.filesystem.client.GetContent(n.bfsNode.Hash, off, int64(len(dest)), client.ContentOptions{
		RoutingKey: sourcePath,
	})
	if err != nil {
		if err == stash.ErrContentNotFound {
			// Content not in cache, store it
			_, err = n.filesystem.client.StoreContentFromFUSE(client.FUSESource{Path: sourcePath}, client.SourceOptions{
				RoutingKey: sourcePath,
				Lock:       true,
			})
			if err != nil && err == stash.ErrUnableToAcquireLock {
				return nil, syscall.EAGAIN
			} else if err != nil {
				return nil, syscall.EIO
			}

			buffer, err = n.filesystem.client.GetContent(n.bfsNode.Hash, off, int64(len(dest)), client.ContentOptions{
				RoutingKey: sourcePath,
			})
			if err != nil {
				return nil, syscall.EIO
			}
		} else {
			return nil, syscall.EIO
		}
	}

	return fuse.ReadResultData(buffer), fs.OK
}

// Readlink reads the target of a symlink.
func (n *Node) Readlink(ctx context.Context) ([]byte, syscall.Errno) {
	n.log("Readlink called")

	if n.bfsNode.Target == "" {
		return nil, syscall.EINVAL
	}

	return []byte(n.bfsNode.Target), fs.OK
}

// Readdir reads directory entries.
func (n *Node) Readdir(ctx context.Context) (fs.DirStream, syscall.Errno) {
	n.log("Readdir called")

	children, err := n.filesystem.coordinator.GetFsNodeChildren(ctx, stash.GenerateFsID(n.bfsNode.Path))
	if err != nil {
		return nil, fs.ENOATTR
	}

	dirEntries := []fuse.DirEntry{}
	for _, child := range children {
		dirEntries = append(dirEntries, fuse.DirEntry{
			Mode: child.Mode,
			Name: child.Name,
			Ino:  child.Ino,
		})
	}

	return fs.NewListDirStream(dirEntries), fs.OK
}

// Create creates a new file.
func (n *Node) Create(ctx context.Context, name string, flags uint32, mode uint32, out *fuse.EntryOut) (inode *fs.Inode, fh fs.FileHandle, fuseFlags uint32, errno syscall.Errno) {
	n.log("Create called with name: %s, flags: %v, mode: %v", name, flags, mode)

	fullPath := path.Join(n.bfsNode.Path, name)
	newFsId := stash.GenerateFsID(fullPath)

	now := time.Now()
	nowSec := uint64(now.Unix())
	nowNsec := uint32(now.Nanosecond())

	meta := &metadata.FsMetadata{
		PID:       n.bfsNode.ID,
		ID:        newFsId,
		Name:      name,
		Path:      fullPath,
		Ino:       0,
		Mode:      mode,
		Atime:     nowSec,
		Mtime:     nowSec,
		Ctime:     nowSec,
		Atimensec: nowNsec,
		Mtimensec: nowNsec,
		Ctimensec: nowNsec,
		Size:      0,
		Hash:      "",
	}

	if err := n.filesystem.coordinator.SetFsNode(ctx, newFsId, meta); err != nil {
		return nil, nil, 0, syscall.EIO
	}

	if err := n.filesystem.coordinator.AddFsNodeChild(ctx, n.bfsNode.ID, newFsId); err != nil {
		return nil, nil, 0, syscall.EIO
	}

	inode = n.NewInode(ctx, &Node{
		filesystem: n.filesystem,
		bfsNode: &BlobFsNode{
			Path: fullPath,
			ID:   newFsId,
			Name: name,
			Attr: fuse.Attr{Mode: mode},
		},
	}, fs.StableAttr{Mode: mode, Ino: meta.Ino})

	out.Attr = fuse.Attr{
		Mode: mode,
		Ino:  meta.Ino,
	}

	return inode, nil, 0, fs.OK
}

// Mkdir creates a new directory.
func (n *Node) Mkdir(ctx context.Context, name string, mode uint32, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
	n.log("Mkdir called with name: %s, mode: %v", name, mode)

	fullPath := path.Join(n.bfsNode.Path, name)
	newFsId := stash.GenerateFsID(fullPath)

	now := time.Now()
	nowSec := uint64(now.Unix())
	nowNsec := uint32(now.Nanosecond())

	meta := &metadata.FsMetadata{
		PID:       n.bfsNode.ID,
		ID:        newFsId,
		Name:      name,
		Path:      fullPath,
		Ino:       0,
		Mode:      fuse.S_IFDIR | mode,
		Atime:     nowSec,
		Mtime:     nowSec,
		Ctime:     nowSec,
		Atimensec: nowNsec,
		Mtimensec: nowNsec,
		Ctimensec: nowNsec,
		Size:      0,
		Hash:      "",
	}

	if err := n.filesystem.coordinator.SetFsNode(ctx, newFsId, meta); err != nil {
		return nil, syscall.EIO
	}

	if err := n.filesystem.coordinator.AddFsNodeChild(ctx, n.bfsNode.ID, newFsId); err != nil {
		return nil, syscall.EIO
	}

	inode := n.NewInode(ctx, &Node{
		filesystem: n.filesystem,
		bfsNode: &BlobFsNode{
			Path: fullPath,
			ID:   newFsId,
			Name: name,
			Attr: fuse.Attr{Mode: fuse.S_IFDIR | mode},
		},
	}, fs.StableAttr{Mode: fuse.S_IFDIR | mode, Ino: meta.Ino})

	out.Attr = fuse.Attr{
		Mode: fuse.S_IFDIR | mode,
		Ino:  meta.Ino,
	}

	return inode, fs.OK
}

// Rmdir removes a directory.
func (n *Node) Rmdir(ctx context.Context, name string) syscall.Errno {
	n.log("Rmdir called with name: %s", name)

	fullPath := path.Join(n.bfsNode.Path, name)
	fsId := stash.GenerateFsID(fullPath)

	children, err := n.filesystem.coordinator.GetFsNodeChildren(ctx, fsId)
	if err != nil {
		return syscall.EIO
	}
	if len(children) > 0 {
		return syscall.ENOTEMPTY
	}

	if err := n.filesystem.coordinator.RemoveFsNode(ctx, fsId); err != nil {
		return syscall.EIO
	}

	if err := n.filesystem.coordinator.RemoveFsNodeChild(ctx, n.bfsNode.ID, fsId); err != nil {
		return syscall.EIO
	}

	return fs.OK
}

// Unlink removes a file.
func (n *Node) Unlink(ctx context.Context, name string) syscall.Errno {
	n.log("Unlink called with name: %s", name)

	fullPath := path.Join(n.bfsNode.Path, name)
	fsId := stash.GenerateFsID(fullPath)

	if err := n.filesystem.coordinator.RemoveFsNode(ctx, fsId); err != nil {
		return syscall.EIO
	}

	if err := n.filesystem.coordinator.RemoveFsNodeChild(ctx, n.bfsNode.ID, fsId); err != nil {
		return syscall.EIO
	}

	return fs.OK
}

// Rename renames or moves a file/directory.
func (n *Node) Rename(ctx context.Context, oldName string, newParent fs.InodeEmbedder, newName string, flags uint32) syscall.Errno {
	n.log("Rename called with oldName: %s, newName: %s, flags: %v", oldName, newName, flags)

	oldFullPath := path.Join(n.bfsNode.Path, oldName)
	newFullPath := path.Join(n.bfsNode.Path, newName)

	oldFsId := stash.GenerateFsID(oldFullPath)
	newFsId := stash.GenerateFsID(newFullPath)

	meta, err := n.filesystem.coordinator.GetFsNode(ctx, oldFsId)
	if err != nil {
		return syscall.ENOENT
	}

	meta.Name = newName
	meta.Path = newFullPath

	if err := n.filesystem.coordinator.SetFsNode(ctx, newFsId, meta); err != nil {
		return syscall.EIO
	}

	if err := n.filesystem.coordinator.RemoveFsNodeChild(ctx, n.bfsNode.ID, oldFsId); err != nil {
		return syscall.EIO
	}

	if err := n.filesystem.coordinator.AddFsNodeChild(ctx, n.bfsNode.ID, newFsId); err != nil {
		return syscall.EIO
	}

	return fs.OK
}
