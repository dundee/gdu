package analyze

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"
)

// ReadAnalysis reads an analysis exported as JSON (the ncdu format EncodeJSON
// writes) and returns its top directory.
//
// An export may come from anywhere, so it is refused when the name of an
// entry is no single name, such as "..", "" or "a/b": its path would lie
// elsewhere than in its directory. Only the top directory is named by its
// path.
//
// A directory's asize and dsize are what the directory itself takes, as ncdu
// writes them, with its totals under gdu_asize and gdu_dsize (see
// appendDirSizes). gdu wrote the totals as asize and dsize before; such an
// export is read as it was written.
func ReadAnalysis(input io.Reader) (dir *Dir, err error) {
	var data any

	var buff bytes.Buffer
	if _, err = buff.ReadFrom(input); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(buff.Bytes(), &data); err != nil {
		return nil, err
	}

	dataArray, ok := data.([]any)
	if !ok {
		return nil, errors.New("JSON file does not contain top level array")
	}
	if len(dataArray) < 4 {
		return nil, errors.New("top level array must have at least 4 items")
	}

	items, ok := dataArray[3].([]any)
	if !ok {
		return nil, errors.New("array of maps not found in the top level array on 4th position")
	}

	return decodeDir(items, nil, writtenByGdu(dataArray[2]))
}

// writtenByGdu reports whether the header meta of an export says gdu wrote
// it, as it does unless it names another program.
func writtenByGdu(meta any) bool {
	header, ok := meta.(map[string]any)
	if !ok {
		return true
	}
	program, ok := header["progname"].(string)
	return !ok || program == "gdu"
}

// decodeDir decodes a directory of parent, or the top directory when parent
// is nil. It lies on the device of parent, unless the export tells another.
// byGdu tells that gdu wrote the export.
func decodeDir(items []any, parent *Dir, byGdu bool) (dir *Dir, err error) {
	if len(items) == 0 {
		return nil, errors.New("directory array is empty")
	}

	var stats exportedStats
	dir, stats, err = parseDirectoryMetadata(items[0], parent == nil, byGdu)
	if err != nil {
		if parent != nil {
			return nil, fmt.Errorf("%s: %w", parent.GetPath(), err)
		}
		return nil, err
	}
	if dev, ok := items[0].(map[string]any)["dev"].(float64); ok {
		dir.Dev = uint64(dev)
	} else if parent != nil {
		dir.Dev = parent.Dev
	}
	if parent != nil {
		dir.Parent = parent
	}

	for _, v := range items[1:] {
		switch item := v.(type) {
		case map[string]any:
			file := &File{}
			name, ok := item["name"].(string)
			if !ok {
				return nil, errors.New("file name is not a string")
			}
			if !isPathElement(name) {
				return nil, fmt.Errorf("%s: invalid file name %q", dir.GetPath(), name)
			}
			file.Name = name

			if asize, ok := item["asize"].(float64); ok {
				file.Size = int64(asize)
			}
			if dsize, ok := item["dsize"].(float64); ok {
				file.Usage = int64(dsize)
			}
			if shsize, ok := item["shsize"].(float64); ok {
				file.SharedUsage = int64(shsize)
			}
			if mtime, ok := item["mtime"].(float64); ok {
				file.Mtime = time.Unix(int64(mtime), 0)
			}
			if _, ok := item["notreg"].(bool); ok {
				file.Flag = '@'
			} else {
				file.Flag = ' '
			}
			if mli, ok := item["ino"].(float64); ok {
				file.Mli = uint64(mli)
			}
			if _, ok := item["hlnkc"].(bool); ok {
				file.Flag = 'H'
			}

			file.Parent = dir

			dir.AddFile(file)
		case []any:
			subdir, err := decodeDir(item, dir, byGdu)
			if err != nil {
				return nil, err
			}
			dir.AddFile(subdir)
		}
	}
	if stats.ownUsageInTotal {
		dir.OwnUsage = exportedOwnUsage(dir)
	}
	preserveTruncatedStats(dir, stats.hasSize, stats.hasUsage, stats.hasItemCount)

	return dir, nil
}

// exportedOwnUsage recovers the disk usage of the directory itself, which an
// export by an older gdu folds into the directory's total. A hard-linked file counted elsewhere
// in the tree can make the entries add up to more than the total, so the
// result is clamped at zero rather than going negative.
func exportedOwnUsage(dir *Dir) int64 {
	own := dir.Usage
	for _, item := range dir.Files {
		own -= item.GetUsage()
	}
	return max(own, 0)
}

// exportedStats tells which totals the export of a directory holds.
type exportedStats struct {
	hasSize, hasUsage, hasItemCount bool
	// ownUsageInTotal tells that the own usage of the directory is folded
	// into its total usage, as older gdu versions wrote it
	ownUsageInTotal bool
}

// parseDirectoryMetadata decodes the directory item; the top directory is
// named by its path, any other by its name. byGdu tells that gdu wrote the
// export.
func parseDirectoryMetadata(item any, top, byGdu bool) (dir *Dir, stats exportedStats, err error) {
	dirMap, ok := item.(map[string]any)
	if !ok {
		return nil, stats, errors.New("directory item is not a map")
	}
	name, ok := dirMap["name"].(string)
	if !ok {
		return nil, stats, errors.New("directory name is not a string")
	}

	dir = &Dir{File: &File{Flag: ' '}}
	if mtime, ok := dirMap["mtime"].(float64); ok {
		dir.Mtime = time.Unix(int64(mtime), 0)
	}
	if readError, ok := dirMap["read_error"].(bool); ok && readError {
		dir.Flag = '!'
	}
	asize, hasAsize := dirMap["asize"].(float64)
	dsize, hasDsize := dirMap["dsize"].(float64)
	totalSize, hasTotalSize := dirMap[totalSizeKey].(float64)
	totalUsage, hasTotalUsage := dirMap[totalUsageKey].(float64)
	if byGdu && !hasTotalSize && !hasTotalUsage {
		// an older gdu wrote the totals as asize and dsize
		totalSize, hasTotalSize = asize, hasAsize
		totalUsage, hasTotalUsage = dsize, hasDsize
		stats.ownUsageInTotal = hasDsize
	} else {
		dir.OwnUsage = int64(dsize)
	}
	if hasTotalSize {
		dir.Size = int64(totalSize)
		stats.hasSize = true
	}
	if hasTotalUsage {
		dir.Usage = int64(totalUsage)
		stats.hasUsage = true
	}
	if itemCount, ok := dirMap["items"].(float64); ok {
		dir.ItemCount = int64(itemCount)
		stats.hasItemCount = true
	}

	if !top {
		if !isPathElement(name) {
			return nil, stats, fmt.Errorf("invalid directory name %q", name)
		}
		dir.Name = name
		return dir, stats, nil
	}

	slashPos := strings.LastIndex(name, "/")
	if slashPos > -1 {
		dir.Name = name[slashPos+1:]
		dir.BasePath = name[:slashPos+1]
	} else {
		dir.Name = name
	}

	return dir, stats, nil
}

func preserveTruncatedStats(dir *Dir, hasSize, hasUsage, hasItemCount bool) {
	if !hasSize || !hasUsage || !hasItemCount || len(dir.Files) != 0 {
		return
	}
	// A leaf directory whose imported stats match the empty-dir placeholder
	// carries nothing worth pinning: recomputation reproduces the same values.
	// Only genuinely-empty leaves reach this point with placeholder stats;
	// depth-truncated directories that held content export with items >= 2.
	if dir.HasEmptyDirPlaceholderStats() {
		return
	}
	dir.SetStatsFromJSON()
}
