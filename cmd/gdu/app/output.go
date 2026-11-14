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
// export does not leave it half written either, unless no file can be
// created next to it. Anything else, such as a device or a named pipe, is
// written to directly.
type outputFile struct {
	path     string
	target   string
	file     *os.File
	replaces bool
	err      error
}

// newOutputFile returns the output file at path. It fails at once when the
// file cannot be written, rather than after a whole scan.
func newOutputFile(path string) (*outputFile, error) {
	target := path
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		target = resolved
	}
	if err := checkWritable(target); err != nil {
		return nil, fmt.Errorf("opening output file: %w", err)
	}
	return &outputFile{path: path, target: target}, nil
}

// checkWritable reports why the file at target cannot be written, changing
// nothing: an existing file is opened without truncating it, otherwise a
// file is created next to it and removed again. A device or a named pipe is
// not opened, as opening a pipe waits for its reader.
func checkWritable(target string) error {
	info, err := os.Stat(target)
	if err == nil {
		if !info.Mode().IsRegular() {
			return nil
		}
		file, err := os.OpenFile(target, os.O_WRONLY, 0)
		if err != nil {
			return err
		}
		return file.Close()
	}
	probe, err := os.CreateTemp(filepath.Dir(target), "."+filepath.Base(target)+".*.tmp")
	if err != nil {
		return err
	}
	probe.Close()
	return os.Remove(probe.Name())
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
	if err != nil && info != nil {
		// a file the user may write in a directory they may not: written in
		// place, as it cannot be replaced
		o.file, err = os.OpenFile(o.target, os.O_WRONLY|os.O_TRUNC, 0)
		if err != nil {
			return fmt.Errorf("opening output file: %w", err)
		}
		return nil
	}
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
