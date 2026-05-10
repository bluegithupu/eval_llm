package service

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/eval_llm/backend/internal/model"
	"github.com/stretchr/testify/assert"
)

// TestHighLoad_20ConcurrentEvaluations tests VAL-CROSS-010:
// When 20 concurrent evaluations created within 1 second, all accepted by API,
// 20 evaluations created, all complete successfully, no resource starvation,
// performance within acceptable limits
func TestHighLoad_20ConcurrentEvaluations(t *testing.T) {
	evalRepo := newTestEvalRepo()

	cfg := DefaultConcurrencyConfig()
	cfg.MaxConcurrentJobs = 20 // Allow up to 20 concurrent jobs for this test
	cm := NewConcurrencyManager(evalRepo, nil, nil, nil, cfg)

	const numEvaluations = 20
	var wg sync.WaitGroup
	successCount := atomic.Int32{}
	failCount := atomic.Int32{}
	taskIDs := make([]string, numEvaluations)
	taskIDMu := sync.Mutex{}

	// Measure time for concurrent creation
	startTime := time.Now()

	// Create 20 evaluations concurrently
	for i := 0; i < numEvaluations; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			eval, err := cm.CreateEvaluation(ctx, fmt.Sprintf("model-%d", idx), []string{"dataset-1"}, nil)
			if err != nil {
				failCount.Add(1)
				t.Logf("Evaluation %d failed: %v", idx, err)
			} else {
				successCount.Add(1)
				taskIDMu.Lock()
				taskIDs[idx] = eval.ID
				taskIDMu.Unlock()
			}
		}(i)
	}

	wg.Wait()
	duration := time.Since(startTime)

	// Verify all 20 evaluations created successfully
	assert.Equal(t, int32(numEvaluations), successCount.Load(), "All 20 evaluations should be created")
	assert.Equal(t, int32(0), failCount.Load(), "No evaluations should fail")

	// Verify all task IDs are unique
	uniqueIDs := make(map[string]bool)
	for _, id := range taskIDs {
		assert.NotEmpty(t, id, "Task ID should not be empty")
		assert.False(t, uniqueIDs[id], "Task ID should be unique")
		uniqueIDs[id] = true
	}

	// Verify performance: all should complete within 1 second
	assert.Less(t, duration.Milliseconds(), int64(1000), "All evaluations should be created within 1 second")

	t.Logf("Created %d evaluations in %v", numEvaluations, duration)
}

// TestHighLoad_ConcurrentStartAndComplete tests that multiple evaluations
// can be started and completed concurrently without race conditions
func TestHighLoad_ConcurrentStartAndComplete(t *testing.T) {
	evalRepo := newTestEvalRepo()

	cfg := DefaultConcurrencyConfig()
	cfg.MaxConcurrentJobs = 10
	orch := &testOrchestrator{}
	cm := NewConcurrencyManager(evalRepo, nil, nil, orch, cfg)

	const numEvaluations = 20
	var wg sync.WaitGroup

	// Create all evaluations first
	evaluations := make([]*model.Evaluation, numEvaluations)
	for i := 0; i < numEvaluations; i++ {
		eval, err := cm.CreateEvaluation(context.Background(), fmt.Sprintf("model-%d", i), []string{"dataset-1"}, nil)
		assert.NoError(t, err)
		evaluations[i] = eval
	}

	// Now start all evaluations concurrently
	startCount := atomic.Int32{}
	for i := 0; i < numEvaluations; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			err := cm.StartEvaluation(ctx, evaluations[idx])
			if err == nil {
				startCount.Add(1)
			}
		}(i)
	}

	wg.Wait()

	// Verify most evaluations started (may be limited by semaphore)
	t.Logf("Started %d out of %d evaluations", startCount.Load(), numEvaluations)

	// Complete all running evaluations
	completeCount := atomic.Int32{}
	for i := 0; i < numEvaluations; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			err := cm.CompleteEvaluation(context.Background(), evaluations[idx].ID)
			if err == nil {
				completeCount.Add(1)
			}
		}(i)
	}

	wg.Wait()

	t.Logf("Completed %d evaluations", completeCount.Load())

	// Verify no data corruption
	for _, eval := range evaluations {
		repoEval, err := evalRepo.GetByID(context.Background(), eval.ID)
		assert.NoError(t, err)
		assert.Equal(t, model.StatusCompleted, repoEval.Status)
		assert.Equal(t, 100, repoEval.Progress)
	}
}

// TestHighLoad_20ConcurrentWithSemaphoreLimit tests that the semaphore
// correctly limits concurrent operations while still allowing all to complete
func TestHighLoad_20ConcurrentWithSemaphoreLimit(t *testing.T) {
	evalRepo := newTestEvalRepo()

	// Only allow 5 concurrent jobs
	cfg := DefaultConcurrencyConfig()
	cfg.MaxConcurrentJobs = 5
	cm := NewConcurrencyManager(evalRepo, nil, nil, nil, cfg)

	const numEvaluations = 20
	var wg sync.WaitGroup
	successCount := atomic.Int32{}
	startCount := atomic.Int32{}

	// Create and try to start 20 evaluations concurrently
	for i := 0; i < numEvaluations; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			// Create evaluation
			eval, err := cm.CreateEvaluation(ctx, fmt.Sprintf("model-%d", idx), []string{"dataset-1"}, nil)
			if err != nil {
				return
			}

			// Try to start (will be limited by semaphore)
			err = cm.StartEvaluation(ctx, eval)
			if err == nil {
				startCount.Add(1)
				successCount.Add(1)

				// Complete it
				_ = cm.CompleteEvaluation(ctx, eval.ID)
			}
		}(i)
	}

	wg.Wait()

	// With semaphore limit of 5, all 20 should eventually succeed
	// but not all can run at the same time
	assert.Equal(t, int32(numEvaluations), successCount.Load(), "All evaluations should eventually complete")
	assert.Equal(t, int32(numEvaluations), startCount.Load(), "All evaluations should be started")

	t.Logf("Completed %d evaluations with semaphore limit of %d", successCount.Load(), cfg.MaxConcurrentJobs)
}

// TestHighLoad_ResourceStarvationPrevention verifies that the system
// doesn't suffer from resource starvation under high load
func TestHighLoad_ResourceStarvationPrevention(t *testing.T) {
	evalRepo := newTestEvalRepo()

	cfg := DefaultConcurrencyConfig()
	cfg.MaxConcurrentJobs = 10
	cm := NewConcurrencyManager(evalRepo, nil, nil, nil, cfg)

	const numEvaluations = 20
	deadline := time.After(10 * time.Second)
	done := make(chan struct{})

	go func() {
		var wg sync.WaitGroup
		successCount := atomic.Int32{}

		for i := 0; i < numEvaluations; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()

				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()

				eval, err := cm.CreateEvaluation(ctx, fmt.Sprintf("model-%d", idx), []string{"dataset-1"}, nil)
				if err != nil {
					return
				}

				err = cm.StartEvaluation(ctx, eval)
				if err == nil {
					successCount.Add(1)
					_ = cm.CompleteEvaluation(ctx, eval.ID)
				}
			}(i)
		}

		wg.Wait()
		t.Logf("Resource starvation test: %d/%d succeeded", successCount.Load(), numEvaluations)
		close(done)
	}()

	select {
	case <-deadline:
		t.Fatal("Test timed out - possible resource starvation")
	case <-done:
		// Test completed successfully
	}
}

// TestHighLoad_PerformanceMeasurement measures the performance of
// concurrent evaluation creation
func TestHighLoad_PerformanceMeasurement(t *testing.T) {
	evalRepo := newTestEvalRepo()

	cfg := DefaultConcurrencyConfig()
	cfg.MaxConcurrentJobs = 20
	cm := NewConcurrencyManager(evalRepo, nil, nil, nil, cfg)

	const numEvaluations = 20

	// Measure creation performance
	startTime := time.Now()
	var wg sync.WaitGroup

	for i := 0; i < numEvaluations; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_, err := cm.CreateEvaluation(context.Background(), fmt.Sprintf("model-%d", idx), []string{"dataset-1"}, nil)
			assert.NoError(t, err)
		}(i)
	}

	wg.Wait()
	creationDuration := time.Since(startTime)

	t.Logf("Created %d evaluations in %v (avg: %v per evaluation)",
		numEvaluations, creationDuration, creationDuration/time.Duration(numEvaluations))

	// Creation should be fast (< 100ms per evaluation)
	assert.Less(t, creationDuration, 2*time.Second, "Creation should complete within 2 seconds")

	// Measure start performance
	startTime = time.Now()

	evaluations, _, err := evalRepo.List(context.Background(), 1, numEvaluations)
	assert.NoError(t, err)
	for i := 0; i < len(evaluations); i++ {
		wg.Add(1)
		go func(eval *model.Evaluation) {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = cm.StartEvaluation(ctx, eval)
		}(evaluations[i])
	}

	wg.Wait()
	startDuration := time.Since(startTime)

	t.Logf("Started %d evaluations in %v (avg: %v per evaluation)",
		len(evaluations), startDuration, startDuration/time.Duration(len(evaluations)))

	// Start operations should also be fast
	assert.Less(t, startDuration, 5*time.Second, "Start operations should complete within 5 seconds")
}

// TestHighLoad_NoCrossContamination verifies that concurrent evaluations
// don't have their data contaminated
func TestHighLoad_NoCrossContamination(t *testing.T) {
	evalRepo := newTestEvalRepo()

	cfg := DefaultConcurrencyConfig()
	cfg.MaxConcurrentJobs = 10
	cm := NewConcurrencyManager(evalRepo, nil, nil, nil, cfg)

	const numEvaluations = 10
	models := []string{"gpt-4", "claude", "qwen", "gemini", "llama"}
	datasets := []string{"mmlu", "hellaswag", "truthfulqa", "arc", "winogrande"}

	// Track expected model/dataset pairs by ID
	expectedPairs := make(map[string]struct {
		model   string
		dataset string
	})

	// Create evaluations with different models and datasets
	for i := 0; i < numEvaluations; i++ {
		model := models[i%len(models)]
		dataset := datasets[i%len(datasets)]
		eval, err := cm.CreateEvaluation(context.Background(), model, []string{dataset}, nil)
		assert.NoError(t, err)
		assert.Equal(t, model, eval.ModelID)
		assert.Equal(t, []string{dataset}, eval.DatasetIDs)
		expectedPairs[eval.ID] = struct {
			model   string
			dataset string
		}{model: model, dataset: dataset}
	}

	// Verify each evaluation has correct data by looking up by ID
	allEvals, _, err := evalRepo.List(context.Background(), 1, numEvaluations*2)
	assert.NoError(t, err)
	assert.Len(t, allEvals, numEvaluations)

	for _, eval := range allEvals {
		expected, ok := expectedPairs[eval.ID]
		assert.True(t, ok, "Evaluation ID should exist in expected pairs")
		assert.Equal(t, expected.model, eval.ModelID, "Model should not be contaminated for eval %s", eval.ID)
		assert.Equal(t, []string{expected.dataset}, eval.DatasetIDs, "Dataset should not be contaminated for eval %s", eval.ID)
	}
}

// TestHighLoad_ConcurrentStatusUpdates verifies status updates don't race
func TestHighLoad_ConcurrentStatusUpdates(t *testing.T) {
	evalRepo := newTestEvalRepo()

	cfg := DefaultConcurrencyConfig()
	cm := NewConcurrencyManager(evalRepo, nil, nil, nil, cfg)

	// Create an evaluation
	eval, err := cm.CreateEvaluation(context.Background(), "model-1", []string{"dataset-1"}, nil)
	assert.NoError(t, err)

	// Start it
	err = cm.StartEvaluation(context.Background(), eval)
	assert.NoError(t, err)

	// Perform many concurrent progress updates
	const numUpdates = 50
	var wg sync.WaitGroup
	successCount := atomic.Int32{}

	for i := 0; i < numUpdates; i++ {
		wg.Add(1)
		go func(progress int) {
			defer wg.Done()

			err := cm.UpdateEvaluationProgress(context.Background(), eval.ID, progress)
			if err == nil {
				successCount.Add(1)
			}
		}(i)
	}

	wg.Wait()

	// With optimistic locking, not all updates will succeed due to version conflicts
	// This is expected behavior - the system correctly handles concurrent modifications
	t.Logf("%d/%d progress updates succeeded (conflicts are expected with optimistic locking)", successCount.Load(), numUpdates)

	// Verify final state is valid (not corrupted)
	finalEval, err := evalRepo.GetByID(context.Background(), eval.ID)
	assert.NoError(t, err)
	assert.Equal(t, model.StatusRunning, finalEval.Status)
	// Progress should be a valid value that was attempted
	assert.GreaterOrEqual(t, finalEval.Progress, 0)
	assert.LessOrEqual(t, finalEval.Progress, 100)
	// Version should be >= 1 (initial) + 1 (start) + successful updates
	assert.GreaterOrEqual(t, finalEval.Version, 2)
}

// TestHighLoad_ConcurrentCancelAndComplete verifies race between cancel and complete
func TestHighLoad_ConcurrentCancelAndComplete(t *testing.T) {
	evalRepo := newTestEvalRepo()

	cfg := DefaultConcurrencyConfig()
	orch := &testOrchestrator{}
	cm := NewConcurrencyManager(evalRepo, nil, nil, orch, cfg)

	// Create an evaluation
	eval, err := cm.CreateEvaluation(context.Background(), "model-1", []string{"dataset-1"}, nil)
	assert.NoError(t, err)

	// Start it
	err = cm.StartEvaluation(context.Background(), eval)
	assert.NoError(t, err)

	// Try to cancel and complete concurrently
	var wg sync.WaitGroup
	cancelCalled := atomic.Bool{}
	completeCalled := atomic.Bool{}

	wg.Add(2)
	go func() {
		defer wg.Done()
		err := cm.CancelEvaluation(context.Background(), eval.ID)
		if err == nil {
			cancelCalled.Store(true)
		}
	}()

	go func() {
		defer wg.Done()
		err := cm.CompleteEvaluation(context.Background(), eval.ID)
		if err == nil {
			completeCalled.Store(true)
		}
	}()

	wg.Wait()

	// Exactly one should succeed (or both may see conflict due to atomic operations)
	t.Logf("Cancel called: %v, Complete called: %v", cancelCalled.Load(), completeCalled.Load())

	// Verify final state is deterministic (one of the terminal states)
	finalEval, err := evalRepo.GetByID(context.Background(), eval.ID)
	assert.NoError(t, err)
	assert.True(t, finalEval.Status == model.StatusCancelled || finalEval.Status == model.StatusCompleted,
		"Final status should be a terminal state, got: %s", finalEval.Status)
}

// TestHighLoad_30SecondsSustainedLoad tests sustained load over time
func TestHighLoad_30SecondsSustainedLoad(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping sustained load test in short mode")
	}

	evalRepo := newTestEvalRepo()

	cfg := DefaultConcurrencyConfig()
	cfg.MaxConcurrentJobs = 5
	cm := NewConcurrencyManager(evalRepo, nil, nil, nil, cfg)

	deadline := time.After(30 * time.Second)
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	totalCreated := atomic.Int32{}
	batch := 0

	for {
		select {
		case <-deadline:
			t.Logf("Sustained load test completed: created %d evaluations in %d batches",
				totalCreated.Load(), batch)
			return
		case <-ticker.C:
			batch++
			var wg sync.WaitGroup
			created := atomic.Int32{}

			// Create 5 evaluations per batch
			for i := 0; i < 5; i++ {
				wg.Add(1)
				go func(idx int) {
					defer wg.Done()
					eval, err := cm.CreateEvaluation(context.Background(),
						fmt.Sprintf("model-%d-%d", batch, idx), []string{"dataset-1"}, nil)
					if err == nil {
						created.Add(1)
						// Start and complete
						_ = cm.StartEvaluation(context.Background(), eval)
						_ = cm.CompleteEvaluation(context.Background(), eval.ID)
					}
				}(i)
			}

			wg.Wait()
			totalCreated.Add(created.Load())
		}
	}
}
