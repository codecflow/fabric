package dir

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/codecflow/fabric/mirage"
	"github.com/vmihailenco/msgpack/v5"
	"go.etcd.io/bbolt"
)

// IndexOptions configures the indexing process
type IndexOptions struct {
	OutputDir string // Directory where index.db and layer.bin will be created
}

// IndexResult contains information about the created index
type IndexResult struct {
	IndexPath  string // Path to index.db
	LayerPath  string // Path to layer.bin
	TotalFiles int    // Number of files indexed
	TotalDirs  int    // Number of directories indexed
	TotalSize  int64  // Total size of all files
}

// Index creates an index from a source directory
func Index(sourcePath string, opts IndexOptions) (*IndexResult, error) {
	// Validate source
	info, err := os.Stat(sourcePath)
	if err != nil {
		return nil, fmt.Errorf("failed to stat source: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("source is not a directory: %s", sourcePath)
	}

	// Create output directory
	if err := os.MkdirAll(opts.OutputDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create output directory: %w", err)
	}

	indexPath := filepath.Join(opts.OutputDir, "index.db")
	layerPath := filepath.Join(opts.OutputDir, "layer.bin")

	// Create layer file
	layerFile, err := os.Create(layerPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create layer file: %w", err)
	}
	defer layerFile.Close()

	// Create bbolt database
	db, err := bbolt.Open(indexPath, 0600, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create index database: %w", err)
	}
	defer db.Close()

	// Create buckets
	err = db.Update(func(tx *bbolt.Tx) error {
		if _, err := tx.CreateBucketIfNotExists(metaBucket); err != nil {
			return err
		}
		if _, err := tx.CreateBucketIfNotExists(nodesBucket); err != nil {
			return err
		}
		if _, err := tx.CreateBucketIfNotExists(childrenBucket); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create buckets: %w", err)
	}

	// Build index
	result := &IndexResult{
		IndexPath: indexPath,
		LayerPath: layerPath,
	}

	builder := &indexBuilder{
		db:         db,
		layerFile:  layerFile,
		sourcePath: sourcePath,
		result:     result,
		children:   make(map[string][]string),
		inoCounter: 1, // Start from 1
	}

	if err := builder.build(); err != nil {
		return nil, err
	}

	// Store metadata
	err = db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(metaBucket)
		b.Put([]byte("layer_path"), []byte(layerPath))
		b.Put([]byte("created"), []byte(fmt.Sprintf("%d", time.Now().Unix())))
		b.Put([]byte("source"), []byte(sourcePath))
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to store metadata: %w", err)
	}

	return result, nil
}

type indexBuilder struct {
	db          *bbolt.DB
	layerFile   *os.File
	sourcePath  string
	result      *IndexResult
	children    map[string][]string // parent path → child names
	inoCounter  uint64
	layerOffset int64
}

func (b *indexBuilder) build() error {
	// Walk directory tree
	err := filepath.Walk(b.sourcePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Get relative path
		relPath, err := filepath.Rel(b.sourcePath, path)
		if err != nil {
			return err
		}

		// Convert to unix-style path
		nodePath := "/" + filepath.ToSlash(relPath)
		if nodePath == "/." {
			nodePath = "/"
		}

		// Create node
		node, err := b.createNode(nodePath, path, info)
		if err != nil {
			return fmt.Errorf("failed to create node for %s: %w", path, err)
		}

		// Store node
		if err := b.storeNode(node); err != nil {
			return err
		}

		// Track parent-child relationship
		if nodePath != "/" {
			parent := filepath.Dir(nodePath)
			if parent == "." {
				parent = "/"
			}
			b.children[parent] = append(b.children[parent], node.Name)
		}

		// Update stats
		if info.IsDir() {
			b.result.TotalDirs++
		} else if info.Mode().IsRegular() {
			b.result.TotalFiles++
			b.result.TotalSize += info.Size()
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to walk directory: %w", err)
	}

	// Store children lists
	return b.storeChildren()
}

func (b *indexBuilder) createNode(nodePath, diskPath string, info os.FileInfo) (*mirage.Node, error) {
	node := &mirage.Node{
		Path:   nodePath,
		Name:   info.Name(),
		Parent: filepath.Dir(nodePath),
		Mode:   uint32(info.Mode()),
		Size:   uint64(info.Size()),
		Mtime:  info.ModTime().Unix(),
		Ino:    b.inoCounter,
	}
	b.inoCounter++

	// Get uid/gid (Unix-specific, defaults to 0 on other platforms)
	if stat, ok := info.Sys().(*syscall.Stat_t); ok {
		node.Uid = stat.Uid
		node.Gid = stat.Gid
	}

	// Determine type and handle content
	switch {
	case info.IsDir():
		node.Type = mirage.NodeTypeDir

	case info.Mode()&os.ModeSymlink != 0:
		node.Type = mirage.NodeTypeSymlink
		target, err := os.Readlink(diskPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read symlink: %w", err)
		}
		node.Target = target

	case info.Mode().IsRegular():
		node.Type = mirage.NodeTypeFile
		// Copy file content to layer and calculate hash
		if err := b.addFileToLayer(node, diskPath); err != nil {
			return nil, err
		}

	default:
		// Skip special files (devices, pipes, etc.)
		return nil, fmt.Errorf("unsupported file type: %s", info.Mode())
	}

	return node, nil
}

func (b *indexBuilder) addFileToLayer(node *mirage.Node, diskPath string) error {
	// Open source file
	src, err := os.Open(diskPath)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer src.Close()

	// Set offset in layer
	node.Offset = b.layerOffset
	node.Blob = "layer" // Single blob for all files

	// Copy content to layer
	written, err := io.Copy(b.layerFile, src)
	if err != nil {
		return fmt.Errorf("failed to copy file: %w", err)
	}

	node.Size = uint64(written)

	// Update layer offset
	b.layerOffset += written

	return nil
}

func (b *indexBuilder) storeNode(node *mirage.Node) error {
	data, err := msgpack.Marshal(node)
	if err != nil {
		return fmt.Errorf("failed to marshal node: %w", err)
	}

	return b.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(nodesBucket)
		return b.Put([]byte(node.Path), data)
	})
}

func (b *indexBuilder) storeChildren() error {
	return b.db.Update(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket(childrenBucket)

		for parent, children := range b.children {
			data, err := msgpack.Marshal(children)
			if err != nil {
				return fmt.Errorf("failed to marshal children for %s: %w", parent, err)
			}

			if err := bucket.Put([]byte(parent), data); err != nil {
				return err
			}
		}

		return nil
	})
}
