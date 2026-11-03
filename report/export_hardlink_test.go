//go:build !windows

package report

import (
	"bytes"
	"os"
	"testing"

	"github.com/dundee/gdu/v5/internal/testdir"
	"github.com/dundee/gdu/v5/pkg/analyze"
	"github.com/dundee/gdu/v5/pkg/fs"
	"github.com/stretchr/testify/assert"
)

func TestAnalyzePathWithDepthKeepsHardLinkAttrs(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()
	assert.Nil(t, os.Link("test_dir/nested/file2", "test_dir/nested/file3"))

	var output, reportOutput bytes.Buffer
	ui := CreateExportUI(&output, &reportOutput, false, false, false, 0, 2, false, nil)
	ui.SetIgnoreDirPaths([]string{"/xxx"})
	assert.Nil(t, ui.AnalyzePath("test_dir", nil))

	// the depth filter copies leaves through the fs.Item interface; Flag and
	// Mli must survive the copy or hard links lose their ino/hlnkc attributes
	assert.Contains(t, reportOutput.String(), `"hlnkc":true`)
	assert.Contains(t, reportOutput.String(), `"ino":`)
}

func TestAnalyzePathWithSelectedAttributesIncludingIno(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()
	assert.Nil(t, os.Link("test_dir/nested/file2", "test_dir/nested/file3"))

	var output, reportOutput bytes.Buffer
	ui := CreateExportUI(&output, &reportOutput, false, false, false, 0, 0, false, fs.JSONAttributes{
		"name":  {},
		"asize": {},
		"dsize": {},
		"items": {},
		"ino":   {},
	})
	ui.SetIgnoreDirPaths([]string{"/xxx"})
	assert.Nil(t, ui.AnalyzePath("test_dir", nil))

	assert.Contains(t, reportOutput.String(), `"hlnkc":true`)
	assert.Contains(t, reportOutput.String(), `"ino":`)

	imported, err := ReadAnalysis(&reportOutput)
	assert.NoError(t, err)
	assert.NotNil(t, imported)

	var foundHardlink bool
	for _, item := range imported.Files {
		if dir, ok := item.(*analyze.Dir); ok && dir.GetName() == "nested" {
			for _, f := range dir.Files {
				if f.GetFlag() == 'H' {
					foundHardlink = true
					assert.NotZero(t, f.GetMultiLinkedInode())
				}
			}
		}
	}
	assert.True(t, foundHardlink)
}

func TestAnalyzePathWithSelectedAttributesExcludingIno(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()
	assert.Nil(t, os.Link("test_dir/nested/file2", "test_dir/nested/file3"))

	var output, reportOutput bytes.Buffer
	ui := CreateExportUI(&output, &reportOutput, false, false, false, 0, 0, false, fs.JSONAttributes{
		"name":  {},
		"asize": {},
		"dsize": {},
		"items": {},
	})
	ui.SetIgnoreDirPaths([]string{"/xxx"})
	assert.Nil(t, ui.AnalyzePath("test_dir", nil))

	assert.NotContains(t, reportOutput.String(), `"hlnkc"`)
	assert.NotContains(t, reportOutput.String(), `"ino"`)
}
