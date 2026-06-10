// Package loghub is the in-memory pub/sub for live deployment logs.
//
// Each topic is identified by (deployment_id, phase). Publishers send lines
// (newline-terminated) and subscribers read from a buffered channel. Late
// subscribers receive the full buffered history followed by any future lines,
// so the UI can drop in mid-stream without losing context.
//
// When a phase completes, the publisher calls Close(id, phase) — the topic's
// channel closes and subscribers see EOF on their range loop.
package loghub

import (
	"sync"
)

const (
	subBuffer = 1024 // per-subscriber channel buffer
	maxBuffer = 8192 // max retained lines per topic (older lines dropped from history)
)

type Hub struct {
	mu     sync.Mutex
	topics map[string]*topic
}

type topic struct {
	mu     sync.Mutex
	buf    []string
	subs   map[*Subscription]struct{}
	closed bool
}

type Subscription struct {
	hub   *Hub
	key   string
	ch    chan string
	close sync.Once
}

func New() *Hub {
	return &Hub{topics: map[string]*topic{}}
}

func key(id, phase string) string { return id + "::" + phase }

func (h *Hub) topic(k string) *topic {
	t, ok := h.topics[k]
	if !ok {
		t = &topic{subs: map[*Subscription]struct{}{}}
		h.topics[k] = t
	}
	return t
}

// Publish appends a line to the topic and fans out to current subscribers.
func (h *Hub) Publish(id, phase, line string) {
	k := key(id, phase)
	h.mu.Lock()
	t := h.topic(k)
	h.mu.Unlock()

	t.mu.Lock()
	t.buf = append(t.buf, line)
	if len(t.buf) > maxBuffer {
		t.buf = t.buf[len(t.buf)-maxBuffer:]
	}
	subs := make([]*Subscription, 0, len(t.subs))
	for s := range t.subs {
		subs = append(subs, s)
	}
	t.mu.Unlock()

	for _, s := range subs {
		// Non-blocking send. If a subscriber is slow, drop the line for that
		// subscriber rather than blocking the publisher.
		select {
		case s.ch <- line:
		default:
		}
	}
}

// Close signals the phase is done. Subscribers' channels are closed and the
// topic is dropped (next Subscribe creates a fresh one — late subscribers see
// the persisted log in the DB instead).
func (h *Hub) Close(id, phase string) {
	k := key(id, phase)
	h.mu.Lock()
	t, ok := h.topics[k]
	if !ok {
		h.mu.Unlock()
		return
	}
	delete(h.topics, k)
	h.mu.Unlock()

	t.mu.Lock()
	t.closed = true
	subs := t.subs
	t.subs = nil
	t.mu.Unlock()
	for s := range subs {
		s.close.Do(func() { close(s.ch) })
	}
}

// Buffered returns the lines accumulated so far. Used when a subscriber wants
// the prior history alongside live updates.
func (h *Hub) Buffered(id, phase string) []string {
	h.mu.Lock()
	t, ok := h.topics[key(id, phase)]
	h.mu.Unlock()
	if !ok {
		return nil
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	out := make([]string, len(t.buf))
	copy(out, t.buf)
	return out
}

// Subscribe attaches and returns a Subscription. Call Close when done.
// Returns (sub, alreadyClosed). When alreadyClosed is true, the topic has
// already finished — caller should serve the persisted log instead.
func (h *Hub) Subscribe(id, phase string) (*Subscription, bool) {
	k := key(id, phase)
	h.mu.Lock()
	t, exists := h.topics[k]
	h.mu.Unlock()
	if !exists {
		return nil, true
	}

	sub := &Subscription{hub: h, key: k, ch: make(chan string, subBuffer)}
	t.mu.Lock()
	if t.closed {
		t.mu.Unlock()
		return nil, true
	}
	// Replay history first so the channel reads start with the prior lines.
	for _, line := range t.buf {
		select {
		case sub.ch <- line:
		default: // shouldn't happen with 1024 buffer for typical builds
		}
	}
	t.subs[sub] = struct{}{}
	t.mu.Unlock()
	return sub, false
}

// Channel returns the read-side of this subscription's stream.
func (s *Subscription) Channel() <-chan string { return s.ch }

// Close detaches the subscription. Idempotent.
func (s *Subscription) Close() {
	s.hub.mu.Lock()
	t, ok := s.hub.topics[s.key]
	s.hub.mu.Unlock()
	if ok {
		t.mu.Lock()
		delete(t.subs, s)
		t.mu.Unlock()
	}
	s.close.Do(func() { close(s.ch) })
}
