//go:build linux
// +build linux

package analyze

import (
	"os"
	"testing"

	"github.com/dundee/gdu/v5/internal/testdir"
	"github.com/dundee/gdu/v5/pkg/fs"
	"github.com/stretchr/testify/assert"
)

func TestErr(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	err := os.Chmod("test_dir/nested", 0)
	assert.Nil(t, err)
	defer func() {
		err = os.Chmod("test_dir/nested", 0755)
		assert.Nil(t, err)
	}()

	analyzer := CreateAnalyzer()
	dir := analyzer.AnalyzeDir(
		"test_dir", func(_, _ string) bool { return false }, false,
	).(*Dir)
	analyzer.GetDone().Wait()
	dir.UpdateStats(make(fs.HardLinkedItems))

	assert.Equal(t, "test_dir", dir.GetName())
	assert.Equal(t, 2, dir.ItemCount)
	assert.Equal(t, '.', dir.GetFlag())

	assert.Equal(t, "nested", dir.Files[0].GetName())
	assert.Equal(t, '!', dir.Files[0].GetFlag())
}
