package repository

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

// TestPredictionCreate tests creating a single prediction
func TestPredictionCreate(t *testing.T) {
	ctx := context.Background()
	repo := NewPredictionRepository(testDB.Pool())
	evalRepo := NewEvaluationRepository(testDB.Pool())

	modelID := getTestModelID(t)
	datasetID := getTestDatasetID(t)

	// Create evaluation first
	eval := createTestEvaluation(ctx, t, evalRepo, modelID, datasetID)

	// Create prediction
	prediction := &Prediction{
		EvaluationID:  eval.ID,
		DatasetID:     datasetID,
		QuestionIndex: 0,
		Question:      "What is the capital of France?",
		Prediction:    "Paris",
		Answer:        "Paris",
		Correct:       true,
		Reasoning:     "France's capital is Paris.",
		Metadata:      map[string]any{"tokens": 15, "latency_ms": 250},
	}

	err := repo.Create(ctx, prediction)
	if err != nil {
		t.Fatalf("Failed to create prediction: %v", err)
	}

	// Verify ID was set
	if prediction.ID == "" {
		t.Error("Expected ID to be set after creation")
	}

	// Cleanup
	_ = evalRepo.Delete(ctx, eval.ID)
}

// TestPredictionBatchInsert tests batch inserting multiple predictions
func TestPredictionBatchInsert(t *testing.T) {
	ctx := context.Background()
	repo := NewPredictionRepository(testDB.Pool())
	evalRepo := NewEvaluationRepository(testDB.Pool())

	modelID := getTestModelID(t)
	datasetID := getTestDatasetID(t)

	// Create evaluation first
	eval := createTestEvaluation(ctx, t, evalRepo, modelID, datasetID)

	// Create batch of predictions
	predictions := []*Prediction{
		{
			EvaluationID:  eval.ID,
			DatasetID:     datasetID,
			QuestionIndex: 0,
			Question:      "Question 1",
			Prediction:    "Answer A",
			Answer:        "Answer A",
			Correct:       true,
		},
		{
			EvaluationID:  eval.ID,
			DatasetID:     datasetID,
			QuestionIndex: 1,
			Question:      "Question 2",
			Prediction:    "Answer B",
			Answer:        "Answer C",
			Correct:       false,
		},
		{
			EvaluationID:  eval.ID,
			DatasetID:     datasetID,
			QuestionIndex: 2,
			Question:      "Question 3",
			Prediction:    "Answer D",
			Answer:        "Answer D",
			Correct:       true,
		},
	}

	err := repo.BatchInsert(ctx, predictions)
	if err != nil {
		t.Fatalf("Failed to batch insert predictions: %v", err)
	}

	// Verify count
	count, err := repo.CountByEvaluationID(ctx, eval.ID)
	if err != nil {
		t.Fatalf("Failed to count predictions: %v", err)
	}

	if count != 3 {
		t.Errorf("Expected 3 predictions, got %d", count)
	}

	// Cleanup
	_ = evalRepo.Delete(ctx, eval.ID)
}

// TestPredictionGetByEvaluationID tests retrieving predictions with pagination
func TestPredictionGetByEvaluationID(t *testing.T) {
	ctx := context.Background()
	repo := NewPredictionRepository(testDB.Pool())
	evalRepo := NewEvaluationRepository(testDB.Pool())

	modelID := getTestModelID(t)
	datasetID := getTestDatasetID(t)

	// Create evaluation and batch predictions
	eval := createTestEvaluation(ctx, t, evalRepo, modelID, datasetID)

	// Create 20 predictions
	predictions := make([]*Prediction, 20)
	for i := 0; i < 20; i++ {
		predictions[i] = &Prediction{
			EvaluationID:  eval.ID,
			DatasetID:     datasetID,
			QuestionIndex: i,
			Question:      "Question " + string(rune('A'+i)),
			Prediction:    "Prediction",
			Answer:        "Answer",
			Correct:       i%2 == 0,
		}
	}
	_ = repo.BatchInsert(ctx, predictions)

	// Get first page (limit 10)
	page1, total, err := repo.GetByEvaluationID(ctx, eval.ID, 1, 10)
	if err != nil {
		t.Fatalf("Failed to get predictions page 1: %v", err)
	}

	// Verify pagination
	if len(page1) > 10 {
		t.Errorf("Expected at most 10 predictions on page 1, got %d", len(page1))
	}

	if total != 20 {
		t.Errorf("Expected total 20, got %d", total)
	}

	// Get second page
	_, total2, err := repo.GetByEvaluationID(ctx, eval.ID, 2, 10)
	if err != nil {
		t.Fatalf("Failed to get predictions page 2: %v", err)
	}

	if total2 != total {
		t.Errorf("Total counts should match: %d vs %d", total, total2)
	}

	// Verify order (should be by question_index ASC)
	if len(page1) >= 2 {
		if page1[0].QuestionIndex > page1[1].QuestionIndex {
			t.Error("Expected predictions ordered by question_index ASC")
		}
	}

	// Cleanup
	_ = evalRepo.Delete(ctx, eval.ID)
}

// TestPredictionCountByEvaluationID tests counting predictions
func TestPredictionCountByEvaluationID(t *testing.T) {
	ctx := context.Background()
	repo := NewPredictionRepository(testDB.Pool())
	evalRepo := NewEvaluationRepository(testDB.Pool())

	modelID := getTestModelID(t)
	datasetID := getTestDatasetID(t)

	// Create evaluation
	eval := createTestEvaluation(ctx, t, evalRepo, modelID, datasetID)

	// Initially should have 0 predictions
	count, err := repo.CountByEvaluationID(ctx, eval.ID)
	if err != nil {
		t.Fatalf("Failed to count predictions: %v", err)
	}
	if count != 0 {
		t.Errorf("Expected initial count 0, got %d", count)
	}

	// Add 5 predictions
	predictions := make([]*Prediction, 5)
	for i := 0; i < 5; i++ {
		predictions[i] = &Prediction{
			EvaluationID:  eval.ID,
			DatasetID:     datasetID,
			QuestionIndex: i,
			Question:      "Q",
			Prediction:    "P",
			Answer:        "A",
			Correct:       true,
		}
	}
	_ = repo.BatchInsert(ctx, predictions)

	// Verify count is now 5
	count, err = repo.CountByEvaluationID(ctx, eval.ID)
	if err != nil {
		t.Fatalf("Failed to count predictions: %v", err)
	}
	if count != 5 {
		t.Errorf("Expected count 5, got %d", count)
	}

	// Cleanup
	_ = evalRepo.Delete(ctx, eval.ID)
}

// TestPredictionForeignKeyEnforcement tests that invalid evaluation_id is rejected
func TestPredictionForeignKeyEnforcement(t *testing.T) {
	ctx := context.Background()
	repo := NewPredictionRepository(testDB.Pool())

	datasetID := getTestDatasetID(t)

	// Try to create prediction with invalid evaluation_id
	prediction := &Prediction{
		EvaluationID:  uuid.New().String(), // Non-existent
		DatasetID:     datasetID,
		QuestionIndex: 0,
		Question:      "Q",
		Prediction:    "P",
		Answer:        "A",
		Correct:       true,
	}

	err := repo.Create(ctx, prediction)
	if err == nil {
		t.Error("Expected error for invalid evaluation_id (foreign key violation)")
	}
}
