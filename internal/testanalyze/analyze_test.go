package testanalyze

import (
	"testing"
	"time"

	"github.com/dundee/gdu/v5/internal/common"
	"github.com/dundee/gdu/v5/pkg/analyze"
	"github.com/dundee/gdu/v5/pkg/fs"
	"github.com/stretchr/testify/assert"
)

// Compile-time check that MockedAnalyzer implements common.Analyzer
var _ common.Analyzer = (*MockedAnalyzer)(nil)

func TestAnalyzeDir(t *testing.T) {
	a := &MockedAnalyzer{}
	result := a.AnalyzeDir(".", nil, nil)

	assert.NotNil(t, result)
	assert.True(t, result.IsDir())
	assert.Equal(t, "test_dir", result.GetName())
	assert.Equal(t, int64(1e12+1), result.GetUsage())
	assert.Equal(t, int64(1e12+2), result.GetSize())
	assert.Equal(t, int64(12), result.GetItemCount())
	assert.Equal(t,
		time.Date(2021, 8, 27, 22, 23, 24, 0, time.UTC),
		result.GetMtime(),
	)

	dir := result.(*analyze.Dir)
	assert.Equal(t, ".", dir.BasePath)
	assert.Len(t, dir.Files, 4)

	// Verify children names and types
	names := make([]string, len(dir.Files))
	for i, f := range dir.Files {
		names[i] = f.GetName()
	}
	assert.Equal(t, []string{"aaa", "bbb", "ccc", "ddd"}, names)

	// Verify "aaa" is a dir with TB-range size
	aaa := dir.Files[0]
	assert.True(t, aaa.IsDir())
	assert.Equal(t, int64(1e12+1), aaa.GetUsage())
	assert.Equal(t, int64(1e12+2), aaa.GetSize())
	assert.Equal(t, result, aaa.GetParent())
	assert.Equal(t,
		time.Date(2021, 8, 27, 22, 23, 27, 0, time.UTC),
		aaa.GetMtime(),
	)

	// Verify "bbb" is a dir with GB-range size
	bbb := dir.Files[1]
	assert.True(t, bbb.IsDir())
	assert.Equal(t, int64(1e9+1), bbb.GetUsage())
	assert.Equal(t, int64(1e9+2), bbb.GetSize())
	assert.Equal(t, result, bbb.GetParent())

	// Verify "ccc" is a dir with MB-range size
	ccc := dir.Files[2]
	assert.True(t, ccc.IsDir())
	assert.Equal(t, int64(1e6+1), ccc.GetUsage())
	assert.Equal(t, int64(1e6+2), ccc.GetSize())
	assert.Equal(t, result, ccc.GetParent())

	// Verify "ddd" is a file with KB-range size
	ddd := dir.Files[3]
	assert.False(t, ddd.IsDir())
	assert.Equal(t, int64(1e3+1), ddd.GetUsage())
	assert.Equal(t, int64(1e3+2), ddd.GetSize())
	assert.Equal(t, result, ddd.GetParent())
}

func TestGetProgress(t *testing.T) {
	a := &MockedAnalyzer{}
	progress := a.GetProgress()

	assert.Equal(t, int64(0), progress.ItemCount)
}

func TestGetDone(t *testing.T) {
	a := &MockedAnalyzer{}
	done := a.GetDone()

	assert.NotNil(t, done)

	// The channel should be already closed (broadcast), so receiving should not block
	select {
	case <-done:
		// expected: channel was broadcast
	case <-time.After(time.Second):
		t.Fatal("GetDone channel was not broadcast")
	}
}

func TestResetProgress(t *testing.T) {
	a := &MockedAnalyzer{}
	assert.NotPanics(t, func() {
		a.ResetProgress()
	})
}

func TestSetFollowSymlinks(t *testing.T) {
	a := &MockedAnalyzer{}
	assert.NotPanics(t, func() {
		a.SetFollowSymlinks(true)
		a.SetFollowSymlinks(false)
	})
}

func TestSetShowAnnexedSize(t *testing.T) {
	a := &MockedAnalyzer{}
	assert.NotPanics(t, func() {
		a.SetShowAnnexedSize(true)
		a.SetShowAnnexedSize(false)
	})
}

func TestSetTimeFilter(t *testing.T) {
	a := &MockedAnalyzer{}
	assert.NotPanics(t, func() {
		a.SetTimeFilter(func(mtime time.Time) bool { return true })
		a.SetTimeFilter(nil)
	})
}

func TestSetArchiveBrowsing(t *testing.T) {
	a := &MockedAnalyzer{}
	assert.NotPanics(t, func() {
		a.SetArchiveBrowsing(true)
		a.SetArchiveBrowsing(false)
	})
}

func TestSetFileTypeFilter(t *testing.T) {
	a := &MockedAnalyzer{}
	assert.NotPanics(t, func() {
		a.SetFileTypeFilter(func(name string) bool { return false })
		a.SetFileTypeFilter(nil)
	})
}

func TestItemFromDirWithErr(t *testing.T) {
	err := ItemFromDirWithErr(nil, nil)

	assert.NotNil(t, err)
	assert.Equal(t, "Failed", err.Error())
}

func TestItemFromDirWithSleep(t *testing.T) {
	// Create a minimal dir + file structure for remove.ItemFromDir
	dir := &analyze.Dir{
		File: &analyze.File{
			Name:  "parent",
			Usage: 5000,
			Size:  5000,
		},
		BasePath: t.TempDir(),
	}
	file := &analyze.File{
		Name:   "child",
		Usage:  1000,
		Size:   1000,
		Parent: dir,
	}
	dir.Files = fs.Files{file}

	start := time.Now()
	err := ItemFromDirWithSleep(dir, file)
	elapsed := time.Since(start)

	// Should take at least 500ms due to sleep
	assert.GreaterOrEqual(t, elapsed.Milliseconds(), int64(500))
	// os.RemoveAll returns nil for non-existent paths
	assert.NoError(t, err)
}

func TestItemFromDirWithSleepAndErr(t *testing.T) {
	start := time.Now()
	err := ItemFromDirWithSleepAndErr(nil, nil)
	elapsed := time.Since(start)

	assert.NotNil(t, err)
	assert.Equal(t, "Failed", err.Error())
	assert.GreaterOrEqual(t, elapsed.Milliseconds(), int64(500))
}
