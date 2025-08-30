package e2e

import (
	"path/filepath"
	"testing"

	"github.com/codecflow/fabric/mirage"
	"github.com/codecflow/fabric/mirage/index/dir"
)

// TestFileSystemOperations tests index creation and FileSystem operations without FUSE
func TestFileSystemOperations(t *testing.T) {
	// Create temporary directories
	tmpDir := t.TempDir()
	sourceDir := filepath.Join(tmpDir, "source")
	indexDir := filepath.Join(tmpDir, "index")

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

	t.Logf("✓ Index created: %s", result.IndexPath)
	t.Logf("  Files: %d, Dirs: %d, Size: %d bytes", result.TotalFiles, result.TotalDirs, result.TotalSize)

	// Verify stats
	if result.TotalFiles != 4 {
		t.Errorf("Expected 4 files, got %d", result.TotalFiles)
	}
	if result.TotalDirs != 2 {
		t.Errorf("Expected 2 directories (including root), got %d", result.TotalDirs)
	}

	// Phase 2: Open the index
	t.Log("\nPhase 2: Opening index...")
	fs, err := dir.Open(result.IndexPath)
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
	if len(rootEntries) != 4 {
		t.Errorf("Expected 4 entries in root, got %d", len(rootEntries))
	}

	// Test file node
	t.Log("Testing Node() for test.txt...")
	testNode, err := fs.Node("/test.txt")
	if err != nil {
		t.Fatalf("Failed to get test.txt node: %v", err)
	}
	if testNode.Type != mirage.NodeTypeFile {
		t.Errorf("test.txt should be file, got %v", testNode.Type)
	}
	t.Logf("✓ test.txt node: size=%d, blob=%s, offset=%d", testNode.Size, testNode.Blob, testNode.Offset)

	// Test reading file content
	t.Log("Testing Read() for test.txt...")
	buf := make([]byte, testNode.Size)
	n, err := fs.Read(testNode, buf, 0)
	if err != nil {
		t.Fatalf("Failed to read test.txt: %v", err)
	}
	content := string(buf[:n])
	expectedContent := "Hello, Mirage!\n"
	if content != expectedContent {
		t.Errorf("Content mismatch: got %q, want %q", content, expectedContent)
	}
	t.Logf("✓ Read %d bytes from test.txt: %q", n, content)

	// Test subdirectory
	t.Log("Testing Node() for subdir/...")
	subdirNode, err := fs.Node("/subdir")
	if err != nil {
		t.Fatalf("Failed to get subdir node: %v", err)
	}
	if subdirNode.Type != mirage.NodeTypeDir {
		t.Errorf("subdir should be directory, got %v", subdirNode.Type)
	}
	t.Log("✓ subdir node retrieved")

	// Test listing subdirectory
	t.Log("Testing List() for subdir/...")
	subdirEntries, err := fs.List("/subdir")
	if err != nil {
		t.Fatalf("Failed to list subdir: %v", err)
	}
	t.Logf("✓ subdir contains %d entries:", len(subdirEntries))
	for _, entry := range subdirEntries {
		t.Logf("  - %s (%s)", entry.Name, entry.Type)
	}

	// Test nested file
	t.Log("Testing Node() for subdir/nested.txt...")
	nestedNode, err := fs.Node("/subdir/nested.txt")
	if err != nil {
		t.Fatalf("Failed to get nested.txt node: %v", err)
	}
	t.Logf("✓ nested.txt node: size=%d", nestedNode.Size)

	// Test reading nested file
	t.Log("Testing Read() for subdir/nested.txt...")
	nestedBuf := make([]byte, nestedNode.Size)
	n, err = fs.Read(nestedNode, nestedBuf, 0)
	if err != nil {
		t.Fatalf("Failed to read nested.txt: %v", err)
	}
	nestedContent := string(nestedBuf[:n])
	expectedNested := "Nested file content\n"
	if nestedContent != expectedNested {
		t.Errorf("Nested content mismatch: got %q, want %q", nestedContent, expectedNested)
	}
	t.Logf("✓ Read %d bytes from nested.txt: %q", n, nestedContent)

	// Test empty file
	t.Log("Testing empty file...")
	emptyNode, err := fs.Node("/empty.txt")
	if err != nil {
		t.Fatalf("Failed to get empty.txt node: %v", err)
	}
	if emptyNode.Size != 0 {
		t.Errorf("empty.txt should have size 0, got %d", emptyNode.Size)
	}
	t.Log("✓ empty.txt has size 0")

	// Test binary file
	t.Log("Testing binary file...")
	binNode, err := fs.Node("/data.bin")
	if err != nil {
		t.Fatalf("Failed to get data.bin node: %v", err)
	}
	binBuf := make([]byte, binNode.Size)
	n, err = fs.Read(binNode, binBuf, 0)
	if err != nil {
		t.Fatalf("Failed to read data.bin: %v", err)
	}
	expectedBinary := []byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05}
	if string(binBuf[:n]) != string(expectedBinary) {
		t.Errorf("Binary content mismatch")
	}
	t.Logf("✓ Read %d binary bytes from data.bin", n)

	// Test partial read
	t.Log("Testing partial read...")
	partialBuf := make([]byte, 5)
	n, err = fs.Read(testNode, partialBuf, 7) // Read from offset 7
	if err != nil {
		t.Fatalf("Failed to do partial read: %v", err)
	}
	partialContent := string(partialBuf[:n])
	expected := "Mirag" // "Hello, Mirage!\n" starting at offset 7
	if partialContent != expected {
		t.Errorf("Partial read mismatch: got %q, want %q", partialContent, expected)
	}
	t.Logf("✓ Partial read at offset 7: %q", partialContent)

	t.Log("\n✅ All FileSystem operations verified successfully!")
}
