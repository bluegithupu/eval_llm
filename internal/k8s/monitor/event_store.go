package monitor

import (
	"context"
	"fmt"
	"time"
)

// EventStore interface for storing job events
type EventStore interface {
	StoreEvent(ctx context.Context, event *JobEvent) error
	GetEvents(ctx context.Context, evalID string) ([]*JobEvent, error)
	GetEventsByType(ctx context.Context, evalID string, eventType JobEventType) ([]*JobEvent, error)
	ClearOldEvents(ctx context.Context, olderThan time.Duration) error
}

// InMemoryEventStore implements EventStore using in-memory storage
// For production, this would be replaced with a database-backed implementation
type InMemoryEventStore struct {
	events    map[string][]*JobEvent
	maxEvents int
}

// NewInMemoryEventStore creates a new in-memory event store
func NewInMemoryEventStore() *InMemoryEventStore {
	return &InMemoryEventStore{
		events:    make(map[string][]*JobEvent),
		maxEvents: 1000,
	}
}

// StoreEvent stores a job event
func (s *InMemoryEventStore) StoreEvent(ctx context.Context, event *JobEvent) error {
	if event == nil || event.EvalID == "" {
		return fmt.Errorf("invalid event")
	}

	s.events[event.EvalID] = append(s.events[event.EvalID], event)

	// Trim old events if we exceed max
	if len(s.events[event.EvalID]) > s.maxEvents {
		s.events[event.EvalID] = s.events[event.EvalID][len(s.events[event.EvalID])-s.maxEvents:]
	}

	return nil
}

// GetEvents retrieves all events for an evaluation
func (s *InMemoryEventStore) GetEvents(ctx context.Context, evalID string) ([]*JobEvent, error) {
	events, ok := s.events[evalID]
	if !ok {
		return []*JobEvent{}, nil
	}
	return events, nil
}

// GetEventsByType retrieves events of a specific type for an evaluation
func (s *InMemoryEventStore) GetEventsByType(ctx context.Context, evalID string, eventType JobEventType) ([]*JobEvent, error) {
	events, ok := s.events[evalID]
	if !ok {
		return []*JobEvent{}, nil
	}

	var filtered []*JobEvent
	for _, e := range events {
		if e.EventType == eventType {
			filtered = append(filtered, e)
		}
	}
	return filtered, nil
}

// ClearOldEvents removes events older than the specified duration
func (s *InMemoryEventStore) ClearOldEvents(ctx context.Context, olderThan time.Duration) error {
	cutoff := time.Now().Add(-olderThan)

	for evalID, events := range s.events {
		var filtered []*JobEvent
		for _, e := range events {
			if e.Timestamp.After(cutoff) {
				filtered = append(filtered, e)
			}
		}
		s.events[evalID] = filtered
	}

	return nil
}

// GetLatestEvent returns the most recent event for an evaluation
func (s *InMemoryEventStore) GetLatestEvent(ctx context.Context, evalID string) (*JobEvent, error) {
	events, ok := s.events[evalID]
	if !ok || len(events) == 0 {
		return nil, nil
	}
	return events[len(events)-1], nil
}
