package remove

import (
	"os"
	"runtime"
	"sync"

	"github.com/dundee/gdu/v5/pkg/fs"
)

var concurrencyLimit = make(chan struct{}, 3*runtime.GOMAXPROCS(0))

// ItemFromDirParallel removes item from dir
func ItemFromDirParallel(dir, item fs.Item) error {
	if !item.IsDir() {
		return ItemFromDir(dir, item)
	}
	errChan := make(chan error, 1) // we show only first error
	var wait sync.WaitGroup

	// remove all files in the directory in parallel
	for _, file := range item.GetFilesLocked() {
		if !file.IsDir() {
			continue
		}

		wait.Add(1)
		go func(itemPath string) {
			concurrencyLimit <- struct{}{}
			defer func() { <-concurrencyLimit }()

			err := os.RemoveAll(itemPath)
			if err != nil {
				select {
				// write error to channel if it's empty
				case errChan <- err:
				default:
				}
			}
			wait.Done()
		}(file.GetPath())
	}

	wait.Wait()

	// check if there was an error
	select {
	case err := <-errChan:
		return err
	default:
	}

	// remove the directory itself
	err := os.RemoveAll(item.GetPath())
	if err != nil {
		return err
	}

	// update parent directory
	dir.RemoveFile(item)
	return nil
}
