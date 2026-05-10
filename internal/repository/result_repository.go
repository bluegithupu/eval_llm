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

// Result represents evaluation results with database fields
type Result struct {
	ID           string
	EvaluationID string
	DatasetID    string
	Accuracy     float64
	SampleCount  int
	CorrectCount int
	Metrics      map[string]any
	Summary      string
	CreatedAt    time.Time
}

// ResultRepository interface for result data access
type ResultRepository interface {
	Create(ctx context.Context, result *Result) error
	GetByEvaluationID(ctx context.Context, evaluationID string) ([]*Result, error)
	GetByEvaluationAndDataset(ctx context.Context, evaluationID, datasetID string) (*Result, error)
	QueryByMetrics(ctx context.Context, key, value string) ([]*Result, error)
}

// PostgresResultRepository implements ResultRepository using PostgreSQL
type PostgresResultRepository struct {
	db *pgxpool.Pool
}

// NewResultRepository creates a new result repository
func NewResultRepository(db *pgxpool.Pool) ResultRepository {
	return &PostgresResultRepository{db: db}
}

// Create inserts a new result into the database with JSONB metrics
func (r *PostgresResultRepository) Create(ctx context.Context, result *Result) error {
	query := `
		INSERT INTO results (evaluation_id, dataset_id, accuracy, sample_count, correct_count, metrics, summary)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, created_at
	`

	// Convert metrics to JSONB
	metricsJSON, err := json.Marshal(result.Metrics)
	if err != nil {
		return fmt.Errorf("failed to marshal metrics: %w", err)
	}

	var returnedID uuid.UUID
	var createdAt time.Time

	err = r.db.QueryRow(ctx, query,
		uuid.MustParse(result.EvaluationID),
		uuid.MustParse(result.DatasetID),
		result.Accuracy,
		result.SampleCount,
		result.CorrectCount,
		metricsJSON,
		result.Summary,
	).Scan(&returnedID, &createdAt)

	if err != nil {
		return fmt.Errorf("failed to create result: %w", err)
	}

	result.ID = returnedID.String()
	result.CreatedAt = createdAt

	return nil
}

// GetByEvaluationID retrieves all results for an evaluation
func (r *PostgresResultRepository) GetByEvaluationID(ctx context.Context, evaluationID string) ([]*Result, error) {
	query := `
		SELECT id, evaluation_id, dataset_id, accuracy, sample_count, correct_count, metrics, summary, created_at
		FROM results
		WHERE evaluation_id = $1
		ORDER BY created_at DESC
	`

	rows, err := r.db.Query(ctx, query, uuid.MustParse(evaluationID))
	if err != nil {
		return nil, fmt.Errorf("failed to get results: %w", err)
	}
	defer rows.Close()

	var results []*Result
	for rows.Next() {
		var result Result
		var metricsJSON []byte

		err := rows.Scan(
			&result.ID,
			&result.EvaluationID,
			&result.DatasetID,
			&result.Accuracy,
			&result.SampleCount,
			&result.CorrectCount,
			&metricsJSON,
			&result.Summary,
			&result.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan result: %w", err)
		}

		// Parse metrics JSON
		if len(metricsJSON) > 0 {
			if err := json.Unmarshal(metricsJSON, &result.Metrics); err != nil {
				return nil, fmt.Errorf("failed to unmarshal metrics: %w", err)
			}
		}

		results = append(results, &result)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return results, nil
}

// GetByEvaluationAndDataset retrieves a result for a specific evaluation and dataset
func (r *PostgresResultRepository) GetByEvaluationAndDataset(ctx context.Context, evaluationID, datasetID string) (*Result, error) {
	query := `
		SELECT id, evaluation_id, dataset_id, accuracy, sample_count, correct_count, metrics, summary, created_at
		FROM results
		WHERE evaluation_id = $1 AND dataset_id = $2
	`

	var result Result
	var metricsJSON []byte

	err := r.db.QueryRow(ctx, query,
		uuid.MustParse(evaluationID),
		uuid.MustParse(datasetID),
	).Scan(
		&result.ID,
		&result.EvaluationID,
		&result.DatasetID,
		&result.Accuracy,
		&result.SampleCount,
		&result.CorrectCount,
		&metricsJSON,
		&result.Summary,
		&result.CreatedAt,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("result not found for evaluation %s and dataset %s", evaluationID, datasetID)
		}
		return nil, fmt.Errorf("failed to get result: %w", err)
	}

	// Parse metrics JSON
	if len(metricsJSON) > 0 {
		if err := json.Unmarshal(metricsJSON, &result.Metrics); err != nil {
			return nil, fmt.Errorf("failed to unmarshal metrics: %w", err)
		}
	}

	return &result, nil
}

// QueryByMetrics queries results where metrics JSONB contains a specific key-value pair
func (r *PostgresResultRepository) QueryByMetrics(ctx context.Context, key, value string) ([]*Result, error) {
	query := `
		SELECT id, evaluation_id, dataset_id, accuracy, sample_count, correct_count, metrics, summary, created_at
		FROM results
		WHERE metrics->>$1 = $2
		ORDER BY created_at DESC
	`

	rows, err := r.db.Query(ctx, query, key, value)
	if err != nil {
		return nil, fmt.Errorf("failed to query results by metrics: %w", err)
	}
	defer rows.Close()

	var results []*Result
	for rows.Next() {
		var result Result
		var metricsJSON []byte

		err := rows.Scan(
			&result.ID,
			&result.EvaluationID,
			&result.DatasetID,
			&result.Accuracy,
			&result.SampleCount,
			&result.CorrectCount,
			&metricsJSON,
			&result.Summary,
			&result.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan result: %w", err)
		}

		// Parse metrics JSON
		if len(metricsJSON) > 0 {
			if err := json.Unmarshal(metricsJSON, &result.Metrics); err != nil {
				return nil, fmt.Errorf("failed to unmarshal metrics: %w", err)
			}
		}

		results = append(results, &result)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return results, nil
}
