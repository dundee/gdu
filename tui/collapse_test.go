package tui

import (
	"bytes"
	"testing"

	"github.com/dundee/gdu/v5/internal/testapp"
	"github.com/dundee/gdu/v5/pkg/analyze"
	"github.com/dundee/gdu/v5/pkg/fs"
	"github.com/stretchr/testify/assert"
)

func TestFindCollapsiblePath(t *testing.T) {
	// Test case 1: Non-directory item should return nil
	file := &analyze.File{
		Name: "test.txt",
	}
	result := findCollapsiblePath(file)
	assert.Nil(t, result)

	// Test case 2: Directory with files and subdirectories should not collapse
	dirWithFiles := &analyze.Dir{
		File: &analyze.File{
			Name: "mixed",
		},
		Files: []fs.Item{
			&analyze.Dir{
				File: &analyze.File{
					Name: "subdir",
				},
				Files: []fs.Item{},
			},
			&analyze.File{
				Name: "file.txt",
			},
		},
	}
	result = findCollapsiblePath(dirWithFiles)
	assert.Nil(t, result)

	// Test case 3: Directory with multiple subdirectories should not collapse
	dirWithMultiSubs := &analyze.Dir{
		File: &analyze.File{
			Name: "multi",
		},
		Files: []fs.Item{
			&analyze.Dir{
				File: &analyze.File{
					Name: "subdir1",
				},
				Files: []fs.Item{},
			},
			&analyze.Dir{
				File: &analyze.File{
					Name: "subdir2",
				},
				Files: []fs.Item{},
			},
		},
	}
	result = findCollapsiblePath(dirWithMultiSubs)
	assert.Nil(t, result)

	// Test case 4: Single subdirectory chain should collapse
	deepestDir := &analyze.Dir{
		File: &analyze.File{
			Name: "deep",
		},
		Files: []fs.Item{
			&analyze.File{
				Name: "finalfile.txt",
			},
		},
	}

	middleDir := &analyze.Dir{
		File: &analyze.File{
			Name: "middle",
		},
		Files: []fs.Item{deepestDir},
	}

	rootDir := &analyze.Dir{
		File: &analyze.File{
			Name: "root",
		},
		Files: []fs.Item{middleDir},
	}

	result = findCollapsiblePath(rootDir)
	assert.NotNil(t, result)
	assert.Equal(t, "root/middle/deep", result.DisplayName)
	assert.Equal(t, deepestDir, result.DeepestDir)
	assert.Equal(t, []string{"middle", "deep"}, result.Segments)

	// Test case 5: Directory with no subdirectories should not collapse
	emptyDir := &analyze.Dir{
		File: &analyze.File{
			Name: "empty",
		},
		Files: []fs.Item{},
	}
	result = findCollapsiblePath(emptyDir)
	assert.Nil(t, result)
}

func TestFindCollapsedParent(t *testing.T) {
	// Test case 1: Nil current directory
	result := findCollapsedParent(nil)
	assert.Nil(t, result)

	// Test case 2: Directory without parent
	rootDir := &analyze.Dir{
		File: &analyze.File{
			Name: "root",
		},
		Files: []fs.Item{},
	}
	result = findCollapsedParent(rootDir)
	assert.Nil(t, result)

	// Test case 3: Directory in a collapsed chain
	otherDir := &analyze.Dir{
		File: &analyze.File{
			Name: "other",
		},
		Files: []fs.Item{},
	}

	grandParent := &analyze.Dir{
		File: &analyze.File{
			Name: "grandparent",
		},
		Files: []fs.Item{otherDir},
	}
	otherDir.SetParent(grandParent)

	parent := &analyze.Dir{
		File: &analyze.File{
			Name: "parent",
		},
		Files: []fs.Item{},
	}
	parent.SetParent(grandParent)
	grandParent.AddFile(parent)

	child := &analyze.Dir{
		File: &analyze.File{
			Name: "child",
		},
		Files: []fs.Item{},
	}
	child.SetParent(parent)
	parent.AddFile(child)

	result = findCollapsedParent(child)
	assert.Equal(t, grandParent, result)

	// Test case 4: Directory not in a collapsed chain
	normalParent := &analyze.Dir{
		File: &analyze.File{
			Name: "normalparent",
		},
		Files: []fs.Item{
			&analyze.File{
				Name: "file.txt",
			},
		},
	}

	normalChild := &analyze.Dir{
		File: &analyze.File{
			Name: "normalchild",
		},
		Files: []fs.Item{},
	}
	normalChild.SetParent(normalParent)
	normalParent.AddFile(normalChild)

	result = findCollapsedParent(normalChild)
	assert.Equal(t, normalParent, result)
}

func TestFormatCollapsedRow(t *testing.T) {
	simScreen := testapp.CreateSimScreen()
	defer simScreen.Fini()

	app := testapp.CreateMockedApp(true)
	ui := CreateUI(app, simScreen, &bytes.Buffer{}, true, false, false, false)

	// Create a test collapsed path
	deepDir := &analyze.Dir{
		File: &analyze.File{
			Name:  "deep",
			Size:  1000,
			Usage: 800,
		},
		Files: []fs.Item{},
	}

	collapsedPath := &CollapsedPath{
		DisplayName: "level1/level2/deep",
		DeepestDir:  deepDir,
		Segments:    []string{"level1", "level2", "deep"},
	}

	// Test normal formatting
	result := ui.formatCollapsedRow(collapsedPath, 1000, 1000, false, false)
	assert.Contains(t, result, "level1/level2/deep")
	assert.Contains(t, result, "/") // Should have directory indicator

	// Test with marked flag
	ui.markedRows = map[int]struct{}{0: {}}
	result = ui.formatCollapsedRow(collapsedPath, 1000, 1000, true, false)
	assert.Contains(t, result, "✓") // Should have marked indicator

	// Test with ignored flag
	result = ui.formatCollapsedRow(collapsedPath, 1000, 1000, false, true)
	assert.Contains(t, result, "level1/level2/deep")

	// Test with ShowApparentSize
	ui.ShowApparentSize = true
	result = ui.formatCollapsedRow(collapsedPath, 1000, 1000, false, false)
	assert.Contains(t, result, "level1/level2/deep")

	// Test with showItemCount
	ui.showItemCount = true
	result = ui.formatCollapsedRow(collapsedPath, 1000, 1000, false, false)
	assert.Contains(t, result, "level1/level2/deep")

	// Test with showMtime
	ui.showMtime = true
	result = ui.formatCollapsedRow(collapsedPath, 1000, 1000, false, false)
	assert.Contains(t, result, "level1/level2/deep")

	// Test without colors
	ui2 := CreateUI(app, simScreen, &bytes.Buffer{}, false, false, false, false)
	result = ui2.formatCollapsedRow(collapsedPath, 1000, 1000, false, false)
	assert.Contains(t, result, "level1/level2/deep")
}

func TestCollapsedPathIntegration(t *testing.T) {
	simScreen := testapp.CreateSimScreen()
	defer simScreen.Fini()

	app := testapp.CreateMockedApp(true)
	ui := CreateUI(app, simScreen, &bytes.Buffer{}, true, false, false, false)

	// Create a directory structure that should be collapsed
	deepestDir := &analyze.Dir{
		File: &analyze.File{
			Name:  "deepest",
			Size:  100,
			Usage: 80,
		},
		Files: []fs.Item{
			&analyze.File{
				Name:  "file.txt",
				Size:  50,
				Usage: 40,
			},
		},
	}

	middleDir := &analyze.Dir{
		File: &analyze.File{
			Name:  "middle",
			Size:  100,
			Usage: 80,
		},
		Files: []fs.Item{deepestDir},
	}

	topDir := &analyze.Dir{
		File: &analyze.File{
			Name:  "top",
			Size:  100,
			Usage: 80,
		},
		Files: []fs.Item{middleDir},
	}

	deepestDir.SetParent(middleDir)
	middleDir.SetParent(topDir)

	ui.currentDir = topDir
	ui.topDir = topDir
	ui.topDirPath = "/test"
	ui.currentDirPath = "/test"
	ui.SetCollapsePath(true)

	// Test that showDir properly handles collapsed paths
	ui.showDir()

	// Test navigation into collapsed path
	ui.table.Select(1, 0) // Select the collapsed entry
	cell := ui.table.GetCell(1, 0)
	assert.NotNil(t, cell)

	ref := cell.GetReference()
	assert.NotNil(t, ref)
	assert.Equal(t, deepestDir, ref) // Should reference the deepest directory
}
