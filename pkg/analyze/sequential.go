package analyze

import (
	"github.com/dundee/gdu/v5/internal/common"
	"github.com/dundee/gdu/v5/pkg/fs"
)

// SequentialAnalyzer implements Analyzer
type SequentialAnalyzer struct {
	BaseAnalyzer
}

// CreateSeqAnalyzer returns Analyzer
func CreateSeqAnalyzer() *SequentialAnalyzer {
	a := &SequentialAnalyzer{}
	a.Init()
	return a
}

// AnalyzeDir analyzes given path
func (a *SequentialAnalyzer) AnalyzeDir(
	path string, ignore common.ShouldDirBeIgnored, fileTypeFilter common.ShouldFileBeIgnored,
) fs.Item {
	return a.analyzeTree(path, ignore, fileTypeFilter, false)
}
