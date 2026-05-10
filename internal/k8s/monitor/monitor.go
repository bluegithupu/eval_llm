package monitor

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/eval_llm/backend/internal/cache"
	"github.com/eval_llm/backend/internal/k8s"
	"github.com/eval_llm/backend/internal/k8s/job"
	"github.com/eval_llm/backend/internal/model"
	"github.com/eval_llm/backend/internal/repository"
)

// JobEventType represents the type of job event
type JobEventType string

const (
	EventJobCreated   JobEventType = "Created"
	EventJobStarted   JobEventType = "Started"
	EventJobCompleted JobEventType = "Completed"
	EventJobFailed    JobEventType = "Failed"
	EventJobDeleted   JobEventType = "Deleted"
	EventPodOOMKilled JobEventType = "OOMKilled"
)

// JobEvent represents a job status change event
type JobEvent struct {
	EvalID      string
	EventType   JobEventType
	Message     string
	Timestamp   time.Time
	JobStatus   *batchv1.JobStatus
	OOMDetected bool
	OOMPodName  string
	OOMExitCode int32
	Stderr      string // Captured stderr from CLI execution for error debugging
}

// StatusUpdater is an interface for updating evaluation status
type StatusUpdater interface {
	UpdateStatus(ctx context.Context, evalID string, status model.EvaluationStatus, progress int) error
}

// MonitorConfig holds configuration for the Job status monitor
type MonitorConfig struct {
	Namespace    string
	PollInterval time.Duration
	EventTTL     time.Duration
}

// DefaultMonitorConfig returns default monitor configuration
func DefaultMonitorConfig() *MonitorConfig {
	return &MonitorConfig{
		Namespace:    k8s.DefaultNamespace,
		PollInterval: 10 * time.Second,
		EventTTL:     24 * time.Hour,
	}
}

// Monitor polls Kubernetes Jobs for status changes and updates the database
type Monitor struct {
	client       kubernetes.Interface
	namespace    string
	pollInterval time.Duration
	evalRepo     repository.EvaluationRepository
	cache        cache.StatusCache
	eventStore   EventStore
	logger       *slog.Logger

	// Running monitors
	monitors map[string]context.CancelFunc
	mu       sync.RWMutex

	// Channel for job events
	eventChan chan *JobEvent
}

// NewMonitor creates a new Job status monitor
func NewMonitor(
	client kubernetes.Interface,
	evalRepo repository.EvaluationRepository,
	cacheClient cache.StatusCache,
	eventStore EventStore,
	logger *slog.Logger,
	cfg *MonitorConfig,
) *Monitor {
	if cfg == nil {
		cfg = DefaultMonitorConfig()
	}

	return &Monitor{
		client:       client,
		namespace:    cfg.Namespace,
		pollInterval: cfg.PollInterval,
		evalRepo:     evalRepo,
		cache:        cacheClient,
		eventStore:   eventStore,
		logger:       logger,
		monitors:     make(map[string]context.CancelFunc),
		eventChan:    make(chan *JobEvent, 100),
	}
}

// StartMonitoring starts monitoring a specific evaluation's Job
func (m *Monitor) StartMonitoring(ctx context.Context, evalID string) error {
	// Check if already monitoring
	m.mu.RLock()
	if _, exists := m.monitors[evalID]; exists {
		m.mu.RUnlock()
		return fmt.Errorf("already monitoring eval %s", evalID)
	}
	m.mu.RUnlock()

	// Create cancellation context
	ctx, cancel := context.WithCancel(ctx)

	// Store the cancel function
	m.mu.Lock()
	m.monitors[evalID] = cancel
	m.mu.Unlock()

	// Start monitoring goroutine
	go m.monitorLoop(ctx, evalID)

	m.logger.Info("started monitoring", "eval_id", evalID)
	return nil
}

// StopMonitoring stops monitoring a specific evaluation
func (m *Monitor) StopMonitoring(evalID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if cancel, exists := m.monitors[evalID]; exists {
		cancel()
		delete(m.monitors, evalID)
		m.logger.Info("stopped monitoring", "eval_id", evalID)
	}
}

// StopAll stops all monitoring goroutines
func (m *Monitor) StopAll() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for evalID, cancel := range m.monitors {
		cancel()
		delete(m.monitors, evalID)
	}

	close(m.eventChan)
	m.logger.Info("stopped all monitors")
}

// monitorLoop is the main monitoring loop for a single evaluation
func (m *Monitor) monitorLoop(ctx context.Context, evalID string) {
	ticker := time.NewTicker(m.pollInterval)
	defer ticker.Stop()

	// Track previous state to detect changes
	var lastState string

	for {
		select {
		case <-ctx.Done():
			m.logger.Info("monitor context cancelled", "eval_id", evalID)
			return
		case <-ticker.C:
			// Get the Job
			k8sJob, err := job.GetJob(ctx, m.client, m.namespace, evalID)
			if err != nil {
				// Job might be deleted - check events and update status
				m.handleJobNotFound(ctx, evalID)
				continue
			}

			// Detect and handle status changes
			newState := m.detectJobState(k8sJob)
			if newState != lastState {
				m.handleStateChange(ctx, evalID, k8sJob, newState)
				lastState = newState
			}

			// Always update progress in DB and Redis
			progress := int(job.GetJobProgress(k8sJob))
			m.updateProgress(ctx, evalID, progress)

			// Check for OOM killed pods
			m.checkForOOMKilled(ctx, evalID, k8sJob)
		}
	}
}

// detectJobState determines the current state of a Job
func (m *Monitor) detectJobState(k8sJob *batchv1.Job) string {
	if k8sJob == nil {
		return "pending"
	}
	if job.JobIsCompleted(k8sJob) {
		return "completed"
	}
	if job.JobIsFailed(k8sJob) {
		return "failed"
	}
	if job.JobIsRunning(k8sJob) {
		return "running"
	}
	return "pending"
}

// handleStateChange processes a Job state transition
func (m *Monitor) handleStateChange(ctx context.Context, evalID string, k8sJob *batchv1.Job, newState string) {
	var eventType JobEventType
	var status model.EvaluationStatus
	var progress int
	var message string

	switch newState {
	case "running":
		eventType = EventJobStarted
		status = model.StatusRunning
		progress = 50 // Mid-progress for running
		message = "Job started running"

	case "completed":
		eventType = EventJobCompleted
		status = model.StatusCompleted
		progress = 100
		message = "Job completed successfully"

	case "failed":
		eventType = EventJobFailed
		status = model.StatusFailed
		progress = 0
		// Get failure reason from Job status
		if k8sJob.Status.Conditions != nil {
			for _, cond := range k8sJob.Status.Conditions {
				if cond.Type == batchv1.JobFailed && cond.Reason != "" {
					message = fmt.Sprintf("Job failed: %s - %s", cond.Reason, cond.Message)
					break
				}
			}
		}
		if message == "" {
			message = "Job failed"
		}

		// Store error with Job failure reason for debugging
		if m.eventStore != nil {
			_ = m.eventStore.StoreError(ctx, evalID, ErrorTypeK8sJobFailed, message, "")
		}

	default:
		// Pending or unknown state - no event needed
		return
	}

	// Record event
	event := &JobEvent{
		EvalID:    evalID,
		EventType: eventType,
		Message:   message,
		Timestamp: time.Now(),
		JobStatus: &k8sJob.Status,
	}
	m.recordEvent(ctx, event)

	// Update DB status
	if err := m.evalRepo.UpdateStatus(ctx, evalID, status, progress); err != nil {
		m.logger.Error("failed to update status", "eval_id", evalID, "error", err)
	}

	// Update Redis status
	if err := m.cache.SetStatus(ctx, evalID, string(status)); err != nil {
		m.logger.Error("failed to set Redis status", "eval_id", evalID, "error", err)
	}
	if err := m.cache.SetProgress(ctx, evalID, progress); err != nil {
		m.logger.Error("failed to set Redis progress", "eval_id", evalID, "error", err)
	}

	// Stop monitoring for terminal states
	if status == model.StatusCompleted || status == model.StatusFailed {
		m.StopMonitoring(evalID)
	}

	m.logger.Info("job state changed",
		"eval_id", evalID,
		"event", eventType,
		"status", status,
	)
}

// handleJobNotFound handles the case when a Job is no longer found
func (m *Monitor) handleJobNotFound(ctx context.Context, evalID string) {
	// Get current evaluation status from DB
	eval, err := m.evalRepo.GetByID(ctx, evalID)
	if err != nil {
		m.logger.Error("failed to get evaluation", "eval_id", evalID, "error", err)
		return
	}

	// Only update if not in terminal state
	if eval.Status == model.StatusPending || eval.Status == model.StatusRunning {
		// Job was deleted/cancelled - update status
		event := &JobEvent{
			EvalID:    evalID,
			EventType: EventJobDeleted,
			Message:   "Job was deleted or not found",
			Timestamp: time.Now(),
		}
		m.recordEvent(ctx, event)

		// Update status to cancelled
		if err := m.evalRepo.UpdateStatus(ctx, evalID, model.StatusCancelled, 0); err != nil {
			m.logger.Error("failed to update status to cancelled", "eval_id", evalID, "error", err)
		}

		m.cache.SetStatus(ctx, evalID, string(model.StatusCancelled))
	}

	// Stop monitoring
	m.StopMonitoring(evalID)
}

// updateProgress updates the progress in DB and Redis
func (m *Monitor) updateProgress(ctx context.Context, evalID string, progress int) {
	// Get current evaluation
	eval, err := m.evalRepo.GetByID(ctx, evalID)
	if err != nil {
		return
	}

	// Only update progress if status is not terminal
	if eval.Status == model.StatusPending || eval.Status == model.StatusRunning {
		m.evalRepo.UpdateStatus(ctx, evalID, eval.Status, progress)
		m.cache.SetProgress(ctx, evalID, progress)
	}
}

// checkForOOMKilled checks if any pods in the Job were OOM killed
func (m *Monitor) checkForOOMKilled(ctx context.Context, evalID string, k8sJob *batchv1.Job) {
	pods, err := m.getJobPods(ctx, evalID)
	if err != nil {
		m.logger.Error("failed to get job pods", "eval_id", evalID, "error", err)
		return
	}

	for _, pod := range pods {
		if IsOOMKilled(&pod) {
			// Record OOM event
			event := &JobEvent{
				EvalID:      evalID,
				EventType:   EventPodOOMKilled,
				Message:     fmt.Sprintf("Pod %s was OOM killed (exit code %d)", pod.Name, GetExitCode(&pod)),
				Timestamp:   time.Now(),
				OOMDetected: true,
				OOMPodName:  pod.Name,
				OOMExitCode: GetExitCode(&pod),
			}
			m.recordEvent(ctx, event)

			// Update evaluation with OOM error
			if eval, err := m.evalRepo.GetByID(ctx, evalID); err == nil && eval.Status != model.StatusCompleted {
				m.evalRepo.UpdateStatus(ctx, evalID, model.StatusFailed, 0)
				m.logger.Warn("pod OOM killed, marking evaluation as failed",
					"eval_id", evalID,
					"pod_name", pod.Name,
				)
			}
		}
	}
}

// getJobPods retrieves pods associated with a Job
func (m *Monitor) getJobPods(ctx context.Context, evalID string) ([]v1.Pod, error) {
	selector := k8s.EvalIDSelector(evalID)
	pods, err := m.client.CoreV1().Pods(m.namespace).List(ctx, metav1.ListOptions{
		LabelSelector: selector,
	})
	if err != nil {
		return nil, err
	}
	return pods.Items, nil
}

// recordEvent stores a job event
func (m *Monitor) recordEvent(ctx context.Context, event *JobEvent) {
	// Store in event store
	if m.eventStore != nil {
		if err := m.eventStore.StoreEvent(ctx, event); err != nil {
			m.logger.Error("failed to store event", "error", err)
		}
	}

	// Send to event channel for real-time processing
	select {
	case m.eventChan <- event:
	default:
		m.logger.Warn("event channel full, dropping event", "eval_id", event.EvalID)
	}
}

// GetEvents returns the event channel for real-time event processing
func (m *Monitor) GetEvents() <-chan *JobEvent {
	return m.eventChan
}

// GetJobStatus retrieves current Job status for an evaluation
func (m *Monitor) GetJobStatus(ctx context.Context, evalID string) (*batchv1.Job, error) {
	return job.GetJob(ctx, m.client, m.namespace, evalID)
}

// GetEventsForEvaluation retrieves all events for a specific evaluation
func (m *Monitor) GetEventsForEvaluation(ctx context.Context, evalID string) ([]*JobEvent, error) {
	if m.eventStore == nil {
		return nil, nil
	}
	return m.eventStore.GetEvents(ctx, evalID)
}

// MonitorAllJobs monitors all pending/running evaluations
func (m *Monitor) MonitorAllJobs(ctx context.Context) error {
	// This would typically be called on startup to resume monitoring
	// for any evaluations that were in-progress
	evaluations, _, err := m.evalRepo.List(ctx, 1, 1000)
	if err != nil {
		return fmt.Errorf("failed to list evaluations: %w", err)
	}

	for _, eval := range evaluations {
		if eval.Status == model.StatusPending || eval.Status == model.StatusRunning {
			if err := m.StartMonitoring(ctx, eval.ID); err != nil {
				m.logger.Error("failed to start monitoring",
					"eval_id", eval.ID,
					"error", err,
				)
			}
		}
	}

	return nil
}
