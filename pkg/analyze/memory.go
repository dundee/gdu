package analyze

import (
	"runtime"
	"runtime/debug"
	"time"

	"github.com/pbnjay/memory"
	log "github.com/sirupsen/logrus"
)

// set GC percentage according to memory usage and system free memory
func manageMemoryUsage(c <-chan struct{}) {
	disabledGC := true

	for {
		select {
		case <-c:
			return
		default:
		}

		time.Sleep(time.Second)

		rebalanceGC(&disabledGC)
	}
}

/*
	Try to balance performance and memory consumption.

	When less memory is used by gdu than the total free memory of the host,
	Garbage Collection is disabled during the analysis phase at all.

	Otherwise GC is enabled.
	The more memory is used and the less memory is free,
	the more often will the GC happen.
*/
func rebalanceGC(disabledGC *bool) {
	memStats := runtime.MemStats{}
	runtime.ReadMemStats(&memStats)
	free := memory.FreeMemory()

	// we use less memory than is free, disable GC
	if memStats.Alloc < free {
		if !*disabledGC {
			log.Printf(
				"disabling GC, alloc: %d, free: %d", memStats.Alloc, free,
			)
			debug.SetGCPercent(-1)
			*disabledGC = true
		}
	} else {
		// the more memory we use and the less memory is free, the more aggresive the GC will be
		gcPercent := int(100 / float64(memStats.Alloc) * float64(free))
		log.Printf(
			"setting GC percent to %d, alloc: %d, free: %d",
			gcPercent, memStats.Alloc, free,
		)
		debug.SetGCPercent(gcPercent)
		*disabledGC = false
	}
}
