package e2e

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/codecflow/fabric/mirage/index/dir"
	"github.com/codecflow/fabric/mirage/serve/fuse"
)

func TestBasicFlow(t *testing.T) {
	// Create temporary directories
	tmpDir := t.TempDir()
	sourceDir := filepath.Join(tmpDir, "source")
	indexDir := filepath.Join(tmpDir, "index")
	mountDir := filepath.Join(tmpDir, "mount")

	// Create source directory with test files
	if err := setupTestDirectory(sourceDir); err != nil {
		t.Fatalf("Failed to setup test directory: %v", err)
	}

	// Phase 1: Create index from directory
	t.Log("Phase 1: Creating index...")
	result, err := dir.Index(sourceDir, dir.IndexOptions{
		OutputDir: indexDir,
	})
	if err != nil {
		t.Fatalf("Failed to create index: %v", err)
	}

	t.Logf("Index created: %s", result.IndexPath)
	t.Logf("  Files: %d, Dirs: %d, Size: %d bytes", result.TotalFiles, result.TotalDirs, result.TotalSize)

	// Phase 2: Open the index
	t.Log("Phase 2: Opening index...")
	fs, err := dir.Open(result.IndexPath)
	if err != nil {
		t.Fatalf("Failed to open index: %v", err)
	}
	defer fs.Close()

	// Phase 3: Mount via FUSE
	t.Log("Phase 3: Mounting via FUSE...")
	if err := os.MkdirAll(mountDir, 0755); err != nil {
		t.Fatalf("Failed to create mount directory: %v", err)
	}

	server, err := fuse.Mount(fs, mountDir, fuse.MountOptions{
		ReadOnly: true,
	})
	if err != nil {
		t.Fatalf("Failed to mount: %v", err)
	}
	defer server.Unmount()

	// Give FUSE a moment to stabilize
	time.Sleep(100 * time.Millisecond)

	// Phase 4: Verify mounted files
	t.Log("Phase 4: Verifying mounted files...")

	// Check root directory
	entries, err := os.ReadDir(mountDir)
	if err != nil {
		t.Fatalf("Failed to read mount directory: %v", err)
	}
	t.Logf("Mount directory contains %d entries", len(entries))

	// Verify test.txt exists and has correct content
	testFile := filepath.Join(mountDir, "test.txt")
	content, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read test.txt: %v", err)
	}
	expectedContent := "Hello, Mirage!\n"
	if string(content) != expectedContent {
		t.Fatalf("Content mismatch: got %q, want %q", string(content), expectedContent)
	}
	t.Log("✓ test.txt content verified")

	// Verify subdirectory
	subdir := filepath.Join(mountDir, "subdir")
	if _, err := os.Stat(subdir); err != nil {
		t.Fatalf("Subdirectory not found: %v", err)
	}
	t.Log("✓ subdir/ exists")

	// Verify file in subdirectory
	nestedFile := filepath.Join(subdir, "nested.txt")
	nestedContent, err := os.ReadFile(nestedFile)
	if err != nil {
		t.Fatalf("Failed to read nested.txt: %v", err)
	}
	expectedNested := "Nested file content\n"
	if string(nestedContent) != expectedNested {
		t.Fatalf("Nested content mismatch: got %q, want %q", string(nestedContent), expectedNested)
	}
	t.Log("✓ subdir/nested.txt content verified")

	t.Log("✅ All tests passed!")
}

func setupTestDirectory(dir string) error {
	// Create directory structure
	if err := os.MkdirAll(filepath.Join(dir, "subdir"), 0755); err != nil {
		return err
	}

	// Create test files
	files := map[string]string{
		"test.txt":          "Hello, Mirage!\n",
		"subdir/nested.txt": "Nested file content\n",
		"empty.txt":         "",
		"data.bin":          "\x00\x01\x02\x03\x04\x05",
	}

	for path, content := range files {
		fullPath := filepath.Join(dir, path)
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			return err
		}
	}

	return nil
}
