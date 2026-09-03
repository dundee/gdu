package webui

import (
	"path/filepath"
	"testing"

	"github.com/dundee/gdu/v5/pkg/analyze"
	"github.com/dundee/gdu/v5/pkg/fs"
)

func TestAnalyzePathsGroupsRootsUnderVirtualDir(t *testing.T) {
	ui := newTestUI()
	first := makeTree(t)
	second := makeTree(t)

	if err := ui.AnalyzePaths([]string{first, second}); err != nil {
		t.Fatalf("AnalyzePaths: %v", err)
	}
	waitDone(t, ui)

	if !analyze.IsVirtualRootDir(ui.topDir) {
		t.Fatalf("expected virtual root, got %T named %q", ui.topDir, ui.topDir.GetName())
	}
	if ui.topDirPath != analyze.VirtualRootName {
		t.Errorf("topDirPath = %q, want %q", ui.topDirPath, analyze.VirtualRootName)
	}

	// both roots are reachable and keep their real paths
	for _, path := range []string{first, second} {
		if _, err := ui.findNode(path); err != nil {
			t.Errorf("findNode(%q): %v", path, err)
		}
	}

	// so is a directory inside one of them
	if _, err := ui.findNode(filepath.Join(first, "sub")); err != nil {
		t.Errorf("findNode(sub of first root): %v", err)
	}

	// and the virtual root itself, as reached from a breadcrumb
	node, err := ui.findNode(analyze.VirtualRootName)
	if err != nil {
		t.Fatalf("findNode(virtual root): %v", err)
	}
	resp := buildNodeResponse(node, fs.SortBySize, fs.SortDesc)
	if len(resp.Children) != 2 {
		t.Fatalf("virtual root has %d children, want 2", len(resp.Children))
	}

	// roots are labelled by absolute path, as their base names can collide
	for _, child := range resp.Children {
		if child.Name != child.Path {
			t.Errorf("root labelled %q, want its path %q", child.Name, child.Path)
		}
	}
}

func TestFindNodeRejectsPathOutsideEveryRoot(t *testing.T) {
	ui := newTestUI()

	if err := ui.AnalyzePaths([]string{makeTree(t), makeTree(t)}); err != nil {
		t.Fatalf("AnalyzePaths: %v", err)
	}
	waitDone(t, ui)

	if _, err := ui.findNode(filepath.Join(t.TempDir(), "elsewhere")); err == nil {
		t.Error("expected an error for a path outside every scanned root")
	}
}

func TestAnalyzePathsWithSinglePathHasNoVirtualDir(t *testing.T) {
	ui := newTestUI()
	root := makeTree(t)

	if err := ui.AnalyzePaths([]string{root}); err != nil {
		t.Fatalf("AnalyzePaths: %v", err)
	}
	waitDone(t, ui)

	if analyze.IsVirtualRootDir(ui.topDir) {
		t.Error("single path must not be wrapped in a virtual root")
	}
	if ui.topDirPath != root {
		t.Errorf("topDirPath = %q, want %q", ui.topDirPath, root)
	}
}
