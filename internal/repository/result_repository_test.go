package repository

import (
	"context"
	"testing"

	"github.com/eval_llm/backend/internal/model"
	"github.com/google/uuid"
)

// TestResultCreate tests creating a result with JSONB metrics
func TestResultCreate(t *testing.T) {
	ctx := context.Background()
	repo := NewResultRepository(testDB.Pool())
	evalRepo := NewEvaluationRepository(testDB.Pool())

	modelID := getTestModelID(t)
	datasetID := getTestDatasetID(t)

	// First create an evaluation
	eval := createTestEvaluation(ctx, t, evalRepo, modelID, datasetID)

	// Create result with JSONB metrics
	result := &Result{
		EvaluationID: eval.ID,
		DatasetID:    datasetID,
		Accuracy:     0.85,
		SampleCount:  100,
		CorrectCount: 85,
		Metrics: map[string]any{
			"precision": 0.87,
			"f1":        0.86,
			"recall":    0.85,
		},
		Summary: "Test evaluation summary",
	}

	err := repo.Create(ctx, result)
	if err != nil {
		t.Fatalf("Failed to create result: %v", err)
	}

	// Verify ID was set
	if result.ID == "" {
		t.Error("Expected ID to be set after creation")
	}

	// Cleanup
	_ = evalRepo.Delete(ctx, eval.ID)
}

// TestResultGetByEvaluationID tests retrieving results for an evaluation
func TestResultGetByEvaluationID(t *testing.T) {
	ctx := context.Background()
	repo := NewResultRepository(testDB.Pool())
	evalRepo := NewEvaluationRepository(testDB.Pool())

	modelID := getTestModelID(t)
	datasetID := getTestDatasetID(t)
	datasetID2 := getSecondDatasetID(t)

	// Create evaluation
	eval := createTestEvaluation(ctx, t, evalRepo, modelID, datasetID)

	// Create two results for the same evaluation
	result1 := &Result{
		EvaluationID: eval.ID,
		DatasetID:    datasetID,
		Accuracy:     0.85,
		SampleCount:  100,
		CorrectCount: 85,
		Metrics:      map[string]any{"f1": 0.86},
	}
	_ = repo.Create(ctx, result1)

	result2 := &Result{
		EvaluationID: eval.ID,
		DatasetID:    datasetID2,
		Accuracy:     0.90,
		SampleCount:  50,
		CorrectCount: 45,
		Metrics:      map[string]any{"f1": 0.91},
	}
	_ = repo.Create(ctx, result2)

	// Retrieve all results for this evaluation
	results, err := repo.GetByEvaluationID(ctx, eval.ID)
	if err != nil {
		t.Fatalf("Failed to get results: %v", err)
	}

	// Verify we got 2 results
	if len(results) != 2 {
		t.Errorf("Expected 2 results, got %d", len(results))
	}

	// Cleanup
	_ = evalRepo.Delete(ctx, eval.ID)
}

// TestResultGetByEvaluationAndDataset tests retrieving a specific result
func TestResultGetByEvaluationAndDataset(t *testing.T) {
	ctx := context.Background()
	repo := NewResultRepository(testDB.Pool())
	evalRepo := NewEvaluationRepository(testDB.Pool())

	modelID := getTestModelID(t)
	datasetID := getTestDatasetID(t)

	// Create evaluation and result
	eval := createTestEvaluation(ctx, t, evalRepo, modelID, datasetID)
	result := &Result{
		EvaluationID: eval.ID,
		DatasetID:    datasetID,
		Accuracy:     0.85,
		SampleCount:  100,
		CorrectCount: 85,
		Metrics:      map[string]any{"precision": 0.87, "f1": 0.86},
	}
	_ = repo.Create(ctx, result)

	// Retrieve specific result
	retrieved, err := repo.GetByEvaluationAndDataset(ctx, eval.ID, datasetID)
	if err != nil {
		t.Fatalf("Failed to get result: %v", err)
	}

	// Verify accuracy was properly stored
	if retrieved.Accuracy != 0.85 {
		t.Errorf("Expected accuracy 0.85, got %f", retrieved.Accuracy)
	}

	// Verify JSONB metrics
	if len(retrieved.Metrics) != 2 {
		t.Errorf("Expected 2 metrics, got %d", len(retrieved.Metrics))
	}

	if f1, ok := retrieved.Metrics["f1"].(float64); !ok || f1 != 0.86 {
		t.Errorf("Expected f1 metric 0.86, got %v", retrieved.Metrics["f1"])
	}

	// Cleanup
	_ = evalRepo.Delete(ctx, eval.ID)
}

// TestResultQueryByMetrics tests querying by JSONB metrics field
func TestResultQueryByMetrics(t *testing.T) {
	ctx := context.Background()
	repo := NewResultRepository(testDB.Pool())
	evalRepo := NewEvaluationRepository(testDB.Pool())

	modelID := getTestModelID(t)
	datasetID := getTestDatasetID(t)

	// Create evaluation and result with specific f1 score
	eval := createTestEvaluation(ctx, t, evalRepo, modelID, datasetID)
	result := &Result{
		EvaluationID: eval.ID,
		DatasetID:    datasetID,
		Accuracy:     0.85,
		SampleCount:  100,
		CorrectCount: 85,
		Metrics:      map[string]any{"f1": "0.87"},
	}
	_ = repo.Create(ctx, result)

	// Query by metrics field
	results, err := repo.QueryByMetrics(ctx, "f1", "0.87")
	if err != nil {
		t.Fatalf("Failed to query by metrics: %v", err)
	}

	// Verify we found the result
	if len(results) == 0 {
		t.Error("Expected to find result with f1=0.87")
	}

	// Cleanup
	_ = evalRepo.Delete(ctx, eval.ID)
}

// TestResultForeignKeyEnforcement tests that invalid evaluation_id is rejected
func TestResultForeignKeyEnforcement(t *testing.T) {
	ctx := context.Background()
	repo := NewResultRepository(testDB.Pool())

	datasetID := getTestDatasetID(t)

	// Try to create result with invalid evaluation_id
	result := &Result{
		EvaluationID: uuid.New().String(), // Non-existent
		DatasetID:    datasetID,
		Accuracy:     0.5,
		SampleCount:  10,
		CorrectCount: 5,
	}

	err := repo.Create(ctx, result)
	if err == nil {
		t.Error("Expected error for invalid evaluation_id (foreign key violation)")
	}
}

// Helper to get second dataset ID
func getSecondDatasetID(t *testing.T) string {
	ctx := context.Background()
	query := "SELECT id FROM datasets WHERE name = 'hellaswag' LIMIT 1"

	var datasetID string
	err := testDB.Pool().QueryRow(ctx, query).Scan(&datasetID)
	if err != nil {
		t.Fatalf("Failed to get second dataset ID: %v", err)
	}
	return datasetID
}

// Helper function to create a test evaluation (shared across test files)
func createTestEvaluation(ctx context.Context, t *testing.T, repo EvaluationRepository, modelID, datasetID string) *model.Evaluation {
	eval := model.NewEvaluation(modelID, []string{datasetID})
	err := repo.Create(ctx, eval)
	if err != nil {
		t.Fatalf("Failed to create test evaluation: %v", err)
	}
	return eval
}
