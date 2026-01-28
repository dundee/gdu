package analyze

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"

	"github.com/dundee/gdu/v5/pkg/fs"
	"github.com/stretchr/testify/assert"
)

func TestSequentialAnalyzerWithZipFile(t *testing.T) {
	// Create temporary directory and zip file
	tempDir := t.TempDir()
	zipPath := filepath.Join(tempDir, "test.zip")

	// Create test zip file
	createTestZipFile(t, zipPath)

	// Create analyzer
	analyzer := CreateSeqAnalyzer()
	analyzer.SetArchiveBrowsing(true)

	// Analyze directory (containing zip file)
	result := analyzer.AnalyzeDir(tempDir, func(string, string) bool { return false }, func(string) bool { return false })

	// Verify result
	assert.NotNil(t, result)
	assert.True(t, result.IsDir())

	// Find zip file
	var zipItem fs.Item
	for file := range result.GetFiles(fs.SortByName, fs.SortAsc) {
		if file.GetName() == "test.zip" {
			zipItem = file
			break
		}
	}

	assert.NotNil(t, zipItem, "should find zip file")
	assert.True(t, zipItem.IsDir(), "zip file should be treated as directory")

	// Verify zip file content
	zipFilesCount := 0
	foundTextFile := false
	for file := range zipItem.GetFiles(fs.SortByName, fs.SortAsc) {
		zipFilesCount++
		if file.GetName() == "test.txt" {
			foundTextFile = true
			assert.False(t, file.IsDir())
		}
	}
	assert.Greater(t, zipFilesCount, 0, "zip file should contain content")
	assert.True(t, foundTextFile, "should find test.txt in zip file")
}

func TestParallelAnalyzerWithZipFile(t *testing.T) {
	// Create temporary directory and zip file
	tempDir := t.TempDir()
	zipPath := filepath.Join(tempDir, "test.jar") // test jar file

	// Create test jar file (actually a zip file)
	createTestZipFile(t, zipPath)

	// Create parallel analyzer
	analyzer := CreateAnalyzer()
	analyzer.SetArchiveBrowsing(true)

	// Analyze directory
	result := analyzer.AnalyzeDir(tempDir, func(string, string) bool { return false }, func(string) bool { return false })

	// Verify result
	assert.NotNil(t, result)
	assert.True(t, result.IsDir())

	// Find jar file
	var jarItem fs.Item
	for file := range result.GetFiles(fs.SortByName, fs.SortAsc) {
		if file.GetName() == "test.jar" {
			jarItem = file
			break
		}
	}

	assert.NotNil(t, jarItem, "should find jar file")
	assert.True(t, jarItem.IsDir(), "jar file should be treated as directory")

	// Verify jar file content
	jarFilesCount := 0
	for range jarItem.GetFiles(fs.SortByName, fs.SortAsc) {
		jarFilesCount++
	}
	assert.Greater(t, jarFilesCount, 0, "jar file should contain content")
}

func TestZipFileWithNestedStructure(t *testing.T) {
	// Create temporary directory
	tempDir := t.TempDir()
	zipPath := filepath.Join(tempDir, "nested.zip")

	// Create zip file with complex nested structure
	createComplexZipFile(t, zipPath)

	// Create analyzer
	analyzer := CreateSeqAnalyzer()
	analyzer.SetArchiveBrowsing(true)

	// Analyze directory
	result := analyzer.AnalyzeDir(tempDir, func(string, string) bool { return false }, func(string) bool { return false })

	// Find zip file
	var zipItem fs.Item
	for file := range result.GetFiles(fs.SortByName, fs.SortAsc) {
		if file.GetName() == "nested.zip" {
			zipItem = file
			break
		}
	}

	assert.NotNil(t, zipItem)

	// Find deeply nested directory
	var level1Dir fs.Item
	for file := range zipItem.GetFiles(fs.SortByName, fs.SortAsc) {
		if file.GetName() == "level1" && file.IsDir() {
			level1Dir = file
			break
		}
	}
	assert.NotNil(t, level1Dir, "should find level1 directory")

	// Find level2 directory
	var level2Dir fs.Item
	for file := range level1Dir.GetFiles(fs.SortByName, fs.SortAsc) {
		if file.GetName() == "level2" && file.IsDir() {
			level2Dir = file
			break
		}
	}
	assert.NotNil(t, level2Dir, "should find level2 directory")

	// Find deepest nested file
	foundDeepFile := false
	for file := range level2Dir.GetFiles(fs.SortByName, fs.SortAsc) {
		if file.GetName() == "deep.txt" {
			foundDeepFile = true
			break
		}
	}
	assert.True(t, foundDeepFile, "should find deeply nested file")
}

// createComplexZipFile creates a zip file with complex nested structure
func createComplexZipFile(t *testing.T, zipPath string) {
	file, err := os.Create(zipPath)
	assert.NoError(t, err)
	defer file.Close()

	zipWriter := zip.NewWriter(file)
	defer zipWriter.Close()

	// Create multi-level nested structure
	files := []struct {
		name    string
		content string
	}{
		{"root.txt", "Root level file"},
		{"level1/file1.txt", "Level 1 file"},
		{"level1/level2/file2.txt", "Level 2 file"},
		{"level1/level2/deep.txt", "Deep nested file"},
		{"level1/level2/level3/file3.txt", "Level 3 file"},
		{"another/path/file.txt", "Another path file"},
	}

	for _, f := range files {
		writer, err := zipWriter.Create(f.name)
		assert.NoError(t, err)
		_, err = writer.Write([]byte(f.content))
		assert.NoError(t, err)
	}
}
