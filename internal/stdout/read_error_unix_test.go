//go:build !windows && !plan9

package stdout

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dundee/gdu/v6/internal/config"
	"github.com/dundee/gdu/v6/pkg/analyze"
)

// As du does, gdu lists what it could read on stdout, and tells on stderr
// what it could not, one line each; the run does not fail.
func TestReadErrorsGoToStderrAsInDu(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root reads every directory")
	}
	root := t.TempDir()
	locked := filepath.Join(root, "locked")
	require.NoError(t, os.Mkdir(locked, 0o755))
	require.NoError(t, os.Mkdir(filepath.Join(root, "ok"), 0o755))
	require.NoError(t, os.Chmod(locked, 0o000))
	t.Cleanup(func() { _ = os.Chmod(locked, 0o755) })

	for name, settings := range map[string]config.Settings{
		"listing":   {},
		"summarize": {Display: config.Display{Summarize: true}},
		"depth":     {Display: config.Display{Depth: 2}},
		"top":       {Display: config.Display{Top: 5}},
	} {
		t.Run(name, func(t *testing.T) {
			var out, errOut bytes.Buffer
			ui := CreateStdoutUI(&out, &errOut, analyze.CreateAnalyzer(), &settings)

			require.NoError(t, ui.AnalyzePath(root, nil))

			assert.Equal(t, "gdu: cannot read directory '"+locked+"': Permission denied\n", errOut.String())
			assert.NotContains(t, out.String(), "Permission denied")
		})
	}
}

func TestReadErrorMessage(t *testing.T) {
	_, err := os.ReadDir("/nonexistent-gdu-test")
	require.Error(t, err)

	assert.Equal(t, "gdu: cannot access '/x/y': No such file or directory",
		readErrorMessage(analyze.ReadError{Path: "/x/y", Err: err}))
}
