package analyze

import (
	"archive/zip"
	"io"
	"math"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/dundee/gdu/v6/pkg/fs"
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
func (zf *ZipFile) EncodeJSON(writer io.Writer, topLevel bool, attributes fs.JSONAttributes) error {
	// Use the embedded File's EncodeJSON method
	return zf.File.EncodeJSON(writer, topLevel, attributes)
}

// itemStats returns the totals of the directory, computed when the archive
// was opened (see finishArchive)
func (zd *ZipDir) itemStats(*fs.HardLinks, bool) (itemCount, size, usage int64) {
	return zd.ItemCount, zd.Size, zd.Usage
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
func (zd *ZipDir) EncodeJSON(writer io.Writer, topLevel bool, attributes fs.JSONAttributes) error {
	// Use the embedded Dir's EncodeJSON method
	return zd.Dir.EncodeJSON(writer, topLevel, attributes)
}

// GetPath returns the virtual path for zip directory
func (zd *ZipDir) GetPath() string {
	if zd.Parent != nil {
		return filepath.Join(zd.Parent.GetPath(), zd.Name)
	}
	return zd.zipPath
}

// clampSize returns the size an archive tells as an int64; a crafted one may
// not fit.
func clampSize(size uint64) int64 {
	return int64(min(size, math.MaxInt64))
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
	if len(reader.File) > maxArchiveEntries {
		return nil, errTooManyEntries
	}

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

		name, ok := archiveEntryPath(f.Name)
		if !ok {
			continue
		}
		// Parse file path and ensure all parent directories exist
		dirPath := path.Dir(name)
		if dirPath == "." {
			dirPath = "" // root directory
		}
		ensureZipDirExists(dirMap, dirPath, zipPath, zipDir)

		// Create file item
		parentDir := dirMap[dirPath]
		zipFile := &ZipFile{
			File: &File{
				Name:   path.Base(name),
				Flag:   ' ',
				Size:   clampSize(f.UncompressedSize64),
				Usage:  clampSize(f.CompressedSize64),
				Mtime:  f.FileInfo().ModTime(),
				Parent: parentDir,
			},
			zipPath:   zipPath,
			inZipPath: name,
		}

		parentDir.AddFile(zipFile)
	}

	return zipDir, nil
}

// ensureZipDirExists ensures all directories of dirPath, a path made by
// archiveEntryPath, exist
func ensureZipDirExists(dirMap map[string]*ZipDir, dirPath, zipPath string, rootDir *ZipDir) {
	if dirPath == "" || dirPath == "." {
		return
	}

	// If directory already exists, return directly
	if _, exists := dirMap[dirPath]; exists {
		return
	}

	// Ensure parent directory exists
	parentPath := path.Dir(dirPath)
	if parentPath != "." && parentPath != "" && parentPath != dirPath {
		ensureZipDirExists(dirMap, parentPath, zipPath, rootDir)
	}

	// Create current directory
	var parent *ZipDir
	if parentPath == "" || parentPath == "." || parentPath == dirPath {
		parent = rootDir
	} else {
		parent = dirMap[parentPath]
	}

	newDir := &ZipDir{
		Dir: &Dir{
			File: &File{
				Name:   path.Base(dirPath),
				Flag:   'Z',
				Mtime:  rootDir.Mtime,
				Parent: parent,
			},
			ItemCount: 1,
			Files:     make(fs.Files, 0),
		},
		zipPath: zipPath,
	}

	dirMap[dirPath] = newDir
	parent.AddFile(newDir)
}
