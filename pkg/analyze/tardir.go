package analyze

import (
	"archive/tar"
	"compress/bzip2"
	"compress/gzip"
	"context"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/dundee/gdu/v6/internal/xzcheck"
	"github.com/dundee/gdu/v6/pkg/fs"
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
func (tf *TarFile) EncodeJSON(writer io.Writer, topLevel bool, attributes fs.JSONAttributes) error {
	return tf.File.EncodeJSON(writer, topLevel, attributes)
}

// itemStats returns the totals of the directory, computed when the archive
// was opened (see finishArchive)
func (td *TarDir) itemStats(*fs.HardLinks, bool) (itemCount, size, usage int64) {
	return td.ItemCount, td.Size, td.Usage
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
func (td *TarDir) EncodeJSON(writer io.Writer, topLevel bool, attributes fs.JSONAttributes) error {
	return td.Dir.EncodeJSON(writer, topLevel, attributes)
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
		if err := checkXZ(f); err != nil {
			f.Close()
			return nil, nil, err
		}
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

// checkXZ fails for an xz file asking the decoder for more memory than
// xzcheck.MaxDict.
func checkXZ(f *os.File) error {
	info, err := f.Stat()
	if err != nil {
		return err
	}
	return xzcheck.CheckDicts(f, info.Size(), xzcheck.MaxDict)
}

// processTarFile reads a tar archive and returns a TarDir representing its
// contents, whose totals finishArchive computes.
func processTarFile(tarPath string, info os.FileInfo) (*TarDir, error) {
	return readTarFile(context.Background(), tarPath, info)
}

// readTarFile is processTarFile, stopping when ctx is cancelled: reading
// the headers of a compressed tar decompresses all of it.
func readTarFile(ctx context.Context, tarPath string, info os.FileInfo) (*TarDir, error) {
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

	for entries := 0; ; entries++ {
		if entries%1000 == 0 && ctx.Err() != nil {
			return nil, ctx.Err()
		}
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if entries == maxArchiveEntries {
			return nil, errTooManyEntries
		}

		name, ok := archiveEntryPath(header.Name)
		if !ok {
			continue
		}

		switch header.Typeflag {
		case tar.TypeDir:
			ensureTarDirExists(dirMap, name, tarPath, tarDir)
			dirMap[name].Mtime = header.ModTime

		case tar.TypeReg, tar.TypeLink, tar.TypeSymlink:
			dirPath := path.Dir(name)
			if dirPath == "." {
				dirPath = ""
			}
			ensureTarDirExists(dirMap, dirPath, tarPath, tarDir)

			parentDir := dirMap[dirPath]

			tarFile := &TarFile{
				File: &File{
					Name:   path.Base(name),
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

	return tarDir, nil
}

// ensureTarDirExists ensures all directories of dirPath, a path made by
// archiveEntryPath, exist within dirMap
func ensureTarDirExists(dirMap map[string]*TarDir, dirPath, tarPath string, rootDir *TarDir) {
	if dirPath == "" || dirPath == "." {
		return
	}

	if _, exists := dirMap[dirPath]; exists {
		return
	}

	// Ensure parent directory exists first
	parentPath := path.Dir(dirPath)
	if parentPath != "." && parentPath != "" && parentPath != dirPath {
		ensureTarDirExists(dirMap, parentPath, tarPath, rootDir)
	}

	var parent *TarDir
	if parentPath == "" || parentPath == "." || parentPath == dirPath {
		parent = rootDir
	} else {
		parent = dirMap[parentPath]
	}

	newDir := &TarDir{
		Dir: &Dir{
			File: &File{
				Name:   path.Base(dirPath),
				Flag:   'T',
				Mtime:  rootDir.Mtime,
				Parent: parent,
			},
			ItemCount: 1,
			Files:     make(fs.Files, 0),
		},
		tarPath: tarPath,
	}

	dirMap[dirPath] = newDir
	parent.AddFile(newDir)
}
