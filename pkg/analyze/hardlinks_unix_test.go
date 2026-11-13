//go:build !windows && !plan9

package analyze

import (
	"os"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type fakeStatInfo struct {
	stat *syscall.Stat_t
}

func (i fakeStatInfo) Name() string       { return "f" }
func (i fakeStatInfo) Size() int64        { return 1000 }
func (i fakeStatInfo) Mode() os.FileMode  { return 0 }
func (i fakeStatInfo) ModTime() time.Time { return time.Time{} }
func (i fakeStatInfo) IsDir() bool        { return false }
func (i fakeStatInfo) Sys() any           { return i.stat }

func TestFileStatsIdentifiesHardLinksByDeviceAndInode(t *testing.T) {
	usage, link := fileStats(fakeStatInfo{&syscall.Stat_t{Dev: 7, Ino: 42, Nlink: 2, Blocks: 8}}, "f")

	assert.Equal(t, int64(8*devBSize), usage)
	assert.Equal(t, inodeKey{dev: 7, ino: 42}, link)
}

func TestFileStatsLeavesSingleLinkFilesUnidentified(t *testing.T) {
	_, link := fileStats(fakeStatInfo{&syscall.Stat_t{Dev: 7, Ino: 42, Nlink: 1}}, "f")

	assert.Zero(t, link)
}
