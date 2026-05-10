package repository

import (
	"context"
	"testing"
	"time"

	"github.com/eval_llm/backend/internal/model"
	"github.com/google/uuid"
)

// TestEvaluationCreate tests creating an evaluation with auto-populated created_at
func TestEvaluationCreate(t *testing.T) {
	ctx := context.Background()
	repo := NewEvaluationRepository(testDB.Pool())

	modelID := getTestModelID(t)
	datasetID := getTestDatasetID(t)

	eval := model.NewEvaluation(modelID, []string{datasetID})
	eval.Config = map[string]any{"batch_size": 10, "temperature": 0.7}

	err := repo.Create(ctx, eval)
	if err != nil {
		t.Fatalf("Failed to create evaluation: %v", err)
	}

	if eval.ID == "" {
		t.Error("Expected ID to be set after creation")
	}

	if eval.CreatedAt.IsZero() {
		t.Error("Expected created_at to be set")
	}

	if eval.UpdatedAt.IsZero() {
		t.Error("Expected updated_at to be set")
	}

	if eval.Status != model.StatusPending {
		t.Errorf("Expected status to be pending, got %s", eval.Status)
	}

	_ = repo.Delete(ctx, eval.ID)
}

// TestEvaluationGetByID tests retrieving an evaluation by ID
func TestEvaluationGetByID(t *testing.T) {
	ctx := context.Background()
	repo := NewEvaluationRepository(testDB.Pool())

	modelID := getTestModelID(t)
	datasetID := getTestDatasetID(t)

	// Create test evaluation
	eval := model.NewEvaluation(modelID, []string{datasetID})
	_ = repo.Create(ctx, eval)

	// Retrieve it
	retrieved, err := repo.GetByID(ctx, eval.ID)
	if err != nil {
		t.Fatalf("Failed to get evaluation: %v", err)
	}

	if retrieved.ID != eval.ID {
		t.Errorf("Expected ID %s, got %s", eval.ID, retrieved.ID)
	}

	if retrieved.ModelID != eval.ModelID {
		t.Errorf("Expected ModelID %s, got %s", eval.ModelID, retrieved.ModelID)
	}

	if retrieved.Status != eval.Status {
		t.Errorf("Expected status %s, got %s", eval.Status, retrieved.Status)
	}

	_ = repo.Delete(ctx, eval.ID)
}

// TestEvaluationGetByIDNotFound tests getting a non-existent evaluation
func TestEvaluationGetByIDNotFound(t *testing.T) {
	ctx := context.Background()
	repo := NewEvaluationRepository(testDB.Pool())

	nonExistentID := uuid.New().String()
	_, err := repo.GetByID(ctx, nonExistentID)
	if err == nil {
		t.Error("Expected error for non-existent evaluation")
	}
}

// TestEvaluationUpdateStatus tests status updates with timestamp management
func TestEvaluationUpdateStatus(t *testing.T) {
	ctx := context.Background()
	repo := NewEvaluationRepository(testDB.Pool())

	modelID := getTestModelID(t)
	datasetID := getTestDatasetID(t)

	// Create test evaluation
	eval := model.NewEvaluation(modelID, []string{datasetID})
	_ = repo.Create(ctx, eval)

	// Update status to running
	err := repo.UpdateStatus(ctx, eval.ID, model.StatusRunning, 0)
	if err != nil {
		t.Fatalf("Failed to update status to running: %v", err)
	}

	retrieved, err := repo.GetByID(ctx, eval.ID)
	if err != nil {
		t.Fatalf("Failed to get evaluation: %v", err)
	}

	if retrieved.StartedAt == nil {
		t.Error("Expected started_at to be set when status changes to running")
	}

	// Update status to completed
	err = repo.UpdateStatus(ctx, eval.ID, model.StatusCompleted, 100)
	if err != nil {
		t.Fatalf("Failed to update status to completed: %v", err)
	}

	retrieved, err = repo.GetByID(ctx, eval.ID)
	if err != nil {
		t.Fatalf("Failed to get evaluation: %v", err)
	}

	if retrieved.CompletedAt == nil {
		t.Error("Expected completed_at to be set when status changes to completed")
	}

	_ = repo.Delete(ctx, eval.ID)
}

// TestEvaluationList tests listing evaluations with pagination
func TestEvaluationList(t *testing.T) {
	ctx := context.Background()
	repo := NewEvaluationRepository(testDB.Pool())

	modelID := getTestModelID(t)
	datasetID := getTestDatasetID(t)

	// Create multiple evaluations
	var createdIDs []string
	for i := 0; i < 15; i++ {
		eval := model.NewEvaluation(modelID, []string{datasetID})
		err := repo.Create(ctx, eval)
		if err != nil {
			t.Fatalf("Failed to create evaluation %d: %v", i, err)
		}
		createdIDs = append(createdIDs, eval.ID)
	}

	// List first page with limit 10
	evals, total, err := repo.List(ctx, 1, 10)
	if err != nil {
		t.Fatalf("Failed to list evaluations: %v", err)
	}

	if len(evals) > 10 {
		t.Errorf("Expected at most 10 evaluations, got %d", len(evals))
	}

	if total < 15 {
		t.Errorf("Expected total >= 15, got %d", total)
	}

	// Verify order (should be DESC by created_at)
	if len(evals) >= 2 {
		if evals[0].CreatedAt.Before(evals[1].CreatedAt) {
			t.Error("Expected evaluations ordered by created_at DESC")
		}
	}

	for _, id := range createdIDs {
		_ = repo.Delete(ctx, id)
	}
}

// TestEvaluationListPagination tests pagination across multiple pages
func TestEvaluationListPagination(t *testing.T) {
	ctx := context.Background()
	repo := NewEvaluationRepository(testDB.Pool())

	modelID := getTestModelID(t)
	datasetID := getTestDatasetID(t)

	// Create evaluations
	var createdIDs []string
	for i := 0; i < 8; i++ {
		eval := model.NewEvaluation(modelID, []string{datasetID})
		_ = repo.Create(ctx, eval)
		createdIDs = append(createdIDs, eval.ID)
	}

	// Get first page
	page1, total1, err := repo.List(ctx, 1, 3)
	if err != nil {
		t.Fatalf("Failed to list page 1: %v", err)
	}

	// Get second page
	page2, total2, err := repo.List(ctx, 2, 3)
	if err != nil {
		t.Fatalf("Failed to list page 2: %v", err)
	}

	if total1 != total2 {
		t.Errorf("Total counts don't match: %d vs %d", total1, total2)
	}

	if len(page1) > 3 {
		t.Errorf("Expected page1 to have at most 3 items, got %d", len(page1))
	}
	if len(page2) > 3 {
		t.Errorf("Expected page2 to have at most 3 items, got %d", len(page2))
	}

	// Verify pages are different (if we have enough items)
	if len(page1) > 0 && len(page2) > 0 {
		if page1[0].ID == page2[0].ID {
			t.Error("Expected different items on different pages")
		}
	}

	for _, id := range createdIDs {
		_ = repo.Delete(ctx, id)
	}
}

// TestEvaluationCount tests counting evaluations
func TestEvaluationCount(t *testing.T) {
	ctx := context.Background()
	repo := NewEvaluationRepository(testDB.Pool())

	// Get current count
	initialCount, err := repo.Count(ctx)
	if err != nil {
		t.Fatalf("Failed to count evaluations: %v", err)
	}

	modelID := getTestModelID(t)
	datasetID := getTestDatasetID(t)

	// Create an evaluation
	eval := model.NewEvaluation(modelID, []string{datasetID})
	_ = repo.Create(ctx, eval)

	newCount, err := repo.Count(ctx)
	if err != nil {
		t.Fatalf("Failed to count evaluations: %v", err)
	}

	if newCount != initialCount+1 {
		t.Errorf("Expected count to increase by 1, got %d -> %d", initialCount, newCount)
	}

	_ = repo.Delete(ctx, eval.ID)
}

// TestEvaluationCountByStatus tests counting by status
func TestEvaluationCountByStatus(t *testing.T) {
	ctx := context.Background()
	repo := NewEvaluationRepository(testDB.Pool())

	modelID := getTestModelID(t)
	datasetID := getTestDatasetID(t)

	// Create evaluations with different statuses
	eval1 := model.NewEvaluation(modelID, []string{datasetID})
	_ = repo.Create(ctx, eval1)

	eval2 := model.NewEvaluation(modelID, []string{datasetID})
	_ = repo.Create(ctx, eval2)
	_ = repo.UpdateStatus(ctx, eval2.ID, model.StatusRunning, 50)

	// Count pending
	pendingCount, err := repo.CountByStatus(ctx, model.StatusPending)
	if err != nil {
		t.Fatalf("Failed to count pending: %v", err)
	}

	// Count running
	runningCount, err := repo.CountByStatus(ctx, model.StatusRunning)
	if err != nil {
		t.Fatalf("Failed to count running: %v", err)
	}

	// Verify counts
	if pendingCount < 1 {
		t.Errorf("Expected at least 1 pending evaluation, got %d", pendingCount)
	}
	if runningCount < 1 {
		t.Errorf("Expected at least 1 running evaluation, got %d", runningCount)
	}

	_ = repo.Delete(ctx, eval1.ID)
	_ = repo.Delete(ctx, eval2.ID)
}

// TestEvaluationForeignKeyEnforcement tests that invalid model_id is rejected
func TestEvaluationForeignKeyEnforcement(t *testing.T) {
	ctx := context.Background()
	repo := NewEvaluationRepository(testDB.Pool())

	// Try to create with invalid model_id (non-existent UUID)
	eval := &model.Evaluation{
		ID:         uuid.New().String(),
		ModelID:    uuid.New().String(), // Non-existent
		DatasetIDs: []string{getTestDatasetID(t)},
		Status:     model.StatusPending,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	err := repo.Create(ctx, eval)
	if err == nil {
		t.Error("Expected error for invalid model_id (foreign key violation)")
		_ = repo.Delete(ctx, eval.ID)
	}
}
