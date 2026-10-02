package standalone

import "sync"

// Event is a named event payload emitted during crawl lifecycle or analysis.
type Event struct {
	Name string
	Data any
}

// EventSink is a generic receiver for lifecycle and diagnostic events.
type EventSink interface {
	Emit(name string, data any)
}

// ProgressSink is a dedicated receiver for progress metrics.
type ProgressSink interface {
	OnProgress(data any)
}

// EventCollector is an in-memory EventSink useful for tests and CLI harnesses.
type EventCollector struct {
	mu     sync.Mutex
	events []Event
}

// NewEventCollector creates a thread-safe EventCollector.
func NewEventCollector() *EventCollector {
	return &EventCollector{
		events: make([]Event, 0),
	}
}

// Emit appends an event to the collector.
func (c *EventCollector) Emit(name string, data any) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.events = append(c.events, Event{Name: name, Data: data})
}

// Events returns a copy of all collected events.
func (c *EventCollector) Events() []Event {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]Event, len(c.events))
	copy(out, c.events)
	return out
}

// Len returns the count of collected events.
func (c *EventCollector) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.events)
}

// Clear resets collected events.
func (c *EventCollector) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.events = c.events[:0]
}
