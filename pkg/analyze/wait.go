package analyze

import "sync"

// A WaitGroup waits for a collection of goroutines to finish.
// In contrast to sync.WaitGroup Add method can be called from a goroutine.
type WaitGroup struct {
	wait   sync.Mutex
	value  int
	access sync.Mutex
}

// Init prepares the WaitGroup for usage, locks
func (s *WaitGroup) Init() *WaitGroup {
	s.wait.Lock()
	return s
}

// Add increments value
func (s *WaitGroup) Add(value int) {
	s.access.Lock()
	s.value = s.value + value
	s.access.Unlock()
}

// Done decrements the value by one, if value is 0, lock is released
func (s *WaitGroup) Done() {
	s.access.Lock()
	s.value--
	s.check()
	s.access.Unlock()
}

// Wait blocks until value is 0
func (s *WaitGroup) Wait() {
	s.access.Lock()
	isValue := s.value > 0
	s.access.Unlock()
	if isValue {
		s.wait.Lock()
	}
}

func (s *WaitGroup) check() {
	if s.value == 0 {
		s.wait.Unlock()
	}
}
