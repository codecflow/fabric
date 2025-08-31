package oci

import (
	"compress/gzip"
	"fmt"
	"io"
	"os"
)

// readDecompressed reads from a decompressed layer file
func (fs *OCIFS) readDecompressed(layerPath string, offset int64, dest []byte) (int, error) {
	f, err := os.Open(layerPath)
	if err != nil {
		return 0, fmt.Errorf("failed to open layer file: %w", err)
	}
	defer f.Close()

	// Seek to offset
	if _, err := f.Seek(offset, io.SeekStart); err != nil {
		return 0, fmt.Errorf("failed to seek: %w", err)
	}

	// Read data
	n, err := io.ReadFull(f, dest)
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		return n, fmt.Errorf("failed to read: %w", err)
	}

	return n, nil
}

// readCompressed reads from a compressed layer using checkpoints
func (fs *OCIFS) readCompressed(layer *LayerInfo, wantUOffset int64, dest []byte) (int, error) {
	// Get gzip index for this layer
	gzipIndex, ok := fs.gzipIndexes[layer.Digest]
	if !ok || len(gzipIndex.Checkpoints) == 0 {
		return 0, fmt.Errorf("no gzip checkpoints available for layer: %s", layer.Digest)
	}

	// Find nearest checkpoint
	cOffset, uOffset := NearestCheckpoint(gzipIndex.Checkpoints, wantUOffset)

	// Open compressed layer file
	f, err := os.Open(layer.Path)
	if err != nil {
		return 0, fmt.Errorf("failed to open compressed layer: %w", err)
	}
	defer f.Close()

	// Seek to checkpoint's compressed offset
	if cOffset > 0 {
		if _, err := f.Seek(cOffset, io.SeekStart); err != nil {
			return 0, fmt.Errorf("failed to seek to checkpoint: %w", err)
		}
	}

	// Create gzip reader from checkpoint
	gzr, err := gzip.NewReader(f)
	if err != nil {
		return 0, fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer gzr.Close()

	// Skip bytes from checkpoint to desired offset
	skipBytes := wantUOffset - uOffset
	if skipBytes > 0 {
		if _, err := io.CopyN(io.Discard, gzr, skipBytes); err != nil {
			return 0, fmt.Errorf("failed to skip to offset: %w", err)
		}
	}

	// Read the requested data
	n, err := io.ReadFull(gzr, dest)
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		return n, fmt.Errorf("failed to read from gzip: %w", err)
	}

	return n, nil
}
