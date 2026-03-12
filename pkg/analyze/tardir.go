package analyze

import (
	"archive/tar"
	"compress/bzip2"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dundee/gdu/v5/pkg/fs"
	"github.com/ulikunitz/xz"
)

// TarDir represents a directory structure inside a tar archive
type TarDir struct {
	*Dir
	tarPath string // path to the original tar file
}

// TarFile represents a file inside a tar archive
type TarFile struct {
	*File
	tarPath   string
	inTarPath string // path inside the tar archive
}

// GetPath returns the virtual path for tar file
func (tf *TarFile) GetPath() string {
	return tf.tarPath + "/" + tf.inTarPath
}

// GetType returns type of tar file
func (tf *TarFile) GetType() string {
	return "TarFile"
}

// EncodeJSON encodes tar file to JSON
func (tf *TarFile) EncodeJSON(writer io.Writer, topLevel bool) error {
	return tf.File.EncodeJSON(writer, topLevel)
}

// GetType returns type of tar directory
func (td *TarDir) GetType() string {
	return "TarDirectory"
}

// IsDir returns true for TarDir
func (td *TarDir) IsDir() bool {
	return true
}

// EncodeJSON encodes tar directory to JSON
func (td *TarDir) EncodeJSON(writer io.Writer, topLevel bool) error {
	return td.Dir.EncodeJSON(writer, topLevel)
}

// GetPath returns the virtual path for tar directory
func (td *TarDir) GetPath() string {
	if td.Parent != nil {
		return filepath.Join(td.Parent.GetPath(), td.Name)
	}
	return td.tarPath
}

// isTarFile checks if a file is a tar archive (tar, tar.gz, tgz, tar.bz2, tbz2, tar.xz, txz)
func isTarFile(filename string) bool {
	lower := strings.ToLower(filename)
	return strings.HasSuffix(lower, ".tar") ||
		strings.HasSuffix(lower, ".tar.gz") ||
		strings.HasSuffix(lower, ".tgz") ||
		strings.HasSuffix(lower, ".tar.bz2") ||
		strings.HasSuffix(lower, ".tbz2") ||
		strings.HasSuffix(lower, ".tar.xz") ||
		strings.HasSuffix(lower, ".txz")
}

// multiCloser closes multiple io.Closer instances in sequence
type multiCloser struct {
	closers []io.Closer
}

func (mc *multiCloser) Close() error {
	var firstErr error
	for _, c := range mc.closers {
		if err := c.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// openTarReader opens a tar archive and returns a tar.Reader and a Closer for cleanup.
// It automatically wraps the reader with the appropriate decompressor based on file extension.
func openTarReader(tarPath string) (*tar.Reader, io.Closer, error) {
	f, err := os.Open(tarPath)
	if err != nil {
		return nil, nil, err
	}

	lower := strings.ToLower(tarPath)
	switch {
	case strings.HasSuffix(lower, ".tar.gz") || strings.HasSuffix(lower, ".tgz"):
		gr, err := gzip.NewReader(f)
		if err != nil {
			f.Close()
			return nil, nil, err
		}
		return tar.NewReader(gr), &multiCloser{closers: []io.Closer{gr, f}}, nil

	case strings.HasSuffix(lower, ".tar.bz2") || strings.HasSuffix(lower, ".tbz2"):
		br := bzip2.NewReader(f)
		return tar.NewReader(br), f, nil

	case strings.HasSuffix(lower, ".tar.xz") || strings.HasSuffix(lower, ".txz"):
		xr, err := xz.NewReader(f)
		if err != nil {
			f.Close()
			return nil, nil, err
		}
		return tar.NewReader(xr), f, nil

	default: // plain .tar
		return tar.NewReader(f), f, nil
	}
}

// processTarFile reads a tar archive and returns a TarDir representing its contents.
// TarDir.Size is set to the total uncompressed content size; TarDir.Usage is the
// size of the archive file on disk.
func processTarFile(tarPath string, info os.FileInfo) (*TarDir, error) {
	tr, closer, err := openTarReader(tarPath)
	if err != nil {
		return nil, err
	}
	defer closer.Close()

	tarDir := &TarDir{
		Dir: &Dir{
			File: &File{
				Name:  filepath.Base(tarPath),
				Flag:  'T',
				Size:  info.Size(),
				Usage: info.Size(),
				Mtime: info.ModTime(),
			},
			ItemCount: 1,
			Files:     make(fs.Files, 0),
		},
		tarPath: tarPath,
	}

	dirMap := make(map[string]*TarDir)
	dirMap[""] = tarDir

	var totalUncompressed int64

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		// Normalize path: strip leading ./ and trailing /
		name := strings.TrimPrefix(header.Name, "./")
		name = strings.TrimSuffix(name, "/")

		switch header.Typeflag {
		case tar.TypeDir:
			if name != "" {
				ensureTarDirExists(dirMap, name, tarPath, tarDir)
			}

		case tar.TypeReg, tar.TypeLink, tar.TypeSymlink:
			dirPath := filepath.Dir(name)
			if dirPath == "." {
				dirPath = ""
			}
			ensureTarDirExists(dirMap, dirPath, tarPath, tarDir)

			parentDir := dirMap[dirPath]
			totalUncompressed += header.Size

			tarFile := &TarFile{
				File: &File{
					Name:   filepath.Base(name),
					Flag:   ' ',
					Size:   header.Size,
					Usage:  header.Size,
					Mtime:  header.ModTime,
					Parent: parentDir,
				},
				tarPath:   tarPath,
				inTarPath: name,
			}
			parentDir.AddFile(tarFile)
		}
		// Other types (device files, fifos, etc.) are silently skipped
	}

	// Size = total uncompressed content; Usage = compressed archive on disk
	tarDir.Size = totalUncompressed
	tarDir.Usage = info.Size()

	return tarDir, nil
}

// ensureTarDirExists ensures all directories in the specified path exist within dirMap
func ensureTarDirExists(dirMap map[string]*TarDir, path, tarPath string, rootDir *TarDir) {
	if path == "" || path == "." {
		return
	}

	if _, exists := dirMap[path]; exists {
		return
	}

	// Ensure parent directory exists first
	parentPath := filepath.Dir(path)
	if parentPath != "." && parentPath != "" {
		ensureTarDirExists(dirMap, parentPath, tarPath, rootDir)
	}

	var parent *TarDir
	if parentPath == "" || parentPath == "." {
		parent = rootDir
	} else {
		parent = dirMap[parentPath]
	}

	newDir := &TarDir{
		Dir: &Dir{
			File: &File{
				Name:   filepath.Base(path),
				Flag:   'T',
				Size:   4096,
				Usage:  4096,
				Mtime:  time.Now(),
				Parent: parent,
			},
			ItemCount: 1,
			Files:     make(fs.Files, 0),
		},
		tarPath: tarPath,
	}

	dirMap[path] = newDir
	parent.AddFile(newDir)
}
