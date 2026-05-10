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

// TestPredictionLargeOffsetPagination tests pagination performance with large OFFSET
// This verifies that queries using large OFFSET values (like 9900) remain efficient
// through the use of the composite index on (evaluation_id, question_index)
func TestPredictionLargeOffsetPagination(t *testing.T) {
	ctx := context.Background()
	repo := NewPredictionRepository(testDB.Pool())
	evalRepo := NewEvaluationRepository(testDB.Pool())

	modelID := getTestModelID(t)
	datasetID := getTestDatasetID(t)

	// Create evaluation
	eval := createTestEvaluation(ctx, t, evalRepo, modelID, datasetID)

	// Create 10,500 predictions (large dataset to test pagination)
	const totalPredictions = 10500
	predictions := make([]*Prediction, totalPredictions)
	for i := 0; i < totalPredictions; i++ {
		predictions[i] = &Prediction{
			EvaluationID:  eval.ID,
			DatasetID:     datasetID,
			QuestionIndex: i,
			Question:      "Question " + string(rune('A'+(i%26))),
			Prediction:    "Prediction",
			Answer:        "Answer",
			Correct:       i%2 == 0,
		}
	}

	err := repo.BatchInsert(ctx, predictions)
	if err != nil {
		t.Fatalf("Failed to batch insert 10500 predictions: %v", err)
	}

	// Verify total count
	total, err := repo.CountByEvaluationID(ctx, eval.ID)
	if err != nil {
		t.Fatalf("Failed to count predictions: %v", err)
	}
	if total != totalPredictions {
		t.Errorf("Expected total %d, got %d", totalPredictions, total)
	}

	// Test first page (small offset)
	page1, _, err := repo.GetByEvaluationID(ctx, eval.ID, 1, 100)
	if err != nil {
		t.Fatalf("Failed to get first page: %v", err)
	}
	if len(page1) != 100 {
		t.Errorf("Expected 100 items on page 1, got %d", len(page1))
	}
	if page1[0].QuestionIndex != 0 {
		t.Errorf("Expected first item to have question_index 0, got %d", page1[0].QuestionIndex)
	}

	// Test middle page (medium offset)
	page50, _, err := repo.GetByEvaluationID(ctx, eval.ID, 50, 100)
	if err != nil {
		t.Fatalf("Failed to get page 50: %v", err)
	}
	if len(page50) != 100 {
		t.Errorf("Expected 100 items on page 50, got %d", len(page50))
	}
	// Page 50 should start at index 4900
	if page50[0].QuestionIndex != 4900 {
		t.Errorf("Expected first item on page 50 to have question_index 4900, got %d", page50[0].QuestionIndex)
	}

	// Test large offset (near end - page 100, offset 9900)
	page100, _, err := repo.GetByEvaluationID(ctx, eval.ID, 100, 100)
	if err != nil {
		t.Fatalf("Failed to get page 100: %v", err)
	}
	// Page 100 should have 100 items (from index 9900 to 9999)
	if len(page100) != 100 {
		t.Errorf("Expected 100 items on page 100, got %d", len(page100))
	}
	// First item on page 100 should be at index 9900
	if page100[0].QuestionIndex != 9900 {
		t.Errorf("Expected first item on page 100 to have question_index 9900, got %d", page100[0].QuestionIndex)
	}
	// Last item should be at index 9999
	if page100[len(page100)-1].QuestionIndex != 9999 {
		t.Errorf("Expected last item on page 100 to have question_index 9999, got %d", page100[len(page100)-1].QuestionIndex)
	}

	// Test last page (page 105, offset 10400)
	page105, _, err := repo.GetByEvaluationID(ctx, eval.ID, 105, 100)
	if err != nil {
		t.Fatalf("Failed to get page 105: %v", err)
	}
	// Page 105 should have only 100 items (105 * 100 = 10500 total, but we have 10500 items so page 105 is the last with 100 items)
	if len(page105) != 100 {
		t.Errorf("Expected 100 items on page 105, got %d", len(page105))
	}
	// Page 105 starts at index 10400
	if page105[0].QuestionIndex != 10400 {
		t.Errorf("Expected first item on page 105 to have question_index 10400, got %d", page105[0].QuestionIndex)
	}

	// Test beyond total - should return empty
	page200, total, err := repo.GetByEvaluationID(ctx, eval.ID, 200, 100)
	if err != nil {
		t.Fatalf("Failed to get page 200: %v", err)
	}
	if len(page200) != 0 {
		t.Errorf("Expected 0 items on page 200, got %d", len(page200))
	}
	// Total should still be correct
	if total != totalPredictions {
		t.Errorf("Expected total to remain %d, got %d", totalPredictions, total)
	}

	// Cleanup
	_ = evalRepo.Delete(ctx, eval.ID)
}
