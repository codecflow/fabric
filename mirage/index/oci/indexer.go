package oci

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/codecflow/fabric/mirage"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/vmihailenco/msgpack/v5"
	"go.etcd.io/bbolt"
)

// IndexOptions configures OCI image indexing
type IndexOptions struct {
	OutputDir          string              // Where to store index.db and layer files
	UseCheckpoints     bool                // Generate gzip checkpoints (default: false for V1)
	CheckpointInterval int64               // Checkpoint every N bytes (default: 1MB)
	Auth               authn.Authenticator // Registry authentication
}

// IndexResult contains information about the indexed OCI image
type IndexResult struct {
	IndexPath    string       // Path to index.db
	ImageRef     string       // Image reference (e.g., "alpine:3.18")
	Digest       string       // Image digest
	TotalFiles   int          // Number of files indexed
	TotalDirs    int          // Number of directories
	TotalSize    int64        // Total uncompressed size
	LayerCount   int          // Number of layers
	Layers       []*LayerInfo // Layer information
	Checkpointed bool         // Whether checkpoints were generated
}

// Index creates an index from an OCI image reference
func Index(imageRef string, opts IndexOptions) (*IndexResult, error) {
	ctx := context.Background()

	// Set defaults
	if opts.CheckpointInterval == 0 {
		opts.CheckpointInterval = 1024 * 1024 // 1MB default
	}

	// Create output directory
	if err := os.MkdirAll(opts.OutputDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create output directory: %w", err)
	}

	indexPath := filepath.Join(opts.OutputDir, "index.db")
	layersDir := filepath.Join(opts.OutputDir, "layers")
	if err := os.MkdirAll(layersDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create layers directory: %w", err)
	}

	// Parse image reference
	ref, err := name.ParseReference(imageRef)
	if err != nil {
		return nil, fmt.Errorf("failed to parse image reference: %w", err)
	}

	// Fetch image from registry
	remoteOpts := []remote.Option{remote.WithContext(ctx)}
	if opts.Auth != nil {
		remoteOpts = append(remoteOpts, remote.WithAuth(opts.Auth))
	} else {
		remoteOpts = append(remoteOpts, remote.WithAuthFromKeychain(authn.DefaultKeychain))
	}

	img, err := remote.Image(ref, remoteOpts...)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch image: %w", err)
	}

	// Get image digest
	digest, err := img.Digest()
	if err != nil {
		return nil, fmt.Errorf("failed to get image digest: %w", err)
	}

	// Get layers
	layers, err := img.Layers()
	if err != nil {
		return nil, fmt.Errorf("failed to get layers: %w", err)
	}

	// Create bbolt database
	db, err := bbolt.Open(indexPath, 0600, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create index database: %w", err)
	}
	defer db.Close()

	// Create buckets
	if err := createBuckets(db); err != nil {
		return nil, err
	}

	// Build index
	result := &IndexResult{
		IndexPath:    indexPath,
		ImageRef:     imageRef,
		Digest:       digest.String(),
		LayerCount:   len(layers),
		Layers:       make([]*LayerInfo, 0, len(layers)),
		Checkpointed: opts.UseCheckpoints,
	}

	builder := &ociIndexBuilder{
		db:                 db,
		layersDir:          layersDir,
		result:             result,
		nodes:              make(map[string]*mirage.Node),
		children:           make(map[string][]string),
		inoCounter:         1,
		useCheckpoints:     opts.UseCheckpoints,
		checkpointInterval: opts.CheckpointInterval,
		gzipIndexes:        make([]*GzipIndex, 0),
	}

	// Process each layer
	for i, layer := range layers {
		if err := builder.processLayer(ctx, i, layer); err != nil {
			return nil, fmt.Errorf("failed to process layer %d: %w", i, err)
		}
	}

	// Store nodes and metadata
	if err := builder.finalize(); err != nil {
		return nil, fmt.Errorf("failed to finalize index: %w", err)
	}

	return result, nil
}

func createBuckets(db *bbolt.DB) error {
	return db.Update(func(tx *bbolt.Tx) error {
		for _, bucket := range [][]byte{metaBucket, nodesBucket, childrenBucket, layersBucket} {
			if _, err := tx.CreateBucketIfNotExists(bucket); err != nil {
				return err
			}
		}
		return nil
	})
}

type ociIndexBuilder struct {
	db                 *bbolt.DB
	layersDir          string
	result             *IndexResult
	nodes              map[string]*mirage.Node
	children           map[string][]string
	inoCounter         uint64
	useCheckpoints     bool
	checkpointInterval int64
	gzipIndexes        []*GzipIndex
	layerOffset        int64 // Current offset in concatenated decompressed data
}

func (b *ociIndexBuilder) processLayer(ctx context.Context, layerIndex int, layer v1.Layer) error {
	// Get layer digest
	digest, err := layer.Digest()
	if err != nil {
		return fmt.Errorf("failed to get layer digest: %w", err)
	}

	layerDigest := digest.String()
	fmt.Printf("Processing layer %d: %s\n", layerIndex, layerDigest)

	// Download compressed layer
	compressedRC, err := layer.Compressed()
	if err != nil {
		return fmt.Errorf("failed to get compressed layer: %w", err)
	}
	defer compressedRC.Close()

	var layerPath string
	var layerInfo *LayerInfo
	var gzipIndex *GzipIndex

	if b.useCheckpoints {
		// Store compressed with checkpoints (TODO: not yet implemented)
		return fmt.Errorf("checkpoint mode not yet implemented")
	} else {
		// Store decompressed (like dir indexer)
		layerPath = filepath.Join(b.layersDir, layerDigest)
		layerInfo, err = b.processDecompressedLayer(layerDigest, compressedRC, layerPath, layerIndex)
	}

	if err != nil {
		return err
	}

	b.result.Layers = append(b.result.Layers, layerInfo)
	if gzipIndex != nil {
		b.gzipIndexes = append(b.gzipIndexes, gzipIndex)
	}

	return nil
}

// processDecompressedLayer stores raw file data (not TAR structure)
func (b *ociIndexBuilder) processDecompressedLayer(digest string, compressedRC io.ReadCloser, layerPath string, layerIndex int) (*LayerInfo, error) {
	// Create temp file for layer data
	tempPath := layerPath + ".tmp"
	layerFile, err := os.Create(tempPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(tempPath)

	// Decompress
	gzr, err := gzip.NewReader(compressedRC)
	if err != nil {
		layerFile.Close()
		return nil, fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer gzr.Close()

	// Parse TAR and extract raw file data
	tr := tar.NewReader(gzr)
	decompressedHash := sha256.New()
	currentOffset := int64(0)

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			layerFile.Close()
			return nil, fmt.Errorf("failed to read tar: %w", err)
		}

		// Convert to clean path
		path := "/" + strings.TrimPrefix(filepath.Clean(header.Name), "/")
		if path == "/." {
			continue
		}

		// Handle whiteouts (overlay filesystem deletion markers)
		if strings.HasPrefix(filepath.Base(path), ".wh.") {
			// TODO: Handle whiteout files
			continue
		}

		// Create or update node
		node := b.getOrCreateNode(path)
		node.Mode = uint32(header.Mode)
		node.Size = uint64(header.Size)
		node.Mtime = header.ModTime.Unix()
		node.Uid = uint32(header.Uid)
		node.Gid = uint32(header.Gid)
		node.Blob = digest

		switch header.Typeflag {
		case tar.TypeDir:
			node.Type = mirage.NodeTypeDir

		case tar.TypeReg:
			node.Type = mirage.NodeTypeFile
			node.Offset = b.layerOffset + currentOffset

			// Copy file data to layer file (raw data, not TAR structure)
			written, err := io.Copy(io.MultiWriter(layerFile, decompressedHash), tr)
			if err != nil {
				layerFile.Close()
				return nil, fmt.Errorf("failed to copy file data: %w", err)
			}

			currentOffset += written

		case tar.TypeSymlink:
			node.Type = mirage.NodeTypeSymlink
			node.Target = header.Linkname

		case tar.TypeLink:
			// Hard link - treat as symlink for now
			node.Type = mirage.NodeTypeSymlink
			node.Target = header.Linkname
		}

		// Track parent-child relationship
		if path != "/" {
			parent := filepath.Dir(path)
			if parent == "." {
				parent = "/"
			}
			b.addChild(parent, filepath.Base(path))
		}

		b.result.TotalFiles++
		if node.Type == mirage.NodeTypeDir {
			b.result.TotalDirs++
		}
		b.result.TotalSize += header.Size
	}

	layerFile.Close()

	// Move to final location
	if err := os.Rename(tempPath, layerPath); err != nil {
		return nil, fmt.Errorf("failed to rename file: %w", err)
	}

	layerInfo := &LayerInfo{
		Digest:           digest,
		DecompressedHash: fmt.Sprintf("sha256:%x", decompressedHash.Sum(nil)),
		Size:             currentOffset,
		Path:             layerPath,
		Compressed:       false,
	}

	b.layerOffset += currentOffset
	return layerInfo, nil
}

func (b *ociIndexBuilder) getOrCreateNode(path string) *mirage.Node {
	if node, exists := b.nodes[path]; exists {
		return node
	}

	node := &mirage.Node{
		Path:   path,
		Name:   filepath.Base(path),
		Parent: filepath.Dir(path),
		Ino:    b.inoCounter,
	}
	b.inoCounter++
	b.nodes[path] = node
	return node
}

func (b *ociIndexBuilder) addChild(parent, childName string) {
	children := b.children[parent]
	// Check if already exists
	for _, existing := range children {
		if existing == childName {
			return
		}
	}
	b.children[parent] = append(children, childName)
}

func (b *ociIndexBuilder) finalize() error {
	// Ensure root exists
	if _, exists := b.nodes["/"]; !exists {
		root := &mirage.Node{
			Path:   "/",
			Name:   "/",
			Parent: "",
			Type:   mirage.NodeTypeDir,
			Mode:   0755 | uint32(os.ModeDir),
			Ino:    b.inoCounter,
		}
		b.inoCounter++
		b.nodes["/"] = root
	}

	// Store all nodes
	if err := b.storeNodes(); err != nil {
		return err
	}

	// Store children lists
	if err := b.storeChildren(); err != nil {
		return err
	}

	// Store layers
	if err := b.storeLayers(); err != nil {
		return err
	}

	// Store metadata
	return b.storeMetadata()
}

func (b *ociIndexBuilder) storeNodes() error {
	return b.db.Update(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket(nodesBucket)
		for path, node := range b.nodes {
			data, err := msgpack.Marshal(node)
			if err != nil {
				return fmt.Errorf("failed to marshal node %s: %w", path, err)
			}
			if err := bucket.Put([]byte(path), data); err != nil {
				return err
			}
		}
		return nil
	})
}

func (b *ociIndexBuilder) storeChildren() error {
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

func (b *ociIndexBuilder) storeLayers() error {
	return b.db.Update(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket(layersBucket)
		for _, layer := range b.result.Layers {
			data, err := msgpack.Marshal(layer)
			if err != nil {
				return fmt.Errorf("failed to marshal layer: %w", err)
			}
			if err := bucket.Put([]byte(layer.Digest), data); err != nil {
				return err
			}
		}
		return nil
	})
}

func (b *ociIndexBuilder) storeMetadata() error {
	return b.db.Update(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket(metaBucket)

		// Store basic metadata
		bucket.Put([]byte("image_ref"), []byte(b.result.ImageRef))
		bucket.Put([]byte("digest"), []byte(b.result.Digest))
		bucket.Put([]byte("created"), []byte(fmt.Sprintf("%d", time.Now().Unix())))

		// Store gzip indexes if present
		if len(b.gzipIndexes) > 0 {
			data, err := msgpack.Marshal(b.gzipIndexes)
			if err != nil {
				return fmt.Errorf("failed to marshal gzip indexes: %w", err)
			}
			bucket.Put([]byte("gzip_indexes"), data)
		}

		return nil
	})
}

func getFileSize(path string) (int64, error) {
	info, err := os.Stat(path)
	if err != nil {
		return 0, err
	}
	return info.Size(), nil
}
