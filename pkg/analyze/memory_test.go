package analyze

import (
	"runtime"
	"runtime/debug"
	"testing"

	"github.com/pbnjay/memory"
	"github.com/stretchr/testify/assert"
)

func TestRebalanceGC(t *testing.T) {
	memStats := runtime.MemStats{}
	runtime.ReadMemStats(&memStats)
	free := memory.FreeMemory()

	disabledGC := false
	rebalanceGC(&disabledGC)

	if free > memStats.Alloc {
		assert.True(t, disabledGC)
		assert.Equal(t, -1, debug.SetGCPercent(100))
	} else {
		assert.False(t, disabledGC)
		assert.Greater(t, 0, debug.SetGCPercent(-1))
	}
}
