package analyze

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"

	"github.com/dundee/gdu/v5/pkg/fs"
	"github.com/stretchr/testify/assert"
)

func TestIsZipFile(t *testing.T) {
	tests := []struct {
		filename string
		expected bool
	}{
		{"test.zip", true},
		{"test.jar", true},
		{"TEST.ZIP", true},
		{"TEST.JAR", true},
		{"test.txt", false},
		{"test.tar.gz", false},
		{"test", false},
		{"", false},
	}

	for _, test := range tests {
		result := isZipFile(test.filename)
		assert.Equal(t, test.expected, result, "filename: %s", test.filename)
	}
}

func TestProcessZipFile(t *testing.T) {
	// Create temporary zip file
	tempDir := t.TempDir()
	zipPath := filepath.Join(tempDir, "test.zip")

	// Create zip file
	createTestZipFile(t, zipPath)

	// Get file info
	info, err := os.Stat(zipPath)
	assert.NoError(t, err)

	// Process zip file
	zipDir, err := processZipFile(zipPath, info)
	assert.NoError(t, err)
	assert.NotNil(t, zipDir)

	// Verify zip directory properties
	assert.Equal(t, "test.zip", zipDir.GetName())
	assert.Equal(t, rune('Z'), zipDir.GetFlag())
	assert.True(t, zipDir.IsDir())
	assert.Equal(t, "ZipDirectory", zipDir.GetType())

	// Verify file structure
	files := zipDir.GetFiles()
	assert.Greater(t, len(files), 0)

	// Debug: print all files
	t.Logf("Found %d files in zip:", len(files))
	for _, file := range files {
		t.Logf("  - %s (isDir: %t, type: %s)", file.GetName(), file.IsDir(), file.GetType())
	}

	// Find files
	foundTextFile := false
	foundSubdir := false

	for _, file := range files {
		if file.GetName() == "test.txt" {
			foundTextFile = true
			assert.False(t, file.IsDir())
			assert.Equal(t, "ZipFile", file.GetType())
		}
		if file.GetName() == "subdir" {
			foundSubdir = true
			assert.True(t, file.IsDir())
			assert.Equal(t, "ZipDirectory", file.GetType())
		}
	}

	assert.True(t, foundTextFile, "should find test.txt file")
	assert.True(t, foundSubdir, "should find subdir directory")
}

func TestGetZipFileSize(t *testing.T) {
	// Create temporary zip file
	tempDir := t.TempDir()
	zipPath := filepath.Join(tempDir, "test.zip")

	// Create zip file
	createTestZipFile(t, zipPath)

	// Get size
	uncompressed, compressed, err := getZipFileSize(zipPath)
	assert.NoError(t, err)
	assert.Greater(t, uncompressed, int64(0))
	assert.Greater(t, compressed, int64(0))
	// Note: for small files, compressed size might be larger
	t.Logf("Uncompressed size: %d, Compressed size: %d", uncompressed, compressed)
}

func TestEnsureZipDirExists(t *testing.T) {
	tempDir := t.TempDir()
	zipPath := filepath.Join(tempDir, "test.zip")

	// Create root directory
	rootDir := &ZipDir{
		Dir: &Dir{
			File: &File{
				Name: "test.zip",
				Flag: 'Z',
			},
			Files: make(fs.Files, 0),
		},
		zipPath: zipPath,
	}

	dirMap := make(map[string]*ZipDir)
	dirMap[""] = rootDir

	// Ensure nested directory structure is created
	ensureZipDirExists(dirMap, "dir1/dir2/dir3", zipPath, rootDir)

	// Verify directory structure
	assert.Contains(t, dirMap, "dir1")
	assert.Contains(t, dirMap, "dir1/dir2")
	assert.Contains(t, dirMap, "dir1/dir2/dir3")

	// Verify parent-child relationships
	dir1 := dirMap["dir1"]
	assert.Equal(t, rootDir, dir1.GetParent())

	dir2 := dirMap["dir1/dir2"]
	assert.Equal(t, dir1, dir2.GetParent())

	dir3 := dirMap["dir1/dir2/dir3"]
	assert.Equal(t, dir2, dir3.GetParent())
}

// createTestZipFile creates a test zip file
func createTestZipFile(t *testing.T, zipPath string) {
	file, err := os.Create(zipPath)
	assert.NoError(t, err)
	defer file.Close()

	zipWriter := zip.NewWriter(file)
	defer zipWriter.Close()

	// Add root directory file
	writer, err := zipWriter.Create("test.txt")
	assert.NoError(t, err)
	_, err = writer.Write([]byte("Hello, this is a test file!"))
	assert.NoError(t, err)

	// Add subdirectory files
	// We don't need to use the writer for the directory entry, avoid SA4006
	_, err = zipWriter.Create("subdir/")
	assert.NoError(t, err)

	writer, err = zipWriter.Create("subdir/nested.txt")
	assert.NoError(t, err)
	_, err = writer.Write([]byte("This is a nested file."))
	assert.NoError(t, err)

	// Add deeper directory structure
	writer, err = zipWriter.Create("dir1/dir2/deep.txt")
	assert.NoError(t, err)
	_, err = writer.Write([]byte("Deep nested file content."))
	assert.NoError(t, err)
}
