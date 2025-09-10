package analyze

import (
	"archive/zip"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dundee/gdu/v5/pkg/fs"
)

// ZipDir represents a directory structure inside a zip file
type ZipDir struct {
	*Dir
	zipPath string // path to the original zip file
}

// ZipFile represents a file inside a zip archive
type ZipFile struct {
	*File
	zipPath   string
	inZipPath string // path inside the zip file
}

// GetPath returns the virtual path for zip file
func (zf *ZipFile) GetPath() string {
	return zf.zipPath + "/" + zf.inZipPath
}

// GetType returns type of zip file
func (zf *ZipFile) GetType() string {
	return "ZipFile"
}

// EncodeJSON encodes zip file to JSON
func (zf *ZipFile) EncodeJSON(writer io.Writer, topLevel bool) error {
	// Use the embedded File's EncodeJSON method
	return zf.File.EncodeJSON(writer, topLevel)
}

// GetType returns type of zip directory
func (zd *ZipDir) GetType() string {
	return "ZipDirectory"
}

// IsDir returns true for ZipDir
func (zd *ZipDir) IsDir() bool {
	return true
}

// EncodeJSON encodes zip directory to JSON
func (zd *ZipDir) EncodeJSON(writer io.Writer, topLevel bool) error {
	// Use the embedded Dir's EncodeJSON method
	return zd.Dir.EncodeJSON(writer, topLevel)
}

// GetPath returns the virtual path for zip directory
func (zd *ZipDir) GetPath() string {
	if zd.Parent != nil {
		return filepath.Join(zd.Parent.GetPath(), zd.Name)
	}
	return zd.zipPath
}

// isZipFile checks if a file is a zip or jar file
func isZipFile(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	return ext == ".zip" || ext == ".jar"
}

// processZipFile processes a zip file and returns a ZipDir representing its contents
func processZipFile(zipPath string, info os.FileInfo) (zipDir *ZipDir, err error) {
	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	// Create root directory
	zipDir = &ZipDir{
		Dir: &Dir{
			File: &File{
				Name:  filepath.Base(zipPath),
				Flag:  'Z', // Use 'Z' to identify zip files
				Size:  info.Size(),
				Usage: info.Size(),
				Mtime: info.ModTime(),
			},
			ItemCount: 1,
			Files:     make(fs.Files, 0),
		},
		zipPath: zipPath,
	}

	// Use map to store directory structure
	dirMap := make(map[string]*ZipDir)
	dirMap[""] = zipDir // root directory

	for _, f := range reader.File {
		if f.FileInfo().IsDir() {
			continue // Skip directory entries, we'll create them automatically based on file paths
		}

		// Parse file path and ensure all parent directories exist
		dirPath := filepath.Dir(f.Name)
		if dirPath == "." {
			dirPath = "" // root directory
		}
		ensureZipDirExists(dirMap, dirPath, zipPath, zipDir)

		// Create file item
		parentDir := dirMap[dirPath]
		zipFile := &ZipFile{
			File: &File{
				Name:   filepath.Base(f.Name),
				Flag:   ' ',
				Size:   int64(f.UncompressedSize64),
				Usage:  int64(f.CompressedSize64),
				Mtime:  f.FileInfo().ModTime(),
				Parent: parentDir,
			},
			zipPath:   zipPath,
			inZipPath: f.Name,
		}

		parentDir.AddFile(zipFile)
	}

	return zipDir, nil
}

// ensureZipDirExists ensures all directories in the specified path exist
func ensureZipDirExists(dirMap map[string]*ZipDir, path, zipPath string, rootDir *ZipDir) {
	if path == "" || path == "." {
		return
	}

	// If directory already exists, return directly
	if _, exists := dirMap[path]; exists {
		return
	}

	// Ensure parent directory exists
	parentPath := filepath.Dir(path)
	if parentPath != "." && parentPath != "" {
		ensureZipDirExists(dirMap, parentPath, zipPath, rootDir)
	}

	// Create current directory
	var parent *ZipDir
	if parentPath == "" || parentPath == "." {
		parent = rootDir
	} else {
		parent = dirMap[parentPath]
	}

	newDir := &ZipDir{
		Dir: &Dir{
			File: &File{
				Name:   filepath.Base(path),
				Flag:   'Z',
				Size:   4096, // virtual directory size
				Usage:  4096,
				Mtime:  time.Now(),
				Parent: parent,
			},
			ItemCount: 1,
			Files:     make(fs.Files, 0),
		},
		zipPath: zipPath,
	}

	dirMap[path] = newDir
	parent.AddFile(newDir)
}

// getZipFileSize gets the total uncompressed size of a zip file
func getZipFileSize(zipPath string) (uncompressed, compressed int64, err error) {
	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		return 0, 0, err
	}
	defer reader.Close()

	var uncompressedSize, compressedSize int64
	for _, f := range reader.File {
		if !f.FileInfo().IsDir() {
			uncompressedSize += int64(f.UncompressedSize64)
			compressedSize += int64(f.CompressedSize64)
		}
	}

	return uncompressedSize, compressedSize, nil
}
