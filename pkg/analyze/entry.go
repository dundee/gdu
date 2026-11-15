package analyze

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/dundee/gdu/v6/pkg/fs"
)

// fileEntry is a file, symlink or other non-directory entry of a scanned
// directory, as resolveFile resolves it.
type fileEntry struct {
	name string
	// info describes the entry, or the file it links to when symlinks are
	// followed
	info  os.FileInfo
	flag  rune
	usage int64
	// shared is the part of the disk usage shared with other files, measured
	// only with exclusive size on; usage then excludes it
	shared int64
	// link identifies a file with several hard links, and is zero otherwise
	link    inodeKey
	symlink string
	// archive is the browsed content of the entry, a *ZipDir or *TarDir, when
	// the entry is an archive
	archive fs.Item
	// duplicate is set for a file met before through another hard link, when
	// resolving with a link tracker; its usage is then not measured
	duplicate bool
}

// resolveOptions tell resolveFile what the analyzer does with the entries.
type resolveOptions struct {
	// browseArchives opens archives as directories, if archive browsing is on
	browseArchives bool
	// links counts each hard-linked file once, for an analyzer that totals as
	// it scans; analyzers building a tree count them when computing totals
	links *linkTracker
}

// resolveFile resolves the entry d of the directory at dirPath. It reports
// false when a filter excludes the entry, and returns an error when the entry
// cannot be read, which makes its directory one read with errors.
//
// Every analyzer resolves the entries it counts here, so that they all count
// the same: the filters, followed symlinks, archives, hard links and disk
// usage are decided in one place.
func (s *scan) resolveFile(dirPath string, d os.DirEntry, opts resolveOptions) (fileEntry, bool, error) {
	name := d.Name()
	if s.opts.IgnoreFile != nil && s.opts.IgnoreFile(name) {
		return fileEntry{}, false, nil
	}
	path := filepath.Join(dirPath, name)

	info, err := d.Info()
	if err != nil {
		return fileEntry{}, false, err
	}
	if s.opts.FollowSymlinks && info.Mode()&os.ModeSymlink != 0 {
		target, err := followSymlink(path, s.opts.AnnexedSize)
		if err != nil {
			return fileEntry{}, false, err
		}
		if target != nil {
			info = target
		}
	}

	// after following, so the filter judges the file whose size is counted
	if s.opts.TimeFilter != nil && !s.opts.TimeFilter(info.ModTime()) {
		return fileEntry{}, false, nil
	}

	entry := fileEntry{
		name:    name,
		info:    info,
		flag:    getFlag(info),
		symlink: readSymlinkTarget(d.Type(), path),
	}

	// Only a regular file is opened as an archive: opening a FIFO would block
	// the scan, and a symlink to an archive would count the archive twice.
	if opts.browseArchives && s.opts.ArchiveBrowsing && info.Mode().IsRegular() {
		if entry.archive = openArchive(s.ctx, path, name, info); entry.archive != nil {
			return entry, true, nil
		}
	}

	entry.usage, entry.link = fileStats(info, path)
	if entry.link != (inodeKey{}) {
		entry.flag = 'H'
		if opts.links != nil && !opts.links.claim(entry.link) {
			entry.duplicate = true
			return entry, true, nil
		}
	}
	entry.usage, entry.shared = s.measureSharedUsage(path, info, entry.usage)
	return entry, true, nil
}

// item returns the entry as an item of the in-memory tree, in parent.
func (e *fileEntry) item(parent fs.Item) fs.Item {
	if e.archive != nil {
		setParent(e.archive, parent)
		return e.archive
	}
	return &File{
		Name:        e.name,
		Flag:        e.flag,
		Size:        e.info.Size(),
		Usage:       e.usage,
		SharedUsage: e.shared,
		Mtime:       e.info.ModTime(),
		Mli:         e.link.ino,
		Symlink:     e.symlink,
		Parent:      parent,
	}
}

// openArchive returns the browsed content of the archive at path, a *ZipDir
// or *TarDir, or nil when name is not an archive's or the archive cannot be
// read. Its totals are computed already, see finishArchive.
func openArchive(ctx context.Context, path, name string, info os.FileInfo) fs.Item {
	switch {
	case isZipFile(name):
		zipDir, err := processZipFile(path, info)
		if err != nil {
			slog.Warn("reading zip file", "path", path, "err", err)
			return nil
		}
		finishArchive(zipDir.Dir, path, info)
		return zipDir
	case isTarFile(name):
		tarDir, err := readTarFile(ctx, path, info)
		if err != nil {
			slog.Warn("reading tar file", "path", path, "err", err)
			return nil
		}
		finishArchive(tarDir.Dir, path, info)
		return tarDir
	}
	return nil
}

// finishArchive computes the totals of the browsed archive dir, at path, once:
// what it holds does not change. Its entries show their sizes inside the
// archive, uncompressed, but the archive itself counts as the file it is,
// with its size, disk usage and mtime, so that browsing it changes nothing
// in the totals of the directories holding it.
func finishArchive(dir *Dir, path string, info os.FileInfo) {
	archiveTotals(dir)
	usage, _ := fileStats(info, path)
	dir.Size, dir.Usage, dir.Mtime = info.Size(), usage, info.ModTime()
}

// archiveTotals computes the totals of dir, a directory of a browsed archive,
// and of the directories in it. Their itemStats return the totals computed,
// so the deepest ones come first.
func archiveTotals(dir *Dir) {
	for _, entry := range dir.Files {
		if sub := archiveDirOf(entry); sub != nil {
			archiveTotals(sub)
		}
	}
	dir.updateStats(fs.NewHardLinks(), false)
}

// archiveDirOf returns the directory of a browsed archive, or of a directory
// in one, and nil for any other item.
func archiveDirOf(item fs.Item) *Dir {
	switch dir := item.(type) {
	case *ZipDir:
		return dir.Dir
	case *TarDir:
		return dir.Dir
	}
	return nil
}
