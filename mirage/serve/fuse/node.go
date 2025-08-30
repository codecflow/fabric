package fuse

import (
	"context"
	"syscall"

	"github.com/codecflow/fabric/mirage"
	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
)

// node implements fs.InodeEmbedder for FUSE operations
type node struct {
	fs.Inode
	filesystem mirage.FileSystem
	meta       *mirage.Node
}

// Ensure node implements required FUSE interfaces
var _ fs.NodeOpener = (*node)(nil)
var _ fs.NodeOpendirer = (*node)(nil)
var _ fs.NodeCreater = (*node)(nil)
var _ fs.NodeMkdirer = (*node)(nil)
var _ fs.NodeRmdirer = (*node)(nil)
var _ fs.NodeUnlinker = (*node)(nil)
var _ fs.NodeRenamer = (*node)(nil)

// Getattr returns file attributes
func (n *node) Getattr(ctx context.Context, fh fs.FileHandle, out *fuse.AttrOut) syscall.Errno {
	out.Attr = n.toFuseAttr()
	return fs.OK
}

// toFuseAttr converts mirage.Node to fuse.Attr
func (n *node) toFuseAttr() fuse.Attr {
	mode := n.meta.Mode
	switch n.meta.Type {
	case mirage.NodeTypeDir:
		mode |= fuse.S_IFDIR
	case mirage.NodeTypeSymlink:
		mode |= fuse.S_IFLNK
	case mirage.NodeTypeFile:
		mode |= fuse.S_IFREG
	}

	return fuse.Attr{
		Ino:   n.meta.Ino,
		Size:  n.meta.Size,
		Mode:  mode,
		Mtime: uint64(n.meta.Mtime),
		Owner: fuse.Owner{
			Uid: n.meta.Uid,
			Gid: n.meta.Gid,
		},
	}
}

// Lookup looks up a child node by name
func (n *node) Lookup(ctx context.Context, name string, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
	// Only directories can be looked up
	if n.meta.Type != mirage.NodeTypeDir {
		return nil, syscall.ENOTDIR
	}

	// Construct child path
	childPath := n.meta.Path
	if childPath != "/" {
		childPath += "/"
	}
	childPath += name

	// Look up in filesystem
	childNode, err := n.filesystem.Node(childPath)
	if err != nil {
		return nil, syscall.ENOENT
	}

	// Create child FUSE node
	child := &node{
		filesystem: n.filesystem,
		meta:       childNode,
	}

	// Set output attributes
	out.Attr = child.toFuseAttr()

	// Create stable inode
	stableAttr := fs.StableAttr{
		Mode: out.Attr.Mode,
		Ino:  childNode.Ino,
	}

	childInode := n.NewInode(ctx, child, stableAttr)
	return childInode, fs.OK
}

// Readdir reads directory entries
func (n *node) Readdir(ctx context.Context) (fs.DirStream, syscall.Errno) {
	// Only directories can be read
	if n.meta.Type != mirage.NodeTypeDir {
		return nil, syscall.ENOTDIR
	}

	// List children from filesystem
	children, err := n.filesystem.List(n.meta.Path)
	if err != nil {
		return nil, syscall.EIO
	}

	// Convert to directory entries
	entries := make([]fuse.DirEntry, len(children))
	for i, child := range children {
		var mode uint32
		switch child.Type {
		case mirage.NodeTypeDir:
			mode = fuse.S_IFDIR
		case mirage.NodeTypeSymlink:
			mode = fuse.S_IFLNK
		case mirage.NodeTypeFile:
			mode = fuse.S_IFREG
		}

		entries[i] = fuse.DirEntry{
			Mode: mode,
			Name: child.Name,
			Ino:  child.Ino,
		}
	}

	return fs.NewListDirStream(entries), fs.OK
}

// Read reads file content
func (n *node) Read(ctx context.Context, f fs.FileHandle, dest []byte, off int64) (fuse.ReadResult, syscall.Errno) {
	// Only files can be read
	if n.meta.Type != mirage.NodeTypeFile {
		return nil, syscall.EISDIR
	}

	// Read from filesystem
	bytesRead, err := n.filesystem.Read(n.meta, dest, off)
	if err != nil {
		return nil, syscall.EIO
	}

	return fuse.ReadResultData(dest[:bytesRead]), fs.OK
}

// Readlink reads the target of a symlink
func (n *node) Readlink(ctx context.Context) ([]byte, syscall.Errno) {
	// Only symlinks can be read
	if n.meta.Type != mirage.NodeTypeSymlink {
		return nil, syscall.EINVAL
	}

	return []byte(n.meta.Target), fs.OK
}

// Open is called when a file is opened - no-op for stateless reads
func (n *node) Open(ctx context.Context, flags uint32) (fs.FileHandle, uint32, syscall.Errno) {
	return nil, 0, fs.OK
}

// Opendir is called when a directory is opened - no-op
func (n *node) Opendir(ctx context.Context) syscall.Errno {
	return fs.OK
}

// Write operation stubs - return EROFS (read-only filesystem)

// Create is not supported on read-only filesystem
func (n *node) Create(ctx context.Context, name string, flags uint32, mode uint32, out *fuse.EntryOut) (*fs.Inode, fs.FileHandle, uint32, syscall.Errno) {
	return nil, nil, 0, syscall.EROFS
}

// Mkdir is not supported on read-only filesystem
func (n *node) Mkdir(ctx context.Context, name string, mode uint32, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
	return nil, syscall.EROFS
}

// Rmdir is not supported on read-only filesystem
func (n *node) Rmdir(ctx context.Context, name string) syscall.Errno {
	return syscall.EROFS
}

// Unlink is not supported on read-only filesystem
func (n *node) Unlink(ctx context.Context, name string) syscall.Errno {
	return syscall.EROFS
}

// Rename is not supported on read-only filesystem
func (n *node) Rename(ctx context.Context, oldName string, newParent fs.InodeEmbedder, newName string, flags uint32) syscall.Errno {
	return syscall.EROFS
}
