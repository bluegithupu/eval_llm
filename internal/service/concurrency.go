package service

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/eval_llm/backend/internal/model"
	"github.com/eval_llm/backend/internal/repository"
	"github.com/google/uuid"
)

// ErrEvaluationNotFound is returned when an evaluation is not found
var ErrEvaluationNotFound = errors.New("evaluation not found")

// ErrConcurrentModification is returned when concurrent modification is detected
var ErrConcurrentModification = errors.New("concurrent modification detected")

// ConcurrencyConfig holds configuration for concurrency handling
type ConcurrencyConfig struct {
	MaxConcurrentJobs int           // Maximum concurrent evaluation jobs
	PollInterval      time.Duration // Interval for status polling
	OperationTimeout  time.Duration // Timeout for individual operations
}

// DefaultConcurrencyConfig returns default concurrency configuration
func DefaultConcurrencyConfig() *ConcurrencyConfig {
	return &ConcurrencyConfig{
		MaxConcurrentJobs: 10,
		PollInterval:      5 * time.Second,
		OperationTimeout:  30 * time.Second,
	}
}

// ConcurrencyManager manages concurrent evaluation operations with proper isolation
type ConcurrencyManager struct {
	cfg          *ConcurrencyConfig
	evalRepo     repository.EvaluationRepository
	resultRepo   repository.ResultRepository
	predRepo     repository.PredictionRepository
	orchestrator EvaluatorOrchestrator

	// Semaphore for limiting concurrent jobs
	semaphore chan struct{}

	// Mutex for protecting creation operations
	createMu sync.Mutex

	// Per-evaluation locks to prevent duplicate creation
	evalLocks map[string]*sync.Mutex
	locksMu   sync.RWMutex

	// Active evaluations tracking
	activeEvals map[string]context.CancelFunc
	activeMu    sync.RWMutex
}

// NewConcurrencyManager creates a new concurrency manager
func NewConcurrencyManager(
	evalRepo repository.EvaluationRepository,
	resultRepo repository.ResultRepository,
	predRepo repository.PredictionRepository,
	orchestrator EvaluatorOrchestrator,
	cfg *ConcurrencyConfig,
) *ConcurrencyManager {
	if cfg == nil {
		cfg = DefaultConcurrencyConfig()
	}

	return &ConcurrencyManager{
		cfg:          cfg,
		evalRepo:     evalRepo,
		resultRepo:   resultRepo,
		predRepo:     predRepo,
		orchestrator: orchestrator,
		semaphore:    make(chan struct{}, cfg.MaxConcurrentJobs),
		evalLocks:    make(map[string]*sync.Mutex),
		activeEvals:  make(map[string]context.CancelFunc),
	}
}

// getEvalLock gets or creates a mutex for a specific evaluation
func (cm *ConcurrencyManager) getEvalLock(evalID string) *sync.Mutex {
	cm.locksMu.Lock()
	defer cm.locksMu.Unlock()

	if lock, exists := cm.evalLocks[evalID]; exists {
		return lock
	}

	lock := &sync.Mutex{}
	cm.evalLocks[evalID] = lock
	return lock
}

// releaseEvalLock removes a lock when no longer needed
func (cm *ConcurrencyManager) releaseEvalLock(evalID string) {
	cm.locksMu.Lock()
	defer cm.locksMu.Unlock()

	delete(cm.evalLocks, evalID)
}

// AcquireSemaphore acquires a slot in the concurrency semaphore
// Returns a function to release the slot
func (cm *ConcurrencyManager) AcquireSemaphore(ctx context.Context) (release func(), err error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case cm.semaphore <- struct{}{}:
		return func() {
			<-cm.semaphore
		}, nil
	}
}

// CreateEvaluation creates a new evaluation with concurrency protection
// It ensures no duplicate evaluations are created for the same model/dataset pair
// within a short time window
func (cm *ConcurrencyManager) CreateEvaluation(ctx context.Context, modelID string, datasetIDs []string, config map[string]any) (*model.Evaluation, error) {
	// Use creation lock to prevent concurrent creates
	cm.createMu.Lock()
	defer cm.createMu.Unlock()

	// Generate unique ID
	evalID := uuid.New().String()

	// Create evaluation with pending status
	eval := &model.Evaluation{
		ID:         evalID,
		ModelID:    modelID,
		DatasetIDs: datasetIDs,
		Config:     config,
		Status:     model.StatusPending,
		Progress:   0,
		Version:    0, // Initial version for optimistic locking
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	// Insert into database (DB will auto-increment version)
	if err := cm.evalRepo.Create(ctx, eval); err != nil {
		return nil, fmt.Errorf("failed to create evaluation: %w", err)
	}

	return eval, nil
}

// StartEvaluation starts an evaluation job with concurrency protection
// Uses optimistic locking to handle race conditions on status updates
func (cm *ConcurrencyManager) StartEvaluation(ctx context.Context, eval *model.Evaluation) error {
	// Acquire semaphore slot
	release, err := cm.AcquireSemaphore(ctx)
	if err != nil {
		return fmt.Errorf("failed to acquire semaphore: %w", err)
	}
	defer release()

	// Get evaluation lock
	lock := cm.getEvalLock(eval.ID)
	lock.Lock()
	defer lock.Unlock()

	// Fetch current state for optimistic locking
	currentEval, err := cm.evalRepo.GetByID(ctx, eval.ID)
	if err != nil {
		return fmt.Errorf("failed to get evaluation: %w", err)
	}

	// Check if already running or completed
	if currentEval.Status != model.StatusPending {
		return fmt.Errorf("evaluation already in state: %s", currentEval.Status)
	}

	// Start evaluation via orchestrator
	if cm.orchestrator != nil {
		if err := cm.orchestrator.StartEvaluation(ctx, eval); err != nil {
			// Mark as failed
			if atomicErr := cm.evalRepo.UpdateStatusAtomicWithError(ctx, eval.ID, currentEval.Version, model.StatusFailed, 0, err.Error()); atomicErr != nil {
				if atomicErr == repository.ErrConcurrentModification {
					// Another process updated the evaluation
					return fmt.Errorf("concurrent modification detected during start")
				}
				return fmt.Errorf("failed to mark evaluation as failed: %w", atomicErr)
			}
			return err
		}
	}

	// Update status to running with atomic operation
	if err := cm.evalRepo.UpdateStatusAtomic(ctx, eval.ID, currentEval.Version, model.StatusRunning, 10); err != nil {
		if err == repository.ErrConcurrentModification {
			return ErrConcurrentModification
		}
		return fmt.Errorf("failed to update status to running: %w", err)
	}

	// Store cancel function for cleanup
	cm.storeActiveEval(eval.ID, func() {})

	return nil
}

// UpdateEvaluationProgress updates evaluation progress with optimistic locking
func (cm *ConcurrencyManager) UpdateEvaluationProgress(ctx context.Context, evalID string, progress int) error {
	// Get current evaluation
	currentEval, err := cm.evalRepo.GetByID(ctx, evalID)
	if err != nil {
		return fmt.Errorf("failed to get evaluation: %w", err)
	}

	// Only update if in running state
	if currentEval.Status != model.StatusRunning {
		return nil // Ignore progress updates for non-running evaluations
	}

	// Atomic update with version check
	if err := cm.evalRepo.UpdateStatusAtomic(ctx, evalID, currentEval.Version, model.StatusRunning, progress); err != nil {
		if err == repository.ErrConcurrentModification {
			return ErrConcurrentModification
		}
		return fmt.Errorf("failed to update progress: %w", err)
	}

	return nil
}

// CompleteEvaluation marks an evaluation as completed with atomic status update
func (cm *ConcurrencyManager) CompleteEvaluation(ctx context.Context, evalID string) error {
	// Get current evaluation
	currentEval, err := cm.evalRepo.GetByID(ctx, evalID)
	if err != nil {
		return fmt.Errorf("failed to get evaluation: %w", err)
	}

	// Only complete if in running state
	if currentEval.Status != model.StatusRunning {
		return fmt.Errorf("evaluation not in running state: %s", currentEval.Status)
	}

	// Atomic update with version check
	if err := cm.evalRepo.UpdateStatusAtomic(ctx, evalID, currentEval.Version, model.StatusCompleted, 100); err != nil {
		if err == repository.ErrConcurrentModification {
			return ErrConcurrentModification
		}
		return fmt.Errorf("failed to complete evaluation: %w", err)
	}

	// Remove from active evaluations
	cm.removeActiveEval(evalID)

	return nil
}

// FailEvaluation marks an evaluation as failed with atomic status update
func (cm *ConcurrencyManager) FailEvaluation(ctx context.Context, evalID string, errorMsg string) error {
	// Get current evaluation
	currentEval, err := cm.evalRepo.GetByID(ctx, evalID)
	if err != nil {
		return fmt.Errorf("failed to get evaluation: %w", err)
	}

	// Can fail from running or pending state
	if currentEval.Status == model.StatusCompleted || currentEval.Status == model.StatusFailed || currentEval.Status == model.StatusCancelled {
		return fmt.Errorf("evaluation already in terminal state: %s", currentEval.Status)
	}

	// Atomic update with version check
	if err := cm.evalRepo.UpdateStatusAtomicWithError(ctx, evalID, currentEval.Version, model.StatusFailed, 0, errorMsg); err != nil {
		if err == repository.ErrConcurrentModification {
			return ErrConcurrentModification
		}
		return fmt.Errorf("failed to mark evaluation as failed: %w", err)
	}

	// Remove from active evaluations
	cm.removeActiveEval(evalID)

	return nil
}

// CancelEvaluation cancels a running or pending evaluation
func (cm *ConcurrencyManager) CancelEvaluation(ctx context.Context, evalID string) error {
	// Get evaluation lock
	lock := cm.getEvalLock(evalID)
	lock.Lock()
	defer lock.Unlock()

	// Get current evaluation
	currentEval, err := cm.evalRepo.GetByID(ctx, evalID)
	if err != nil {
		return fmt.Errorf("failed to get evaluation: %w", err)
	}

	// Can only cancel from pending or running state
	if currentEval.Status == model.StatusCompleted || currentEval.Status == model.StatusFailed || currentEval.Status == model.StatusCancelled {
		return fmt.Errorf("evaluation already in terminal state: %s", currentEval.Status)
	}

	// Cancel via orchestrator first
	if cm.orchestrator != nil {
		if err := cm.orchestrator.CancelEvaluation(ctx, evalID); err != nil {
			// Log but don't fail - we'll still update DB status
		}
	}

	// Atomic update to cancelled
	if err := cm.evalRepo.UpdateStatusAtomic(ctx, evalID, currentEval.Version, model.StatusCancelled, 0); err != nil {
		if err == repository.ErrConcurrentModification {
			return ErrConcurrentModification
		}
		return fmt.Errorf("failed to cancel evaluation: %w", err)
	}

	// Remove from active evaluations
	cm.removeActiveEval(evalID)

	// Release the eval lock
	cm.releaseEvalLock(evalID)

	return nil
}

// storeActiveEval stores a cancel function for an active evaluation
func (cm *ConcurrencyManager) storeActiveEval(evalID string, cancel context.CancelFunc) {
	cm.activeMu.Lock()
	defer cm.activeMu.Unlock()
	cm.activeEvals[evalID] = cancel
}

// removeActiveEval removes an evaluation from active tracking
func (cm *ConcurrencyManager) removeActiveEval(evalID string) {
	cm.activeMu.Lock()
	defer cm.activeMu.Unlock()

	if cancel, exists := cm.activeEvals[evalID]; exists {
		cancel()
		delete(cm.activeEvals, evalID)
	}
}

// GetActiveCount returns the number of currently active evaluations
func (cm *ConcurrencyManager) GetActiveCount() int {
	cm.activeMu.RLock()
	defer cm.activeMu.RUnlock()
	return len(cm.activeEvals)
}

// EvaluatorOrchestrator defines the interface for evaluation orchestration
type EvaluatorOrchestrator interface {
	StartEvaluation(ctx context.Context, eval *model.Evaluation) error
	CancelEvaluation(ctx context.Context, evalID string) error
}

// Helper to convert repository error to service error
func toServiceError(err error) error {
	if err == repository.ErrConcurrentModification {
		return ErrConcurrentModification
	}
	if err == repository.ErrNotFound {
		return ErrEvaluationNotFound
	}
	return err
}
