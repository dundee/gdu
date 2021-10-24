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
func ReadAnalysis(input io.Reader) (*analyze.Dir, error) {
	var data interface{}

	var buff bytes.Buffer
	if _, err := buff.ReadFrom(input); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(buff.Bytes(), &data); err != nil {
		return nil, err
	}

	dataArray, ok := data.([]interface{})
	if !ok {
		return nil, errors.New("JSON file does not contain top level array")
	}
	if len(dataArray) < 4 {
		return nil, errors.New("Top level array must have at least 4 items")
	}

	items, ok := dataArray[3].([]interface{})
	if !ok {
		return nil, errors.New("Array of maps not found in the top level array on 4th position")
	}

	return processDir(items)
}

func processDir(items []interface{}) (*analyze.Dir, error) {
	dir := &analyze.Dir{
		File: &analyze.File{
			Flag: ' ',
		},
	}
	dirMap, ok := items[0].(map[string]interface{})
	if !ok {
		return nil, errors.New("Directory item is not a map")
	}
	name, ok := dirMap["name"].(string)
	if !ok {
		return nil, errors.New("Directory name is not a string")
	}
	if mtime, ok := dirMap["mtime"].(float64); ok {
		dir.Mtime = time.Unix(int64(mtime), 0)
	}

	slashPos := strings.LastIndex(name, "/")
	if slashPos > -1 {
		dir.Name = name[slashPos+1:]
		dir.BasePath = name[:slashPos+1]
	} else {
		dir.Name = name
	}

	for _, v := range items[1:] {
		switch item := v.(type) {
		case map[string]interface{}:
			file := &analyze.File{}
			file.Name = item["name"].(string)

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

			dir.Files.Append(file)
		case []interface{}:
			subdir, err := processDir(item)
			if err != nil {
				return nil, err
			}
			subdir.Parent = dir
			dir.Files.Append(subdir)
		}
	}

	return dir, nil
}
