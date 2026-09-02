package webui

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/dundee/gdu/v5/pkg/analyze"
	"github.com/dundee/gdu/v5/pkg/fs"
)

// errNotFound is returned when a requested path is not present in the tree.
var errNotFound = errors.New("node not found")

// errOutsideRoot is returned when a requested path escapes the scanned root.
var errOutsideRoot = errors.New("path is outside the scanned root")

// nodeJSON is the wire representation of a single tree item. Sizes are raw
// byte counts; the browser is responsible for human-readable formatting.
type nodeJSON struct {
	Name      string `json:"name"`
	Path      string `json:"path"`
	IsDir     bool   `json:"isDir"`
	Size      int64  `json:"size"`
	Usage     int64  `json:"usage"`
	ItemCount int64  `json:"itemCount"`
	Mtime     int64  `json:"mtime"`
	Flag      string `json:"flag,omitempty"`
}

// nodeResponse is the payload of GET /api/v1/nodes: the node itself, its
// breadcrumb ancestry, and only its immediate children.
type nodeResponse struct {
	Node        nodeJSON   `json:"node"`
	Breadcrumbs []nodeJSON `json:"breadcrumbs"`
	Children    []nodeJSON `json:"children"`
}

func toNodeJSON(it fs.Item) nodeJSON {
	flag := ""
	if f := it.GetFlag(); f != ' ' && f != 0 {
		flag = string(f)
	}
	return nodeJSON{
		// scanned roots are labelled with their absolute path, since their base
		// names collide when the roots come from different parent directories
		Name:      analyze.ItemDisplayName(it.GetParent(), it),
		Path:      it.GetPath(),
		IsDir:     it.IsDir(),
		Size:      it.GetSize(),
		Usage:     it.GetUsage(),
		ItemCount: fs.DisplayedItemCount(it),
		Mtime:     it.GetMtime().Unix(),
		Flag:      flag,
	}
}

// findNode locates an item by absolute path, descending from the scanned root.
// It guards against path traversal outside the root.
func (ui *UI) findNode(path string) (fs.Item, error) {
	ui.mu.RLock()
	root := ui.topDir
	rootPath := ui.topDirPath
	ui.mu.RUnlock()

	if root == nil {
		return nil, errNotFound
	}
	if rootPath == "" {
		rootPath = root.GetPath()
	}

	cleanRoot := filepath.Clean(rootPath)
	if path == "" {
		return root, nil
	}
	cleanPath := filepath.Clean(path)
	if cleanPath == cleanRoot {
		return root, nil
	}

	// The virtual top level dir has no path of its own, so paths are resolved
	// against each of the scanned roots it holds instead.
	if analyze.IsVirtualRootDir(root) {
		for child := range root.GetFiles(fs.SortByName, fs.SortAsc) {
			if node, err := descendFrom(child, child.GetPath(), cleanPath); err == nil {
				return node, nil
			}
		}
		return nil, errOutsideRoot
	}

	return descendFrom(root, cleanRoot, cleanPath)
}

// descendFrom walks from root, which lives at cleanRoot, down to cleanPath.
// It guards against path traversal outside the root.
func descendFrom(root fs.Item, cleanRoot, cleanPath string) (fs.Item, error) {
	if cleanPath == cleanRoot {
		return root, nil
	}

	rel, err := filepath.Rel(cleanRoot, cleanPath)
	if err != nil {
		return nil, errOutsideRoot
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return nil, errOutsideRoot
	}

	current := root
	for _, segment := range strings.Split(rel, string(os.PathSeparator)) {
		if segment == "" || segment == "." {
			continue
		}
		next, found := childByName(current, segment)
		if !found {
			return nil, errNotFound
		}
		current = next
	}
	return current, nil
}

func childByName(parent fs.Item, name string) (fs.Item, bool) {
	if !parent.IsDir() {
		return nil, false
	}
	for child := range parent.GetFilesLocked(fs.SortByName, fs.SortAsc) {
		if child.GetName() == name {
			return child, true
		}
	}
	return nil, false
}

// buildNodeResponse assembles a node, its breadcrumbs, and immediate children,
// sorted as requested.
func buildNodeResponse(it fs.Item, sortBy fs.SortBy, order fs.SortOrder) nodeResponse {
	resp := nodeResponse{
		Node:        toNodeJSON(it),
		Breadcrumbs: breadcrumbs(it),
		Children:    []nodeJSON{},
	}
	if it.IsDir() {
		for child := range it.GetFilesLocked(sortBy, order) {
			resp.Children = append(resp.Children, toNodeJSON(child))
		}
	}
	return resp
}

// breadcrumbs walks parents from the item up to the root and returns them
// root-first.
func breadcrumbs(it fs.Item) []nodeJSON {
	var chain []nodeJSON
	for cur := it; cur != nil; cur = cur.GetParent() {
		chain = append(chain, toNodeJSON(cur))
	}
	// reverse so the root comes first
	for i, j := 0, len(chain)-1; i < j; i, j = i+1, j-1 {
		chain[i], chain[j] = chain[j], chain[i]
	}
	return chain
}

// parseSort maps query parameters to fs sort enums, defaulting to size
// descending (largest first), which matches Gdu's usual presentation.
func parseSort(sortParam, orderParam string) (fs.SortBy, fs.SortOrder) {
	sortBy := fs.SortBySize
	switch sortParam {
	case "name":
		sortBy = fs.SortByName
	case "itemCount", "itemcount":
		sortBy = fs.SortByItemCount
	case "mtime":
		sortBy = fs.SortByMtime
	case "size", "":
		sortBy = fs.SortBySize
	}

	order := fs.SortDesc
	if orderParam == "asc" {
		order = fs.SortAsc
	}
	return sortBy, order
}
