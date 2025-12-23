package common_test

import (
	"os"
	"path/filepath"
	"testing"

	log "github.com/sirupsen/logrus"

	"github.com/dundee/gdu/v5/internal/common"
	"github.com/stretchr/testify/assert"
)

func init() {
	log.SetLevel(log.WarnLevel)
}

func TestCreateIgnorePattern(t *testing.T) {
	re, err := common.CreateIgnorePattern([]string{"[abc]+"})

	assert.Nil(t, err)
	assert.True(t, re.MatchString("aa"))
}

func TestCreateIgnorePatternWithErr(t *testing.T) {
	re, err := common.CreateIgnorePattern([]string{"[[["})

	assert.NotNil(t, err)
	assert.Nil(t, re)
}

func TestEmptyIgnore(t *testing.T) {
	ui := &common.UI{}
	shouldBeIgnored := ui.CreateIgnoreFunc()

	assert.False(t, shouldBeIgnored("abc", "/abc"))
	assert.False(t, shouldBeIgnored("xxx", "/xxx"))
}

func TestIgnoreByAbsPath(t *testing.T) {
	ui := &common.UI{}
	ui.SetIgnoreDirPaths([]string{"/abc"})
	shouldBeIgnored := ui.CreateIgnoreFunc()

	assert.True(t, shouldBeIgnored("abc", "/abc"))
	assert.False(t, shouldBeIgnored("xxx", "/xxx"))
}

func TestIgnoreByPattern(t *testing.T) {
	ui := &common.UI{}
	err := ui.SetIgnoreDirPatterns([]string{"/[abc]+"})
	assert.Nil(t, err)
	shouldBeIgnored := ui.CreateIgnoreFunc()

	assert.True(t, shouldBeIgnored("aaa", "/aaa"))
	assert.True(t, shouldBeIgnored("aaa", "/aaabc"))
	assert.False(t, shouldBeIgnored("xxx", "/xxx"))
}

func TestIgnoreFromFile(t *testing.T) {
	file, err := os.OpenFile("ignore", os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		panic(err)
	}
	defer file.Close()

	if _, err := file.WriteString("/aaa\n"); err != nil {
		panic(err)
	}
	if _, err := file.WriteString("/aaabc\n"); err != nil {
		panic(err)
	}
	if _, err := file.WriteString("/[abd]+\n"); err != nil {
		panic(err)
	}

	ui := &common.UI{}
	err = ui.SetIgnoreFromFile("ignore")
	assert.Nil(t, err)
	shouldBeIgnored := ui.CreateIgnoreFunc()

	assert.True(t, shouldBeIgnored("aaa", "/aaa"))
	assert.True(t, shouldBeIgnored("aaabc", "/aaabc"))
	assert.True(t, shouldBeIgnored("aaabd", "/aaabd"))
	assert.False(t, shouldBeIgnored("xxx", "/xxx"))
}

func TestIgnoreFromNotExistingFile(t *testing.T) {
	ui := &common.UI{}
	err := ui.SetIgnoreFromFile("xxx")
	assert.NotNil(t, err)
}

func TestIgnoreHidden(t *testing.T) {
	ui := &common.UI{}
	ui.SetIgnoreHidden(true)
	shouldBeIgnored := ui.CreateIgnoreFunc()

	assert.True(t, shouldBeIgnored(".git", "/aaa/.git"))
	assert.True(t, shouldBeIgnored(".bbb", "/aaa/.bbb"))
	assert.False(t, shouldBeIgnored("xxx", "/xxx"))
}

func TestIgnoreByAbsPathAndHidden(t *testing.T) {
	ui := &common.UI{}
	ui.SetIgnoreDirPaths([]string{"/abc"})
	ui.SetIgnoreHidden(true)
	shouldBeIgnored := ui.CreateIgnoreFunc()

	assert.True(t, shouldBeIgnored("abc", "/abc"))
	assert.True(t, shouldBeIgnored(".git", "/aaa/.git"))
	assert.True(t, shouldBeIgnored(".bbb", "/aaa/.bbb"))
	assert.False(t, shouldBeIgnored("xxx", "/xxx"))
}

func TestIgnoreByAbsPathAndPattern(t *testing.T) {
	ui := &common.UI{}
	ui.SetIgnoreDirPaths([]string{"/abc"})
	err := ui.SetIgnoreDirPatterns([]string{"/[abc]+"})
	assert.Nil(t, err)
	shouldBeIgnored := ui.CreateIgnoreFunc()

	assert.True(t, shouldBeIgnored("abc", "/abc"))
	assert.True(t, shouldBeIgnored("aabc", "/aabc"))
	assert.True(t, shouldBeIgnored("ccc", "/ccc"))
	assert.False(t, shouldBeIgnored("xxx", "/xxx"))
}

func TestIgnoreByPatternAndHidden(t *testing.T) {
	ui := &common.UI{}
	err := ui.SetIgnoreDirPatterns([]string{"/[abc]+"})
	assert.Nil(t, err)
	ui.SetIgnoreHidden(true)
	shouldBeIgnored := ui.CreateIgnoreFunc()

	assert.True(t, shouldBeIgnored("abbc", "/abbc"))
	assert.True(t, shouldBeIgnored(".git", "/aaa/.git"))
	assert.True(t, shouldBeIgnored(".bbb", "/aaa/.bbb"))
	assert.False(t, shouldBeIgnored("xxx", "/xxx"))
}

func TestIgnoreByAll(t *testing.T) {
	ui := &common.UI{}
	ui.SetIgnoreDirPaths([]string{"/abc"})
	err := ui.SetIgnoreDirPatterns([]string{"/[abc]+"})
	assert.Nil(t, err)
	ui.SetIgnoreHidden(true)
	shouldBeIgnored := ui.CreateIgnoreFunc()

	assert.True(t, shouldBeIgnored("abc", "/abc"))
	assert.True(t, shouldBeIgnored("aabc", "/aabc"))
	assert.True(t, shouldBeIgnored(".git", "/aaa/.git"))
	assert.True(t, shouldBeIgnored(".bbb", "/aaa/.bbb"))
	assert.False(t, shouldBeIgnored("xxx", "/xxx"))
}

func TestIgnoreByRelativePath(t *testing.T) {
	ui := &common.UI{}
	ui.SetIgnoreDirPaths([]string{"test_dir/abc"})
	shouldBeIgnored := ui.CreateIgnoreFunc()

	assert.True(t, shouldBeIgnored("abc", "test_dir/abc"))
	absPath, err := filepath.Abs("test_dir/abc")
	assert.Nil(t, err)
	assert.True(t, shouldBeIgnored("abc", absPath))
	assert.False(t, shouldBeIgnored("xxx", "test_dir/xxx"))
}

func TestIgnoreByRelativePattern(t *testing.T) {
	ui := &common.UI{}
	err := ui.SetIgnoreDirPatterns([]string{"test_dir/[abc]+"})
	assert.Nil(t, err)
	shouldBeIgnored := ui.CreateIgnoreFunc()

	assert.True(t, shouldBeIgnored("abc", "test_dir/abc"))
	absPath, err := filepath.Abs("test_dir/abc")
	assert.Nil(t, err)
	assert.True(t, shouldBeIgnored("abc", absPath))
	assert.False(t, shouldBeIgnored("xxx", "test_dir/xxx"))
}

func TestIgnoreFromFileWithRelativePaths(t *testing.T) {
	file, err := os.OpenFile("ignore", os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		panic(err)
	}
	defer file.Close()
	defer os.Remove("ignore")

	if _, err := file.WriteString("test_dir/aaa\n"); err != nil {
		panic(err)
	}
	if _, err := file.WriteString("node_modules/[^/]+\n"); err != nil {
		panic(err)
	}

	ui := &common.UI{}
	err = ui.SetIgnoreFromFile("ignore")
	assert.Nil(t, err)
	shouldBeIgnored := ui.CreateIgnoreFunc()

	assert.True(t, shouldBeIgnored("aaa", "test_dir/aaa"))
	absPath, err := filepath.Abs("test_dir/aaa")
	assert.Nil(t, err)
	assert.True(t, shouldBeIgnored("aaa", absPath))
	assert.False(t, shouldBeIgnored("xxx", "test_dir/xxx"))
}

func TestShouldFileBeIgnoredByType(t *testing.T) {
	tests := []struct {
		name           string
		ignoreTypes    []string
		filename       string
		expectedIgnored bool
	}{
		{
			name:           "no ignore types",
			ignoreTypes:    []string{},
			filename:       "test.yaml",
			expectedIgnored: false,
		},
		{
			name:           "ignore yaml",
			ignoreTypes:    []string{"yaml"},
			filename:       "test.yaml",
			expectedIgnored: true,
		},
		{
			name:           "ignore json",
			ignoreTypes:    []string{"json"},
			filename:       "test.json",
			expectedIgnored: true,
		},
		{
			name:           "ignore multiple types",
			ignoreTypes:    []string{"yaml", "json"},
			filename:       "test.yaml",
			expectedIgnored: true,
		},
		{
			name:           "ignore multiple types - not matched",
			ignoreTypes:    []string{"yaml", "json"},
			filename:       "test.txt",
			expectedIgnored: false,
		},
		{
			name:           "ignore with exclamation",
			ignoreTypes:    []string{"!yaml"},
			filename:       "test.yaml",
			expectedIgnored: true,
		},
		{
			name:           "ignore with uppercase",
			ignoreTypes:    []string{"YAML"},
			filename:       "test.yaml",
			expectedIgnored: true,
		},
		{
			name:           "ignore file without extension",
			ignoreTypes:    []string{"yaml"},
			filename:       "test",
			expectedIgnored: false,
		},
		{
			name:           "ignore with dot in extension",
			ignoreTypes:    []string{".yaml"},
			filename:       "test.yaml",
			expectedIgnored: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ui := &common.UI{}
			ui.SetIgnoreTypes(tt.ignoreTypes)
			
			actual := ui.ShouldFileBeIgnoredByType(tt.filename)
			assert.Equal(t, tt.expectedIgnored, actual)
		})
	}
}

func TestShouldFileBeIncludedByType(t *testing.T) {
	tests := []struct {
		name             string
		includeTypes     []string
		filename         string
		expectedIncluded bool
	}{
		{
			name:             "no include types",
			includeTypes:     []string{},
			filename:         "test.yaml",
			expectedIncluded: true,
		},
		{
			name:             "include yaml",
			includeTypes:     []string{"yaml"},
			filename:         "test.yaml",
			expectedIncluded: true,
		},
		{
			name:             "include json",
			includeTypes:     []string{"json"},
			filename:         "test.json",
			expectedIncluded: true,
		},
		{
			name:             "include multiple types",
			includeTypes:     []string{"yaml", "json"},
			filename:         "test.yaml",
			expectedIncluded: true,
		},
		{
			name:             "include multiple types - not matched",
			includeTypes:     []string{"yaml", "json"},
			filename:         "test.txt",
			expectedIncluded: false,
		},
		{
			name:             "include with exclamation",
			includeTypes:     []string{"!yaml"},
			filename:         "test.yaml",
			expectedIncluded: true,
		},
		{
			name:             "include with uppercase",
			includeTypes:     []string{"YAML"},
			filename:         "test.yaml",
			expectedIncluded: true,
		},
		{
			name:             "include file without extension",
			includeTypes:     []string{"yaml"},
			filename:         "test",
			expectedIncluded: false,
		},
		{
			name:             "include with dot in extension",
			includeTypes:     []string{".yaml"},
			filename:         "test.yaml",
			expectedIncluded: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ui := &common.UI{}
			ui.SetIncludeTypes(tt.includeTypes)
			
			actual := ui.ShouldFileBeIncludedByType(tt.filename)
			assert.Equal(t, tt.expectedIncluded, actual)
		})
	}
}

func TestCreateFileTypeFilter(t *testing.T) {
	tests := []struct {
		name             string
		includeTypes     []string
		ignoreTypes      []string
		filename         string
		expectedFiltered bool
	}{
		{
			name:             "no filters",
			includeTypes:     []string{},
			ignoreTypes:      []string{},
			filename:         "test.yaml",
			expectedFiltered: false,
		},
		{
			name:             "include filter - matched",
			includeTypes:     []string{"yaml"},
			ignoreTypes:      []string{},
			filename:         "test.yaml",
			expectedFiltered: false,
		},
		{
			name:             "include filter - not matched",
			includeTypes:     []string{"json"},
			ignoreTypes:      []string{},
			filename:         "test.yaml",
			expectedFiltered: true,
		},
		{
			name:             "ignore filter - matched",
			includeTypes:     []string{},
			ignoreTypes:      []string{"yaml"},
			filename:         "test.yaml",
			expectedFiltered: true,
		},
		{
			name:             "ignore filter - not matched",
			includeTypes:     []string{},
			ignoreTypes:      []string{"json"},
			filename:         "test.yaml",
			expectedFiltered: false,
		},
		{
			name:             "include filter takes precedence",
			includeTypes:     []string{"yaml"},
			ignoreTypes:      []string{"yaml"},
			filename:         "test.yaml",
			expectedFiltered: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ui := &common.UI{}
			ui.SetIncludeTypes(tt.includeTypes)
			ui.SetIgnoreTypes(tt.ignoreTypes)
			
			filter := ui.CreateFileTypeFilter()
			actual := filter(tt.filename)
			assert.Equal(t, tt.expectedFiltered, actual)
		})
	}
}

func TestFileTypeFilterWithRealFiles(t *testing.T) {
	// Create a temporary directory with test files
	tmpDir := t.TempDir()

	// Create test files
	testFiles := []struct {
		name     string
		content  string
		expected bool // expected to be included
	}{
		{"test.yaml", "key: value", true},
		{"test.json", "{\"key\": \"value\"}", true},
		{"test.txt", "plain text", false},
		{"test.go", "package main", false},
		{"noextension", "no extension", false},
	}

	for _, tf := range testFiles {
		filePath := filepath.Join(tmpDir, tf.name)
		err := os.WriteFile(filePath, []byte(tf.content), 0644)
		assert.NoError(t, err)
	}

	// Test include filter
	ui := &common.UI{}
	ui.SetIncludeTypes([]string{"yaml", "json"})
	filter := ui.CreateFileTypeFilter()

	for _, tf := range testFiles {
		actual := filter(tf.name)
		expected := !tf.expected // filter returns true if file should be filtered out
		assert.Equal(t, expected, actual, "Failed for file: %s", tf.name)
	}

	// Test ignore filter
	ui2 := &common.UI{}
	ui2.SetIgnoreTypes([]string{"txt", "go"})
	filter2 := ui2.CreateFileTypeFilter()

	for _, tf := range testFiles {
		actual := filter2(tf.name)
		// For ignore filter, yaml and json should not be filtered, txt and go should be filtered
		expected := tf.name == "test.txt" || tf.name == "test.go"
		assert.Equal(t, expected, actual, "Failed for file: %s", tf.name)
	}
}
