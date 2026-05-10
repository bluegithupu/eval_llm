package monitor

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/eval_llm/backend/internal/k8s/job"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockEvalRepo is a mock evaluation repository for testing
type mockEvalRepo struct {
	evaluations map[string]*mockEval
}

type mockEval struct {
	status   string
	progress int
}

func (m *mockEvalRepo) Create(ctx context.Context, eval interface{}) error {
	return nil
}

func (m *mockEvalRepo) GetByID(ctx context.Context, id string) (interface{}, error) {
	if e, ok := m.evaluations[id]; ok {
		return &struct {
			Status   string
			Progress int
		}{Status: e.status, Progress: e.progress}, nil
	}
	return nil, nil
}

func (m *mockEvalRepo) List(ctx context.Context, page, limit int) (interface{}, int, error) {
	return nil, 0, nil
}

func (m *mockEvalRepo) UpdateStatus(ctx context.Context, id string, status interface{}, progress int) error {
	if e, ok := m.evaluations[id]; ok {
		e.status = status.(string)
		e.progress = progress
	}
	return nil
}

func (m *mockEvalRepo) Cancel(ctx context.Context, id string) error {
	if e, ok := m.evaluations[id]; ok {
		e.status = "cancelled"
	}
	return nil
}

func (m *mockEvalRepo) Delete(ctx context.Context, id string) error {
	delete(m.evaluations, id)
	return nil
}

func (m *mockEvalRepo) Count(ctx context.Context) (int, error) {
	return len(m.evaluations), nil
}

func (m *mockEvalRepo) CountByStatus(ctx context.Context, status string) (int, error) {
	count := 0
	for _, e := range m.evaluations {
		if e.status == status {
			count++
		}
	}
	return count, nil
}

// mockCache is a mock Redis cache for testing
type mockCache struct {
	status   map[string]string
	progress map[string]int
}

func newMockCache() *mockCache {
	return &mockCache{
		status:   make(map[string]string),
		progress: make(map[string]int),
	}
}

func (c *mockCache) SetStatus(ctx context.Context, evalID, status string) error {
	c.status[evalID] = status
	return nil
}

func (c *mockCache) GetStatus(ctx context.Context, evalID string) (string, error) {
	return c.status[evalID], nil
}

func (c *mockCache) SetProgress(ctx context.Context, evalID string, progress int) error {
	c.progress[evalID] = progress
	return nil
}

func (c *mockCache) GetProgress(ctx context.Context, evalID string) (int, error) {
	return c.progress[evalID], nil
}

func (c *mockCache) DeleteStatus(ctx context.Context, evalID string) error {
	delete(c.status, evalID)
	return nil
}

func (c *mockCache) DeleteProgress(ctx context.Context, evalID string) error {
	delete(c.progress, evalID)
	return nil
}

func (c *mockCache) Ping(ctx context.Context) error {
	return nil
}

func (c *mockCache) Close() error {
	return nil
}

func (c *mockCache) IsAvailable(ctx context.Context) bool {
	return true
}

func TestJobName(t *testing.T) {
	evalID := "test-eval-123"
	name := job.JobName(evalID)
	assert.Equal(t, "eval-job-test-eval-123", name)
}

func TestDetectJobState(t *testing.T) {
	tests := []struct {
		name     string
		job      *batchv1.Job
		expected string
	}{
		{
			name:     "nil job",
			job:      nil,
			expected: "pending",
		},
		{
			name: "running job",
			job: &batchv1.Job{
				Status: batchv1.JobStatus{
					Active: 1,
				},
			},
			expected: "running",
		},
		{
			name: "completed job",
			job: &batchv1.Job{
				Status: batchv1.JobStatus{
					Succeeded: 1,
				},
			},
			expected: "completed",
		},
		{
			name: "failed job",
			job: &batchv1.Job{
				Status: batchv1.JobStatus{
					Failed: 1,
				},
			},
			expected: "failed",
		},
		{
			name: "pending job (no active, failed, succeeded)",
			job: &batchv1.Job{
				Status: batchv1.JobStatus{
					Active: 0,
				},
			},
			expected: "pending",
		},
	}

	monitor := &Monitor{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := monitor.detectJobState(tt.job)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDefaultMonitorConfig(t *testing.T) {
	cfg := DefaultMonitorConfig()
	require.NotNil(t, cfg)
	assert.Equal(t, "llm-eval", cfg.Namespace)
	assert.Equal(t, 10*time.Second, cfg.PollInterval)
	assert.Equal(t, 24*time.Hour, cfg.EventTTL)
}

func TestNewMonitor(t *testing.T) {
	client := fake.NewSimpleClientset()
	cache := newMockCache()
	eventStore := NewInMemoryEventStore()
	logger := slog.New(slog.NewTextHandler(nil, nil))

	monitor := NewMonitor(client, nil, cache, eventStore, logger, nil)
	require.NotNil(t, monitor)
	assert.Equal(t, "llm-eval", monitor.namespace)
	assert.Equal(t, 10*time.Second, monitor.pollInterval)
}

// mockLogger implements slog.Logger for testing
type mockLogger struct{}

func (l *mockLogger) Info(msg string, args ...any)                                       {}
func (l *mockLogger) Error(msg string, args ...any)                                      {}
func (l *mockLogger) Warn(msg string, args ...any)                                       {}
func (l *mockLogger) Debug(msg string, args ...any)                                      {}
func (l *mockLogger) Log(ctx context.Context, level slog.Level, msg string, args ...any) {}

// ToSlogLogger converts mockLogger to *slog.Logger for testing
func (l *mockLogger) ToSlogLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(nil, nil))
}

// Logger function to get a real slog.Logger for testing
func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestMonitorStartStop(t *testing.T) {
	client := fake.NewSimpleClientset()
	cache := newMockCache()
	eventStore := NewInMemoryEventStore()
	logger := testLogger()

	monitor := NewMonitor(client, nil, cache, eventStore, logger, nil)
	require.NotNil(t, monitor)

	ctx := context.Background()
	evalID := "test-eval-1"

	// Start monitoring
	err := monitor.StartMonitoring(ctx, evalID)
	require.NoError(t, err)

	// Verify it's being monitored
	monitor.mu.RLock()
	_, exists := monitor.monitors[evalID]
	monitor.mu.RUnlock()
	assert.True(t, exists, "eval should be monitored")

	// Stop monitoring
	monitor.StopMonitoring(evalID)

	// Verify it's stopped
	monitor.mu.RLock()
	_, exists = monitor.monitors[evalID]
	monitor.mu.RUnlock()
	assert.False(t, exists, "eval should not be monitored")
}

func TestMonitorDuplicateStart(t *testing.T) {
	client := fake.NewSimpleClientset()
	cache := newMockCache()
	eventStore := NewInMemoryEventStore()
	logger := testLogger()

	monitor := NewMonitor(client, nil, cache, eventStore, logger, nil)

	ctx := context.Background()
	evalID := "test-eval-dup"

	// Start monitoring first time
	err := monitor.StartMonitoring(ctx, evalID)
	require.NoError(t, err)

	// Try to start monitoring again - should fail
	err = monitor.StartMonitoring(ctx, evalID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already monitoring")

	// Clean up
	monitor.StopMonitoring(evalID)
}

func TestStopAll(t *testing.T) {
	client := fake.NewSimpleClientset()
	cache := newMockCache()
	eventStore := NewInMemoryEventStore()
	logger := testLogger()

	monitor := NewMonitor(client, nil, cache, eventStore, logger, nil)

	ctx := context.Background()
	evalIDs := []string{"eval-1", "eval-2", "eval-3"}

	// Start multiple monitors
	for _, id := range evalIDs {
		err := monitor.StartMonitoring(ctx, id)
		require.NoError(t, err)
	}

	// Stop all
	monitor.StopAll()

	// Verify all stopped
	monitor.mu.RLock()
	count := len(monitor.monitors)
	monitor.mu.RUnlock()
	assert.Equal(t, 0, count)
}

func TestJobIsCompleted(t *testing.T) {
	tests := []struct {
		name     string
		job      *batchv1.Job
		expected bool
	}{
		{
			name:     "nil job",
			job:      nil,
			expected: false,
		},
		{
			name: "succeeded job",
			job: &batchv1.Job{
				Status: batchv1.JobStatus{
					Succeeded: 1,
				},
			},
			expected: true,
		},
		{
			name: "job with active pods",
			job: &batchv1.Job{
				Status: batchv1.JobStatus{
					Active: 1,
				},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := job.JobIsCompleted(tt.job)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestJobIsFailed(t *testing.T) {
	tests := []struct {
		name     string
		job      *batchv1.Job
		expected bool
	}{
		{
			name:     "nil job",
			job:      nil,
			expected: false,
		},
		{
			name: "failed job",
			job: &batchv1.Job{
				Status: batchv1.JobStatus{
					Failed: 1,
				},
			},
			expected: true,
		},
		{
			name: "job with active pods",
			job: &batchv1.Job{
				Status: batchv1.JobStatus{
					Active: 1,
				},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := job.JobIsFailed(tt.job)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetJobProgress(t *testing.T) {
	tests := []struct {
		name     string
		job      *batchv1.Job
		expected int32
	}{
		{
			name:     "nil job",
			job:      nil,
			expected: 0,
		},
		{
			name: "completed job",
			job: &batchv1.Job{
				Status: batchv1.JobStatus{
					Succeeded: 1,
				},
			},
			expected: 100,
		},
		{
			name: "running job",
			job: &batchv1.Job{
				Status: batchv1.JobStatus{
					Active: 1,
				},
			},
			expected: 50,
		},
		{
			name: "pending job",
			job: &batchv1.Job{
				Status: batchv1.JobStatus{
					Active: 0,
				},
			},
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := job.GetJobProgress(tt.job)
			assert.Equal(t, tt.expected, result)
		})
	}
}
