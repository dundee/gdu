package session

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dundee/gdu/v5/internal/common"
	"github.com/dundee/gdu/v5/pkg/analyze"
	"github.com/dundee/gdu/v5/pkg/fs"
)

// ctx is the context of the scans that are not cancelled.
var ctx = context.Background()

// makeTree creates <tmp>/<name>/{dir/a, dir/b/c, file} and returns its path.
func makeTree(t *testing.T, name string) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), name)
	require.NoError(t, os.MkdirAll(filepath.Join(root, "dir", "b"), 0o755))
	writeFile(t, filepath.Join(root, "dir", "a"), "aaaa")
	writeFile(t, filepath.Join(root, "dir", "b", "c"), "cc")
	writeFile(t, filepath.Join(root, "file"), "file")
	return root
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
}

func newSession(analyzer common.Analyzer) *Session {
	return New(&common.UI{Analyzer: analyzer})
}

func child(t *testing.T, dir fs.Item, name string) fs.Item {
	t.Helper()
	for entry := range dir.GetFilesLocked(fs.SortByName, fs.SortAsc) {
		if entry.GetName() == name {
			return entry
		}
	}
	t.Fatalf("%s has no entry %q", dir.GetPath(), name)
	return nil
}

func entries(dir fs.Item) []fs.Item {
	var items []fs.Item
	for entry := range dir.GetFilesLocked(fs.SortByName, fs.SortAsc) {
		items = append(items, entry)
	}
	return items
}

// fastProgress makes observers called every millisecond for the test.
func fastProgress(t *testing.T) {
	t.Helper()
	interval := progressInterval
	progressInterval = time.Millisecond
	t.Cleanup(func() { progressInterval = interval })
}

// gatedAnalyzer is a real analyzer whose scans of each root wait until the
// test lets them go, and whose result computes its totals only when allowed.
type gatedAnalyzer struct {
	*analyze.ParallelAnalyzer
	started  chan string
	release  chan struct{}
	finalize chan struct{}
}

func newGatedAnalyzer() *gatedAnalyzer {
	return &gatedAnalyzer{
		ParallelAnalyzer: analyze.CreateAnalyzer(),
		started:          make(chan string, 10),
		release:          make(chan struct{}),
		finalize:         make(chan struct{}),
	}
}

func (a *gatedAnalyzer) AnalyzeDir(ctx context.Context, path string, opts common.ScanOptions) fs.Item {
	a.started <- path
	<-a.release
	return &gatedTotals{Item: a.ParallelAnalyzer.AnalyzeDir(ctx, path, opts), gate: a.finalize}
}

// gatedTotals waits for its gate before computing its totals.
type gatedTotals struct {
	fs.Item
	gate chan struct{}
}

func (g *gatedTotals) UpdateStats(links fs.HardLinkedItems) {
	<-g.gate
	g.Item.UpdateStats(links)
}

// GetItemStats is how a parent, such as the virtual top level dir, computes
// the totals of its entries.
func (g *gatedTotals) GetItemStats(links fs.HardLinkedItems, filtering bool) (count, size, usage int64) {
	<-g.gate
	return g.Item.GetItemStats(links, filtering)
}

// recorder keeps what an observer was told.
type recorder struct {
	mu   sync.Mutex
	seen []Progress
}

func (r *recorder) observe(p Progress) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.seen = append(r.seen, p)
}

func (r *recorder) count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.seen)
}

// waitFor waits until the observer has been told a progress matching want.
func (r *recorder) waitFor(t *testing.T, want func(Progress) bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		r.mu.Lock()
		for _, p := range r.seen {
			if want(p) {
				r.mu.Unlock()
				return
			}
		}
		r.mu.Unlock()
		time.Sleep(time.Millisecond)
	}
	t.Fatal("the observer was never told the expected progress")
}

func TestScanOnePath(t *testing.T) {
	path := makeTree(t, "scan")
	s := newSession(analyze.CreateAnalyzer())

	root := s.Scan(ctx, []string{path}, nil)

	assert.Same(t, root, s.Root())
	assert.Equal(t, path, s.RootPath())
	assert.Equal(t, []string{path}, s.RootPaths())
	// dir, a, b, c and file, without counting the root itself
	assert.Equal(t, int64(5), fs.DisplayedItemCount(root))
}

func TestScanSeveralPathsGroupsThemUnderVirtualRoot(t *testing.T) {
	first, second := makeTree(t, "first"), makeTree(t, "second")
	s := newSession(analyze.CreateAnalyzer())

	root := s.Scan(ctx, []string{first, second}, nil)

	assert.True(t, analyze.IsVirtualRootDir(root))
	assert.ElementsMatch(t, []string{first, second}, s.RootPaths())
	assert.Equal(t, int64(12), fs.DisplayedItemCount(root))
}

func TestScanTellsObserverTheProgress(t *testing.T) {
	fastProgress(t)
	first, second := makeTree(t, "first"), makeTree(t, "second")
	analyzer := newGatedAnalyzer()
	s := newSession(analyzer)
	var rec recorder

	done := make(chan fs.Item)
	go func() { done <- s.Scan(ctx, []string{first, second}, rec.observe) }()

	assert.Equal(t, first, <-analyzer.started)
	rec.waitFor(t, func(p Progress) bool { return p.Phase == Scanning && p.Root == 1 && p.Roots == 2 })
	analyzer.release <- struct{}{}

	assert.Equal(t, second, <-analyzer.started)
	rec.waitFor(t, func(p Progress) bool { return p.Phase == Scanning && p.Root == 2 })
	analyzer.release <- struct{}{}

	// the roots are grouped under the virtual root, whose totals ask theirs
	rec.waitFor(t, func(p Progress) bool { return p.Phase == Finalizing })
	close(analyzer.finalize)
	<-done

	told := rec.count()
	time.Sleep(20 * time.Millisecond)
	assert.Equal(t, told, rec.count(), "the observer was told progress after the scan returned")
}

func TestCancelKeepsWhatWasFoundAndSkipsRemainingRoots(t *testing.T) {
	first, second := makeTree(t, "first"), makeTree(t, "second")
	analyzer := newGatedAnalyzer()
	close(analyzer.finalize)
	s := newSession(analyzer)

	scan, cancel := context.WithCancel(context.Background())
	done := make(chan fs.Item)
	go func() { done <- s.Scan(scan, []string{first, second}, nil) }()

	assert.Equal(t, first, <-analyzer.started)
	cancel()
	analyzer.release <- struct{}{}
	root := <-done

	assert.Empty(t, analyzer.started, "the second root was scanned after cancelling")
	require.True(t, analyze.IsVirtualRootDir(root))
	roots := entries(root)
	require.Len(t, roots, 1)
	assert.Equal(t, first, roots[0].GetPath())
	// cancelled before it started walking, so the first root was left empty
	assert.Equal(t, int64(0), fs.DisplayedItemCount(roots[0]))

	// the next scan is not cancelled
	go func() { done <- s.Scan(context.Background(), []string{second}, nil) }()
	<-analyzer.started
	analyzer.release <- struct{}{}
	assert.Equal(t, int64(5), fs.DisplayedItemCount(<-done))
}

// A frontend may cancel a scan before the goroutine running it has started;
// the scan must still stop.
func TestScanCancelledBeforeItStarts(t *testing.T) {
	first, second := makeTree(t, "first"), makeTree(t, "second")
	s := newSession(analyze.CreateAnalyzer())
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()

	root := s.Scan(cancelled, []string{first, second}, nil)

	roots := entries(root)
	require.Len(t, roots, 1)
	assert.Equal(t, int64(0), fs.DisplayedItemCount(roots[0]))
}

func TestRescanReplacesTheDirectory(t *testing.T) {
	path := makeTree(t, "scan")
	s := newSession(analyze.CreateAnalyzer())
	root := s.Scan(ctx, []string{path}, nil)
	old := child(t, root, "dir")
	writeFile(t, filepath.Join(path, "dir", "added"), "added")

	rescanned := s.Rescan(ctx, old, nil)

	assert.Same(t, root, s.Root())
	assert.Same(t, rescanned, child(t, root, "dir"))
	assert.NotSame(t, old, rescanned)
	assert.Equal(t, path+"/dir", rescanned.GetPath())
	assert.Equal(t, int64(6), fs.DisplayedItemCount(root))
	// a, b, b/c and the added file
	assert.Equal(t, int64(4), fs.DisplayedItemCount(rescanned))
}

// Scanned roots under the virtual top level dir can share a base name; a
// rescan must replace the one rescanned.
func TestRescanReplacesTheRootItWasGiven(t *testing.T) {
	first, second := makeTree(t, "data"), makeTree(t, "data")
	require.Equal(t, filepath.Base(first), filepath.Base(second))
	s := newSession(analyze.CreateAnalyzer())
	top := s.Scan(ctx, []string{first, second}, nil)
	var firstRoot, secondRoot fs.Item
	for _, root := range entries(top) {
		if root.GetPath() == first {
			firstRoot = root
		} else {
			secondRoot = root
		}
	}
	writeFile(t, filepath.Join(second, "added"), "added")

	rescanned := s.Rescan(ctx, secondRoot, nil)

	roots := entries(top)
	assert.Len(t, roots, 2)
	assert.Contains(t, roots, firstRoot)
	assert.Contains(t, roots, rescanned)
	assert.Equal(t, second, rescanned.GetPath())
	// two roots of 5 items each, the added file, and the roots themselves
	assert.Equal(t, int64(13), fs.DisplayedItemCount(top))
}

func TestRescanOfTheTopScansAgain(t *testing.T) {
	path := makeTree(t, "scan")
	s := newSession(analyze.CreateAnalyzer())
	old := s.Scan(ctx, []string{path}, nil)
	writeFile(t, filepath.Join(path, "added"), "added")

	root := s.Rescan(ctx, old, nil)

	assert.NotSame(t, old, root)
	assert.Same(t, root, s.Root())
	assert.Equal(t, int64(6), fs.DisplayedItemCount(root))
}

// A database holds the analysis of whole scanned roots, so rescanning a part
// of one scans the whole tree again.
func TestRescanWithDatabaseScansTheWholeTree(t *testing.T) {
	path := makeTree(t, "scan")
	analyzer, err := analyze.CreateSqliteAnalyzer(filepath.Join(t.TempDir(), "gdu.sqlite"))
	require.NoError(t, err)
	s := newSession(analyzer)
	root := s.Scan(ctx, []string{path}, nil)
	writeFile(t, filepath.Join(path, "dir", "added"), "added")

	rescanned := s.Rescan(ctx, child(t, root, "dir"), nil)

	assert.Same(t, rescanned, s.Root())
	assert.Equal(t, path, rescanned.GetPath())
	assert.Equal(t, int64(6), fs.DisplayedItemCount(rescanned))
}

func TestLoadReadsAnExport(t *testing.T) {
	path := makeTree(t, "scan")
	scanned := newSession(analyze.CreateAnalyzer()).Scan(ctx, []string{path}, nil)
	// the ncdu header the export writes before the analysis
	export := bytes.NewBufferString(`[1, 2, {"progname": "gdu"},` + "\n")
	require.NoError(t, scanned.EncodeJSON(export, true, nil))
	export.WriteString("]")
	s := newSession(analyze.CreateAnalyzer())

	root, err := s.Load(export)

	require.NoError(t, err)
	assert.Same(t, root, s.Root())
	assert.Equal(t, path, s.RootPath())
	assert.Equal(t, fs.DisplayedItemCount(scanned), fs.DisplayedItemCount(root))
	assert.Equal(t, scanned.GetUsage(), root.GetUsage())
}

func TestLoadFailureKeepsTheTree(t *testing.T) {
	s := newSession(analyze.CreateAnalyzer())
	root := s.Scan(ctx, []string{makeTree(t, "scan")}, nil)

	_, err := s.Load(strings.NewReader("not json"))

	assert.Error(t, err)
	assert.Same(t, root, s.Root())
}

func TestOpenNeedsDatabase(t *testing.T) {
	_, err := newSession(analyze.CreateAnalyzer()).Open("/")

	assert.ErrorIs(t, err, ErrNoDatabase)
}

func TestOpenLoadsTheStoredAnalysis(t *testing.T) {
	path := makeTree(t, "scan")
	db := filepath.Join(t.TempDir(), "gdu.sqlite")
	analyzer, err := analyze.CreateSqliteAnalyzer(db)
	require.NoError(t, err)
	scanned := newSession(analyzer).Scan(ctx, []string{path}, nil)
	// changes made since the scan are not in the stored analysis
	writeFile(t, filepath.Join(path, "added"), "added")

	analyzer, err = analyze.CreateSqliteAnalyzer(db)
	require.NoError(t, err)
	s := newSession(analyzer)
	root, err := s.Open(path)

	require.NoError(t, err)
	assert.Same(t, root, s.Root())
	assert.Equal(t, path, s.RootPath())
	assert.Equal(t, fs.DisplayedItemCount(scanned), fs.DisplayedItemCount(root))

	_, err = s.Open(filepath.Join(path, "dir"))
	assert.Error(t, err)
	assert.Same(t, root, s.Root())
}

func TestSetRoot(t *testing.T) {
	s := newSession(analyze.CreateAnalyzer())
	root := &analyze.Dir{File: &analyze.File{Name: "root"}, BasePath: "/"}

	s.SetRoot(root)

	assert.Same(t, root, s.Root())
	assert.Equal(t, "/root", s.RootPath())
	assert.Nil(t, newSession(analyze.CreateAnalyzer()).Root())
	assert.Empty(t, newSession(analyze.CreateAnalyzer()).RootPath())
}

func TestSnapshot(t *testing.T) {
	assert.Nil(t, newSession(analyze.CreateAnalyzer()).Snapshot(), "no scan yet")
	assert.Nil(t, newSession(analyze.CreateTopDirAnalyzer()).Snapshot(), "analyzer without snapshots")

	path := makeTree(t, "scan")
	s := newSession(analyze.CreateAnalyzer())
	s.Scan(ctx, []string{path}, nil)
	snapshot := s.Snapshot()
	require.NotNil(t, snapshot)
	assert.Equal(t, path, snapshot.GetPath())
}
