package snapshots

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/codecflow/fabric/weaver/internal/config"
	"github.com/codecflow/fabric/weaver/internal/workload"
)

// CRIUManager manages CRIU snapshots
type CRIUManager struct {
	config *config.CRIUConfig
}

// Snapshot represents a CRIU snapshot
type Snapshot struct {
	ID          string    `json:"id"`
	WorkloadID  string    `json:"workload_id"`
	Path        string    `json:"path"`
	Size        int64     `json:"size"`
	CreatedAt   time.Time `json:"created_at"`
	Description string    `json:"description"`
	IrohCID     string    `json:"iroh_cid,omitempty"`
}

// New creates a new CRIU manager
func New(cfg *config.CRIUConfig) (*CRIUManager, error) {
	if !cfg.Enabled {
		return &CRIUManager{config: cfg}, nil
	}

	// Check if CRIU is available
	if err := checkCRIUAvailable(); err != nil {
		return nil, fmt.Errorf("CRIU not available: %w", err)
	}

	// Ensure snapshot directory exists
	if err := os.MkdirAll(cfg.SnapshotDir, 0o750); err != nil {
		return nil, fmt.Errorf("failed to create snapshot directory: %w", err)
	}

	return &CRIUManager{
		config: cfg,
	}, nil
}

// CreateSnapshot creates a snapshot of a running workload
func (c *CRIUManager) CreateSnapshot(ctx context.Context, w *workload.Workload, containerID string, description string) (*Snapshot, error) {
	if !c.config.Enabled {
		return nil, fmt.Errorf("CRIU snapshots are disabled")
	}

	snapshotID := fmt.Sprintf("%s-%d", w.ID, time.Now().Unix())
	snapshotPath := filepath.Join(c.config.SnapshotDir, snapshotID)

	log.Printf("Creating CRIU snapshot for workload %s (container %s)", w.ID, containerID)

	// Create snapshot directory
	if err := os.MkdirAll(snapshotPath, 0o750); err != nil {
		return nil, fmt.Errorf("failed to create snapshot directory: %w", err)
	}

	// Get container PID
	pid, err := c.getContainerPID(containerID)
	if err != nil {
		return nil, fmt.Errorf("failed to get container PID: %w", err)
	}

	// Create CRIU dump
	if err := c.createDump(ctx, pid, snapshotPath); err != nil {
		_ = os.RemoveAll(snapshotPath)
		return nil, fmt.Errorf("failed to create CRIU dump: %w", err)
	}

	// Calculate snapshot size
	size, err := c.calculateSnapshotSize(snapshotPath)
	if err != nil {
		log.Printf("Warning: failed to calculate snapshot size: %v", err)
	}

	snapshot := &Snapshot{
		ID:          snapshotID,
		WorkloadID:  w.ID,
		Path:        snapshotPath,
		Size:        size,
		CreatedAt:   time.Now(),
		Description: description,
	}

	// Upload to Iroh if enabled
	if c.config.IrohEnabled {
		cid, err := c.uploadToIroh(ctx, snapshotPath)
		if err != nil {
			log.Printf("Warning: failed to upload snapshot to Iroh: %v", err)
		} else {
			snapshot.IrohCID = cid
		}
	}

	log.Printf("Created snapshot %s for workload %s (size: %d bytes)", snapshotID, w.ID, size)
	return snapshot, nil
}

// RestoreSnapshot restores a workload from a snapshot
func (c *CRIUManager) RestoreSnapshot(ctx context.Context, snapshot *Snapshot, containerID string) error {
	if !c.config.Enabled {
		return fmt.Errorf("CRIU snapshots are disabled")
	}

	log.Printf("Restoring snapshot %s for container %s", snapshot.ID, containerID)

	snapshotPath := snapshot.Path

	// Download from Iroh if needed
	if snapshot.IrohCID != "" && !c.snapshotExists(snapshotPath) {
		if err := c.downloadFromIroh(ctx, snapshot.IrohCID, snapshotPath); err != nil {
			return fmt.Errorf("failed to download snapshot from Iroh: %w", err)
		}
	}

	// Verify snapshot exists
	if !c.snapshotExists(snapshotPath) {
		return fmt.Errorf("snapshot not found: %s", snapshotPath)
	}

	// Restore from CRIU dump
	if err := c.restoreDump(ctx, snapshotPath, containerID); err != nil {
		return fmt.Errorf("failed to restore CRIU dump: %w", err)
	}

	log.Printf("Successfully restored snapshot %s", snapshot.ID)
	return nil
}

// DeleteSnapshot deletes a snapshot
func (c *CRIUManager) DeleteSnapshot(ctx context.Context, snapshot *Snapshot) error {
	log.Printf("Deleting snapshot %s", snapshot.ID)

	// Remove local files
	if err := os.RemoveAll(snapshot.Path); err != nil {
		return fmt.Errorf("failed to remove snapshot files: %w", err)
	}

	// Remove from Iroh if needed
	if snapshot.IrohCID != "" && c.config.IrohEnabled {
		if err := c.removeFromIroh(ctx, snapshot.IrohCID); err != nil {
			log.Printf("Warning: failed to remove snapshot from Iroh: %v", err)
		}
	}

	log.Printf("Deleted snapshot %s", snapshot.ID)
	return nil
}

// ListSnapshots lists all snapshots for a workload
func (c *CRIUManager) ListSnapshots(workloadID string) ([]*Snapshot, error) {
	if !c.config.Enabled {
		return []*Snapshot{}, nil
	}

	snapshots := []*Snapshot{}

	entries, err := os.ReadDir(c.config.SnapshotDir)
	if err != nil {
		return snapshots, nil // Return empty list if directory doesn't exist
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		// Check if snapshot belongs to workload
		if workloadID != "" && !c.snapshotBelongsToWorkload(entry.Name(), workloadID) {
			continue
		}

		snapshotPath := filepath.Join(c.config.SnapshotDir, entry.Name())
		size, _ := c.calculateSnapshotSize(snapshotPath)

		snapshot := &Snapshot{
			ID:         entry.Name(),
			WorkloadID: workloadID,
			Path:       snapshotPath,
			Size:       size,
			CreatedAt:  time.Now(), // In a real implementation, parse from filename or metadata
		}

		snapshots = append(snapshots, snapshot)
	}

	return snapshots, nil
}

// checkCRIUAvailable checks if CRIU is available on the system
func checkCRIUAvailable() error {
	cmd := exec.Command("criu", "check")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("CRIU check failed: %w", err)
	}
	return nil
}

// getContainerPID gets the PID of a container
func (c *CRIUManager) getContainerPID(_ string) (int, error) {
	// In a real implementation, this would query containerd for the container PID
	// For now, return a placeholder
	return 1234, nil
}

// createDump creates a CRIU dump
func (c *CRIUManager) createDump(ctx context.Context, pid int, snapshotPath string) error {
	args := []string{
		"dump",
		"-t", fmt.Sprintf("%d", pid),
		"-D", snapshotPath,
		"--shell-job",
		"--leave-running",
	}

	if c.config.TCPEstablished {
		args = append(args, "--tcp-established")
	}

	if c.config.FileLocks {
		args = append(args, "--file-locks")
	}

	cmd := exec.CommandContext(ctx, "criu", args...) // nolint:gosec
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

// restoreDump restores from a CRIU dump
func (c *CRIUManager) restoreDump(ctx context.Context, snapshotPath, _ string) error {
	args := []string{
		"restore",
		"-D", snapshotPath,
		"--shell-job",
	}

	cmd := exec.CommandContext(ctx, "criu", args...) // nolint:gosec
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

// calculateSnapshotSize calculates the total size of a snapshot
func (c *CRIUManager) calculateSnapshotSize(snapshotPath string) (int64, error) {
	var size int64

	err := filepath.Walk(snapshotPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			size += info.Size()
		}
		return nil
	})

	return size, err
}

// snapshotExists checks if a snapshot exists locally
func (c *CRIUManager) snapshotExists(snapshotPath string) bool {
	_, err := os.Stat(snapshotPath)
	return err == nil
}

// snapshotBelongsToWorkload checks if a snapshot belongs to a workload
func (c *CRIUManager) snapshotBelongsToWorkload(snapshotName, workloadID string) bool {
	// Simple check - in a real implementation, this would be more sophisticated
	return len(snapshotName) > len(workloadID) && snapshotName[:len(workloadID)] == workloadID
}

// uploadToIroh uploads a snapshot to Iroh
func (c *CRIUManager) uploadToIroh(ctx context.Context, snapshotPath string) (string, error) {
	// In a real implementation, this would use the Iroh API
	// For now, return a placeholder CID
	log.Printf("Uploading snapshot to Iroh: %s", snapshotPath)
	return "bafkreiabcdef1234567890", nil
}

// downloadFromIroh downloads a snapshot from Iroh
func (c *CRIUManager) downloadFromIroh(_ context.Context, cid, snapshotPath string) error {
	// In a real implementation, this would use the Iroh API
	log.Printf("Downloading snapshot from Iroh: %s -> %s", cid, snapshotPath)
	return os.MkdirAll(snapshotPath, 0o750)
}

// removeFromIroh removes a snapshot from Iroh
func (c *CRIUManager) removeFromIroh(_ context.Context, cid string) error {
	// In a real implementation, this would use the Iroh API
	log.Printf("Removing snapshot from Iroh: %s", cid)
	return nil
}
