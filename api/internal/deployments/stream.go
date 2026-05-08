package deployments

import (
	"sync"
	"time"
)

type EventType string

const (
	EventStepStart          EventType = "step_start"
	EventStepEnd            EventType = "step_end"
	EventLog                EventType = "log"
	EventAnsible            EventType = "ansible_event"
	EventDeploymentComplete EventType = "deployment_complete"
)

type Event struct {
	Type   EventType `json:"type"`
	Step   string    `json:"step,omitempty"`
	Stream string    `json:"stream,omitempty"`
	Text   string    `json:"text,omitempty"`
	Status string    `json:"status,omitempty"`
	Exit   int       `json:"exit_code,omitempty"`
	Data   any       `json:"data,omitempty"`
	TS     time.Time `json:"ts"`
}

// HistoryCap caps per-deployment retained events. Events past this are
// dropped from the front (FIFO). Sized to comfortably hold a typical
// deploy (a few hundred ansible events + step transitions).
const HistoryCap = 5000

// Hub fans out events from one publisher to N subscribers per deployment,
// while also retaining a per-deployment ring buffer of recent events so
// late subscribers (eg. browser tab that navigated away and back) can
// replay history before the live stream resumes.
type Hub struct {
	mu      sync.Mutex
	subs    map[string]map[chan Event]struct{}
	history map[string][]Event
}

func NewHub() *Hub {
	return &Hub{
		subs:    map[string]map[chan Event]struct{}{},
		history: map[string][]Event{},
	}
}

func (h *Hub) Publish(deploymentID string, e Event) {
	if e.TS.IsZero() {
		e.TS = time.Now().UTC()
	}
	h.mu.Lock()
	defer h.mu.Unlock()

	// Append to history with FIFO eviction.
	hist := h.history[deploymentID]
	if len(hist) >= HistoryCap {
		hist = hist[len(hist)-HistoryCap+1:]
	}
	h.history[deploymentID] = append(hist, e)

	for ch := range h.subs[deploymentID] {
		select {
		case ch <- e:
		default:
		}
	}
}

// Subscribe returns the current event history (oldest first) and a channel
// that will receive every subsequent event. Caller writes the snapshot to
// the client first, then drains the channel for live updates.
func (h *Hub) Subscribe(deploymentID string, buf int) (snapshot []Event, ch <-chan Event, unsubscribe func()) {
	c := make(chan Event, buf)
	h.mu.Lock()
	if h.subs[deploymentID] == nil {
		h.subs[deploymentID] = map[chan Event]struct{}{}
	}
	h.subs[deploymentID][c] = struct{}{}
	// Copy the history slice so the caller can iterate without holding the lock.
	hist := h.history[deploymentID]
	snap := make([]Event, len(hist))
	copy(snap, hist)
	h.mu.Unlock()
	return snap, c, func() {
		h.mu.Lock()
		defer h.mu.Unlock()
		delete(h.subs[deploymentID], c)
		close(c)
	}
}
