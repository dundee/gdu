package webui

import (
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBrowserCommandEmptyCustomCommand(t *testing.T) {
	_, _, err := browserCommand("linux", "http://x", "   ")
	assert.Error(t, err)
}

func TestRunDetachedSuccess(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip(`uses the unix "true" command`)
	}
	require.NoError(t, runDetached("true"))
	time.Sleep(50 * time.Millisecond) // let the reaping goroutine run
}

func TestRunDetachedStartError(t *testing.T) {
	err := runDetached("gdu-test-nonexistent-binary-xyz")
	assert.Error(t, err)
}

func TestOpenBrowserSuccess(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip(`uses the unix "true" command`)
	}
	require.NoError(t, openBrowser("http://example.com", "true"))
	time.Sleep(50 * time.Millisecond)
}

func TestOpenBrowserPropagatesCommandError(t *testing.T) {
	err := openBrowser("http://example.com", "   ")
	assert.Error(t, err)
}
