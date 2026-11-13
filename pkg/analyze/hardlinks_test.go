package analyze

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Inode numbers repeat across filesystems, so files on two devices are told
// apart even when their inode numbers are equal.
func TestLinkTrackerTellsDevicesApart(t *testing.T) {
	var links linkTracker

	assert.True(t, links.claim(inodeKey{dev: 1, ino: 42}))
	assert.False(t, links.claim(inodeKey{dev: 1, ino: 42}))
	assert.True(t, links.claim(inodeKey{dev: 2, ino: 42}))

	links.reset()
	assert.True(t, links.claim(inodeKey{dev: 1, ino: 42}))
}

func TestLinkTrackerClaimsOnce(t *testing.T) {
	var (
		links  linkTracker
		wg     sync.WaitGroup
		mu     sync.Mutex
		claims int
	)
	for range 64 {
		wg.Go(func() {
			if links.claim(inodeKey{dev: 1, ino: 7}) {
				mu.Lock()
				claims++
				mu.Unlock()
			}
		})
	}
	wg.Wait()

	assert.Equal(t, 1, claims)
}
