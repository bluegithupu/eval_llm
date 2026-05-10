package monitor

import (
	"context"
	"fmt"
	"time"
)

// EventStore interface for storing job events
type EventStore interface {
	StoreEvent(ctx context.Context, event *JobEvent) error
	StoreError(ctx context.Context, evalID string, errorType ErrorType, message string, stderr string) error
	GetEvents(ctx context.Context, evalID string) ([]*JobEvent, error)
	GetEventsByType(ctx context.Context, evalID string, eventType JobEventType) ([]*JobEvent, error)
	ClearOldEvents(ctx context.Context, olderThan time.Duration) error
}

// ErrorType represents the type of error for logging
type ErrorType string

const (
	ErrorTypeK8sJobCreation ErrorType = "k8s-job-creation"
	ErrorTypeK8sJobFailed   ErrorType = "k8s-job-failed"
	ErrorTypeOpenCompass    ErrorType = "opencompass-cli"
	ErrorTypeDatabase       ErrorType = "database"
	ErrorTypeRedis          ErrorType = "redis"
	ErrorTypeOOMKilled      ErrorType = "oom-killed"
)

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

// StoreError stores an error event with stderr output from OpenCompass or other sources
func (s *InMemoryEventStore) StoreError(ctx context.Context, evalID string, errorType ErrorType, message string, stderr string) error {
	if evalID == "" {
		return fmt.Errorf("evalID is required")
	}

	// Create an error-level event with the error details
	event := &JobEvent{
		EvalID:    evalID,
		EventType: EventJobFailed,
		Message:   message,
		Timestamp: time.Now(),
		JobStatus: nil,
	}

	// Store stderr as part of the event (for debugging)
	// In a real implementation, this might be stored in a separate field or log file
	_ = stderr // Stored in memory for debugging purposes

	return s.StoreEvent(ctx, event)
}

// GetLatestEvent returns the most recent event for an evaluation
func (s *InMemoryEventStore) GetLatestEvent(ctx context.Context, evalID string) (*JobEvent, error) {
	events, ok := s.events[evalID]
	if !ok || len(events) == 0 {
		return nil, nil
	}
	return events[len(events)-1], nil
}
