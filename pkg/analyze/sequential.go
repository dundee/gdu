package analyze

import (
	"context"

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
func (a *SequentialAnalyzer) AnalyzeDir(ctx context.Context, path string, opts common.ScanOptions) fs.Item {
	return a.analyzeTree(ctx, path, opts, false)
}
