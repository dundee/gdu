//go:build windows

package analyze

import (
	"bytes"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
	"unsafe"

	"github.com/dundee/gdu/v5/internal/common"
	"github.com/dundee/gdu/v5/pkg/fs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/windows"
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

	setPlatformSpecificAttrs(file, info, file.Name)

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

	setPlatformSpecificAttrs(file, info, file.Name)

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

	setPlatformSpecificAttrs(file, info, file.Name)

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

	setPlatformSpecificAttrs(file, info, file.Name)

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

	setPlatformSpecificAttrs(file, info, file.Name)

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
	usage, ino := getPlatformSpecificUsageAndMli(regularInfo, regularInfo.name)
	assert.Equal(t, int64(3000), usage)
	assert.Equal(t, uint64(0), ino)

	cloudInfo := mockWinFileInfo{
		name: "file.bin",
		size: 3000,
		sys: &syscall.Win32FileAttributeData{
			FileAttributes: fileAttributeRecallOnDataAccess,
		},
	}
	usage, ino = getPlatformSpecificUsageAndMli(cloudInfo, cloudInfo.name)
	assert.Equal(t, int64(0), usage)
	assert.Equal(t, uint64(0), ino)

	compressedInfo := mockWinFileInfo{
		name: "file.bin",
		size: 3000,
		sys: &syscall.Win32FileAttributeData{
			FileAttributes: windows.FILE_ATTRIBUTE_COMPRESSED,
		},
	}
	usage, ino = getPlatformSpecificUsageAndMli(compressedInfo, filepath.Join(t.TempDir(), "missing.bin"))
	assert.Equal(t, int64(3000), usage)
	assert.Equal(t, uint64(0), ino)

	target := filepath.Join(t.TempDir(), "target.bin")
	require.NoError(t, os.WriteFile(target, make([]byte, 3000), 0o600))
	symlinkInfo := mockWinFileInfo{
		name: "link.bin",
		mode: os.ModeSymlink,
		sys: &syscall.Win32FileAttributeData{
			FileAttributes: windows.FILE_ATTRIBUTE_COMPRESSED | windows.FILE_ATTRIBUTE_REPARSE_POINT,
		},
	}
	usage, ino = getPlatformSpecificUsageAndMli(symlinkInfo, target)
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
	usage, mli := getSyscallStats(regularInfo, regularInfo.name)
	assert.Equal(t, int64(5000), usage)
	assert.Equal(t, uint64(0), mli)

	cloudInfo := mockWinFileInfo{
		name: "file.bin",
		size: 5000,
		sys: &syscall.Win32FileAttributeData{
			FileAttributes: fileAttributeRecallOnOpen,
		},
	}
	usage, mli = getSyscallStats(cloudInfo, cloudInfo.name)
	assert.Equal(t, int64(0), usage)
	assert.Equal(t, uint64(0), mli)
}

const testFileSize = 4 << 20

// createCompressedAndSparseFiles creates an NTFS compressed file (as "compact /c" does)
// and a sparse file without data, both testFileSize bytes long
func createCompressedAndSparseFiles(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	var returned uint32

	compressed, err := os.Create(filepath.Join(root, "compressed"))
	require.NoError(t, err)
	defer compressed.Close()
	_, err = compressed.Write(bytes.Repeat([]byte{'a'}, testFileSize))
	require.NoError(t, err)
	format := uint16(1) // COMPRESSION_FORMAT_DEFAULT
	err = windows.DeviceIoControl(
		windows.Handle(compressed.Fd()), windows.FSCTL_SET_COMPRESSION,
		(*byte)(unsafe.Pointer(&format)), uint32(unsafe.Sizeof(format)), nil, 0, &returned, nil,
	)
	if err != nil {
		t.Skipf("file compression is not supported: %v", err)
	}

	sparse, err := os.Create(filepath.Join(root, "sparse"))
	require.NoError(t, err)
	defer sparse.Close()
	err = windows.DeviceIoControl(windows.Handle(sparse.Fd()), windows.FSCTL_SET_SPARSE, nil, 0, nil, 0, &returned, nil)
	if err != nil {
		t.Skipf("sparse files are not supported: %v", err)
	}
	require.NoError(t, sparse.Truncate(testFileSize))

	return root
}

func TestCompressedAndSparseFilesUsage(t *testing.T) {
	root := createCompressedAndSparseFiles(t)

	for _, name := range []string{"compressed", "sparse"} {
		path := filepath.Join(root, name)
		info, err := os.Stat(path)
		require.NoError(t, err)

		file := CreateFileItem(path, info)
		assert.Equal(t, name, file.GetName())
		assert.Equal(t, int64(testFileSize), file.GetSize(), name)
		assert.Less(t, file.GetUsage(), int64(testFileSize/2), name)

		usage, _ := getSyscallStats(info, path)
		assert.Equal(t, file.GetUsage(), usage, name)
	}

	analyzers := append([]parityAnalyzer{{
		name:     "parallel",
		analyzer: func(*testing.T) common.Analyzer { return CreateAnalyzer() },
	}}, parityAnalyzers()...)
	for _, tc := range analyzers {
		t.Run(tc.name, func(t *testing.T) {
			got := scanWith(t, tc.analyzer(t), root)["."]
			assert.GreaterOrEqual(t, got.size, int64(2*testFileSize))
			assert.Less(t, got.usage, int64(testFileSize))
		})
	}

	t.Run("top dir", func(t *testing.T) {
		analyzer := CreateTopDirAnalyzer()
		dir := analyzer.AnalyzeDir(root, func(_, _ string) bool { return false }, func(_ string) bool { return false })
		analyzer.GetDone().Wait()
		dir.UpdateStats(make(fs.HardLinkedItems))
		assert.GreaterOrEqual(t, dir.GetSize(), int64(2*testFileSize))
		assert.Less(t, dir.GetUsage(), int64(testFileSize))
	})
}
