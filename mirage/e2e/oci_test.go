package e2e

import (
	"strings"
	"testing"

	"github.com/codecflow/fabric/mirage"
	"github.com/codecflow/fabric/mirage/index/oci"
)

// TestOCIIndexing tests the complete OCI indexing flow without FUSE
func TestOCIIndexing(t *testing.T) {
	// Skip if running in CI without network access
	if testing.Short() {
		t.Skip("Skipping OCI test in short mode")
	}

	tmpDir := t.TempDir()

	// Phase 1: Index alpine:latest (small, public image ~3MB)
	t.Log("Phase 1: Indexing alpine:latest from registry...")
	result, err := oci.Index("alpine:latest", oci.IndexOptions{
		OutputDir:      tmpDir,
		UseCheckpoints: false, // Start with decompressed mode (simpler)
	})
	if err != nil {
		t.Fatalf("Failed to index alpine:latest: %v", err)
	}

	t.Logf("✓ Index created: %s", result.IndexPath)
	t.Logf("  Image: %s", result.ImageRef)
	t.Logf("  Digest: %s", result.Digest)
	t.Logf("  Layers: %d", result.LayerCount)
	t.Logf("  Files: %d, Dirs: %d, Size: %d bytes", result.TotalFiles, result.TotalDirs, result.TotalSize)

	if result.LayerCount == 0 {
		t.Fatal("Expected at least one layer")
	}

	// Phase 2: Open the index
	t.Log("\nPhase 2: Opening index...")
	fs, err := oci.Open(result.IndexPath)
	if err != nil {
		t.Fatalf("Failed to open index: %v", err)
	}
	defer fs.Close()
	t.Log("✓ Index opened successfully")

	// Phase 3: Test FileSystem operations
	t.Log("\nPhase 3: Testing FileSystem operations...")

	// Test root node
	t.Log("Testing Node() for root...")
	root, err := fs.Node("/")
	if err != nil {
		t.Fatalf("Failed to get root node: %v", err)
	}
	if root.Type != mirage.NodeTypeDir {
		t.Errorf("Root should be directory, got %v", root.Type)
	}
	t.Logf("✓ Root node: path=%s, type=%s", root.Path, root.Type)

	// Test listing root directory
	t.Log("Testing List() for root...")
	rootEntries, err := fs.List("/")
	if err != nil {
		t.Fatalf("Failed to list root: %v", err)
	}
	t.Logf("✓ Root contains %d entries:", len(rootEntries))
	for _, entry := range rootEntries {
		t.Logf("  - %s (%s)", entry.Name, entry.Type)
	}

	// Common directories in alpine
	expectedDirs := []string{"/etc", "/bin", "/lib"}
	for _, dir := range expectedDirs {
		if _, err := fs.Node(dir); err != nil {
			t.Errorf("Expected directory not found: %s", dir)
		} else {
			t.Logf("✓ Found expected directory: %s", dir)
		}
	}

	// Test reading alpine-release file
	t.Log("Testing Node() for /etc/alpine-release...")
	releaseNode, err := fs.Node("/etc/alpine-release")
	if err != nil {
		t.Logf("Note: /etc/alpine-release not found (may not exist in this version)")
	} else {
		if releaseNode.Type != mirage.NodeTypeFile {
			t.Errorf("alpine-release should be file, got %v", releaseNode.Type)
		}
		t.Logf("✓ alpine-release node: size=%d, blob=%s, offset=%d",
			releaseNode.Size, releaseNode.Blob, releaseNode.Offset)

		// Read content
		t.Log("Testing Read() for /etc/alpine-release...")
		buf := make([]byte, releaseNode.Size)
		n, err := fs.Read(releaseNode, buf, 0)
		if err != nil {
			t.Fatalf("Failed to read alpine-release: %v", err)
		}
		content := string(buf[:n])
		t.Logf("✓ Read %d bytes from alpine-release: %q", n, content)

		// Alpine release file just contains version number
		if len(content) == 0 {
			t.Errorf("Expected content in alpine-release, got empty")
		}
	}

	// Test /etc/os-release as alternative (may be symlink)
	t.Log("Testing /etc/os-release...")
	osRelNode, err := fs.Node("/etc/os-release")
	if err != nil {
		t.Logf("Note: /etc/os-release not found")
	} else if osRelNode.Type == mirage.NodeTypeSymlink {
		t.Logf("✓ /etc/os-release is a symlink to: %s", osRelNode.Target)
	} else if osRelNode.Type == mirage.NodeTypeFile {
		buf := make([]byte, osRelNode.Size)
		n, err := fs.Read(osRelNode, buf, 0)
		if err != nil {
			t.Fatalf("Failed to read os-release: %v", err)
		}
		content := string(buf[:n])
		t.Logf("✓ Read %d bytes from os-release", n)
		t.Logf("  Content preview: %s", strings.Split(content, "\n")[0])
	}

	// Test listing /etc
	t.Log("Testing List() for /etc...")
	etcEntries, err := fs.List("/etc")
	if err != nil {
		t.Fatalf("Failed to list /etc: %v", err)
	}
	t.Logf("✓ /etc contains %d entries", len(etcEntries))
	if len(etcEntries) > 5 {
		t.Log("  Sample entries:")
		for i := 0; i < 5; i++ {
			t.Logf("    - %s (%s)", etcEntries[i].Name, etcEntries[i].Type)
		}
	}

	// Test a binary file (e.g., /bin/sh)
	t.Log("Testing binary file read (/bin/sh)...")
	shNode, err := fs.Node("/bin/sh")
	if err != nil {
		t.Logf("Note: /bin/sh not found")
	} else if shNode.Type == mirage.NodeTypeSymlink {
		t.Logf("✓ /bin/sh is a symlink to: %s", shNode.Target)
	} else if shNode.Type == mirage.NodeTypeFile {
		// Read first 4 bytes (ELF magic)
		buf := make([]byte, 4)
		n, err := fs.Read(shNode, buf, 0)
		if err != nil {
			t.Fatalf("Failed to read /bin/sh: %v", err)
		}
		t.Logf("✓ Read %d bytes from /bin/sh", n)
		if n == 4 && buf[0] == 0x7f && buf[1] == 'E' && buf[2] == 'L' && buf[3] == 'F' {
			t.Log("  ✓ ELF magic bytes verified")
		}
	}

	// Test partial read
	if releaseNode != nil && releaseNode.Size > 10 {
		t.Log("Testing partial read...")
		partialBuf := make([]byte, 5)
		n, err := fs.Read(releaseNode, partialBuf, 2) // Read 5 bytes from offset 2
		if err != nil {
			t.Fatalf("Failed to do partial read: %v", err)
		}
		t.Logf("✓ Partial read at offset 2: %d bytes: %q", n, string(partialBuf[:n]))
	}

	t.Log("\n✅ All OCI filesystem operations verified successfully!")
}

// TestOCIWithCheckpoints tests OCI indexing with checkpoint generation
func TestOCIWithCheckpoints(t *testing.T) {
	// Skip if running in CI without network access
	if testing.Short() {
		t.Skip("Skipping OCI checkpoint test in short mode")
	}

	tmpDir := t.TempDir()

	t.Log("Indexing alpine:latest with checkpoint generation...")
	result, err := oci.Index("alpine:latest", oci.IndexOptions{
		OutputDir:          tmpDir,
		UseCheckpoints:     true,
		CheckpointInterval: 512 * 1024, // 512KB intervals for small image
	})
	if err != nil {
		t.Fatalf("Failed to index with checkpoints: %v", err)
	}

	t.Logf("✓ Index created with checkpoints")
	t.Logf("  Checkpointed: %v", result.Checkpointed)

	// Open and test
	fs, err := oci.Open(result.IndexPath)
	if err != nil {
		t.Fatalf("Failed to open index: %v", err)
	}
	defer fs.Close()

	// Verify we can read from compressed layers
	t.Log("Testing reads from compressed layers...")
	node, err := fs.Node("/etc/os-release")
	if err != nil {
		t.Skip("os-release not found, skipping compressed read test")
	}

	buf := make([]byte, node.Size)
	n, err := fs.Read(node, buf, 0)
	if err != nil {
		t.Fatalf("Failed to read from compressed layer: %v", err)
	}

	t.Logf("✓ Successfully read %d bytes from compressed layer with checkpoints", n)
	t.Log("\n✅ Checkpoint-based reading verified!")
}
