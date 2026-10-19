package webui

import (
	"cmp"
	"net/http"
	"os"
	"slices"
	"sync"

	log "github.com/sirupsen/logrus"

	"github.com/dundee/gdu/v5/pkg/cleanup"
	"github.com/dundee/gdu/v5/pkg/fs"
)

// categoryNodePrefix marks the synthetic category nodes of the "By category"
// projection's tree. Such a path never resolves through findNode, so actions
// on it are refused.
const categoryNodePrefix = "category:"

// cleanupCache memoizes the classification of the current tree. It is valid
// while the root and the tree generation it was computed for are current.
type cleanupCache struct {
	mu   sync.Mutex
	root fs.Item
	gen  uint64
	res  *cleanup.Result
}

// userHomeDir returns the home directory "~/" in cleanup rules stands for, or
// "" when it is unknown, in which case home-relative rules match nothing.
func userHomeDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		log.Printf("webui: cleanup rules relative to home are disabled: %s", err)
		return ""
	}
	return home
}

// SetCleanupConfig replaces the default classification with one that also
// applies the user's rules and exclusions.
func (ui *UI) SetCleanupConfig(cfg cleanup.Config) error {
	classifier, err := cleanup.NewClassifier(cfg, userHomeDir())
	if err != nil {
		return err
	}
	ui.classifier = classifier
	return nil
}

// cleanupResult returns the classification of the current tree, computing it
// on first use after a scan or a deletion. It is nil while scanning.
func (ui *UI) cleanupResult() *cleanup.Result {
	ui.mu.RLock()
	root, scanning := ui.topDir, ui.scanning
	ui.mu.RUnlock()
	if root == nil || scanning {
		return nil
	}

	gen := ui.treeGen.Load()
	c := &ui.cleanup
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.res != nil && c.root == root && c.gen == gen {
		return c.res
	}
	seq := ui.removalSeq.Load()
	res := ui.classifier.Analyze(root)
	if seq%2 != 0 || ui.removalSeq.Load() != seq {
		// a deletion ran during the walk, which may have seen the tree
		// before or after it, so the result cannot be updated for it
		return res
	}
	c.root, c.gen, c.res = root, gen, res
	return c.res
}

// requireCleanupResult is cleanupResult for requests that cannot be answered
// without it: when there is none, it responds 503 and returns nil.
func (ui *UI) requireCleanupResult(w http.ResponseWriter) *cleanup.Result {
	res := ui.cleanupResult()
	if res == nil {
		writeError(w, http.StatusServiceUnavailable, "classification is unavailable while scanning")
	}
	return res
}

// treeChanged invalidates everything derived from the tree.
func (ui *UI) treeChanged() {
	ui.treeGen.Add(1)
}

// removeNode runs removeFunc and updates the classification for the removal.
// It reports whether the item is gone from the tree.
func (ui *UI) removeNode(removeFunc func(fs.Item, fs.Item) error, parent, node fs.Item) (bool, error) {
	ui.removalSeq.Add(1)
	defer ui.removalSeq.Add(1)

	if err := removeFunc(parent, node); err != nil {
		// a failed removal may still have removed part of the tree
		ui.treeChanged()
		return false, err
	}
	// a trash command may succeed and leave the item in place
	if current, ok := childByName(parent, node.GetName()); ok && current == node {
		return false, nil
	}
	ui.itemRemoved(node)
	return true, nil
}

// itemRemoved updates the cached classification for one item that was removed
// from the tree, instead of invalidating it.
func (ui *UI) itemRemoved(it fs.Item) {
	c := &ui.cleanup
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.res != nil && c.gen == ui.treeGen.Load() {
		c.res.Forget(it)
	}
}

type categoryJSON struct {
	cleanup.CategoryInfo
	cleanup.Amount
}

// categoriesResponse is the payload of GET /api/v1/categories: the space each
// category takes within the requested directory.
type categoriesResponse struct {
	Total      cleanup.Amount `json:"total"`
	Categories []categoryJSON `json:"categories"`
}

func (ui *UI) handleCategories(w http.ResponseWriter, r *http.Request) {
	node, err := ui.findNode(r.URL.Query().Get("path"))
	if err != nil {
		writeNodeError(w, err)
		return
	}
	res := ui.requireCleanupResult(w)
	if res == nil {
		return
	}

	totals := res.Totals(node)
	resp := categoriesResponse{
		Total:      cleanup.Amount{Usage: node.GetUsage(), Size: node.GetSize()},
		Categories: make([]categoryJSON, 0, len(cleanup.Categories)),
	}
	for _, info := range cleanup.Categories {
		resp.Categories = append(resp.Categories, categoryJSON{CategoryInfo: info, Amount: totals[info.ID]})
	}
	writeJSON(w, http.StatusOK, resp)
}

// buildCategoryTree regroups the candidates below it by category: a synthetic
// node per category holds the matched items (the topmost ones, which are
// units) with their real paths, largest first. Category nodes report the full
// category total even when the node budget cuts off their smallest items.
func buildCategoryTree(it fs.Item, res *cleanup.Result) treeJSON {
	root := treeJSON{nodeJSON: toNodeJSON(it, res), Children: []*treeJSON{}}
	totals := res.Totals(it)

	units := make(map[cleanup.Category][]fs.Item)
	collectUnits(it, res, units)

	type candidate struct {
		category cleanup.Category
		item     fs.Item
	}
	var all []candidate
	for category, items := range units {
		for _, item := range items {
			all = append(all, candidate{category, item})
		}
	}
	slices.SortFunc(all, func(a, b candidate) int {
		return cmp.Compare(b.item.GetUsage(), a.item.GetUsage())
	})
	if len(all) > maxTreeNodes {
		all = all[:maxTreeNodes]
	}

	nodes := make(map[cleanup.Category]*treeJSON)
	for _, info := range cleanup.Categories {
		amount, ok := totals[info.ID]
		if !ok {
			continue
		}
		node := &treeJSON{
			nodeJSON: nodeJSON{
				Name:      info.Label,
				Path:      categoryNodePrefix + string(info.ID),
				IsDir:     true,
				Size:      amount.Size,
				Usage:     amount.Usage,
				ItemCount: int64(len(units[info.ID])),
				Category:  info.ID,
			},
			Children: []*treeJSON{},
		}
		nodes[info.ID] = node
		root.Children = append(root.Children, node)
	}
	for _, c := range all {
		node := nodes[c.category]
		child := &treeJSON{nodeJSON: toNodeJSON(c.item, res), Children: []*treeJSON{}}
		node.Children = append(node.Children, child)
	}
	return root
}

// collectUnits gathers the matched items below it, descending only into
// directories that hold candidates.
func collectUnits(it fs.Item, res *cleanup.Result, units map[cleanup.Category][]fs.Item) {
	if m, ok := res.Match(it); ok {
		units[m.Category] = append(units[m.Category], it)
		return
	}
	if !it.IsDir() || len(res.Totals(it)) == 0 {
		return
	}
	for child := range it.GetFilesLocked(fs.SortBySize, fs.SortDesc) {
		collectUnits(child, res, units)
	}
}
