package k8s

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/eval_llm/backend/internal/model"
	"github.com/eval_llm/backend/internal/repository"
)

// OrphanCleanupConfig holds configuration for orphaned Job cleanup
type OrphanCleanupConfig struct {
	// Namespace is the Kubernetes namespace to scan
	Namespace string
	// ScanInterval is how often to scan for orphaned Jobs
	ScanInterval time.Duration
	// MaxCleanupAge is the maximum age of an orphaned Job before cleanup
	// Jobs younger than this won't be cleaned up (to avoid race conditions)
	MaxCleanupAge time.Duration
}

// DefaultOrphanCleanupConfig returns default orphan cleanup configuration
func DefaultOrphanCleanupConfig() *OrphanCleanupConfig {
	return &OrphanCleanupConfig{
		Namespace:     DefaultNamespace,
		ScanInterval:  5 * time.Minute,
		MaxCleanupAge: 1 * time.Minute, // Don't clean Jobs younger than 1 minute
	}
}

// OrphanCleaner scans for and cleans up orphaned Kubernetes Jobs
// An orphaned Job is a Job whose evaluation record is:
// 1. Deleted from the database (no record exists)
// 2. In cancelled/failed state with an active Job
type OrphanCleaner struct {
	client    kubernetes.Interface
	namespace string
	evalRepo  repository.EvaluationRepository
	cfg       *OrphanCleanupConfig
	logger    *slog.Logger

	// Channel to signal stop
	stopChan chan struct{}
	// Channel for cleanup events
	cleanupChan chan *CleanupEvent
}

// CleanupEvent represents a cleanup event for an orphaned Job
type CleanupEvent struct {
	EvalID   string
	Event    string // "job_deleted", "job_not_found", "skipped", "error"
	Message  string
	Timestamp time.Time
}

// NewOrphanCleaner creates a new orphaned Job cleaner
func NewOrphanCleaner(
	client kubernetes.Interface,
	evalRepo repository.EvaluationRepository,
	logger *slog.Logger,
	cfg *OrphanCleanupConfig,
) *OrphanCleaner {
	if cfg == nil {
		cfg = DefaultOrphanCleanupConfig()
	}

	return &OrphanCleaner{
		client:     client,
		namespace:  cfg.Namespace,
		evalRepo:   evalRepo,
		cfg:        cfg,
		logger:     logger,
		stopChan:   make(chan struct{}),
		cleanupChan: make(chan *CleanupEvent, 100),
	}
}

// Start starts the background orphaned Job cleanup scanner
func (c *OrphanCleaner) Start(ctx context.Context) {
	c.logger.Info("starting orphan cleaner",
		"namespace", c.namespace,
		"scan_interval", c.cfg.ScanInterval,
	)

	ticker := time.NewTicker(c.cfg.ScanInterval)
	defer ticker.Stop()

	// Run initial scan immediately
	c.scanAndCleanup(ctx)

	for {
		select {
		case <-ctx.Done():
			c.logger.Info("orphan cleaner context cancelled, stopping")
			return
		case <-c.stopChan:
			c.logger.Info("orphan cleaner stop signal received, stopping")
			return
		case <-ticker.C:
			c.scanAndCleanup(ctx)
		}
	}
}

// Stop stops the orphan cleaner scanner
func (c *OrphanCleaner) Stop() {
	close(c.stopChan)
}

// GetCleanupEvents returns the cleanup event channel
func (c *OrphanCleaner) GetCleanupEvents() <-chan *CleanupEvent {
	return c.cleanupChan
}

// scanAndCleanup performs a single scan and cleanup pass
func (c *OrphanCleaner) scanAndCleanup(ctx context.Context) {
	c.logger.Debug("scanning for orphaned jobs")

	// Get all Jobs with app=llm-eval label in the namespace
	jobs, err := c.listEvalJobs(ctx)
	if err != nil {
		c.logger.Error("failed to list jobs for orphan detection", "error", err)
		return
	}

	c.logger.Debug("found jobs to check", "count", len(jobs))

	for _, job := range jobs {
		c.checkAndCleanupJob(ctx, &job)
	}
}

// listEvalJobs returns all evaluation Jobs in the namespace
func (c *OrphanCleaner) listEvalJobs(ctx context.Context) ([]batchv1.Job, error) {
	selector := AppSelector()
	list, err := c.client.BatchV1().Jobs(c.namespace).List(ctx, metav1.ListOptions{
		LabelSelector: selector,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list jobs: %w", err)
	}
	return list.Items, nil
}

// checkAndCleanupJob checks if a Job is orphaned and cleans it up if necessary
func (c *OrphanCleaner) checkAndCleanupJob(ctx context.Context, job *batchv1.Job) {
	evalID := job.Labels[EvalIDLabelKey]
	if evalID == "" {
		c.logger.Warn("job missing eval-id label, skipping", "job", job.Name)
		return
	}

	// Check if the Job is too young to clean up (avoid race conditions)
	if c.isJobTooYoung(job) {
		c.logger.Debug("job too young to cleanup", "eval_id", evalID, "job", job.Name)
		return
	}

	// Get the corresponding evaluation from DB
	eval, err := c.evalRepo.GetByID(ctx, evalID)
	if err != nil {
		if err == repository.ErrNotFound {
			// Evaluation doesn't exist - Job is orphaned (case 1)
			c.cleanupOrphanedJob(ctx, evalID, job, "evaluation record not found")
			return
		}
		c.logger.Error("failed to get evaluation for orphan check",
			"eval_id", evalID,
			"error", err,
		)
		return
	}

	// Check if the evaluation is in a terminal state with an active Job
	if c.shouldCleanupJob(eval, job) {
		c.cleanupOrphanedJob(ctx, evalID, job, fmt.Sprintf("evaluation in %s state with active job", eval.Status))
	}
}

// isJobTooYoung checks if the Job was created too recently to be cleaned up
// This prevents race conditions where a Job is created but the evaluation
// record hasn't been fully initialized yet
func (c *OrphanCleaner) isJobTooYoung(job *batchv1.Job) bool {
	if job.CreationTimestamp.IsZero() {
		return false
	}

	age := time.Since(job.CreationTimestamp.Time)
	return age < c.cfg.MaxCleanupAge
}

// shouldCleanupJob determines if a Job should be cleaned up based on evaluation state
func (c *OrphanCleaner) shouldCleanupJob(eval *model.Evaluation, job *batchv1.Job) bool {
	// Don't cleanup if evaluation is pending or running
	if eval.Status == model.StatusPending || eval.Status == model.StatusRunning {
		return false
	}

	// Cleanup if evaluation is completed/failed/cancelled but Job is still active
	// This handles cases where the Job wasn't properly cleaned up
	if job.Status.Active > 0 {
		return true
	}

	// Also cleanup if Job failed but evaluation is already in terminal state
	// (meaning we already processed the failure but Job wasn't deleted)
	if job.Status.Failed > 0 {
		return true
	}

	return false
}

// cleanupOrphanedJob deletes an orphaned Job and emits a cleanup event
func (c *OrphanCleaner) cleanupOrphanedJob(ctx context.Context, evalID string, job *batchv1.Job, reason string) {
	c.logger.Info("cleaning up orphaned job",
		"eval_id", evalID,
		"job_name", job.Name,
		"reason", reason,
	)

	// Delete the Job
	if err := c.client.BatchV1().Jobs(c.namespace).Delete(ctx, job.Name, metav1.DeleteOptions{}); err != nil {
		c.logger.Error("failed to delete orphaned job",
			"eval_id", evalID,
			"job_name", job.Name,
			"error", err,
		)
		c.emitCleanupEvent(evalID, "error", fmt.Sprintf("failed to delete job: %v", err))
		return
	}

	// Also try to cleanup associated ConfigMap and Secret
	c.cleanupRelatedResources(ctx, evalID)

	c.logger.Info("successfully cleaned up orphaned job",
		"eval_id", evalID,
		"job_name", job.Name,
	)
	c.emitCleanupEvent(evalID, "job_deleted", reason)
}

// cleanupRelatedResources cleans up ConfigMap and Secret for an evaluation
func (c *OrphanCleaner) cleanupRelatedResources(ctx context.Context, evalID string) {
	// Import these from the respective packages - using hardcoded names since we can't import them
	configMapName := fmt.Sprintf("eval-config-%s", evalID)
	secretName := fmt.Sprintf("eval-secret-%s", evalID)

	// Delete ConfigMap
	if err := c.client.CoreV1().ConfigMaps(c.namespace).Delete(ctx, configMapName, metav1.DeleteOptions{}); err != nil {
		c.logger.Debug("failed to delete orphaned configmap (may not exist)",
			"configmap", configMapName,
			"error", err,
		)
	} else {
		c.logger.Debug("deleted orphaned configmap", "configmap", configMapName)
	}

	// Delete Secret
	if err := c.client.CoreV1().Secrets(c.namespace).Delete(ctx, secretName, metav1.DeleteOptions{}); err != nil {
		c.logger.Debug("failed to delete orphaned secret (may not exist)",
			"secret", secretName,
			"error", err,
		)
	} else {
		c.logger.Debug("deleted orphaned secret", "secret", secretName)
	}
}

// emitCleanupEvent sends a cleanup event to the channel
func (c *OrphanCleaner) emitCleanupEvent(evalID, event, message string) {
	cleanupEvent := &CleanupEvent{
		EvalID:    evalID,
		Event:     event,
		Message:   message,
		Timestamp: time.Now(),
	}

	select {
	case c.cleanupChan <- cleanupEvent:
	default:
		c.logger.Warn("cleanup event channel full, dropping event", "eval_id", evalID)
	}
}

// CleanupNow performs an immediate cleanup scan and returns results
// This is useful for testing or manual cleanup triggers
func (c *OrphanCleaner) CleanupNow(ctx context.Context) ([]*CleanupEvent, error) {
	jobs, err := c.listEvalJobs(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list jobs: %w", err)
	}

	var events []*CleanupEvent
	for _, job := range jobs {
		evalID := job.Labels[EvalIDLabelKey]
		if evalID == "" {
			continue
		}

		eval, err := c.evalRepo.GetByID(ctx, evalID)
		if err != nil {
			if err == repository.ErrNotFound {
				// Orphaned - evaluation doesn't exist
				c.checkAndCleanupJob(ctx, &job)
				events = append(events, &CleanupEvent{
					EvalID:    evalID,
					Event:     "job_deleted",
					Message:   "evaluation record not found",
					Timestamp: time.Now(),
				})
			}
			continue
		}

		if c.shouldCleanupJob(eval, &job) {
			c.checkAndCleanupJob(ctx, &job)
			events = append(events, &CleanupEvent{
				EvalID:    evalID,
				Event:     "job_deleted",
				Message:   fmt.Sprintf("evaluation in %s state with active job", eval.Status),
				Timestamp: time.Now(),
			})
		}
	}

	return events, nil
}

// GetOrphanedJobCount returns the count of orphaned Jobs without scanning
func (c *OrphanCleaner) GetOrphanedJobCount(ctx context.Context) (int, error) {
	jobs, err := c.listEvalJobs(ctx)
	if err != nil {
		return 0, err
	}

	count := 0
	for _, job := range jobs {
		evalID := job.Labels[EvalIDLabelKey]
		if evalID == "" {
			continue
		}

		if c.isJobTooYoung(&job) {
			continue
		}

		_, err := c.evalRepo.GetByID(ctx, evalID)
		if err == repository.ErrNotFound {
			count++
			continue
		}
		if err != nil {
			c.logger.Error("error checking evaluation", "eval_id", evalID, "error", err)
			continue
		}
	}

	return count, nil
}
