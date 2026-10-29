package analyze

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dundee/gdu/v5/pkg/fs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTopDirAnalyzeHardLinks(t *testing.T) {
	for _, tc := range []struct {
		name        string
		paths       []string
		ignored     string
		independent bool
		copies      int64
	}{
		{name: "root links", paths: []string{"original", "copy"}, copies: 1},
		{name: "root and earlier directory", paths: []string{"original", "aaa/copy"}, copies: 1},
		{name: "root and later directory", paths: []string{"original", "zzz/copy"}, copies: 1},
		{name: "nested links", paths: []string{"nested/original", "nested/copy"}, copies: 1},
		{name: "links across directories", paths: []string{"aaa/original", "zzz/copy"}, copies: 1},
		{
			name: "filtered root link", paths: []string{"original.log", "nested/copy.txt"},
			ignored: "original.log", copies: 1,
		},
		{
			name: "independent files", paths: []string{"original", "copy"},
			independent: true, copies: 2,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			content := []byte("hello")
			original := filepath.Join(root, tc.paths[0])
			for _, name := range tc.paths {
				path := filepath.Join(root, name)
				require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
				if path == original || tc.independent {
					require.NoError(t, os.WriteFile(path, content, 0o600))
				} else {
					require.NoError(t, os.Link(original, path))
				}
			}

			info, err := os.Stat(original)
			require.NoError(t, err)
			usage, mli := getPlatformSpecificUsageAndMli(info)
			if !tc.independent && mli == 0 {
				t.Skip("hard-link accounting is not supported on this platform")
			}

			analyzer := CreateTopDirAnalyzer()
			for scan := range 2 {
				if scan > 0 {
					analyzer.ResetProgress()
				}
				dir := analyzer.AnalyzeDir(
					root, func(_, _ string) bool { return false },
					func(name string) bool { return tc.ignored != "" && name == tc.ignored },
				)
				analyzer.GetDone().Wait()
				dir.UpdateStats(make(fs.HardLinkedItems))

				assert.Equal(t, int64(len(content))*tc.copies, dir.GetSize(), "scan %d apparent size", scan)
				assert.Equal(t, usage*tc.copies, dir.GetUsage(), "scan %d disk usage", scan)

				// Every root-level file still shows its own size, even when its
				// storage has already been counted in the directory total.
				rootFiles := make(map[string]fs.Item)
				for item := range dir.GetFiles(fs.SortByName, fs.SortAsc) {
					if !item.IsDir() {
						rootFiles[item.GetName()] = item
					}
				}
				for _, name := range tc.paths {
					if filepath.Dir(name) != "." || name == tc.ignored {
						continue
					}
					item, ok := rootFiles[name]
					require.True(t, ok, "missing file %q", name)
					assert.Equal(t, int64(len(content)), item.GetSize())
					assert.Equal(t, usage, item.GetUsage())
				}
			}
		})
	}
}
