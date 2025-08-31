package oci

import (
	"fmt"

	"github.com/codecflow/fabric/mirage"
	"github.com/vmihailenco/msgpack/v5"
	"go.etcd.io/bbolt"
)

// Bucket names in bbolt
var (
	metaBucket     = []byte("meta")
	nodesBucket    = []byte("nodes")
	childrenBucket = []byte("children")
	layersBucket   = []byte("layers")
)

// OCIFS implements mirage.FileSystem for OCI container images
type OCIFS struct {
	db            *bbolt.DB
	layers        map[string]*LayerInfo
	gzipIndexes   map[string]*GzipIndex // Checkpoints per layer
	useCompressed bool                  // If true, keep layers compressed with checkpoints
}

// LayerInfo contains information about a layer in the OCI image
type LayerInfo struct {
	Digest           string // SHA256 digest of the compressed layer
	DecompressedHash string // SHA256 of decompressed content
	Size             int64  // Size of the layer
	Path             string // Path to the layer file on disk (compressed or decompressed)
	Compressed       bool   // Whether the layer file is compressed
}

// Open opens an existing OCI index
func Open(indexPath string) (*OCIFS, error) {
	// Open bbolt database
	db, err := bbolt.Open(indexPath, 0600, &bbolt.Options{ReadOnly: true})
	if err != nil {
		return nil, fmt.Errorf("failed to open index: %w", err)
	}

	// Load layer information and gzip indexes
	layers := make(map[string]*LayerInfo)
	gzipIndexes := make(map[string]*GzipIndex)
	useCompressed := false

	err = db.View(func(tx *bbolt.Tx) error {
		// Load layers
		layersBkt := tx.Bucket(layersBucket)
		if layersBkt == nil {
			return fmt.Errorf("layers bucket not found")
		}

		err := layersBkt.ForEach(func(k, v []byte) error {
			var layer LayerInfo
			if err := msgpack.Unmarshal(v, &layer); err != nil {
				return fmt.Errorf("failed to decode layer: %w", err)
			}
			layers[string(k)] = &layer
			if layer.Compressed {
				useCompressed = true
			}
			return nil
		})
		if err != nil {
			return err
		}

		// Load gzip indexes if present (for compressed layers)
		metaBkt := tx.Bucket(metaBucket)
		if metaBkt != nil {
			gzipIndexData := metaBkt.Get([]byte("gzip_indexes"))
			if gzipIndexData != nil {
				var indexes []GzipIndex
				if err := msgpack.Unmarshal(gzipIndexData, &indexes); err == nil {
					for _, idx := range indexes {
						gzipIndexes[idx.LayerDigest] = &idx
					}
				}
			}
		}

		return nil
	})
	if err != nil {
		db.Close()
		return nil, err
	}

	return &OCIFS{
		db:            db,
		layers:        layers,
		gzipIndexes:   gzipIndexes,
		useCompressed: useCompressed,
	}, nil
}

// Node retrieves a node by path
func (fs *OCIFS) Node(path string) (*mirage.Node, error) {
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
func (fs *OCIFS) List(dirPath string) ([]*mirage.Node, error) {
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

// Read reads file content from the appropriate layer file
func (fs *OCIFS) Read(node *mirage.Node, dest []byte, offset int64) (int, error) {
	// Only files can be read
	if node.Type != mirage.NodeTypeFile {
		return 0, fmt.Errorf("cannot read non-file node")
	}

	// Get layer info
	layer, ok := fs.layers[node.Blob]
	if !ok {
		return 0, fmt.Errorf("layer not found: %s", node.Blob)
	}

	// Calculate absolute offset in layer
	layerOffset := node.Offset + offset

	// Handle based on compression
	if layer.Compressed {
		// Use checkpoint-based decompression
		return fs.readCompressed(layer, layerOffset, dest)
	}

	// Direct read from decompressed layer
	return fs.readDecompressed(layer.Path, layerOffset, dest)
}

// Close closes the filesystem
func (fs *OCIFS) Close() error {
	if fs.db != nil {
		return fs.db.Close()
	}
	return nil
}
