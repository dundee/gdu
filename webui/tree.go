package webui

import (
	"container/heap"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"

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

// treeJSON is the recursive wire representation used by GET /api/v1/tree.
// Children is a slice of pointers so buildTree can hand out a stable address
// to each child (queued for later expansion) without it being invalidated by
// later appends growing a sibling slice.
type treeJSON struct {
	nodeJSON
	Children []*treeJSON `json:"children"`
}

func toNodeJSON(it fs.Item) nodeJSON {
	flag := ""
	if f := it.GetFlag(); f != ' ' && f != 0 {
		flag = string(f)
	}
	return nodeJSON{
		Name:      it.GetName(),
		Path:      it.GetPath(),
		IsDir:     it.IsDir(),
		Size:      it.GetSize(),
		Usage:     it.GetUsage(),
		ItemCount: it.GetItemCount(),
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
	// Reject ".." segments before cleaning. Archive entries (zip/tar) expose
	// virtual paths built from untrusted member names; filepath.Clean would
	// silently collapse a "../victim" segment into a real sibling path that
	// happens to exist in the scanned tree, letting a malicious archive entry
	// resolve to (and, for delete, remove) an unrelated real node.
	if slices.Contains(strings.Split(filepath.ToSlash(path), "/"), "..") {
		return nil, errOutsideRoot
	}
	cleanPath := filepath.Clean(path)
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

// isArchiveDescendant reports whether it sits below the root of a browsed
// zip/tar archive, i.e. its parent is itself inside an archive. Such nodes
// have a synthetic path (archive.zip/folder/file) that does not exist on
// disk, so os.RemoveAll and OS reveal cannot operate on them directly. The
// archive's own top-level node is not a descendant: its Parent is a real
// filesystem directory, and its GetPath() resolves to the real archive file.
func isArchiveDescendant(it fs.Item) bool {
	parent := it.GetParent()
	if parent == nil {
		return false
	}
	switch parent.GetType() {
	case "ZipDirectory", "TarDirectory":
		return true
	default:
		return false
	}
}

// realPathAncestor walks up from it until it finds a node whose GetPath
// resolves to a real filesystem path: either it itself, or the nearest
// ancestor that is not nested inside a browsed archive (the archive's own
// top-level node, whose Parent is a real directory).
func realPathAncestor(it fs.Item) fs.Item {
	for isArchiveDescendant(it) {
		it = it.GetParent()
	}
	return it
}

// isArchiveRoot reports whether it is the top-level node of a browsed
// zip/tar archive. Its GetPath still resolves to a real filesystem path, but
// that path names the archive file itself, not a directory: opening it would
// launch the archive's associated application (and may start extracting it)
// instead of revealing it in a file manager.
func isArchiveRoot(it fs.Item) bool {
	switch it.GetType() {
	case "ZipDirectory", "TarDirectory":
		return true
	default:
		return false
	}
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

// maxTreeNodes bounds how many nodes GET /api/v1/tree will serialize in one
// response. Without it, requesting the tree for a large root (e.g. "/")
// walks and JSON-encodes every scanned file at once, which can hang both the
// server and the browser parsing the payload.
//
// buildTree spends that budget best-first (largest usage across the whole
// tree first, via a priority queue), not depth-first. A naive depth-first
// walk would sink the entire budget into the single largest top-level
// directory's deepest descendants before ever expanding its siblings,
// leaving the rest of the tree empty; best-first instead fills in the
// biggest, most visually significant nodes wherever they are, which is what
// the treemap actually needs.
const maxTreeNodes = 5000

// pendingExpansion is a directory whose children have not yet been added to
// the output, queued with a pointer to the treeJSON node they attach to.
type pendingExpansion struct {
	item fs.Item
	slot *treeJSON
}

// expansionQueue is a max-heap of pendingExpansion ordered by disk usage,
// matching the size-desc ordering buildTree already uses for children.
type expansionQueue []pendingExpansion

func (q expansionQueue) Len() int           { return len(q) }
func (q expansionQueue) Less(i, j int) bool { return q[i].item.GetUsage() > q[j].item.GetUsage() }
func (q expansionQueue) Swap(i, j int)      { q[i], q[j] = q[j], q[i] }
func (q *expansionQueue) Push(x any)        { *q = append(*q, x.(pendingExpansion)) }
func (q *expansionQueue) Pop() any {
	old := *q
	n := len(old)
	last := old[n-1]
	*q = old[:n-1]
	return last
}

func buildTree(it fs.Item) treeJSON {
	root := &treeJSON{nodeJSON: toNodeJSON(it), Children: []*treeJSON{}}
	if !it.IsDir() {
		return *root
	}

	remaining := maxTreeNodes
	queue := &expansionQueue{{item: it, slot: root}}
	for queue.Len() > 0 && remaining > 0 {
		next := heap.Pop(queue).(pendingExpansion)
		for child := range next.item.GetFilesLocked(fs.SortBySize, fs.SortDesc) {
			if remaining <= 0 {
				break
			}
			remaining--
			childJSON := &treeJSON{nodeJSON: toNodeJSON(child), Children: []*treeJSON{}}
			next.slot.Children = append(next.slot.Children, childJSON)
			if child.IsDir() {
				heap.Push(queue, pendingExpansion{item: child, slot: childJSON})
			}
		}
	}
	return *root
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

	const sortOrderAsc = "asc"
	order := fs.SortDesc
	if orderParam == sortOrderAsc {
		order = fs.SortAsc
	}
	return sortBy, order
}
