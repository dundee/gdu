//go:build !windows

package remove

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dundee/gdu/v5/pkg/analyze"
)

type stubTrashInfoFile struct {
	writeErr error
	closeErr error
}

func (f *stubTrashInfoFile) WriteString(s string) (int, error) {
	if f.writeErr != nil {
		return 0, f.writeErr
	}
	return len(s), nil
}

func (f *stubTrashInfoFile) Close() error {
	return f.closeErr
}

type stubWriteCloser struct {
	bytes.Buffer
	closeErr error
}

func (w *stubWriteCloser) Close() error {
	return w.closeErr
}

func TestMoveItemToTrashSetupErrors(t *testing.T) {
	sentinel := errors.New("sentinel")

	tests := []struct {
		name      string
		configure func(*trashOSOps)
		unsetXDG  bool
	}{
		{
			name: "home directory",
			configure: func(ops *trashOSOps) {
				ops.userHomeDir = func() (string, error) { return "", sentinel }
			},
			unsetXDG: true,
		},
		{
			name: "files directory",
			configure: func(ops *trashOSOps) {
				ops.mkdirAll = func(string, os.FileMode) error { return sentinel }
			},
		},
		{
			name: "info directory",
			configure: func(ops *trashOSOps) {
				calls := 0
				ops.mkdirAll = func(string, os.FileMode) error {
					calls++
					if calls == 2 {
						return sentinel
					}
					return nil
				}
			},
		},
		{
			name: "absolute source path",
			configure: func(ops *trashOSOps) {
				ops.abs = func(string) (string, error) { return "", sentinel }
			},
		},
		{
			name: "trashinfo reservation",
			configure: func(ops *trashOSOps) {
				ops.openTrashInfo = func(string, int, os.FileMode) (trashInfoFile, error) {
					return nil, sentinel
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.unsetXDG {
				t.Setenv("XDG_DATA_HOME", "")
			} else {
				t.Setenv("XDG_DATA_HOME", t.TempDir())
			}
			mockTrashOS(t, tt.configure)
			parent := &analyze.Dir{
				File:     &analyze.File{Name: "parent"},
				BasePath: ".",
			}
			item := &analyze.File{Name: "item", Parent: parent}

			err := MoveItemToTrash(nil, item)

			require.ErrorIs(t, err, sentinel)
		})
	}
}

func TestMoveItemToTrashExhaustsDestinationRetries(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	mockTrashOS(t, func(ops *trashOSOps) {
		ops.lstat = func(string) (os.FileInfo, error) { return nil, os.ErrNotExist }
		ops.openTrashInfo = func(string, int, os.FileMode) (trashInfoFile, error) {
			return &stubTrashInfoFile{}, nil
		}
		ops.rename = func(string, string) error { return os.ErrExist }
		ops.remove = func(string) error { return nil }
	})
	parent := &analyze.Dir{
		File:     &analyze.File{Name: "parent"},
		BasePath: ".",
	}
	item := &analyze.File{Name: "item", Parent: parent}

	err := MoveItemToTrash(nil, item)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "could not find unique trash name")
}

func TestReserveTrashInfoErrorPaths(t *testing.T) {
	sentinel := errors.New("sentinel")
	info, err := os.Stat(t.TempDir())
	require.NoError(t, err)

	t.Run("initial lstat", func(t *testing.T) {
		mockTrashOS(t, func(ops *trashOSOps) {
			ops.lstat = func(string) (os.FileInfo, error) { return nil, sentinel }
		})

		_, _, err := reserveTrashInfo("files", "info", "item", "/item")

		require.ErrorIs(t, err, sentinel)
	})

	t.Run("post-reservation lstat", func(t *testing.T) {
		calls := 0
		removed := false
		mockTrashOS(t, func(ops *trashOSOps) {
			ops.lstat = func(string) (os.FileInfo, error) {
				calls++
				if calls == 1 {
					return nil, os.ErrNotExist
				}
				return nil, sentinel
			}
			ops.openTrashInfo = func(string, int, os.FileMode) (trashInfoFile, error) {
				return &stubTrashInfoFile{}, nil
			}
			ops.remove = func(string) error {
				removed = true
				return nil
			}
		})

		_, _, err := reserveTrashInfo("files", "info", "item", "/item")

		require.ErrorIs(t, err, sentinel)
		assert.True(t, removed)
	})

	t.Run("destination appears after reservation", func(t *testing.T) {
		calls := 0
		removed := false
		mockTrashOS(t, func(ops *trashOSOps) {
			ops.lstat = func(string) (os.FileInfo, error) {
				calls++
				switch calls {
				case 1, 3, 4:
					return nil, os.ErrNotExist
				default:
					return info, nil
				}
			}
			ops.openTrashInfo = func(string, int, os.FileMode) (trashInfoFile, error) {
				return &stubTrashInfoFile{}, nil
			}
			ops.remove = func(string) error {
				removed = true
				return nil
			}
		})

		name, _, err := reserveTrashInfo("files", "info", "item", "/item")

		require.NoError(t, err)
		assert.Equal(t, "item.2", name)
		assert.True(t, removed)
	})

	t.Run("all names occupied", func(t *testing.T) {
		mockTrashOS(t, func(ops *trashOSOps) {
			ops.lstat = func(string) (os.FileInfo, error) { return info, nil }
		})

		_, _, err := reserveTrashInfo("files", "info", "item", "/item")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "could not find unique trash name")
	})
}

func TestWriteTrashInfoErrors(t *testing.T) {
	sentinel := errors.New("sentinel")

	t.Run("write", func(t *testing.T) {
		removed := false
		mockTrashOS(t, func(ops *trashOSOps) {
			ops.openTrashInfo = func(string, int, os.FileMode) (trashInfoFile, error) {
				return &stubTrashInfoFile{writeErr: sentinel}, nil
			}
			ops.remove = func(string) error {
				removed = true
				return nil
			}
		})

		err := writeTrashInfo("item.trashinfo", "/item")

		require.ErrorIs(t, err, sentinel)
		assert.True(t, removed)
	})

	t.Run("close", func(t *testing.T) {
		removed := false
		mockTrashOS(t, func(ops *trashOSOps) {
			ops.openTrashInfo = func(string, int, os.FileMode) (trashInfoFile, error) {
				return &stubTrashInfoFile{closeErr: sentinel}, nil
			}
			ops.remove = func(string) error {
				removed = true
				return nil
			}
		})

		err := writeTrashInfo("item.trashinfo", "/item")

		require.ErrorIs(t, err, sentinel)
		assert.True(t, removed)
	})
}

func TestCopyRecursivelyErrorPaths(t *testing.T) {
	sentinel := errors.New("sentinel")

	t.Run("read directory", func(t *testing.T) {
		src := t.TempDir()
		mockTrashOS(t, func(ops *trashOSOps) {
			ops.readDir = func(string) ([]os.DirEntry, error) { return nil, sentinel }
		})

		err := copyRecursively(src, filepath.Join(t.TempDir(), "dst"))

		require.ErrorIs(t, err, sentinel)
	})

	t.Run("child copy", func(t *testing.T) {
		src := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(src, "child"), []byte("data"), 0o600))
		mockTrashOS(t, func(ops *trashOSOps) {
			ops.openSource = func(string) (io.ReadCloser, error) { return nil, sentinel }
		})

		err := copyRecursively(src, filepath.Join(t.TempDir(), "dst"))

		require.ErrorIs(t, err, sentinel)
	})

	t.Run("readlink", func(t *testing.T) {
		src := filepath.Join(t.TempDir(), "link")
		require.NoError(t, os.Symlink("target", src))
		mockTrashOS(t, func(ops *trashOSOps) {
			ops.readlink = func(string) (string, error) { return "", sentinel }
		})

		err := copyRecursively(src, filepath.Join(t.TempDir(), "dst"))

		require.ErrorIs(t, err, sentinel)
	})

	t.Run("symlink", func(t *testing.T) {
		src := filepath.Join(t.TempDir(), "link")
		require.NoError(t, os.Symlink("target", src))
		mockTrashOS(t, func(ops *trashOSOps) {
			ops.symlink = func(string, string) error { return sentinel }
		})

		err := copyRecursively(src, filepath.Join(t.TempDir(), "dst"))

		require.ErrorIs(t, err, sentinel)
	})

	t.Run("open source", func(t *testing.T) {
		src := filepath.Join(t.TempDir(), "file")
		require.NoError(t, os.WriteFile(src, []byte("data"), 0o600))
		mockTrashOS(t, func(ops *trashOSOps) {
			ops.openSource = func(string) (io.ReadCloser, error) { return nil, sentinel }
		})

		err := copyRecursively(src, filepath.Join(t.TempDir(), "dst"))

		require.ErrorIs(t, err, sentinel)
	})

	t.Run("open destination", func(t *testing.T) {
		src := filepath.Join(t.TempDir(), "file")
		require.NoError(t, os.WriteFile(src, []byte("data"), 0o600))
		mockTrashOS(t, func(ops *trashOSOps) {
			ops.openDestination = func(string, int, os.FileMode) (io.WriteCloser, error) {
				return nil, sentinel
			}
		})

		err := copyRecursively(src, filepath.Join(t.TempDir(), "dst"))

		require.ErrorIs(t, err, sentinel)
	})

	t.Run("copy", func(t *testing.T) {
		src := filepath.Join(t.TempDir(), "file")
		require.NoError(t, os.WriteFile(src, []byte("data"), 0o600))
		mockTrashOS(t, func(ops *trashOSOps) {
			ops.copy = func(io.Writer, io.Reader) (int64, error) { return 0, sentinel }
		})

		err := copyRecursively(src, filepath.Join(t.TempDir(), "dst"))

		require.ErrorIs(t, err, sentinel)
	})

	t.Run("close destination", func(t *testing.T) {
		src := filepath.Join(t.TempDir(), "file")
		require.NoError(t, os.WriteFile(src, []byte("data"), 0o600))
		mockTrashOS(t, func(ops *trashOSOps) {
			ops.openDestination = func(string, int, os.FileMode) (io.WriteCloser, error) {
				return &stubWriteCloser{closeErr: sentinel}, nil
			}
		})

		err := copyRecursively(src, filepath.Join(t.TempDir(), "dst"))

		require.ErrorIs(t, err, sentinel)
	})
}
