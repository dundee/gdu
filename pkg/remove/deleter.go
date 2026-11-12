package remove

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/dundee/gdu/v5/pkg/analyze"
	"github.com/dundee/gdu/v5/pkg/fs"
)

// Mode selects what Deleter.Delete does with an item.
type Mode uint8

const (
	// ModeRemove deletes the item permanently, a directory with everything in it.
	ModeRemove Mode = iota
	// ModeEmpty truncates a file to zero bytes, or deletes everything inside a
	// directory while keeping the directory itself.
	ModeEmpty
	// ModeTrash moves the item to the trash.
	ModeTrash
)

// ErrRefused matches every error Deleter.Delete returns when it refuses to
// touch an item. Nothing on disk or in the tree has changed when it does.
var ErrRefused = errors.New("refused to change the item")

type refusal string

func (r refusal) Error() string        { return string(r) }
func (r refusal) Is(target error) bool { return target == ErrRefused }

// Reasons Deleter.Delete refuses an item. Each of them matches ErrRefused.
var (
	ErrScannedRoot  error = refusal("a scanned root cannot be removed")
	ErrOutsideRoots error = refusal("the item is outside the scanned roots")
	ErrInArchive    error = refusal("items inside a browsed archive cannot be changed")
	ErrNotInDir     error = refusal("the item is not an entry of the directory it is removed from")
	ErrInvalidName  error = refusal("the item name is not a single path element")
	ErrNotRegular   error = refusal("only regular files can be emptied")
	ErrChanged      error = refusal("the item changed on disk while it was being emptied")
	ErrRelocated    error = refusal("path changed on disk since the scan and now resolves outside the scanned root")
)

// Deleter is the single gate between gdu and the destructive changes it can
// make to the filesystem. It refuses anything that is not a real item inside
// one of the scanned roots, so a wrong path built from the tree (an archive
// entry, a name taken from an imported report, a directory swapped for a link
// since the scan) cannot delete something the user never saw.
type Deleter struct {
	roots    []string
	parallel bool
	trash    func(dir, item fs.Item) error
}

// Option configures a Deleter.
type Option func(*Deleter)

// WithParallel removes the subdirectories of a removed directory concurrently.
func WithParallel() Option {
	return func(d *Deleter) { d.parallel = true }
}

// WithTrash sets how ModeTrash moves an item to the trash. The function must
// remove the item from dir once it is gone from disk, as MoveItemToTrash and
// the functions returned by TrashCommand do. MoveItemToTrash is the default.
// Delete checks the item before calling it.
func WithTrash(trash func(dir, item fs.Item) error) Option {
	return func(d *Deleter) {
		if trash != nil {
			d.trash = trash
		}
	}
}

// NewDeleter returns a Deleter which changes only items inside roots, the
// absolute paths of the scanned roots.
func NewDeleter(roots []string, opts ...Option) *Deleter {
	d := &Deleter{trash: MoveItemToTrash}
	for _, root := range roots {
		if root != "" {
			d.roots = append(d.roots, filepath.Clean(root))
		}
	}
	for _, opt := range opts {
		opt(d)
	}
	return d
}

// Delete changes item, an entry of dir, on disk as mode says and then updates
// the tree to match. It returns an error matching ErrRefused without touching
// anything when item
//   - lies inside a browsed archive,
//   - is a scanned root (except for ModeEmpty, which keeps the root itself),
//   - has a name which is not a single path element, or is not an entry of dir,
//   - is outside every scanned root, or
//   - is now reached through a link leading out of its scanned root.
//
// ModeEmpty only truncates regular files, so it never writes through a link.
func (d *Deleter) Delete(dir, item fs.Item, mode Mode) error {
	if dir == nil || item == nil {
		return ErrNotInDir
	}
	switch mode {
	case ModeRemove:
		return d.remove(dir, item)
	case ModeEmpty:
		if item.IsDir() {
			return d.emptyDir(dir, item)
		}
		return d.emptyFile(dir, item)
	case ModeTrash:
		if _, err := d.resolve(dir, item); err != nil {
			return err
		}
		// The trash moves the item by its path, so unlike remove and empty it
		// cannot be confined to the root; resolve has just checked that the
		// path still leads inside it.
		return d.trash(dir, item)
	}
	return fmt.Errorf("unknown removal mode %d", mode)
}

// Check returns the error matching ErrRefused that Delete would return for the
// same arguments without touching anything, or nil. Delete checks again, so a
// nil result does not promise that the change will succeed.
func (d *Deleter) Check(dir, item fs.Item, mode Mode) error {
	if dir == nil || item == nil {
		return ErrNotInDir
	}
	if mode == ModeEmpty && item.IsDir() {
		return d.checkEmptyDir(dir, item)
	}
	_, err := d.resolve(dir, item)
	return err
}

// target is an item resolved against the scanned root it lies in.
type target struct {
	root string // path of the scanned root
	rel  string // path of the item relative to root
}

func (d *Deleter) resolve(dir, item fs.Item) (target, error) {
	if analyze.IsArchiveEntry(item) || analyze.IsArchiveDir(dir) {
		return target{}, ErrInArchive
	}
	path := filepath.Clean(item.GetPath())
	if d.isRoot(path) {
		return target{}, ErrScannedRoot
	}
	name := item.GetName()
	if !isPathElement(name) {
		return target{}, ErrInvalidName
	}
	if filepath.Join(dir.GetPath(), name) != path {
		return target{}, ErrNotInDir
	}
	t, ok := d.within(path)
	if !ok {
		return target{}, ErrOutsideRoots
	}
	if err := checkNotRelocated(t); err != nil {
		return target{}, err
	}
	return t, nil
}

func (d *Deleter) isRoot(path string) bool {
	for _, root := range d.roots {
		if root == path {
			return true
		}
	}
	return false
}

func (d *Deleter) within(path string) (target, bool) {
	for _, root := range d.roots {
		rel, err := filepath.Rel(root, path)
		if err == nil && rel != "." && filepath.IsLocal(rel) {
			return target{root: root, rel: rel}, true
		}
	}
	return target{}, false
}

// isPathElement reports whether name can only ever name an entry of its
// directory. Names read from an archive or an imported report can be anything,
// and joining "..", "a/../.." or an absolute path onto the parent's path would
// lead somewhere else entirely.
func isPathElement(name string) bool {
	switch name {
	case "", ".", "..":
		return false
	}
	return !strings.ContainsAny(name, "/\x00"+string(filepath.Separator)) &&
		filepath.VolumeName(name) == ""
}

// checkNotRelocated reports whether the directory holding t still resolves to
// a location inside t's scanned root.
//
// The tree records paths as plain strings, resolved again only when the item
// is removed, so anyone able to write inside the scanned tree between the scan
// and the removal could swap an ancestor directory for a link and redirect the
// removal. Removing and emptying go through an os.Root, which refuses that by
// itself; this check gives them a clear error, and it is all that protects the
// trash, which moves the item by its path.
//
// It is the containing directory that is resolved, not the item: a link as
// the item itself is removed rather than followed, and resolving the item
// would need search permission on a directory whose removal may legitimately
// fail with a permission error. Both sides are resolved, so that a root reached
// through a link (macOS /tmp, for instance) is not mistaken for an escape.
func checkNotRelocated(t target) error {
	realRoot, err := filepath.EvalSymlinks(t.root)
	if err != nil {
		return ignoreNotExist(err)
	}
	realParent, err := filepath.EvalSymlinks(filepath.Dir(filepath.Join(t.root, t.rel)))
	if err != nil {
		return ignoreNotExist(err)
	}
	rel, err := filepath.Rel(realRoot, realParent)
	if err != nil || (rel != "." && !filepath.IsLocal(rel)) {
		return ErrRelocated
	}
	return nil
}

// ignoreNotExist lets a check through when the path it resolves is already
// gone: there is nothing left to remove, and the removal itself reports
// anything else. That keeps deletion idempotent for a user who removed the
// item in another window.
func ignoreNotExist(err error) error {
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

func (d *Deleter) remove(dir, item fs.Item) error {
	t, err := d.resolve(dir, item)
	if err != nil {
		return err
	}
	root, err := os.OpenRoot(t.root)
	if err != nil {
		return err
	}
	defer root.Close()

	// The node standing for an archive is a file on disk, so its entries must
	// not be removed one by one.
	if d.parallel && item.IsDir() && !analyze.IsArchiveDir(item) {
		if err := removeSubdirsParallel(root, t.rel, item); err != nil {
			return err
		}
	}
	if err := root.RemoveAll(t.rel); err != nil {
		return err
	}
	dir.RemoveFile(item)
	return nil
}

// concurrencyLimit bounds the subdirectories removed at once by WithParallel.
var concurrencyLimit = make(chan struct{}, 3*runtime.GOMAXPROCS(0))

// removeSubdirsParallel removes the subdirectories of dir, which lies at rel
// inside root, concurrently.
func removeSubdirsParallel(root *os.Root, rel string, dir fs.Item) error {
	var (
		wait     sync.WaitGroup
		errOnce  sync.Once
		firstErr error
	)
	for _, sub := range entries(dir) {
		if !sub.IsDir() {
			continue
		}
		name := sub.GetName()
		if !isPathElement(name) {
			return ErrInvalidName
		}
		wait.Go(func() {
			concurrencyLimit <- struct{}{}
			defer func() { <-concurrencyLimit }()
			if err := root.RemoveAll(filepath.Join(rel, name)); err != nil {
				errOnce.Do(func() { firstErr = err })
			}
		})
	}
	wait.Wait()
	return firstErr
}

func (d *Deleter) checkEmptyDir(dir, item fs.Item) error {
	if analyze.IsArchiveDir(item) || analyze.IsArchiveEntry(item) {
		return ErrInArchive
	}
	// Emptying a scanned root keeps the root, so it is allowed; every entry
	// is still checked on its own when it is removed.
	if d.isRoot(filepath.Clean(item.GetPath())) {
		return nil
	}
	_, err := d.resolve(dir, item)
	return err
}

func (d *Deleter) emptyDir(dir, item fs.Item) error {
	if err := d.checkEmptyDir(dir, item); err != nil {
		return err
	}
	for _, entry := range entries(item) {
		if err := d.remove(item, entry); err != nil {
			return err
		}
	}
	return nil
}

func (d *Deleter) emptyFile(dir, item fs.Item) error {
	t, err := d.resolve(dir, item)
	if err != nil {
		return err
	}
	root, err := os.OpenRoot(t.root)
	if err != nil {
		return err
	}
	defer root.Close()

	// Lstat rather than Stat: truncating through a link would empty its
	// target, which may lie anywhere.
	info, err := root.Lstat(t.rel)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return ErrNotRegular
	}
	file, err := root.OpenFile(t.rel, os.O_WRONLY, 0)
	if err != nil {
		return err
	}
	opened, err := file.Stat()
	if err == nil && (!opened.Mode().IsRegular() || !sameFile(info, opened)) {
		err = ErrChanged
	}
	if err == nil {
		err = file.Truncate(0)
	}
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}

	replaceWithEmptyFile(dir, item)
	return nil
}

// replaceWithEmptyFile updates the tree after file was truncated to 0 bytes.
func replaceWithEmptyFile(dir, file fs.Item) {
	dir.RemoveFile(file)
	dir.AddFile(&analyze.File{
		Name:   file.GetName(),
		Flag:   file.GetFlag(),
		Parent: dir,
	})
}

// sameFile reports whether a and b describe the same file. Windows can only
// tell for infos taken by path, which a file opened through an os.Root lacks,
// so it trusts the regular file check there.
func sameFile(a, b os.FileInfo) bool {
	return runtime.GOOS == "windows" || os.SameFile(a, b)
}

// entries returns the entries of dir. Taking the read lock and iterating
// GetFiles works for every directory implementation, including the stored one,
// whose GetFilesLocked does not load its entries.
func entries(dir fs.Item) fs.Files {
	unlock := dir.RLock()
	defer unlock()
	var files fs.Files
	for entry := range dir.GetFiles(fs.SortBySize, fs.SortDesc) {
		files = append(files, entry)
	}
	return files
}
