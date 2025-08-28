package stash

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"path/filepath"
)

// GenerateFsID generates a unique filesystem node ID from a path.
func GenerateFsID(path string) string {
	cleanPath := filepath.Clean(path)
	hash := sha1.Sum([]byte(cleanPath))
	return hex.EncodeToString(hash[:])
}

// SHA1StringToUint64 converts a SHA1 hex string to uint64 (for inode generation).
func SHA1StringToUint64(hashStr string) (uint64, error) {
	if len(hashStr) < 16 {
		return 0, fmt.Errorf("hash string too short")
	}

	// Use first 16 hex chars (8 bytes) for uint64
	var result uint64
	for i := 0; i < 16; i++ {
		result <<= 4
		c := hashStr[i]
		if c >= '0' && c <= '9' {
			result |= uint64(c - '0')
		} else if c >= 'a' && c <= 'f' {
			result |= uint64(c - 'a' + 10)
		} else if c >= 'A' && c <= 'F' {
			result |= uint64(c - 'A' + 10)
		}
	}

	return result, nil
}
