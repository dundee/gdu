package annex

import (
	"fmt"
	"io/fs"
	"log"
	"strconv"
	"strings"
)

// SizeFromKey returns size from git-annex key.
func SizeFromKey(name string) (int64, error) {
	nameParts := strings.SplitN(name, "--", 2)
	backendKVs := nameParts[0]
	backendKVParts := strings.Split(backendKVs, "-")

	if len(backendKVParts) < 2 {
		return 0, fmt.Errorf("key is is missing backend")
	}

	for _, p := range backendKVParts[1:] {
		if p == "" || p[0] != 's' {
			continue
		}

		size, err := strconv.ParseInt(p[1:], 10, 64)
		if err != nil {
			return 0, fmt.Errorf("failed to parse size: %w", err)
		}

		return size, nil
	}

	return 0, fmt.Errorf("size not found in key")
}

// AnnexedFileInfo returns a new FileInfo with size from git-annex key.
func AnnexedFileInfo(fi fs.FileInfo, name string) *FileInfo {
	size, err := SizeFromKey(name)
	if err != nil {
		log.Print(err.Error())
		return &FileInfo{FileInfo: fi}
	}

	afi := &FileInfo{
		FileInfo: fi,
		size:     size,
	}

	return afi
}

var _ fs.FileInfo = (*FileInfo)(nil)

// FileInfo is a wrapper around fs.FileInfo to overwrite the size.
type FileInfo struct {
	fs.FileInfo

	size int64
}

// Length in bytes for regular files; system-dependent for others
func (fi *FileInfo) Size() int64 {
	return int64(fi.size)
}
