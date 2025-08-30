package dir

import (
	"fmt"
	"os"

	"github.com/codecflow/fabric/mirage"
	"github.com/vmihailenco/msgpack/v5"
	"go.etcd.io/bbolt"
)

// Bucket names in bbolt
var (
	metaBucket     = []byte("meta")
	nodesBucket    = []byte("nodes")
	childrenBucket = []byte("children")
)

// DirFS implements mirage.FileSystem for a directory-based index
type DirFS struct {
	db        *bbolt.DB
	layerPath string
	layerFile *os.File
}

// Open opens an existing directory index
func Open(indexPath string) (*DirFS, error) {
	// Open bbolt database
	db, err := bbolt.Open(indexPath, 0600, &bbolt.Options{ReadOnly: true})
	if err != nil {
		return nil, fmt.Errorf("failed to open index: %w", err)
	}

	// Read metadata to get layer path
	var layerPath string
	err = db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(metaBucket)
		if b == nil {
			return fmt.Errorf("meta bucket not found")
		}

		data := b.Get([]byte("layer_path"))
		if data == nil {
			return fmt.Errorf("layer_path not found in metadata")
		}

		layerPath = string(data)
		return nil
	})
	if err != nil {
		db.Close()
		return nil, err
	}

	// Open layer file
	layerFile, err := os.Open(layerPath)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to open layer file: %w", err)
	}

	return &DirFS{
		db:        db,
		layerPath: layerPath,
		layerFile: layerFile,
	}, nil
}

// Node retrieves a node by path
func (fs *DirFS) Node(path string) (*mirage.Node, error) {
	var node *mirage.Node

	err := fs.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(nodesBucket)
		if b == nil {
			return fmt.Errorf("nodes bucket not found")
		}

		data := b.Get([]byte(path))
		if data == nil {
			return fmt.Errorf("node not found: %s", path)
		}

		node = &mirage.Node{}
		if err := msgpack.Unmarshal(data, node); err != nil {
			return fmt.Errorf("failed to decode node: %w", err)
		}

		return nil
	})

	return node, err
}

// List returns immediate children of a directory
func (fs *DirFS) List(dirPath string) ([]*mirage.Node, error) {
	var nodes []*mirage.Node

	err := fs.db.View(func(tx *bbolt.Tx) error {
		childrenBkt := tx.Bucket(childrenBucket)
		if childrenBkt == nil {
			return fmt.Errorf("children bucket not found")
		}

		nodesBkt := tx.Bucket(nodesBucket)
		if nodesBkt == nil {
			return fmt.Errorf("nodes bucket not found")
		}

		// Get list of child names
		childrenData := childrenBkt.Get([]byte(dirPath))
		if childrenData == nil {
			// Empty directory or doesn't exist
			return nil
		}

		var childNames []string
		if err := msgpack.Unmarshal(childrenData, &childNames); err != nil {
			return fmt.Errorf("failed to decode children list: %w", err)
		}

		// Fetch each child node
		nodes = make([]*mirage.Node, 0, len(childNames))
		for _, name := range childNames {
			childPath := dirPath
			if dirPath != "/" {
				childPath += "/"
			}
			childPath += name

			nodeData := nodesBkt.Get([]byte(childPath))
			if nodeData == nil {
				continue // Skip missing nodes
			}

			node := &mirage.Node{}
			if err := msgpack.Unmarshal(nodeData, node); err != nil {
				return fmt.Errorf("failed to decode node %s: %w", childPath, err)
			}

			nodes = append(nodes, node)
		}

		return nil
	})

	return nodes, err
}

// Read reads file content from the layer file
func (fs *DirFS) Read(node *mirage.Node, dest []byte, offset int64) (int, error) {
	// Only files can be read
	if node.Type != mirage.NodeTypeFile {
		return 0, fmt.Errorf("cannot read non-file node")
	}

	// Calculate position in layer file
	layerOffset := node.Offset + offset

	// Read from layer file
	n, err := fs.layerFile.ReadAt(dest, layerOffset)
	if err != nil && err.Error() != "EOF" {
		return 0, fmt.Errorf("failed to read from layer: %w", err)
	}

	return n, nil
}

// Close closes the filesystem
func (fs *DirFS) Close() error {
	var err1, err2 error
	if fs.layerFile != nil {
		err1 = fs.layerFile.Close()
	}
	if fs.db != nil {
		err2 = fs.db.Close()
	}

	if err1 != nil {
		return err1
	}
	return err2
}
