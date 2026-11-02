//go:build windows

package analyze

import (
	"os"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type mockWinFileInfo struct {
	name    string
	size    int64
	mode    os.FileMode
	modTime time.Time
	isDir   bool
	sys     any
}

func (m mockWinFileInfo) Name() string       { return m.name }
func (m mockWinFileInfo) Size() int64        { return m.size }
func (m mockWinFileInfo) Mode() os.FileMode  { return m.mode }
func (m mockWinFileInfo) ModTime() time.Time { return m.modTime }
func (m mockWinFileInfo) IsDir() bool        { return m.isDir }
func (m mockWinFileInfo) Sys() any           { return m.sys }

func TestSetPlatformSpecificAttrsRegularFile(t *testing.T) {
	now := time.Now()
	nsec := now.UnixNano()
	info := mockWinFileInfo{
		name: "regular.txt",
		size: 1024,
		sys: &syscall.Win32FileAttributeData{
			FileAttributes: 0x80, // FILE_ATTRIBUTE_NORMAL
			LastWriteTime: syscall.Filetime{
				LowDateTime:  uint32(nsec / 100),
				HighDateTime: uint32(nsec / 100 >> 32),
			},
		},
	}

	file := &File{
		Name: "regular.txt",
		Size: 1024,
	}

	setPlatformSpecificAttrs(file, info)

	assert.Equal(t, int64(1024), file.Size)
	assert.Equal(t, int64(1024), file.Usage)
}

func TestSetPlatformSpecificAttrsRecallOnOpen(t *testing.T) {
	info := mockWinFileInfo{
		name: "online_open.txt",
		size: 2048,
		sys: &syscall.Win32FileAttributeData{
			FileAttributes: fileAttributeRecallOnOpen,
		},
	}

	file := &File{
		Name: "online_open.txt",
		Size: 2048,
	}

	setPlatformSpecificAttrs(file, info)

	assert.Equal(t, int64(2048), file.Size)
	assert.Equal(t, int64(0), file.Usage)
}

func TestSetPlatformSpecificAttrsRecallOnDataAccess(t *testing.T) {
	info := mockWinFileInfo{
		name: "online_access.txt",
		size: 4096,
		sys: &syscall.Win32FileAttributeData{
			FileAttributes: fileAttributeRecallOnDataAccess,
		},
	}

	file := &File{
		Name: "online_access.txt",
		Size: 4096,
	}

	setPlatformSpecificAttrs(file, info)

	assert.Equal(t, int64(4096), file.Size)
	assert.Equal(t, int64(0), file.Usage)
}

func TestSetPlatformSpecificAttrsBothRecallAttributes(t *testing.T) {
	info := mockWinFileInfo{
		name: "online_both.txt",
		size: 8192,
		sys: &syscall.Win32FileAttributeData{
			FileAttributes: fileAttributeRecallOnOpen | fileAttributeRecallOnDataAccess,
		},
	}

	file := &File{
		Name: "online_both.txt",
		Size: 8192,
	}

	setPlatformSpecificAttrs(file, info)

	assert.Equal(t, int64(8192), file.Size)
	assert.Equal(t, int64(0), file.Usage)
}

func TestSetPlatformSpecificAttrsNonWinSys(t *testing.T) {
	info := mockWinFileInfo{
		name: "other.txt",
		size: 512,
		sys:  nil,
	}

	file := &File{
		Name: "other.txt",
		Size: 512,
	}

	setPlatformSpecificAttrs(file, info)

	assert.Equal(t, int64(512), file.Size)
	assert.Equal(t, int64(512), file.Usage)
}

func TestCreateFileItemOnlineOnly(t *testing.T) {
	info := mockWinFileInfo{
		name: "cloud.docx",
		size: 16384,
		sys: &syscall.Win32FileAttributeData{
			FileAttributes: fileAttributeRecallOnOpen,
		},
	}

	file := CreateFileItem("cloud.docx", info)

	assert.Equal(t, "cloud.docx", file.GetName())
	assert.Equal(t, int64(16384), file.GetSize())
	assert.Equal(t, int64(0), file.GetUsage())
}

func TestGetPlatformSpecificUsageAndMli(t *testing.T) {
	regularInfo := mockWinFileInfo{
		name: "file.bin",
		size: 3000,
		sys: &syscall.Win32FileAttributeData{
			FileAttributes: 0,
		},
	}
	usage, ino := getPlatformSpecificUsageAndMli(regularInfo)
	assert.Equal(t, int64(3000), usage)
	assert.Equal(t, uint64(0), ino)

	cloudInfo := mockWinFileInfo{
		name: "file.bin",
		size: 3000,
		sys: &syscall.Win32FileAttributeData{
			FileAttributes: fileAttributeRecallOnDataAccess,
		},
	}
	usage, ino = getPlatformSpecificUsageAndMli(cloudInfo)
	assert.Equal(t, int64(0), usage)
	assert.Equal(t, uint64(0), ino)
}

func TestGetSyscallStats(t *testing.T) {
	regularInfo := mockWinFileInfo{
		name: "file.bin",
		size: 5000,
		sys: &syscall.Win32FileAttributeData{
			FileAttributes: 0,
		},
	}
	usage, mli := getSyscallStats(regularInfo)
	assert.Equal(t, int64(5000), usage)
	assert.Equal(t, uint64(0), mli)

	cloudInfo := mockWinFileInfo{
		name: "file.bin",
		size: 5000,
		sys: &syscall.Win32FileAttributeData{
			FileAttributes: fileAttributeRecallOnOpen,
		},
	}
	usage, mli = getSyscallStats(cloudInfo)
	assert.Equal(t, int64(0), usage)
	assert.Equal(t, uint64(0), mli)
}
