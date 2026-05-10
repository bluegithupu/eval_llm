package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/eval_llm/backend/internal/model"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// EvaluationRepository interface for evaluation data access
type EvaluationRepository interface {
	Create(ctx context.Context, eval *model.Evaluation) error
	GetByID(ctx context.Context, id string) (*model.Evaluation, error)
	List(ctx context.Context, page, limit int) ([]*model.Evaluation, int, error)
	UpdateStatus(ctx context.Context, id string, status model.EvaluationStatus, progress int) error
	Delete(ctx context.Context, id string) error
	Count(ctx context.Context) (int, error)
	CountByStatus(ctx context.Context, status model.EvaluationStatus) (int, error)
}

// PostgresEvaluationRepository implements EvaluationRepository using PostgreSQL
type PostgresEvaluationRepository struct {
	db *pgxpool.Pool
}

// NewEvaluationRepository creates a new evaluation repository
func NewEvaluationRepository(db *pgxpool.Pool) EvaluationRepository {
	return &PostgresEvaluationRepository{db: db}
}

// Create inserts a new evaluation into the database
// The database will auto-populate created_at and id via defaults
func (r *PostgresEvaluationRepository) Create(ctx context.Context, eval *model.Evaluation) error {
	query := `
		INSERT INTO evaluations (id, model_id, dataset_ids, config, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, created_at, updated_at
	`

	// Convert dataset_ids to PostgreSQL array format
	datasetIDs := make([]uuid.UUID, len(eval.DatasetIDs))
	for i, id := range eval.DatasetIDs {
		datasetIDs[i] = uuid.MustParse(id)
	}

	// Convert config to JSONB
	configJSON, err := json.Marshal(eval.Config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	var returnedID uuid.UUID
	var createdAt, updatedAt time.Time

	err = r.db.QueryRow(ctx, query,
		uuid.MustParse(eval.ID),
		uuid.MustParse(eval.ModelID),
		datasetIDs,
		configJSON,
		eval.Status,
		eval.CreatedAt,
		eval.UpdatedAt,
	).Scan(&returnedID, &createdAt, &updatedAt)

	if err != nil {
		return fmt.Errorf("failed to create evaluation: %w", err)
	}

	// Update eval with returned values
	eval.ID = returnedID.String()
	eval.CreatedAt = createdAt
	eval.UpdatedAt = updatedAt

	return nil
}

// GetByID retrieves an evaluation by its ID
func (r *PostgresEvaluationRepository) GetByID(ctx context.Context, id string) (*model.Evaluation, error) {
	query := `
		SELECT id, model_id, dataset_ids, config, status, progress, error_message,
		       created_at, updated_at, started_at, completed_at
		FROM evaluations
		WHERE id = $1
	`

	var eval model.Evaluation
	var datasetIDs []uuid.UUID
	var configJSON []byte
	var startedAt, completedAt *time.Time
	var errorMessage *string

	err := r.db.QueryRow(ctx, query, uuid.MustParse(id)).Scan(
		&eval.ID,
		&eval.ModelID,
		&datasetIDs,
		&configJSON,
		&eval.Status,
		&eval.Progress,
		&errorMessage,
		&eval.CreatedAt,
		&eval.UpdatedAt,
		&startedAt,
		&completedAt,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("evaluation not found: %s", id)
		}
		return nil, fmt.Errorf("failed to get evaluation: %w", err)
	}

	// Convert dataset IDs
	eval.DatasetIDs = make([]string, len(datasetIDs))
	for i, id := range datasetIDs {
		eval.DatasetIDs[i] = id.String()
	}

	// Parse config JSON
	if len(configJSON) > 0 {
		if err := json.Unmarshal(configJSON, &eval.Config); err != nil {
			return nil, fmt.Errorf("failed to unmarshal config: %w", err)
		}
	}

	// Set nullable timestamps
	eval.StartedAt = startedAt
	eval.CompletedAt = completedAt
	if errorMessage != nil {
		eval.ErrorMessage = *errorMessage
	}

	return &eval, nil
}

// List retrieves evaluations with pagination, ordered by created_at DESC
func (r *PostgresEvaluationRepository) List(ctx context.Context, page, limit int) ([]*model.Evaluation, int, error) {
	// Calculate offset
	offset := (page - 1) * limit
	if offset < 0 {
		offset = 0
	}
	if limit < 1 {
		limit = 10
	}

	query := `
		SELECT id, model_id, dataset_ids, config, status, progress, error_message,
		       created_at, updated_at, started_at, completed_at
		FROM evaluations
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2
	`

	rows, err := r.db.Query(ctx, query, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list evaluations: %w", err)
	}
	defer rows.Close()

	var evaluations []*model.Evaluation
	for rows.Next() {
		var eval model.Evaluation
		var datasetIDs []uuid.UUID
		var configJSON []byte
		var startedAt, completedAt *time.Time
		var errorMessage *string

		err := rows.Scan(
			&eval.ID,
			&eval.ModelID,
			&datasetIDs,
			&configJSON,
			&eval.Status,
			&eval.Progress,
			&errorMessage,
			&eval.CreatedAt,
			&eval.UpdatedAt,
			&startedAt,
			&completedAt,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to scan evaluation: %w", err)
		}

		// Convert dataset IDs
		eval.DatasetIDs = make([]string, len(datasetIDs))
		for i, id := range datasetIDs {
			eval.DatasetIDs[i] = id.String()
		}

		// Parse config JSON
		if len(configJSON) > 0 {
			if err := json.Unmarshal(configJSON, &eval.Config); err != nil {
				return nil, 0, fmt.Errorf("failed to unmarshal config: %w", err)
			}
		}

		// Set nullable timestamps
		eval.StartedAt = startedAt
		eval.CompletedAt = completedAt
		if errorMessage != nil {
			eval.ErrorMessage = *errorMessage
		}

		evaluations = append(evaluations, &eval)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("error iterating rows: %w", err)
	}

	// Get total count
	total, err := r.Count(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count evaluations: %w", err)
	}

	return evaluations, total, nil
}

// UpdateStatus updates the status and progress of an evaluation
// The database trigger will automatically set started_at and completed_at
func (r *PostgresEvaluationRepository) UpdateStatus(ctx context.Context, id string, status model.EvaluationStatus, progress int) error {
	query := `
		UPDATE evaluations
		SET status = $1, progress = $2, updated_at = CURRENT_TIMESTAMP
		WHERE id = $3
	`

	result, err := r.db.Exec(ctx, query, status, progress, uuid.MustParse(id))
	if err != nil {
		return fmt.Errorf("failed to update status: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("evaluation not found: %s", id)
	}

	return nil
}

// Delete deletes an evaluation by ID
func (r *PostgresEvaluationRepository) Delete(ctx context.Context, id string) error {
	query := `DELETE FROM evaluations WHERE id = $1`

	result, err := r.db.Exec(ctx, query, uuid.MustParse(id))
	if err != nil {
		return fmt.Errorf("failed to delete evaluation: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("evaluation not found: %s", id)
	}

	return nil
}

// Count returns the total number of evaluations
func (r *PostgresEvaluationRepository) Count(ctx context.Context) (int, error) {
	query := `SELECT COUNT(*) FROM evaluations`

	var count int
	err := r.db.QueryRow(ctx, query).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count evaluations: %w", err)
	}

	return count, nil
}

// CountByStatus returns the count of evaluations with a specific status
func (r *PostgresEvaluationRepository) CountByStatus(ctx context.Context, status model.EvaluationStatus) (int, error) {
	query := `SELECT COUNT(*) FROM evaluations WHERE status = $1`

	var count int
	err := r.db.QueryRow(ctx, query, status).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count evaluations by status: %w", err)
	}

	return count, nil
}
