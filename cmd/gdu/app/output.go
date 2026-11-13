package app

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// outputFile is the file an export is written to (--output-file). Nothing is
// created until the export writes, so a run failing before, such as one
// rejecting its flags, leaves an existing file as it was. A regular file is
// written to a temporary file next to it and replaces it on Close, so a failed
// export does not leave it half written either. Anything else, such as a
// device or a named pipe, is written to directly.
type outputFile struct {
	path     string
	target   string
	file     *os.File
	replaces bool
	err      error
}

// newOutputFile returns the output file at path. It fails at once when the
// directory to write to does not exist, rather than after a whole scan.
func newOutputFile(path string) (*outputFile, error) {
	target := path
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		target = resolved
	}
	info, err := os.Stat(filepath.Dir(target))
	if err != nil {
		return nil, fmt.Errorf("opening output file: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("opening output file: %s is not a directory", filepath.Dir(target))
	}
	return &outputFile{path: path, target: target}, nil
}

func (o *outputFile) Write(data []byte) (int, error) {
	if o.err != nil {
		return 0, o.err
	}
	if o.file == nil {
		if o.err = o.open(); o.err != nil {
			return 0, o.err
		}
	}
	n, err := o.file.Write(data)
	if err != nil {
		o.err = err
	}
	return n, err
}

func (o *outputFile) open() error {
	info, err := os.Stat(o.target)
	if err == nil && !info.Mode().IsRegular() {
		o.file, err = os.OpenFile(o.target, os.O_WRONLY, 0)
		if err != nil {
			return fmt.Errorf("opening output file: %w", err)
		}
		return nil
	}

	o.file, err = os.CreateTemp(filepath.Dir(o.target), "."+filepath.Base(o.target)+".*.tmp")
	if err != nil {
		return fmt.Errorf("opening output file: %w", err)
	}
	o.replaces = true
	if info != nil {
		// keep the permissions of the file replaced
		if err := o.file.Chmod(info.Mode().Perm()); err != nil {
			return o.discard(err)
		}
	}
	return nil
}

// Close finishes the export: the written file replaces the one at the path.
// After a failed write, the path is left as it was.
func (o *outputFile) Close() error {
	if o.file == nil {
		return o.err
	}
	if !o.replaces {
		err := o.file.Close()
		o.file = nil
		o.err = errors.Join(o.err, err)
		return o.err
	}
	if o.err != nil {
		return o.discard(o.err)
	}
	if err := o.file.Close(); err != nil {
		return o.discard(err)
	}
	if err := os.Rename(o.file.Name(), o.target); err != nil {
		return o.discard(fmt.Errorf("writing output file: %w", err))
	}
	o.file = nil
	return nil
}

// discard removes the temporary file and returns err.
func (o *outputFile) discard(err error) error {
	o.file.Close()
	os.Remove(o.file.Name())
	o.err = err
	return err
}
