//go:build !windows && !plan9

package analyze

import (
	"archive/zip"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Only a regular file is opened as an archive: opening a FIFO blocks until
// someone writes to it, and a symlink to an archive would count the archive
// twice.
func TestOnlyRegularFilesAreBrowsedAsArchives(t *testing.T) {
	root := filepath.Join(t.TempDir(), "root")
	require.NoError(t, os.MkdirAll(root, 0o755))
	zipped, err := os.Create(filepath.Join(root, "real.zip"))
	require.NoError(t, err)
	zw := zip.NewWriter(zipped)
	w, err := zw.Create("inner")
	require.NoError(t, err)
	_, err = w.Write(make([]byte, 5000))
	require.NoError(t, err)
	require.NoError(t, zw.Close())
	require.NoError(t, zipped.Close())
	require.NoError(t, os.Symlink("real.zip", filepath.Join(root, "link.zip")))
	pipe := filepath.Join(root, "pipe.tar")
	require.NoError(t, syscall.Mkfifo(pipe, 0o600))
	t.Cleanup(func() {
		// let a scan stuck opening the FIFO go
		if f, err := os.OpenFile(pipe, os.O_WRONLY|syscall.O_NONBLOCK, 0); err == nil {
			_ = f.Close()
		}
	})

	for _, tc := range entryParityAnalyzers() {
		t.Run(tc.name, func(t *testing.T) {
			got := collectEntries(scanConfigured(t, tc.analyzer(t), root, func(o *Options) {
				o.ArchiveBrowsing = true
			}))

			assert.True(t, got["real.zip"].dir, "real.zip is browsed")
			assert.False(t, got["link.zip"].dir, "link.zip is a symlink")
			assert.False(t, got["pipe.tar"].dir, "pipe.tar is a FIFO")
		})
	}
}

// Files with several hard links are counted once, however many directories
// hold them, by every analyzer.
func TestHardLinkTotalsParity(t *testing.T) {
	root := filepath.Join(t.TempDir(), "root")
	original := filepath.Join(root, "a", "original")
	writeSized(t, original, 40000, time.Time{})
	writeSized(t, filepath.Join(root, "a", "plain"), 3000, time.Time{})
	for i := range 64 {
		link := filepath.Join(root, fmt.Sprintf("d%02d", i), "link")
		require.NoError(t, os.MkdirAll(filepath.Dir(link), 0o755))
		require.NoError(t, os.Link(original, link))
	}

	// directories occupy blocks of their own on most filesystems
	var once int64
	for _, path := range []string{root, filepath.Dir(original), original, filepath.Join(root, "a", "plain")} {
		once += usageOf(t, path)
	}
	for i := range 64 {
		once += usageOf(t, filepath.Join(root, fmt.Sprintf("d%02d", i)))
	}

	analyzers := entryParityAnalyzers()
	want := scanConfigured(t, analyzers[0].analyzer(t), root, nil)
	require.Equal(t, once, want.GetUsage(), "the in-memory analyzer counts the file once")

	for _, tc := range append(analyzers[1:], parityAnalyzer{
		name:     "top-dir",
		analyzer: func(*testing.T) Analyzer { return CreateTopDirAnalyzer() },
	}) {
		t.Run(tc.name, func(t *testing.T) {
			// the order links are met in varies, so scan several times
			for range 5 {
				got := scanConfigured(t, tc.analyzer(t), root, nil)
				require.Equal(t, want.GetSize(), got.GetSize(), "size at the root")
				require.Equal(t, want.GetUsage(), got.GetUsage(), "usage at the root")
			}
		})
	}
}
