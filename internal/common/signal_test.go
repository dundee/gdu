package common

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestSignalGroupWait(t *testing.T) {
	sg := make(SignalGroup)
	done := make(chan struct{})

	go func() {
		sg.Wait()
		done <- struct{}{}
	}()

	// Give the goroutine time to block on Wait()
	time.Sleep(10 * time.Millisecond)

	// Broadcast should unblock the Wait
	sg.Broadcast()

	// Verify goroutine exited
	select {
	case <-done:
		// Success
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Wait() did not unblock after Broadcast()")
	}
}

func TestSignalGroupBroadcast(t *testing.T) {
	sg := make(SignalGroup)

	// Broadcast should not panic on a fresh channel
	assert.NotPanics(t, func() {
		sg.Broadcast()
	})
}

func TestSignalGroupWaitAfterBroadcast(t *testing.T) {
	sg := make(SignalGroup)
	sg.Broadcast()

	// Wait on already-closed channel should return immediately
	done := make(chan struct{})
	go func() {
		sg.Wait()
		done <- struct{}{}
	}()

	select {
	case <-done:
		// Success
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Wait() did not return after already-broadcast channel")
	}
}
