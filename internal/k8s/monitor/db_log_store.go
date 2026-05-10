package monitor

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// LogLevel represents log severity levels
type LogLevel string

const (
	LogLevelDebug LogLevel = "debug"
	LogLevelInfo  LogLevel = "info"
	LogLevelWarn  LogLevel = "warn"
	LogLevelError LogLevel = "error"
	LogLevelFatal LogLevel = "fatal"
)

// LogEntry represents a log entry for database storage
type LogEntry struct {
	ID           string
	EvaluationID string
	Timestamp    time.Time
	Level        LogLevel
	Message      string
	Source       string
	Metadata     map[string]any
}

// DBLogStore implements EventStore with database-backed storage
type DBLogStore struct {
	db *pgxpool.Pool
}

// NewDBLogStore creates a new database-backed log store
func NewDBLogStore(db *pgxpool.Pool) *DBLogStore {
	return &DBLogStore{db: db}
}

// StoreEvent stores a job event in the database logs table
func (s *DBLogStore) StoreEvent(ctx context.Context, event *JobEvent) error {
	if event == nil {
		return fmt.Errorf("event is nil")
	}

	// Determine log level based on event type
	var level LogLevel
	var source string
	switch event.EventType {
	case EventJobCreated:
		level = LogLevelDebug
		source = "k8s-job-creator"
	case EventJobStarted:
		level = LogLevelInfo
		source = "k8s-job-monitor"
	case EventJobCompleted:
		level = LogLevelInfo
		source = "k8s-job-monitor"
	case EventJobFailed:
		level = LogLevelError
		source = "k8s-job-monitor"
	case EventJobDeleted:
		level = LogLevelWarn
		source = "k8s-job-monitor"
	case EventPodOOMKilled:
		level = LogLevelError
		source = "k8s-job-monitor"
	}

	// Build metadata
	metadata := make(map[string]any)
	if event.JobStatus != nil {
		metadata["active"] = event.JobStatus.Active
		metadata["succeeded"] = event.JobStatus.Succeeded
		metadata["failed"] = event.JobStatus.Failed
	}
	if event.OOMDetected {
		metadata["oom_detected"] = true
		metadata["oom_pod_name"] = event.OOMPodName
		metadata["oom_exit_code"] = event.OOMExitCode
	}

	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	query := `
		INSERT INTO logs (id, evaluation_id, timestamp, level, message, source, metadata)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`

	_, err = s.db.Exec(ctx, query,
		uuid.New().String(),
		event.EvalID,
		event.Timestamp,
		level,
		event.Message,
		source,
		metadataJSON,
	)

	if err != nil {
		return fmt.Errorf("failed to insert log: %w", err)
	}

	return nil
}

// GetEvents retrieves all job events for an evaluation from the logs table
func (s *DBLogStore) GetEvents(ctx context.Context, evalID string) ([]*JobEvent, error) {
	query := `
		SELECT timestamp, level, message, source, metadata
		FROM logs
		WHERE evaluation_id = $1 AND source LIKE 'k8s-%'
		ORDER BY timestamp ASC
	`

	rows, err := s.db.Query(ctx, query, uuid.MustParse(evalID))
	if err != nil {
		return nil, fmt.Errorf("failed to query events: %w", err)
	}
	defer rows.Close()

	var events []*JobEvent
	for rows.Next() {
		var timestamp time.Time
		var level string
		var message string
		var source string
		var metadataJSON []byte

		err := rows.Scan(&timestamp, &level, &message, &source, &metadataJSON)
		if err != nil {
			return nil, fmt.Errorf("failed to scan event: %w", err)
		}

		// Convert log entry to JobEvent
		eventType := s.logLevelToEventType(level)
		metadata := make(map[string]any)
		if len(metadataJSON) > 0 {
			json.Unmarshal(metadataJSON, &metadata)
		}

		event := &JobEvent{
			EvalID:    evalID,
			EventType: eventType,
			Message:   message,
			Timestamp: timestamp,
		}

		// Extract OOM info if present
		if oomDetected, ok := metadata["oom_detected"].(bool); ok && oomDetected {
			event.OOMDetected = true
			if podName, ok := metadata["oom_pod_name"].(string); ok {
				event.OOMPodName = podName
			}
			if exitCode, ok := metadata["oom_exit_code"].(float64); ok {
				event.OOMExitCode = int32(exitCode)
			}
		}

		events = append(events, event)
	}

	return events, nil
}

// GetEventsByType retrieves events of a specific type
func (s *DBLogStore) GetEventsByType(ctx context.Context, evalID string, eventType JobEventType) ([]*JobEvent, error) {
	// Get all events and filter
	events, err := s.GetEvents(ctx, evalID)
	if err != nil {
		return nil, err
	}

	var filtered []*JobEvent
	for _, e := range events {
		if e.EventType == eventType {
			filtered = append(filtered, e)
		}
	}

	return filtered, nil
}

// ClearOldEvents removes old log entries
func (s *DBLogStore) ClearOldEvents(ctx context.Context, olderThan time.Duration) error {
	cutoff := time.Now().Add(-olderThan)

	query := `DELETE FROM logs WHERE timestamp < $1 AND source LIKE 'k8s-%'`
	_, err := s.db.Exec(ctx, query, cutoff)
	if err != nil {
		return fmt.Errorf("failed to clear old events: %w", err)
	}

	return nil
}

// logLevelToEventType converts log level to job event type
func (s *DBLogStore) logLevelToEventType(level string) JobEventType {
	switch level {
	case "debug":
		return EventJobCreated
	case "info":
		return EventJobCompleted
	case "warn":
		return EventJobDeleted
	case "error":
		return EventJobFailed
	default:
		return EventJobStarted
	}
}

// GetRecentEvents retrieves recent events across all evaluations
func (s *DBLogStore) GetRecentEvents(ctx context.Context, limit int) ([]*JobEvent, error) {
	query := `
		SELECT evaluation_id, timestamp, level, message, metadata
		FROM logs
		WHERE source LIKE 'k8s-%'
		ORDER BY timestamp DESC
		LIMIT $1
	`

	rows, err := s.db.Query(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query recent events: %w", err)
	}
	defer rows.Close()

	var events []*JobEvent
	for rows.Next() {
		var evalID string
		var timestamp time.Time
		var level string
		var message string
		var metadataJSON []byte

		err := rows.Scan(&evalID, &timestamp, &level, &message, &metadataJSON)
		if err != nil {
			return nil, fmt.Errorf("failed to scan event: %w", err)
		}

		event := &JobEvent{
			EvalID:    evalID,
			EventType: s.logLevelToEventType(level),
			Message:   message,
			Timestamp: timestamp,
		}
		events = append(events, event)
	}

	return events, nil
}

// StoreLogEntry stores a generic log entry
func (s *DBLogStore) StoreLogEntry(ctx context.Context, entry *LogEntry) error {
	if entry == nil {
		return fmt.Errorf("log entry is nil")
	}

	metadataJSON, err := json.Marshal(entry.Metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	query := `
		INSERT INTO logs (id, evaluation_id, timestamp, level, message, source, metadata)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`

	_, err = s.db.Exec(ctx, query,
		uuid.New().String(),
		entry.EvaluationID,
		entry.Timestamp,
		entry.Level,
		entry.Message,
		entry.Source,
		metadataJSON,
	)

	if err != nil {
		return fmt.Errorf("failed to insert log: %w", err)
	}

	return nil
}
