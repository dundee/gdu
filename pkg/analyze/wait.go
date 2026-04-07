package analyze

import "sync"

// A WaitGroup waits for a collection of goroutines to finish.
// In contrast to sync.WaitGroup Add method can be called from a goroutine.
type WaitGroup struct {
	value  int
	access sync.Mutex
	done   chan struct{}
}

// Init prepares the WaitGroup for usage, locks
func (s *WaitGroup) Init() *WaitGroup {
	s.done = make(chan struct{})
	return s
}

// Add increments value
func (s *WaitGroup) Add(value int) {
	s.access.Lock()
	defer s.access.Unlock()
	s.value += value
}

// Done decrements the value by one, if value is 0, lock is released
func (s *WaitGroup) Done() {
	s.access.Lock()
	defer s.access.Unlock()
	s.value--
	if s.value == 0 {
		select {
		case <-s.done:
			// already closed
		default:
			close(s.done)
		}
	}
}

// Wait blocks until value is 0
func (s *WaitGroup) Wait() {
	s.access.Lock()
	isValue := s.value > 0
	s.access.Unlock()
	if isValue {
		<-s.done
	}
}
