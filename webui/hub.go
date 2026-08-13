package webui

import "sync"

// hub is a minimal Server-Sent Events broadcaster. Subscribers register a
// buffered channel and receive the latest status messages. The most recent
// message is retained so newly connected clients get the current state at once.
type hub struct {
	mu   sync.Mutex
	subs map[chan string]struct{}
	last string
}

func newHub() *hub {
	return &hub{subs: make(map[chan string]struct{})}
}

// subscribe registers a new subscriber and returns its channel plus the last
// broadcast message (empty if none yet).
func (h *hub) subscribe() (events chan string, last string) {
	events = make(chan string, 8)
	h.mu.Lock()
	h.subs[events] = struct{}{}
	last = h.last
	h.mu.Unlock()
	return events, last
}

// unsubscribe removes a subscriber and closes its channel.
func (h *hub) unsubscribe(ch chan string) {
	h.mu.Lock()
	if _, ok := h.subs[ch]; ok {
		delete(h.subs, ch)
		close(ch)
	}
	h.mu.Unlock()
}

// publish stores the message as the latest state and delivers it to every
// subscriber without blocking on slow consumers.
func (h *hub) publish(msg string) {
	h.mu.Lock()
	h.last = msg
	for ch := range h.subs {
		select {
		case ch <- msg:
		default:
		}
	}
	h.mu.Unlock()
}
