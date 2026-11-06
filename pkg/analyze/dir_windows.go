//go:build windows

package analyze

import (
	"errors"
	"os"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Checking to see if this file is an "online only" OneDrive file, as they don't take any disk space.
// https://learn.microsoft.com/en-us/windows/win32/fileio/file-attribute-constants
const (
	fileAttributeRecallOnOpen       = 0x100000
	fileAttributeRecallOnDataAccess = 0x400000
)

func isRecallFile(info os.FileInfo) bool {
	if data, ok := info.Sys().(*syscall.Win32FileAttributeData); ok {
		return data.FileAttributes&(fileAttributeRecallOnOpen|fileAttributeRecallOnDataAccess) != 0
	}
	return false
}

func isCompressedOrSparseFile(info os.FileInfo) bool {
	if data, ok := info.Sys().(*syscall.Win32FileAttributeData); ok {
		return data.FileAttributes&(windows.FILE_ATTRIBUTE_COMPRESSED|windows.FILE_ATTRIBUTE_SPARSE_FILE) != 0
	}
	return false
}

const invalidFileSize = 0xFFFFFFFF

var procGetCompressedFileSizeW = windows.NewLazySystemDLL("kernel32.dll").NewProc("GetCompressedFileSizeW")

// getCompressedFileSize returns the space a compressed or sparse file actually occupies on disk
// https://learn.microsoft.com/en-us/windows/win32/api/fileapi/nf-fileapi-getcompressedfilesizew
func getCompressedFileSize(path string) (int64, error) {
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return 0, err
	}
	var high uint32
	low, _, err := procGetCompressedFileSizeW.Call(uintptr(unsafe.Pointer(p)), uintptr(unsafe.Pointer(&high)))
	if uint32(low) == invalidFileSize && !errors.Is(err, windows.ERROR_SUCCESS) {
		return 0, err
	}
	return int64(high)<<32 | int64(uint32(low)), nil
}

func getPlatformSpecificUsageAndMli(info os.FileInfo, path string) (usage int64, ino uint64) {
	if isRecallFile(info) {
		return 0, 0
	}
	// Only compressed and sparse files are worth the extra syscall, which follows symlinks
	if info.Mode()&os.ModeSymlink == 0 && isCompressedOrSparseFile(info) {
		if size, err := getCompressedFileSize(path); err == nil {
			return size, 0
		}
	}
	return info.Size(), 0 // No block info on Windows, use apparent size
}

func setPlatformSpecificAttrs(file *File, f os.FileInfo, path string) {
	if stat, ok := f.Sys().(*syscall.Win32FileAttributeData); ok {
		file.Mtime = time.Unix(0, stat.LastWriteTime.Nanoseconds())
	}
	file.Usage, _ = getPlatformSpecificUsageAndMli(f, path)
}

// setDirPlatformSpecificAttrs fills the attributes of a directory from the info the caller already has
func setDirPlatformSpecificAttrs(dir *Dir, info os.FileInfo) {
	dir.Mtime = info.ModTime()
}

// getInode returns 0, inode numbers are not used on this platform
func getInode(os.FileInfo) uint64 {
	return 0
}

// dirOwnUsage returns 0: Windows exposes no block allocation for a
// directory, so gdu charges it nothing rather than guessing a block.
func dirOwnUsage(string) int64 {
	return 0
}

// dirInfoOwnUsage is dirOwnUsage for a directory that has already been stat'ed.
func dirInfoOwnUsage(os.FileInfo) int64 {
	return 0
}

// getSyscallStats extracts usage and inode info from os.FileInfo using syscall
func getSyscallStats(info os.FileInfo, path string) (usage int64, mli uint64) {
	return getPlatformSpecificUsageAndMli(info, path)
}
