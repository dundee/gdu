package analyze

import (
	"context"
	"testing"

	"github.com/dundee/gdu/v5/internal/common"
	"github.com/dundee/gdu/v5/internal/testdir"
	"github.com/stretchr/testify/assert"
)

func TestSequentialAnalyzerAnalyzeDirWithIgnoreDir(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	analyzer := CreateSeqAnalyzer()
	dir := analyzer.AnalyzeDir(context.Background(), "test_dir", common.ScanOptions{
		IgnoreDir:  func(name, _ string) bool { return name == "nested" },
		IgnoreFile: func(_ string) bool { return false },
	}).(*Dir)

	assert.NotNil(t, dir)
	assert.Equal(t, "test_dir", dir.Name)
	// Should have fewer items since nested directory was ignored
	assert.Less(t, dir.ItemCount, int64(5))
}
