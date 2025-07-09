package storage

import (
	"context"
	"io"
	"time"
)

// Storage defines the interface for P2P content storage
type Storage interface {
	// Content operations
	Put(ctx context.Context, reader io.Reader, metadata ContentMetadata) (*ContentInfo, error)
	Get(ctx context.Context, cid string, writer io.Writer) error
	GetReader(ctx context.Context, cid string) (io.ReadCloser, error)
	Delete(ctx context.Context, cid string) error
	Exists(ctx context.Context, cid string) (bool, error)

	// Content information
	GetInfo(ctx context.Context, cid string) (*ContentInfo, error)
	List(ctx context.Context, filters ContentFilter) ([]*ContentInfo, error)

	// Snapshots
	CreateSnapshot(ctx context.Context, workloadID string, data io.Reader) (*SnapshotInfo, error)
	RestoreSnapshot(ctx context.Context, snapshotID string, writer io.Writer) error
	ListSnapshots(ctx context.Context, workloadID string) ([]*SnapshotInfo, error)
	DeleteSnapshot(ctx context.Context, snapshotID string) error

	// Health and maintenance
	HealthCheck(ctx context.Context) error
	Close() error
}

// ContentMetadata provides metadata for stored content
type ContentMetadata struct {
	Name        string            `json:"name,omitempty"`
	Description string            `json:"description,omitempty"`
	ContentType string            `json:"contentType,omitempty"`
	Tags        []string          `json:"tags,omitempty"`
	Properties  map[string]string `json:"properties,omitempty"`
	Namespace   string            `json:"namespace,omitempty"`
	WorkloadID  string            `json:"workloadId,omitempty"`
}

// ContentInfo represents information about stored content
type ContentInfo struct {
	CID         string          `json:"cid"`
	Size        int64           `json:"size"`
	ContentType string          `json:"contentType,omitempty"`
	Hash        string          `json:"hash"`
	CreatedAt   time.Time       `json:"createdAt"`
	AccessedAt  time.Time       `json:"accessedAt"`
	Metadata    ContentMetadata `json:"metadata"`
	Replicas    []ReplicaInfo   `json:"replicas,omitempty"`
	Status      ContentStatus   `json:"status"`
}

// ContentStatus defines the status of content
type ContentStatus string

const (
	ContentStatusPending     ContentStatus = "pending"     // Being uploaded
	ContentStatusAvailable   ContentStatus = "available"   // Available for download
	ContentStatusReplicating ContentStatus = "replicating" // Being replicated
	ContentStatusCorrupted   ContentStatus = "corrupted"   // Data corruption detected
	ContentStatusDeleted     ContentStatus = "deleted"     // Marked for deletion
)

// ReplicaInfo represents information about a content replica
type ReplicaInfo struct {
	NodeID    string        `json:"nodeId"`
	Location  string        `json:"location,omitempty"` // Geographic location
	CreatedAt time.Time     `json:"createdAt"`
	Verified  bool          `json:"verified"`
	Health    ReplicaHealth `json:"health"`
}

// ReplicaHealth defines the health status of a replica
type ReplicaHealth string

const (
	ReplicaHealthHealthy   ReplicaHealth = "healthy"
	ReplicaHealthDegraded  ReplicaHealth = "degraded"
	ReplicaHealthCorrupted ReplicaHealth = "corrupted"
	ReplicaHealthUnknown   ReplicaHealth = "unknown"
)

// ContentFilter defines filters for content queries
type ContentFilter struct {
	Namespace     string            `json:"namespace,omitempty"`
	WorkloadID    string            `json:"workloadId,omitempty"`
	ContentType   string            `json:"contentType,omitempty"`
	Tags          []string          `json:"tags,omitempty"`
	Status        ContentStatus     `json:"status,omitempty"`
	CreatedAfter  *time.Time        `json:"createdAfter,omitempty"`
	CreatedBefore *time.Time        `json:"createdBefore,omitempty"`
	MinSize       *int64            `json:"minSize,omitempty"`
	MaxSize       *int64            `json:"maxSize,omitempty"`
	Properties    map[string]string `json:"properties,omitempty"`
	Limit         int               `json:"limit,omitempty"`
	Offset        int               `json:"offset,omitempty"`
}

// SnapshotInfo represents information about a workload snapshot
type SnapshotInfo struct {
	ID          string            `json:"id"`
	WorkloadID  string            `json:"workloadId"`
	Namespace   string            `json:"namespace"`
	CID         string            `json:"cid"`
	Size        int64             `json:"size"`
	Type        SnapshotType      `json:"type"`
	CreatedAt   time.Time         `json:"createdAt"`
	Description string            `json:"description,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	Status      SnapshotStatus    `json:"status"`
	Parent      string            `json:"parent,omitempty"` // Parent snapshot ID for incremental
}

// SnapshotType defines the type of snapshot
type SnapshotType string

const (
	SnapshotTypeFull        SnapshotType = "full"        // Complete snapshot
	SnapshotTypeIncremental SnapshotType = "incremental" // Incremental snapshot
	SnapshotTypeMemory      SnapshotType = "memory"      // Memory-only snapshot
	SnapshotTypeFilesystem  SnapshotType = "filesystem"  // Filesystem-only snapshot
)

// SnapshotStatus defines the status of a snapshot
type SnapshotStatus string

const (
	SnapshotStatusCreating  SnapshotStatus = "creating"
	SnapshotStatusReady     SnapshotStatus = "ready"
	SnapshotStatusRestoring SnapshotStatus = "restoring"
	SnapshotStatusFailed    SnapshotStatus = "failed"
	SnapshotStatusDeleted   SnapshotStatus = "deleted"
)

// StorageConfig defines configuration for the storage system
type StorageConfig struct {
	Provider          string            `json:"provider"` // "iroh", "ipfs", "local"
	Endpoint          string            `json:"endpoint,omitempty"`
	DataDir           string            `json:"dataDir,omitempty"`
	MaxSize           int64             `json:"maxSize,omitempty"`           // Max storage size in bytes
	ReplicationFactor int               `json:"replicationFactor,omitempty"` // Desired number of replicas
	Encryption        EncryptionConfig  `json:"encryption,omitempty"`
	Compression       CompressionConfig `json:"compression,omitempty"`
	Properties        map[string]string `json:"properties,omitempty"`
}

// EncryptionConfig defines encryption settings
type EncryptionConfig struct {
	Enabled   bool   `json:"enabled"`
	Algorithm string `json:"algorithm,omitempty"` // "AES-256-GCM", "ChaCha20-Poly1305"
	KeySource string `json:"keySource,omitempty"` // "local", "vault", "kms"
}

// CompressionConfig defines compression settings
type CompressionConfig struct {
	Enabled   bool   `json:"enabled"`
	Algorithm string `json:"algorithm,omitempty"` // "gzip", "lz4", "zstd"
	Level     int    `json:"level,omitempty"`     // Compression level
}

// TransferProgress represents progress of a content transfer
type TransferProgress struct {
	CID              string         `json:"cid"`
	BytesTotal       int64          `json:"bytesTotal"`
	BytesTransferred int64          `json:"bytesTransferred"`
	Speed            int64          `json:"speed"` // Bytes per second
	ETA              time.Duration  `json:"eta"`   // Estimated time to completion
	Status           TransferStatus `json:"status"`
	Error            string         `json:"error,omitempty"`
}

// TransferStatus defines the status of a content transfer
type TransferStatus string

const (
	TransferStatusPending   TransferStatus = "pending"
	TransferStatusActive    TransferStatus = "active"
	TransferStatusCompleted TransferStatus = "completed"
	TransferStatusFailed    TransferStatus = "failed"
	TransferStatusCanceled  TransferStatus = "canceled"
)

// PinInfo represents information about pinned content
type PinInfo struct {
	CID       string     `json:"cid"`
	PinnedAt  time.Time  `json:"pinnedAt"`
	PinnedBy  string     `json:"pinnedBy"` // Node ID or user
	Reason    string     `json:"reason,omitempty"`
	Priority  int        `json:"priority,omitempty"` // Higher = more important
	ExpiresAt *time.Time `json:"expiresAt,omitempty"`
}

// GarbageCollectionInfo represents garbage collection statistics
type GarbageCollectionInfo struct {
	LastRun      time.Time     `json:"lastRun"`
	NextRun      time.Time     `json:"nextRun"`
	ItemsDeleted int           `json:"itemsDeleted"`
	BytesFreed   int64         `json:"bytesFreed"`
	Duration     time.Duration `json:"duration"`
	Status       string        `json:"status"`
}

// StorageStats represents storage system statistics
type StorageStats struct {
	TotalSize     int64                  `json:"totalSize"`
	UsedSize      int64                  `json:"usedSize"`
	AvailableSize int64                  `json:"availableSize"`
	ContentCount  int                    `json:"contentCount"`
	SnapshotCount int                    `json:"snapshotCount"`
	ReplicaCount  int                    `json:"replicaCount"`
	LastGC        *GarbageCollectionInfo `json:"lastGc,omitempty"`
}
