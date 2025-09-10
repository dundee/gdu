package analyze

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestZipFileGetPath(t *testing.T) {
	zipFile := &ZipFile{
		zipPath:   "/path/to/archive.zip",
		inZipPath: "folder/file.txt",
	}

	path := zipFile.GetPath()
	assert.Equal(t, "/path/to/archive.zip/folder/file.txt", path)
}

func TestZipFileEncodeJSON(t *testing.T) {
	zipFile := &ZipFile{
		File: &File{
			Name: "test.txt",
			Size: 100,
		},
		zipPath:   "/path/to/archive.zip",
		inZipPath: "test.txt",
	}

	var buf bytes.Buffer
	err := zipFile.EncodeJSON(&buf, false)
	assert.NoError(t, err)
	assert.NotEmpty(t, buf.String())
}

func TestZipDirEncodeJSON(t *testing.T) {
	zipDir := &ZipDir{
		Dir: &Dir{
			File: &File{
				Name: "folder",
			},
		},
		zipPath: "/path/to/archive.zip",
	}

	var buf bytes.Buffer
	err := zipDir.EncodeJSON(&buf, false)
	assert.NoError(t, err)
	assert.NotEmpty(t, buf.String())
}

func TestZipDirGetPathWithParent(t *testing.T) {
	parent := &ZipDir{
		Dir: &Dir{
			File: &File{
				Name: "parent",
			},
		},
		zipPath: "/path/to/archive.zip",
	}

	zipDir := &ZipDir{
		Dir: &Dir{
			File: &File{
				Name: "child",
			},
		},
		zipPath: "/path/to/archive.zip",
	}
	zipDir.Parent = parent

	path := zipDir.GetPath()
	assert.Equal(t, filepath.Join(parent.GetPath(), "child"), path)
}

func TestZipDirGetPathWithoutParent(t *testing.T) {
	zipDir := &ZipDir{
		Dir: &Dir{
			File: &File{
				Name: "root",
			},
		},
		zipPath: "/path/to/archive.zip",
	}

	path := zipDir.GetPath()
	assert.Equal(t, "/path/to/archive.zip", path)
}

func TestProcessZipFileWithEmptyZip(t *testing.T) {
	// Create a temporary zip file
	zipPath := "/tmp/empty.zip"
	defer os.Remove(zipPath)

	// Create an empty zip file
	file, err := os.Create(zipPath)
	assert.NoError(t, err)
	file.Close()

	// Create a zip file with no entries
	zipFile, err := os.Create(zipPath)
	assert.NoError(t, err)
	writer := zip.NewWriter(zipFile)
	writer.Close()
	zipFile.Close()

	info, err := os.Stat(zipPath)
	assert.NoError(t, err)

	zipDir, err := processZipFile(zipPath, info)
	assert.NoError(t, err)
	assert.NotNil(t, zipDir)
	assert.Equal(t, "empty.zip", zipDir.Name)
	assert.Equal(t, 'Z', zipDir.Flag)
}

func TestProcessZipFileWithDirectoryEntries(t *testing.T) {
	// Create a temporary zip file
	zipPath := "/tmp/dir_entries.zip"
	defer os.Remove(zipPath)

	// Create a zip file with directory entries
	zipFile, err := os.Create(zipPath)
	assert.NoError(t, err)
	writer := zip.NewWriter(zipFile)

	// Add a directory entry
	_, err = writer.Create("folder/")
	assert.NoError(t, err)

	// Add a file in the directory
	fileWriter, err := writer.Create("folder/file.txt")
	assert.NoError(t, err)
	fileWriter.Write([]byte("test content"))

	writer.Close()
	zipFile.Close()

	info, err := os.Stat(zipPath)
	assert.NoError(t, err)

	zipDir, err := processZipFile(zipPath, info)
	assert.NoError(t, err)
	assert.NotNil(t, zipDir)
	assert.Equal(t, "dir_entries.zip", zipDir.Name)
}

func TestProcessZipFileWithNestedDirectories(t *testing.T) {
	// Create a temporary zip file
	zipPath := "/tmp/nested.zip"
	defer os.Remove(zipPath)

	// Create a zip file with nested directories
	zipFile, err := os.Create(zipPath)
	assert.NoError(t, err)
	writer := zip.NewWriter(zipFile)

	// Add files in nested directories
	fileWriter, err := writer.Create("level1/level2/file.txt")
	assert.NoError(t, err)
	fileWriter.Write([]byte("nested content"))

	writer.Close()
	zipFile.Close()

	info, err := os.Stat(zipPath)
	assert.NoError(t, err)

	zipDir, err := processZipFile(zipPath, info)
	assert.NoError(t, err)
	assert.NotNil(t, zipDir)
	assert.Equal(t, "nested.zip", zipDir.Name)
}

func TestProcessZipFileWithRootFiles(t *testing.T) {
	// Create a temporary zip file
	zipPath := "/tmp/root_files.zip"
	defer os.Remove(zipPath)

	// Create a zip file with files in root
	zipFile, err := os.Create(zipPath)
	assert.NoError(t, err)
	writer := zip.NewWriter(zipFile)

	// Add files in root directory
	fileWriter, err := writer.Create("file1.txt")
	assert.NoError(t, err)
	fileWriter.Write([]byte("file1 content"))

	fileWriter, err = writer.Create("file2.txt")
	assert.NoError(t, err)
	fileWriter.Write([]byte("file2 content"))

	writer.Close()
	zipFile.Close()

	info, err := os.Stat(zipPath)
	assert.NoError(t, err)

	zipDir, err := processZipFile(zipPath, info)
	assert.NoError(t, err)
	assert.NotNil(t, zipDir)
	assert.Equal(t, "root_files.zip", zipDir.Name)
}

func TestProcessZipFileError(t *testing.T) {
	// Test with non-existent file
	zipDir, err := processZipFile("/non/existent/file.zip", nil)
	assert.Error(t, err)
	assert.Nil(t, zipDir)
}

func TestGetZipFileSizeWithEmptyZip(t *testing.T) {
	// Create a temporary zip file
	zipPath := "/tmp/empty_size.zip"
	defer os.Remove(zipPath)

	// Create an empty zip file
	zipFile, err := os.Create(zipPath)
	assert.NoError(t, err)
	writer := zip.NewWriter(zipFile)
	writer.Close()
	zipFile.Close()

	uncompressed, compressed, err := getZipFileSize(zipPath)
	assert.NoError(t, err)
	assert.Equal(t, int64(0), uncompressed)
	assert.Equal(t, int64(0), compressed)
}

func TestGetZipFileSizeWithFiles(t *testing.T) {
	// Create a temporary zip file
	zipPath := "/tmp/size_test.zip"
	defer os.Remove(zipPath)

	// Create a zip file with files
	zipFile, err := os.Create(zipPath)
	assert.NoError(t, err)
	writer := zip.NewWriter(zipFile)

	// Add a file
	fileWriter, err := writer.Create("test.txt")
	assert.NoError(t, err)
	fileWriter.Write([]byte("test content"))

	writer.Close()
	zipFile.Close()

	uncompressed, compressed, err := getZipFileSize(zipPath)
	assert.NoError(t, err)
	assert.Greater(t, uncompressed, int64(0))
	assert.Greater(t, compressed, int64(0))
}

func TestGetZipFileSizeWithDirectories(t *testing.T) {
	// Create a temporary zip file
	zipPath := "/tmp/dir_size.zip"
	defer os.Remove(zipPath)

	// Create a zip file with directories
	zipFile, err := os.Create(zipPath)
	assert.NoError(t, err)
	writer := zip.NewWriter(zipFile)

	// Add a directory entry (should be ignored)
	_, err = writer.Create("folder/")
	assert.NoError(t, err)

	// Add a file
	fileWriter, err := writer.Create("file.txt")
	assert.NoError(t, err)
	fileWriter.Write([]byte("test content"))

	writer.Close()
	zipFile.Close()

	uncompressed, compressed, err := getZipFileSize(zipPath)
	assert.NoError(t, err)
	assert.Greater(t, uncompressed, int64(0))
	assert.Greater(t, compressed, int64(0))
}

func TestGetZipFileSizeError(t *testing.T) {
	// Test with non-existent file
	uncompressed, compressed, err := getZipFileSize("/non/existent/file.zip")
	assert.Error(t, err)
	assert.Equal(t, int64(0), uncompressed)
	assert.Equal(t, int64(0), compressed)
}

func TestEnsureZipDirExistsWithEmptyPath(t *testing.T) {
	dirMap := make(map[string]*ZipDir)
	rootDir := &ZipDir{
		Dir: &Dir{
			File: &File{
				Name: "root",
			},
		},
		zipPath: "/test.zip",
	}

	ensureZipDirExists(dirMap, "", "/test.zip", rootDir)
	// Should not create any new directories for empty path
	assert.Len(t, dirMap, 0)
}

func TestEnsureZipDirExistsWithDotPath(t *testing.T) {
	dirMap := make(map[string]*ZipDir)
	rootDir := &ZipDir{
		Dir: &Dir{
			File: &File{
				Name: "root",
			},
		},
		zipPath: "/test.zip",
	}

	ensureZipDirExists(dirMap, ".", "/test.zip", rootDir)
	// Should not create any new directories for dot path
	assert.Len(t, dirMap, 0)
}

func TestEnsureZipDirExistsWithExistingPath(t *testing.T) {
	dirMap := make(map[string]*ZipDir)
	existingDir := &ZipDir{
		Dir: &Dir{
			File: &File{
				Name: "existing",
			},
		},
		zipPath: "/test.zip",
	}
	dirMap["existing"] = existingDir

	rootDir := &ZipDir{
		Dir: &Dir{
			File: &File{
				Name: "root",
			},
		},
		zipPath: "/test.zip",
	}

	ensureZipDirExists(dirMap, "existing", "/test.zip", rootDir)
	// Should not create a new directory for existing path
	assert.Len(t, dirMap, 1)
	assert.Equal(t, existingDir, dirMap["existing"])
}

func TestEnsureZipDirExistsWithNestedPath(t *testing.T) {
	dirMap := make(map[string]*ZipDir)
	rootDir := &ZipDir{
		Dir: &Dir{
			File: &File{
				Name: "root",
			},
		},
		zipPath: "/test.zip",
	}
	dirMap[""] = rootDir

	ensureZipDirExists(dirMap, "level1/level2", "/test.zip", rootDir)

	// Should create both level1 and level1/level2 directories
	assert.Contains(t, dirMap, "level1")
	assert.Contains(t, dirMap, "level1/level2")
	assert.Equal(t, "level1", dirMap["level1"].Name)
	assert.Equal(t, "level2", dirMap["level1/level2"].Name)
}

func TestIsZipFileFunction(t *testing.T) {
	assert.True(t, isZipFile("test.zip"))
	assert.True(t, isZipFile("test.ZIP"))
	assert.True(t, isZipFile("test.jar"))
	assert.True(t, isZipFile("test.JAR"))
	assert.False(t, isZipFile("test.txt"))
	assert.False(t, isZipFile("test.tar"))
	assert.False(t, isZipFile("test.gz"))
}
