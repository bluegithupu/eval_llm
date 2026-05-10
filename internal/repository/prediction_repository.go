package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Prediction represents a per-sample prediction from evaluation
type Prediction struct {
	ID            string
	EvaluationID  string
	DatasetID     string
	QuestionIndex int
	Question      string
	Prediction    string
	Answer        string
	Correct       bool
	Reasoning     string
	Metadata      map[string]any
	CreatedAt     time.Time
}

// PredictionRepository interface for prediction data access
type PredictionRepository interface {
	Create(ctx context.Context, prediction *Prediction) error
	BatchInsert(ctx context.Context, predictions []*Prediction) error
	GetByEvaluationID(ctx context.Context, evaluationID string, page, limit int) ([]*Prediction, int, error)
	CountByEvaluationID(ctx context.Context, evaluationID string) (int, error)
}

// PostgresPredictionRepository implements PredictionRepository using PostgreSQL
type PostgresPredictionRepository struct {
	db *pgxpool.Pool
}

// NewPredictionRepository creates a new prediction repository
func NewPredictionRepository(db *pgxpool.Pool) PredictionRepository {
	return &PostgresPredictionRepository{db: db}
}

// Create inserts a single prediction into the database
func (r *PostgresPredictionRepository) Create(ctx context.Context, prediction *Prediction) error {
	query := `
		INSERT INTO predictions (evaluation_id, dataset_id, question_index, question, prediction, answer, correct, reasoning, metadata)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id, created_at
	`

	// Convert metadata to JSONB
	metadataJSON, err := json.Marshal(prediction.Metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	var returnedID uuid.UUID
	var createdAt time.Time

	err = r.db.QueryRow(ctx, query,
		uuid.MustParse(prediction.EvaluationID),
		uuid.MustParse(prediction.DatasetID),
		prediction.QuestionIndex,
		prediction.Question,
		prediction.Prediction,
		prediction.Answer,
		prediction.Correct,
		prediction.Reasoning,
		metadataJSON,
	).Scan(&returnedID, &createdAt)

	if err != nil {
		return fmt.Errorf("failed to create prediction: %w", err)
	}

	prediction.ID = returnedID.String()
	prediction.CreatedAt = createdAt

	return nil
}

// BatchInsert inserts multiple predictions in a single transaction
func (r *PostgresPredictionRepository) BatchInsert(ctx context.Context, predictions []*Prediction) error {
	if len(predictions) == 0 {
		return nil
	}

	// Start transaction
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	// Prepare batch insert
	query := `
		INSERT INTO predictions (evaluation_id, dataset_id, question_index, question, prediction, answer, correct, reasoning, metadata)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`

	batch := &pgx.Batch{}
	for _, pred := range predictions {
		metadataJSON, marshalErr := json.Marshal(pred.Metadata)
		if marshalErr != nil {
			tx.Rollback(ctx)
			return fmt.Errorf("failed to marshal metadata: %w", marshalErr)
		}

		batch.Queue(query,
			uuid.MustParse(pred.EvaluationID),
			uuid.MustParse(pred.DatasetID),
			pred.QuestionIndex,
			pred.Question,
			pred.Prediction,
			pred.Answer,
			pred.Correct,
			pred.Reasoning,
			metadataJSON,
		)
	}

	// Execute batch and close results before commit
	results := tx.SendBatch(ctx, batch)

	for i := 0; i < len(predictions); i++ {
		_, execErr := results.Exec()
		if execErr != nil {
			results.Close()
			tx.Rollback(ctx)
			return fmt.Errorf("failed to insert prediction %d: %w", i, execErr)
		}
	}

	// Close results before committing
	results.Close()

	// Commit transaction
	commitErr := tx.Commit(ctx)
	if commitErr != nil {
		return fmt.Errorf("failed to commit transaction: %w", commitErr)
	}

	return nil
}

// GetByEvaluationID retrieves predictions for an evaluation with pagination
func (r *PostgresPredictionRepository) GetByEvaluationID(ctx context.Context, evaluationID string, page, limit int) ([]*Prediction, int, error) {
	// Calculate offset
	offset := (page - 1) * limit
	if offset < 0 {
		offset = 0
	}
	if limit < 1 {
		limit = 100
	}

	query := `
		SELECT id, evaluation_id, dataset_id, question_index, question, prediction, answer, correct, reasoning, metadata, created_at
		FROM predictions
		WHERE evaluation_id = $1
		ORDER BY question_index ASC
		LIMIT $2 OFFSET $3
	`

	rows, err := r.db.Query(ctx, query,
		uuid.MustParse(evaluationID),
		limit,
		offset,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get predictions: %w", err)
	}
	defer rows.Close()

	var predictions []*Prediction
	for rows.Next() {
		var pred Prediction
		var metadataJSON []byte
		var reasoning *string

		err := rows.Scan(
			&pred.ID,
			&pred.EvaluationID,
			&pred.DatasetID,
			&pred.QuestionIndex,
			&pred.Question,
			&pred.Prediction,
			&pred.Answer,
			&pred.Correct,
			&reasoning,
			&metadataJSON,
			&pred.CreatedAt,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to scan prediction: %w", err)
		}

		if reasoning != nil {
			pred.Reasoning = *reasoning
		}

		// Parse metadata JSON
		if len(metadataJSON) > 0 {
			if err := json.Unmarshal(metadataJSON, &pred.Metadata); err != nil {
				return nil, 0, fmt.Errorf("failed to unmarshal metadata: %w", err)
			}
		}

		predictions = append(predictions, &pred)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("error iterating rows: %w", err)
	}

	// Get total count
	total, err := r.CountByEvaluationID(ctx, evaluationID)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count predictions: %w", err)
	}

	return predictions, total, nil
}

// CountByEvaluationID returns the count of predictions for an evaluation
func (r *PostgresPredictionRepository) CountByEvaluationID(ctx context.Context, evaluationID string) (int, error) {
	query := `SELECT COUNT(*) FROM predictions WHERE evaluation_id = $1`

	var count int
	err := r.db.QueryRow(ctx, query, uuid.MustParse(evaluationID)).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count predictions: %w", err)
	}

	return count, nil
}
