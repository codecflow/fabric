package mirage

// FileSystem provides lazy-loading filesystem access.
// Two-phase design:
//
//	Phase 1: index/local, index/oci create FileSystem from sources
//	Phase 2: serve/fuse, serve/http consume FileSystem
type FileSystem interface {
	// Node retrieves metadata for a single path
	Node(path string) (*Node, error)

	// List returns immediate children of a directory
	List(dirPath string) ([]*Node, error)

	// Read fetches file content bytes
	// For files: reads from blob at offset
	// For directories: returns EISDIR error
	Read(node *Node, buf []byte, offset int64) (int, error)

	// Close releases resources (database, connections, etc.)
	Close() error
}

// NodeType represents the type of filesystem node
type NodeType uint8

const (
	NodeTypeFile NodeType = iota
	NodeTypeDir
	NodeTypeSymlink
)

// String returns a string representation of the NodeType
func (nt NodeType) String() string {
	switch nt {
	case NodeTypeFile:
		return "file"
	case NodeTypeDir:
		return "dir"
	case NodeTypeSymlink:
		return "symlink"
	default:
		return "unknown"
	}
}

// Node represents a file/directory in the mirage filesystem
type Node struct {
	// Identity
	Path   string   // Full path (e.g., "/usr/bin/python3")
	Name   string   // Basename (e.g., "python3")
	Parent string   // Parent directory (e.g., "/usr/bin")
	Type   NodeType // File, directory, or symlink

	// Attributes
	Mode  uint32 // Unix permissions + type bits
	Uid   uint32 // Owner user ID
	Gid   uint32 // Owner group ID
	Size  uint64 // File size in bytes
	Mtime int64  // Modification time (Unix timestamp)
	Ino   uint64 // Inode number

	// Symlink target (only for symlinks)
	Target string

	// Content location (only for files)
	// The base unit: which blob + where in blob + how many bytes
	Blob   string // Blob/layer digest (e.g., "sha256:abc...")
	Offset int64  // Byte offset within blob
}
