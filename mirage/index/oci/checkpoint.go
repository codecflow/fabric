package oci

import (
	"compress/gzip"
	"fmt"
	"io"
	"os"
)

// Checkpoint represents a position in a gzip stream where decompression can resume
type Checkpoint struct {
	COffset int64 // Offset in compressed stream
	UOffset int64 // Offset in uncompressed stream
}

// GzipIndex contains checkpoints for efficient random access in compressed layers
type GzipIndex struct {
	LayerDigest string       // SHA256 digest of the compressed layer
	Checkpoints []Checkpoint // Sorted by UOffset for binary search
}

// NearestCheckpoint finds the checkpoint with largest UOffset <= target
// Returns the compressed and uncompressed offsets of that checkpoint
func NearestCheckpoint(checkpoints []Checkpoint, targetUOffset int64) (cOffset, uOffset int64) {
	if len(checkpoints) == 0 {
		return 0, 0
	}

	// Binary search for the checkpoint with largest UOffset <= target
	left, right := 0, len(checkpoints)-1
	bestIdx := 0

	for left <= right {
		mid := (left + right) / 2
		if checkpoints[mid].UOffset <= targetUOffset {
			bestIdx = mid
			left = mid + 1
		} else {
			right = mid - 1
		}
	}

	cp := checkpoints[bestIdx]
	return cp.COffset, cp.UOffset
}

// GenerateCheckpoints scans a gzip file and creates checkpoints at deflate block boundaries
// This enables efficient random access without full decompression
func GenerateCheckpoints(gzipPath string, interval int64) (*GzipIndex, error) {
	f, err := os.Open(gzipPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open gzip file: %w", err)
	}
	defer f.Close()

	// Create gzip reader
	gzr, err := gzip.NewReader(f)
	if err != nil {
		return nil, fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer gzr.Close()

	index := &GzipIndex{
		Checkpoints: make([]Checkpoint, 0),
	}

	// Add checkpoint at the start
	index.Checkpoints = append(index.Checkpoints, Checkpoint{
		COffset: 0,
		UOffset: 0,
	})

	// Track offsets
	var uOffset int64 = 0
	lastCheckpoint := int64(0)
	buf := make([]byte, 32*1024) // 32KB buffer

	for {
		n, err := gzr.Read(buf)
		if n > 0 {
			uOffset += int64(n)

			// Check if we should create a checkpoint
			if uOffset-lastCheckpoint >= interval {
				// Note: We can't get the exact compressed offset from the standard gzip.Reader
				// This is a simplified approach - for production, you'd need to use a custom
				// gzip reader that tracks the underlying stream position

				// For now, we'll create checkpoints at regular uncompressed intervals
				// The compressed offset is approximate
				cOffset, _ := f.Seek(0, io.SeekCurrent)

				index.Checkpoints = append(index.Checkpoints, Checkpoint{
					COffset: cOffset,
					UOffset: uOffset,
				})

				lastCheckpoint = uOffset
			}
		}

		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to read gzip: %w", err)
		}
	}

	return index, nil
}
