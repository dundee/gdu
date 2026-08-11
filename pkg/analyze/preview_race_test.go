package analyze

import (
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"testing"

	"github.com/dundee/gdu/v5/internal/common"
	"github.com/dundee/gdu/v5/pkg/fs"
	"github.com/stretchr/testify/assert"
)

// createWideTree builds a tree large enough to keep the parallel analyzer busy
// so that GetCurrentDir can be read while the scan is still appending to it.
func createWideTree(t *testing.T) (string, func()) {
	t.Helper()
	root, err := os.MkdirTemp("", "gdu-preview-race")
	assert.NoError(t, err)
	for i := 0; i < 40; i++ {
		sub := filepath.Join(root, "dir"+strconv.Itoa(i))
		assert.NoError(t, os.MkdirAll(filepath.Join(sub, "nested"), os.ModePerm))
		for j := 0; j < 20; j++ {
			f := filepath.Join(sub, "f"+strconv.Itoa(j))
			assert.NoError(t, os.WriteFile(f, []byte("data"), 0o600))
			nf := filepath.Join(sub, "nested", "f"+strconv.Itoa(j))
			assert.NoError(t, os.WriteFile(nf, []byte("data"), 0o600))
		}
	}
	return root, func() { _ = os.RemoveAll(root) }
}

// TestPreviewWhileScanning exercises reading and computing stats on the live
// directory tree while the analyzer is still building it. Run with -race to
// detect any unsynchronized access between AddFile and the preview readers.
func TestPreviewWhileScanning(t *testing.T) {
	root, fin := createWideTree(t)
	defer fin()

	analyzer := CreateAnalyzer()

	var wg sync.WaitGroup
	stop := make(chan struct{})

	// concurrently peek at the partial tree, mirroring what the TUI preview does
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
			}
			if cur := analyzer.GetCurrentDir(); cur != nil {
				cur.UpdateStats(make(fs.HardLinkedItems))
				var total int64
				for item := range cur.GetFilesLocked(fs.SortBySize, fs.SortDesc) {
					total += item.GetSize()
				}
				_ = total
			}
		}
	}()

	dir := analyzer.AnalyzeDir(
		root,
		func(_, _ string) bool { return false },
		func(_ string) bool { return false },
	)
	close(stop)
	wg.Wait()

	dir.UpdateStats(make(fs.HardLinkedItems))
	assert.Equal(t, root, dir.GetPath())
	// 40 dirs * (20 files + nested dir with 20 files) + nested dirs + root
	assert.Greater(t, dir.GetItemCount(), int64(40*40))

	// the analyzer exposes a point-in-time snapshot of the returned root
	var _ common.Analyzer = analyzer
	assert.Equal(t, dir.GetPath(), analyzer.GetCurrentDir().GetPath())
}

func TestCurrentDirSnapshotCannotOverwriteLiveStats(t *testing.T) {
	live := &Dir{
		File:      &File{Name: "root", Size: 100, Usage: 100},
		Files:     make(fs.Files, 0, 1),
		ItemCount: 1,
	}
	analyzer := CreateAnalyzer()
	analyzer.setCurrentDir(live)

	snapshot := analyzer.GetCurrentDir()
	live.AddFile(&File{Name: "complete", Size: 7, Usage: 5, Parent: live})
	live.UpdateStats(make(fs.HardLinkedItems))
	snapshot.UpdateStats(make(fs.HardLinkedItems))

	assert.Equal(t, int64(7), live.GetSize())
	assert.Equal(t, int64(5), live.GetUsage())
	assert.Equal(t, int64(2), live.GetItemCount())
	assert.Empty(t, itemNames(snapshot))
}

func TestCurrentDirSnapshotPreservesArchiveItems(t *testing.T) {
	live := &Dir{
		File:     &File{Name: "root"},
		BasePath: "/tmp",
		Files:    make(fs.Files, 0, 2),
	}
	zipDir := &ZipDir{
		Dir: &Dir{
			File:  &File{Name: "archive.zip", Parent: live},
			Files: make(fs.Files, 0, 1),
		},
		zipPath: "/tmp/archive.zip",
	}
	zipDir.Files = append(zipDir.Files, &ZipFile{
		File:      &File{Name: "inside.zip", Parent: zipDir},
		zipPath:   "/tmp/archive.zip",
		inZipPath: "inside.zip",
	})
	tarDir := &TarDir{
		Dir: &Dir{
			File:  &File{Name: "archive.tar", Parent: live},
			Files: make(fs.Files, 0, 1),
		},
		tarPath: "/tmp/archive.tar",
	}
	tarDir.Files = append(tarDir.Files, &TarFile{
		File:      &File{Name: "inside.tar", Parent: tarDir},
		tarPath:   "/tmp/archive.tar",
		inTarPath: "inside.tar",
	})
	live.Files = append(live.Files, zipDir, tarDir)

	analyzer := CreateAnalyzer()
	analyzer.setCurrentDir(live)
	snapshot := analyzer.GetCurrentDir()

	items := make(map[string]fs.Item)
	for item := range snapshot.GetFiles(fs.SortByName, fs.SortAsc) {
		items[item.GetName()] = item
	}
	zipSnapshot, ok := items["archive.zip"].(*ZipDir)
	assert.True(t, ok)
	assert.Equal(t, zipDir.GetType(), zipSnapshot.GetType())
	assert.Equal(t, zipDir.GetPath(), zipSnapshot.GetPath())
	for item := range zipSnapshot.GetFiles(fs.SortByName, fs.SortAsc) {
		_, isZipFile := item.(*ZipFile)
		assert.True(t, isZipFile)
		assert.Equal(t, zipDir.Files[0].GetPath(), item.GetPath())
	}
	tarSnapshot, ok := items["archive.tar"].(*TarDir)
	assert.True(t, ok)
	assert.Equal(t, tarDir.GetType(), tarSnapshot.GetType())
	assert.Equal(t, tarDir.GetPath(), tarSnapshot.GetPath())
	for item := range tarSnapshot.GetFiles(fs.SortByName, fs.SortAsc) {
		_, isTarFile := item.(*TarFile)
		assert.True(t, isTarFile)
		assert.Equal(t, tarDir.Files[0].GetPath(), item.GetPath())
	}
}
