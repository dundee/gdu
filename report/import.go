package report

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"time"

	"github.com/dundee/gdu/v5/pkg/analyze"
)

// ReadAnalysis reads analysis report from JSON file and returns directory item
func ReadAnalysis(input io.Reader) (dir *analyze.Dir, err error) {
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

	return processDir(items)
}

func processDir(items []any) (dir *analyze.Dir, err error) {
	if len(items) == 0 {
		return nil, errors.New("directory array is empty")
	}

	var hasSize, hasUsage, hasItemCount bool
	dir, hasSize, hasUsage, hasItemCount, err = parseDirectoryMetadata(items[0])
	if err != nil {
		return nil, err
	}

	for _, v := range items[1:] {
		switch item := v.(type) {
		case map[string]any:
			file := &analyze.File{}
			name, ok := item["name"].(string)
			if !ok {
				return nil, errors.New("file name is not a string")
			}
			file.Name = name

			if asize, ok := item["asize"].(float64); ok {
				file.Size = int64(asize)
			}
			if dsize, ok := item["dsize"].(float64); ok {
				file.Usage = int64(dsize)
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
			subdir, err := processDir(item)
			if err != nil {
				return nil, err
			}
			subdir.Parent = dir
			dir.AddFile(subdir)
		}
	}
	preserveTruncatedStats(dir, hasSize, hasUsage, hasItemCount)

	return dir, nil
}

func parseDirectoryMetadata(item any) (
	dir *analyze.Dir,
	hasSize bool,
	hasUsage bool,
	hasItemCount bool,
	err error,
) {
	dirMap, ok := item.(map[string]any)
	if !ok {
		return nil, false, false, false, errors.New("directory item is not a map")
	}
	name, ok := dirMap["name"].(string)
	if !ok {
		return nil, false, false, false, errors.New("directory name is not a string")
	}

	dir = &analyze.Dir{File: &analyze.File{Flag: ' '}}
	if mtime, ok := dirMap["mtime"].(float64); ok {
		dir.Mtime = time.Unix(int64(mtime), 0)
	}
	if asize, ok := dirMap["asize"].(float64); ok {
		dir.Size = int64(asize)
		hasSize = true
	}
	if dsize, ok := dirMap["dsize"].(float64); ok {
		dir.Usage = int64(dsize)
		hasUsage = true
	}
	if itemCount, ok := dirMap["items"].(float64); ok {
		dir.ItemCount = int64(itemCount)
		hasItemCount = true
	}

	slashPos := strings.LastIndex(name, "/")
	if slashPos > -1 {
		dir.Name = name[slashPos+1:]
		dir.BasePath = name[:slashPos+1]
	} else {
		dir.Name = name
	}

	return dir, hasSize, hasUsage, hasItemCount, nil
}

func preserveTruncatedStats(dir *analyze.Dir, hasSize, hasUsage, hasItemCount bool) {
	if !hasSize || !hasUsage || !hasItemCount || len(dir.Files) != 0 {
		return
	}
	if dir.Size == 512 && dir.Usage == 0 && dir.ItemCount == 1 {
		return
	}
	dir.SetStatsFromJSON()
}
