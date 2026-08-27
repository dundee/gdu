package analyze

import (
	"io"
	"iter"
	"os"
	"path/filepath"
	"time"

	"github.com/dundee/gdu/v5/internal/common"
	"github.com/dundee/gdu/v5/pkg/fs"
	log "github.com/sirupsen/logrus"
)

// StoredAnalyzer implements Analyzer
type StoredAnalyzer struct {
	BaseAnalyzer
	storage     *Storage
	storagePath string
}

// CreateStoredAnalyzer returns Analyzer
func CreateStoredAnalyzer(storagePath string) *StoredAnalyzer {
	a := &StoredAnalyzer{
		storagePath: storagePath,
	}
	a.Init()
	return a
}

// AnalyzeDir analyzes given path
func (a *StoredAnalyzer) AnalyzeDir(
	path string, ignore common.ShouldDirBeIgnored, fileTypeFilter common.ShouldFileBeIgnored,
) fs.Item {
	a.ignoreDir = ignore
	a.ignoreFileType = fileTypeFilter

	a.storage = NewStorage(a.storagePath, path)
	closeFn := a.storage.Open()
	defer func() {
		// nasty hack to close storage after all goroutines are done
		// Wait returns immediately if value is 0
		// few last goroutines might still start after that
		time.Sleep(1 * time.Second)
		closeFn()
	}()

	a.ignoreDir = ignore

	go a.UpdateProgress()
	dir := a.processDir(path)

	a.wait.Wait()

	a.progressDoneChan <- struct{}{}
	a.doneChan.Broadcast()

	return dir
}

func (a *StoredAnalyzer) processDir(path string) *StoredDir {
	var (
		file       fs.Item
		err        error
		totalUsage int64
		info       os.FileInfo
		dirCount   int
	)

	a.wait.Add(1)

	files, err := os.ReadDir(path)
	if err != nil {
		log.Print(err.Error())
	}

	dir := &StoredDir{
		Dir: &Dir{
			File: &File{
				Name: filepath.Base(path),
				Flag: getDirFlag(err, len(files)),
			},
			BasePath:  filepath.Dir(path),
			ItemCount: 1,
			Files:     make(fs.Files, 0, len(files)),
		},
	}
	parent := &ParentDir{Path: path}

	setDirPlatformSpecificAttrs(dir.Dir, path)

	for _, f := range files {
		if a.IsCancelled() {
			break
		}
		name := f.Name()
		entryPath := filepath.Join(path, name)
		if f.IsDir() {
			if a.shouldSkipDir(name, entryPath) {
				continue
			}
			dirCount++

			subdir := &StoredDir{
				Dir: &Dir{
					File: &File{
						Name: name,
					},
					BasePath: path,
				},
			}
			dir.AddFile(subdir)

			go func(entryPath string) {
				concurrencyLimit <- struct{}{}
				a.processDir(entryPath)
				<-concurrencyLimit
			}(entryPath)
		} else {
			// Apply file type filter if set
			if a.ignoreFileType != nil && a.ignoreFileType(name) {
				continue // Skip this file
			}

			info, err = f.Info()
			if err != nil {
				log.Print(err.Error())
				continue
			}

			if a.followSymlinks && info.Mode()&os.ModeSymlink != 0 {
				infoF, err := followSymlink(entryPath, a.gitAnnexedSize)
				if err != nil {
					log.Print(err.Error())
					continue
				}
				if infoF != nil {
					info = infoF
				}
			}

			symlinkTarget := readSymlinkTarget(f.Type(), entryPath)

			// Apply time filter if set
			if a.matchesTimeFilterFn != nil && !a.matchesTimeFilterFn(info.ModTime()) {
				continue // Skip this file
			}

			// Check if it's a zip or jar file
			if a.archiveBrowsing && isZipFile(name) {
				zipDir, err := processZipFile(entryPath, info)
				if err != nil {
					// If unable to process zip file, treat as regular file
					log.Printf("Failed to process zip file %s: %v", entryPath, err)
					file = &File{
						Name:   name,
						Flag:   getFlag(info),
						Size:   info.Size(),
						Parent: parent,
					}
				} else {
					// Successfully processed zip file, use zip content size
					uncompressedSize, compressedSize, err := getZipFileSize(entryPath)
					if err == nil {
						zipDir.Size = uncompressedSize
						zipDir.Usage = compressedSize
					}
					zipDir.Parent = parent
					file = zipDir
				}
			} else {
				file = &File{
					Name:    name,
					Flag:    getFlag(info),
					Size:    info.Size(),
					Parent:  parent,
					Symlink: symlinkTarget,
				}
			}

			if file != nil {
				// Only set platform-specific attributes for regular files
				if regularFile, ok := file.(*File); ok {
					setPlatformSpecificAttrs(regularFile, info)
				}
				totalUsage += file.GetUsage()
				dir.AddFile(file)
			}
		}
	}

	err = a.storage.StoreDir(dir)
	if err != nil {
		log.Print(err.Error())
	}

	a.wait.Done()

	a.progressCurrentItemName.Store(path)
	a.progressItemCount.Add(int64(len(files)))
	a.progressTotalUsage.Add(totalUsage)

	return dir
}

// StoredDir implements Dir item stored on disk
//
// It follows the same locking protocol as the embedded Dir: every mutable
// field, including cachedFiles, is read and written under Dir.m, so readers
// calling the synchronized accessors (GetSize, GetUsage, GetFlag, GetMtime,
// GetItemCount) never observe a partially applied stats update.
type StoredDir struct {
	*Dir
	cachedFiles fs.Files
}

// GetParent returns parent dir
func (f *StoredDir) GetParent() fs.Item {
	if DefaultStorage.GetTopDir() == f.GetPath() {
		return nil
	}

	// Storage.Open is reference counted, so taking a reference unconditionally
	// is both cheap and free of the check-then-open race that an IsOpen guard
	// has: another goroutine could close the database between the two.
	closeFn := DefaultStorage.Open()
	defer closeFn()

	dir, err := DefaultStorage.GetDirForPath(f.BasePath)
	if err != nil {
		log.Print(err.Error())
	}
	return dir
}

// GetFiles returns files in directory as a sorted iterator
// If files are already cached, use them
// Otherwise load them from storage
func (f *StoredDir) GetFiles(sortBy fs.SortBy, order fs.SortOrder) iter.Seq[fs.Item] {
	return func(yield func(fs.Item) bool) {
		files := f.loadFiles()
		sortFiles(files, sortBy, order)

		for _, item := range files {
			if !yield(item) {
				return
			}
		}
	}
}

// loadFiles loads files from storage or returns cached files
func (f *StoredDir) loadFiles() fs.Files {
	f.m.RLock()
	if cached := copyFiles(f.cachedFiles); cached != nil {
		f.m.RUnlock()
		return cached
	}
	f.m.RUnlock()

	closeFn := DefaultStorage.Open()
	defer closeFn()

	// The write lock covers both f.Files and f.cachedFiles for the whole load,
	// so a concurrent updateStats cannot observe a half-filled cache.
	f.m.Lock()
	defer f.m.Unlock()

	if cached := copyFiles(f.cachedFiles); cached != nil {
		// another goroutine populated the cache while we were unlocked
		return cached
	}

	var files fs.Files
	for _, file := range f.Files {
		if file.IsDir() {
			dir := &StoredDir{
				Dir: &Dir{
					File: &File{
						Name: file.GetName(),
					},
					BasePath: f.GetPath(),
				},
			}

			err := DefaultStorage.LoadDir(dir)
			if err != nil {
				log.Print(err.Error())
			}
			files = append(files, dir)
		} else {
			files = append(files, file)
		}
	}

	f.cachedFiles = files
	return copyFiles(files)
}

// copyFiles returns a copy of files so callers cannot mutate the cached slice.
// It returns nil for a nil input, letting callers use the result to distinguish
// "not cached" from "cached but empty".
func copyFiles(files fs.Files) fs.Files {
	if files == nil {
		return nil
	}
	result := make(fs.Files, len(files))
	copy(result, files)
	return result
}

// RemoveFile removes file from stored directory
// It also updates size and item count of parent directories
func (f *StoredDir) RemoveFile(item fs.Item) {
	closeFn := DefaultStorage.Open()
	defer closeFn()

	f.m.Lock()
	f.Files = f.Files.Remove(item)
	f.cachedFiles = nil
	f.m.Unlock()

	f.subtractStats(item)
}

// subtractStats removes item's totals from this dir and every ancestor,
// persisting each one. Every directory is updated and stored under its own
// write lock so readers never see the stats and the stored copy disagree.
func (f *StoredDir) subtractStats(item fs.Item) {
	itemCount, size, usage := item.GetItemCount(), item.GetSize(), item.GetUsage()

	cur := f
	for {
		cur.m.Lock()
		cur.ItemCount -= itemCount
		cur.Size -= size
		cur.Usage -= usage
		err := DefaultStorage.StoreDir(cur)
		cur.m.Unlock()

		if err != nil {
			log.Print(err.Error())
		}

		parent := cur.GetParent()
		if parent == nil {
			break
		}
		cur = parent.(*StoredDir)
	}
}

// GetItemStats returns item count, apparent usage and real usage of this dir
func (f *StoredDir) GetItemStats(linkedItems fs.HardLinkedItems, filteringFiles bool) (itemCount, size, usage int64) {
	f.updateStats(linkedItems, filteringFiles)
	return f.GetItemCount(), f.GetSize(), f.GetUsage()
}

func (f *StoredDir) UpdateStatsWithFileFiltering(linkedItems fs.HardLinkedItems) {
	f.updateStats(linkedItems, true)
}

// UpdateStats recursively updates size and item count
func (f *StoredDir) UpdateStats(linkedItems fs.HardLinkedItems) {
	f.updateStats(linkedItems, false)
}

func (f *StoredDir) updateStats(linkedItems fs.HardLinkedItems, filteringFiles bool) {
	closeFn := DefaultStorage.Open()
	defer closeFn()

	// Drop the cache and snapshot the fields we accumulate into, so the stats
	// are computed from locals while no lock is held. Recursing into
	// entry.GetItemStats below must not happen under f.m.
	f.m.Lock()
	f.cachedFiles = nil
	mtime := f.Mtime
	flag := f.Flag
	f.m.Unlock()

	totalSize := int64(4096)
	totalUsage := int64(4096)
	var itemCount int64
	files := f.loadFiles()
	for _, entry := range files {
		count, size, usage := entry.GetItemStats(linkedItems, filteringFiles)
		totalSize += size
		totalUsage += usage
		itemCount += count

		entryMtime := entry.GetMtime()
		if entryMtime.After(mtime) {
			mtime = entryMtime
		}

		switch entry.GetFlag() {
		case '!', '.':
			if flag != '!' {
				flag = '.'
			}
		}
	}

	// Commit and persist under one write lock so neither a concurrent reader
	// nor the gob encoder can observe a half-updated directory.
	f.m.Lock()
	defer f.m.Unlock()

	f.Mtime = mtime
	f.Flag = flag
	f.cachedFiles = nil
	f.ItemCount = itemCount + 1
	f.Size = totalSize
	f.Usage = totalUsage

	if err := DefaultStorage.StoreDir(f); err != nil {
		log.Print(err.Error())
	}
}

// RemoveFileByName removes file by name from stored directory
func (f *StoredDir) RemoveFileByName(name string) {
	closeFn := DefaultStorage.Open()
	defer closeFn()

	f.m.Lock()
	idx, ok := f.Files.FindByName(name)
	if !ok {
		f.m.Unlock()
		return
	}
	item := f.Files[idx]
	f.Files = append(f.Files[:idx], f.Files[idx+1:]...)
	f.cachedFiles = nil
	f.m.Unlock()

	f.subtractStats(item)
}

// ParentDir represents parent directory of single file
// It is used to get path to parent directory of a file
type ParentDir struct {
	Path string
}

func (p *ParentDir) GetPath() string {
	return p.Path
}
func (p *ParentDir) GetName() string                                     { panic("must not be called") }
func (p *ParentDir) GetFlag() rune                                       { panic("must not be called") }
func (p *ParentDir) IsDir() bool                                         { panic("must not be called") }
func (p *ParentDir) GetSize() int64                                      { panic("must not be called") }
func (p *ParentDir) GetType() string                                     { panic("must not be called") }
func (p *ParentDir) GetUsage() int64                                     { panic("must not be called") }
func (p *ParentDir) GetMtime() time.Time                                 { panic("must not be called") }
func (p *ParentDir) GetItemCount() int64                                 { panic("must not be called") }
func (p *ParentDir) GetParent() fs.Item                                  { panic("must not be called") }
func (p *ParentDir) SetParent(fs.Item)                                   { panic("must not be called") }
func (p *ParentDir) GetMultiLinkedInode() uint64                         { panic("must not be called") }
func (p *ParentDir) EncodeJSON(io.Writer, bool, fs.JSONAttributes) error { panic("must not be called") }
func (p *ParentDir) UpdateStats(linkedItems fs.HardLinkedItems)          { panic("must not be called") }
func (p *ParentDir) UpdateStatsWithFileFiltering(linkedItems fs.HardLinkedItems) {
	panic("must not be called")
}
func (p *ParentDir) AddFile(fs.Item)                                    { panic("must not be called") }
func (p *ParentDir) GetFiles(fs.SortBy, fs.SortOrder) iter.Seq[fs.Item] { panic("must not be called") }
func (p *ParentDir) GetFilesLocked(fs.SortBy, fs.SortOrder) iter.Seq[fs.Item] {
	panic("must not be called")
}
func (p *ParentDir) RLock() func()                { panic("must not be called") }
func (p *ParentDir) RemoveFile(item fs.Item)      { panic("must not be called") }
func (p *ParentDir) RemoveFileByName(name string) { panic("must not be called") }
func (p *ParentDir) GetItemStats(
	linkedItems fs.HardLinkedItems, filteringFiles bool,
) (itemCount, size, usage int64) {
	panic("must not be called")
}
