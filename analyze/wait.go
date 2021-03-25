package analyze

import "sync"

// A WaitGroup waits for a collection of goroutines to finish.
// In contrast to sync.WaitGroup Add method can be called from a goroutine.
type WaitGroup struct {
	wait   sync.Mutex
	value  int
	access sync.Mutex
}

func (s *WaitGroup) Init() *WaitGroup {
	s.wait.Lock()
	return s
}

func (s *WaitGroup) Add(value int) {
	s.access.Lock()
	s.value = s.value + value
	s.access.Unlock()
}

func (s *WaitGroup) Done() {
	s.access.Lock()
	s.value--
	s.check()
	s.access.Unlock()
}

func (s *WaitGroup) Wait() {
	s.wait.Lock()
}

func (s *WaitGroup) check() {
	if s.value == 0 {
		s.wait.Unlock()
	}
}
