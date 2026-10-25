package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dundee/gdu/v5/internal/testdev"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestNonInteractiveConfiguredSorting(t *testing.T) {
	dir := t.TempDir()
	for index, file := range []struct {
		name string
		size int
	}{{"alpha", 3}, {"bravo", 1}, {"charlie", 2}} {
		path := filepath.Join(dir, file.name)
		require.NoError(t, os.WriteFile(path, []byte(strings.Repeat("x", file.size)), 0o600))
		modified := time.Unix(int64(300-index*100), 0)
		require.NoError(t, os.Chtimes(path, modified, modified))
	}

	for _, tc := range []struct {
		name, by, order string
		reverse         bool
		depth, top      int
		want            []string
	}{
		{"default", "", "", false, 0, 0, []string{"alpha", "charlie", "bravo"}},
		{"name ascending", "name", "asc", false, 0, 0, []string{"alpha", "bravo", "charlie"}},
		{"name descending", "name", "desc", false, 0, 0, []string{"charlie", "bravo", "alpha"}},
		{"size ascending", "size", "asc", false, 0, 0, []string{"bravo", "charlie", "alpha"}},
		{"mtime ascending", "mtime", "asc", false, 0, 0, []string{"charlie", "bravo", "alpha"}},
		{"mtime descending", "mtime", "desc", false, 0, 0, []string{"alpha", "bravo", "charlie"}},
		{"reverse configured order", "size", "asc", true, 0, 0, []string{"alpha", "charlie", "bravo"}},
		{"depth", "name", "asc", false, 1, 0, []string{"alpha", "bravo", "charlie"}},
		{"top still selects largest files", "name", "asc", false, 0, 2, []string{"alpha", "charlie"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			flags := &Flags{
				LogFile: os.DevNull, NonInteractive: true, NoPrefix: true,
				ShowApparentSize: true, ReverseSort: tc.reverse, Depth: tc.depth, Top: tc.top,
			}
			config := "sorting:\n  by: " + tc.by + "\n  order: " + tc.order + "\n"
			require.NoError(t, yaml.Unmarshal([]byte(config), flags))
			output, err := runApp(flags, []string{dir}, false, testdev.DevicesInfoGetterMock{})
			require.NoError(t, err)
			var names []string
			for _, line := range strings.Split(output, "\n") {
				fields := strings.Fields(line)
				if len(fields) == 0 {
					continue
				}
				name := filepath.Base(fields[len(fields)-1])
				if name == "alpha" || name == "bravo" || name == "charlie" {
					names = append(names, name)
				}
			}
			require.Equal(t, tc.want, names, "output:\n%s", output)
		})
	}
}

func TestNonInteractiveConfiguredDirectorySorting(t *testing.T) {
	dir := t.TempDir()
	for index, entry := range []struct {
		name  string
		count int
	}{{"alpha", 3}, {"bravo", 1}, {"charlie", 2}} {
		path := filepath.Join(dir, entry.name)
		require.NoError(t, os.Mkdir(path, 0o700))
		for i := 0; i < entry.count; i++ {
			require.NoError(t, os.WriteFile(filepath.Join(path, string(rune('a'+i))), []byte("x"), 0o600))
		}
		modified := time.Unix(int64(300-index*100), 0)
		require.NoError(t, os.Chtimes(path, modified, modified))
	}
	for _, tc := range []struct {
		by, order string
		want      []string
	}{
		{"itemCount", "asc", []string{"bravo", "charlie", "alpha"}},
		{"itemCount", "desc", []string{"alpha", "charlie", "bravo"}},
		{"mtime", "asc", []string{"charlie", "bravo", "alpha"}},
		{"mtime", "desc", []string{"alpha", "bravo", "charlie"}},
	} {
		t.Run(tc.by+"/"+tc.order, func(t *testing.T) {
			output, err := runApp(&Flags{
				LogFile: os.DevNull, NonInteractive: true,
				Sorting: Sorting{By: tc.by, Order: tc.order},
			}, []string{dir}, false, testdev.DevicesInfoGetterMock{})
			require.NoError(t, err)
			var names []string
			for _, line := range strings.Split(output, "\n") {
				fields := strings.Fields(line)
				if len(fields) != 0 {
					names = append(names, strings.TrimLeft(fields[len(fields)-1], "/"))
				}
			}
			require.Equal(t, tc.want, names, "output:\n%s", output)
		})
	}
}
