package k8s

import (
	"context"
	"log/slog"
	"testing"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/eval_llm/backend/internal/model"
	"github.com/eval_llm/backend/internal/repository"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockEvalRepo is a mock evaluation repository for testing
type mockEvalRepo struct {
	evaluations map[string]*model.Evaluation
}

func newMockEvalRepo() *mockEvalRepo {
	return &mockEvalRepo{
		evaluations: make(map[string]*model.Evaluation),
	}
}

func (m *mockEvalRepo) Create(ctx context.Context, eval *model.Evaluation) error {
	m.evaluations[eval.ID] = eval
	return nil
}

func (m *mockEvalRepo) GetByID(ctx context.Context, id string) (*model.Evaluation, error) {
	eval, ok := m.evaluations[id]
	if !ok {
		return nil, repository.ErrNotFound
	}
	return eval, nil
}

func (m *mockEvalRepo) List(ctx context.Context, page, limit int) ([]*model.Evaluation, int, error) {
	var evals []*model.Evaluation
	for _, e := range m.evaluations {
		evals = append(evals, e)
	}
	return evals, len(evals), nil
}

func (m *mockEvalRepo) UpdateStatus(ctx context.Context, id string, status model.EvaluationStatus, progress int) error {
	if eval, ok := m.evaluations[id]; ok {
		eval.Status = status
		eval.Progress = progress
	}
	return nil
}

func (m *mockEvalRepo) Cancel(ctx context.Context, id string) error {
	if eval, ok := m.evaluations[id]; ok {
		eval.Status = model.StatusCancelled
		now := time.Now()
		eval.CompletedAt = &now
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

func (m *mockEvalRepo) CountByStatus(ctx context.Context, status model.EvaluationStatus) (int, error) {
	count := 0
	for _, e := range m.evaluations {
		if e.Status == status {
			count++
		}
	}
	return count, nil
}

// newMockLogger creates a logger for testing
func newMockLogger() *slog.Logger {
	return slog.Default()
}

func TestNewOrphanCleaner(t *testing.T) {
	client := fake.NewSimpleClientset()
	logger := newMockLogger()
	evalRepo := newMockEvalRepo()

	cfg := &OrphanCleanupConfig{
		Namespace:     "test-ns",
		ScanInterval:  10 * time.Minute,
		MaxCleanupAge: 2 * time.Minute,
	}

	cleaner := NewOrphanCleaner(client, evalRepo, logger, cfg)

	assert.Equal(t, "test-ns", cleaner.namespace)
	assert.Equal(t, 10*time.Minute, cleaner.cfg.ScanInterval)
	assert.Equal(t, 2*time.Minute, cleaner.cfg.MaxCleanupAge)
}

func TestOrphanCleanupConfig_Defaults(t *testing.T) {
	cfg := DefaultOrphanCleanupConfig()

	assert.Equal(t, DefaultNamespace, cfg.Namespace)
	assert.Equal(t, 5*time.Minute, cfg.ScanInterval)
	assert.Equal(t, 1*time.Minute, cfg.MaxCleanupAge)
}

func TestIsJobTooYoung(t *testing.T) {
	client := fake.NewSimpleClientset()
	evalRepo := newMockEvalRepo()
	logger := newMockLogger()

	tests := []struct {
		name       string
		jobAge     time.Duration
		maxAge     time.Duration
		shouldSkip bool
	}{
		{
			name:       "job is old enough",
			jobAge:     5 * time.Minute,
			maxAge:     1 * time.Minute,
			shouldSkip: false,
		},
		{
			name:       "job is too young",
			jobAge:     30 * time.Second,
			maxAge:     1 * time.Minute,
			shouldSkip: true,
		},
		{
			name:       "job age equals max age",
			jobAge:     1 * time.Minute,
			maxAge:     1 * time.Minute,
			shouldSkip: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &OrphanCleanupConfig{
				MaxCleanupAge: tt.maxAge,
			}
			cleaner := NewOrphanCleaner(client, evalRepo, logger, cfg)

			job := &batchv1.Job{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "test-job",
					Namespace:         "test-ns",
					CreationTimestamp: metav1.NewTime(time.Now().Add(-tt.jobAge)),
				},
			}

			result := cleaner.isJobTooYoung(job)
			assert.Equal(t, tt.shouldSkip, result)
		})
	}
}

func TestShouldCleanupJob(t *testing.T) {
	client := fake.NewSimpleClientset()
	evalRepo := newMockEvalRepo()
	logger := newMockLogger()

	tests := []struct {
		name          string
		evalStatus    model.EvaluationStatus
		jobActive     int32
		jobFailed     int32
		shouldCleanup bool
	}{
		{
			name:          "pending eval, running job - don't cleanup",
			evalStatus:    model.StatusPending,
			jobActive:     1,
			shouldCleanup: false,
		},
		{
			name:          "running eval, running job - don't cleanup",
			evalStatus:    model.StatusRunning,
			jobActive:     1,
			shouldCleanup: false,
		},
		{
			name:          "completed eval, running job - cleanup",
			evalStatus:    model.StatusCompleted,
			jobActive:     1,
			shouldCleanup: true,
		},
		{
			name:          "failed eval, running job - cleanup",
			evalStatus:    model.StatusFailed,
			jobActive:     1,
			shouldCleanup: true,
		},
		{
			name:          "cancelled eval, running job - cleanup",
			evalStatus:    model.StatusCancelled,
			jobActive:     1,
			shouldCleanup: true,
		},
		{
			name:          "completed eval, no active job - don't cleanup",
			evalStatus:    model.StatusCompleted,
			jobActive:     0,
			shouldCleanup: false,
		},
		{
			name:          "failed eval, failed job - cleanup",
			evalStatus:    model.StatusFailed,
			jobFailed:     1,
			shouldCleanup: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := DefaultOrphanCleanupConfig()
			cleaner := NewOrphanCleaner(client, evalRepo, logger, cfg)

			eval := &model.Evaluation{
				ID:     uuid.New().String(),
				Status: tt.evalStatus,
			}

			job := &batchv1.Job{
				Status: batchv1.JobStatus{
					Active: tt.jobActive,
					Failed: tt.jobFailed,
				},
			}

			result := cleaner.shouldCleanupJob(eval, job)
			assert.Equal(t, tt.shouldCleanup, result)
		})
	}
}

func TestCleanupOrphanedJob(t *testing.T) {
	client := fake.NewSimpleClientset()
	evalRepo := newMockEvalRepo()
	logger := newMockLogger()

	cfg := DefaultOrphanCleanupConfig()
	cleaner := NewOrphanCleaner(client, evalRepo, logger, cfg)

	ctx := context.Background()
	evalID := uuid.New().String()
	jobName := "eval-job-" + evalID

	// Create a fake job
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName,
			Namespace: cfg.Namespace,
			Labels: map[string]string{
				AppLabelKey:    AppLabelValue,
				EvalIDLabelKey: evalID,
			},
		},
	}

	// Create the job in the fake client
	_, err := client.BatchV1().Jobs(cfg.Namespace).Create(ctx, job, metav1.CreateOptions{})
	require.NoError(t, err)

	// Verify job exists
	_, err = client.BatchV1().Jobs(cfg.Namespace).Get(ctx, jobName, metav1.GetOptions{})
	require.NoError(t, err)

	// Cleanup
	cleaner.cleanupOrphanedJob(ctx, evalID, job, "test cleanup")

	// Verify job is deleted
	_, err = client.BatchV1().Jobs(cfg.Namespace).Get(ctx, jobName, metav1.GetOptions{})
	assert.Error(t, err)
}

func TestGetOrphanedJobCount(t *testing.T) {
	client := fake.NewSimpleClientset()
	evalRepo := newMockEvalRepo()
	logger := newMockLogger()

	cfg := &OrphanCleanupConfig{
		MaxCleanupAge: 0, // No minimum age for this test
	}
	cleaner := NewOrphanCleaner(client, evalRepo, logger, cfg)

	ctx := context.Background()

	// Create a job without a corresponding evaluation
	orphanEvalID := uuid.New().String()
	orphanJob := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "eval-job-" + orphanEvalID,
			Namespace:         cfg.Namespace,
			Labels:            Labels(orphanEvalID, "gpt-4", "mmlu"),
			CreationTimestamp: metav1.NewTime(time.Now().Add(-5 * time.Minute)), // Old enough
		},
	}
	_, err := client.BatchV1().Jobs(cfg.Namespace).Create(ctx, orphanJob, metav1.CreateOptions{})
	require.NoError(t, err)

	// Create a job with a corresponding evaluation (not orphaned)
	validEvalID := uuid.New().String()
	validJob := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "eval-job-" + validEvalID,
			Namespace:         cfg.Namespace,
			Labels:            Labels(validEvalID, "gpt-4", "mmlu"),
			CreationTimestamp: metav1.NewTime(time.Now().Add(-5 * time.Minute)),
		},
	}
	_, err = client.BatchV1().Jobs(cfg.Namespace).Create(ctx, validJob, metav1.CreateOptions{})
	require.NoError(t, err)

	// Add the evaluation record for the valid job
	evalRepo.evaluations[validEvalID] = &model.Evaluation{
		ID:     validEvalID,
		Status: model.StatusRunning,
	}

	// Get orphaned count
	count, err := cleaner.GetOrphanedJobCount(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, count)
}

func TestCleanupNow(t *testing.T) {
	client := fake.NewSimpleClientset()
	evalRepo := newMockEvalRepo()
	logger := newMockLogger()

	cfg := &OrphanCleanupConfig{
		MaxCleanupAge: 0,
	}
	cleaner := NewOrphanCleaner(client, evalRepo, logger, cfg)

	ctx := context.Background()

	// Create an orphaned job (no corresponding evaluation)
	orphanEvalID := uuid.New().String()
	orphanJob := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "eval-job-" + orphanEvalID,
			Namespace:         cfg.Namespace,
			Labels:            Labels(orphanEvalID, "gpt-4", "mmlu"),
			CreationTimestamp: metav1.NewTime(time.Now().Add(-5 * time.Minute)),
		},
	}
	_, err := client.BatchV1().Jobs(cfg.Namespace).Create(ctx, orphanJob, metav1.CreateOptions{})
	require.NoError(t, err)

	// Create a job for a cancelled evaluation
	cancelledEvalID := uuid.New().String()
	cancelledJob := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "eval-job-" + cancelledEvalID,
			Namespace:         cfg.Namespace,
			Labels:            Labels(cancelledEvalID, "gpt-4", "mmlu"),
			CreationTimestamp: metav1.NewTime(time.Now().Add(-5 * time.Minute)),
		},
		Status: batchv1.JobStatus{
			Active: 1, // Still has active pods
		},
	}
	_, err = client.BatchV1().Jobs(cfg.Namespace).Create(ctx, cancelledJob, metav1.CreateOptions{})
	require.NoError(t, err)

	// Add cancelled evaluation record
	evalRepo.evaluations[cancelledEvalID] = &model.Evaluation{
		ID:     cancelledEvalID,
		Status: model.StatusCancelled,
	}

	// Run cleanup now
	events, err := cleaner.CleanupNow(ctx)
	require.NoError(t, err)

	// Should have 2 cleanup events (one for orphan, one for cancelled)
	assert.Len(t, events, 2)

	// Verify both jobs are deleted
	_, err = client.BatchV1().Jobs(cfg.Namespace).Get(ctx, "eval-job-"+orphanEvalID, metav1.GetOptions{})
	assert.Error(t, err)

	_, err = client.BatchV1().Jobs(cfg.Namespace).Get(ctx, "eval-job-"+cancelledEvalID, metav1.GetOptions{})
	assert.Error(t, err)
}

func TestCleanupEventsChannel(t *testing.T) {
	client := fake.NewSimpleClientset()
	evalRepo := newMockEvalRepo()
	logger := newMockLogger()

	cfg := &OrphanCleanupConfig{
		MaxCleanupAge: 0,
	}
	cleaner := NewOrphanCleaner(client, evalRepo, logger, cfg)

	// Get cleanup events channel
	events := cleaner.GetCleanupEvents()
	assert.NotNil(t, events)
}

func TestOrphanCleaner_StartStop(t *testing.T) {
	client := fake.NewSimpleClientset()
	evalRepo := newMockEvalRepo()
	logger := newMockLogger()

	cfg := &OrphanCleanupConfig{
		ScanInterval: 100 * time.Millisecond,
	}
	cleaner := NewOrphanCleaner(client, evalRepo, logger, cfg)

	ctx, cancel := context.WithCancel(context.Background())

	// Start the cleaner
	go cleaner.Start(ctx)

	// Let it run for a short time
	time.Sleep(250 * time.Millisecond)

	// Stop it
	cancel()
	time.Sleep(50 * time.Millisecond)

	// Verify Stop also works
	cleaner.Stop()
}
