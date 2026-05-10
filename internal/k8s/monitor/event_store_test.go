package monitor

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewInMemoryEventStore(t *testing.T) {
	store := NewInMemoryEventStore()
	require.NotNil(t, store)
}

func TestInMemoryEventStore_StoreEvent(t *testing.T) {
	store := NewInMemoryEventStore()
	ctx := context.Background()

	event := &JobEvent{
		EvalID:    "eval-123",
		EventType: EventJobStarted,
		Message:   "Job started",
		Timestamp: time.Now(),
	}

	err := store.StoreEvent(ctx, event)
	require.NoError(t, err)

	// Verify event was stored
	events, err := store.GetEvents(ctx, "eval-123")
	require.NoError(t, err)
	assert.Len(t, events, 1)
	assert.Equal(t, EventJobStarted, events[0].EventType)
}

func TestInMemoryEventStore_StoreMultipleEvents(t *testing.T) {
	store := NewInMemoryEventStore()
	ctx := context.Background()

	evalID := "eval-multi"

	// Store multiple events
	for i := 0; i < 5; i++ {
		event := &JobEvent{
			EvalID:    evalID,
			EventType: EventJobStarted,
			Message:   "Event " + string(rune('0'+i)),
			Timestamp: time.Now(),
		}
		err := store.StoreEvent(ctx, event)
		require.NoError(t, err)
	}

	events, err := store.GetEvents(ctx, evalID)
	require.NoError(t, err)
	assert.Len(t, events, 5)
}

func TestInMemoryEventStore_GetEvents_NotFound(t *testing.T) {
	store := NewInMemoryEventStore()
	ctx := context.Background()

	events, err := store.GetEvents(ctx, "non-existent")
	require.NoError(t, err)
	assert.Len(t, events, 0)
}

func TestInMemoryEventStore_GetEventsByType(t *testing.T) {
	store := NewInMemoryEventStore()
	ctx := context.Background()

	evalID := "eval-type-filter"

	// Store events of different types
	events := []*JobEvent{
		{EvalID: evalID, EventType: EventJobStarted, Message: "started", Timestamp: time.Now()},
		{EvalID: evalID, EventType: EventJobCompleted, Message: "completed", Timestamp: time.Now()},
		{EvalID: evalID, EventType: EventJobStarted, Message: "started again", Timestamp: time.Now()},
	}

	for _, e := range events {
		err := store.StoreEvent(ctx, e)
		require.NoError(t, err)
	}

	// Get only Started events
	startedEvents, err := store.GetEventsByType(ctx, evalID, EventJobStarted)
	require.NoError(t, err)
	assert.Len(t, startedEvents, 2)

	// Get only Completed events
	completedEvents, err := store.GetEventsByType(ctx, evalID, EventJobCompleted)
	require.NoError(t, err)
	assert.Len(t, completedEvents, 1)
}

func TestInMemoryEventStore_GetLatestEvent(t *testing.T) {
	store := NewInMemoryEventStore()
	ctx := context.Background()

	evalID := "eval-latest"

	// Store events with timestamps
	for i := 0; i < 3; i++ {
		event := &JobEvent{
			EvalID:    evalID,
			EventType: EventJobStarted,
			Message:   "Event " + string(rune('0'+i)),
			Timestamp: time.Now().Add(time.Duration(i) * time.Second),
		}
		err := store.StoreEvent(ctx, event)
		require.NoError(t, err)
		time.Sleep(10 * time.Millisecond) // Ensure different timestamps
	}

	// Get latest event
	latest, err := store.GetLatestEvent(ctx, evalID)
	require.NoError(t, err)
	require.NotNil(t, latest)
	assert.Equal(t, "Event 2", latest.Message)
}

func TestInMemoryEventStore_GetLatestEvent_NotFound(t *testing.T) {
	store := NewInMemoryEventStore()
	ctx := context.Background()

	latest, err := store.GetLatestEvent(ctx, "non-existent")
	require.NoError(t, err)
	assert.Nil(t, latest)
}

func TestInMemoryEventStore_ClearOldEvents(t *testing.T) {
	store := NewInMemoryEventStore()
	ctx := context.Background()

	evalID := "eval-clear"

	// Store old event (1 hour ago)
	oldEvent := &JobEvent{
		EvalID:    evalID,
		EventType: EventJobStarted,
		Message:   "Old event",
		Timestamp: time.Now().Add(-1 * time.Hour),
	}
	err := store.StoreEvent(ctx, oldEvent)
	require.NoError(t, err)

	// Store new event (now)
	newEvent := &JobEvent{
		EvalID:    evalID,
		EventType: EventJobCompleted,
		Message:   "New event",
		Timestamp: time.Now(),
	}
	err = store.StoreEvent(ctx, newEvent)
	require.NoError(t, err)

	// Clear events older than 30 minutes
	err = store.ClearOldEvents(ctx, 30*time.Minute)
	require.NoError(t, err)

	// Should only have the new event
	events, err := store.GetEvents(ctx, evalID)
	require.NoError(t, err)
	assert.Len(t, events, 1)
	assert.Equal(t, "New event", events[0].Message)
}

func TestInMemoryEventStore_MaxEventsLimit(t *testing.T) {
	store := NewInMemoryEventStore()
	ctx := context.Background()

	evalID := "eval-max"

	// Store more than max events (1000)
	for i := 0; i < 1500; i++ {
		event := &JobEvent{
			EvalID:    evalID,
			EventType: EventJobStarted,
			Message:   "Event " + string(rune(i)),
			Timestamp: time.Now().Add(time.Duration(i) * time.Millisecond),
		}
		err := store.StoreEvent(ctx, event)
		require.NoError(t, err)
	}

	// Should only have max events (1000)
	events, err := store.GetEvents(ctx, evalID)
	require.NoError(t, err)
	assert.Equal(t, 1000, len(events))
}

func TestInMemoryEventStore_InvalidEvent(t *testing.T) {
	store := NewInMemoryEventStore()
	ctx := context.Background()

	// Test nil event
	err := store.StoreEvent(ctx, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid event")

	// Test event with empty EvalID
	event := &JobEvent{
		EvalID:    "",
		EventType: EventJobStarted,
		Message:   "No eval ID",
		Timestamp: time.Now(),
	}
	err = store.StoreEvent(ctx, event)
	assert.Error(t, err)
}
