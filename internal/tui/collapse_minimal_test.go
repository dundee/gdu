package tui

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCollapsedPathStruct(t *testing.T) {
	// Test CollapsedPath struct creation and fields
	cp := &CollapsedPath{
		DisplayName: "test/path",
		DeepestDir:  nil,
		Segments:    []string{"test", "path"},
	}

	assert.Equal(t, "test/path", cp.DisplayName)
	assert.Nil(t, cp.DeepestDir)
	assert.Equal(t, []string{"test", "path"}, cp.Segments)
}

func TestFindCollapsedParentNilCases(t *testing.T) {
	// Test nil input
	result := findCollapsedParent(nil)
	assert.Nil(t, result)
}

// Test that our new functions exist and don't panic with basic inputs
func TestFunctionExistence(t *testing.T) {
	// Test that findCollapsiblePath exists and handles nil gracefully
	result := findCollapsiblePath(nil)
	assert.Nil(t, result)
}
