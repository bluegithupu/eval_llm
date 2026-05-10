package repository

import (
	"context"
	"fmt"

	"github.com/eval_llm/backend/internal/model"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ModelRepository interface for model data access
type ModelRepository interface {
	GetByID(ctx context.Context, id string) (*model.Model, error)
	GetByName(ctx context.Context, name string) (*model.Model, error)
	List(ctx context.Context) ([]*model.Model, error)
}

// PostgresModelRepository implements ModelRepository using PostgreSQL
type PostgresModelRepository struct {
	db *pgxpool.Pool
}

// NewModelRepository creates a new model repository
func NewModelRepository(db *pgxpool.Pool) ModelRepository {
	return &PostgresModelRepository{db: db}
}

// GetByID retrieves a model by its ID
func (r *PostgresModelRepository) GetByID(ctx context.Context, id string) (*model.Model, error) {
	query := `
		SELECT id, name, type, provider, api_key_ref, config, created_at, updated_at
		FROM models
		WHERE id = $1
	`

	var m model.Model
	var apiKeyRef *string
	var configJSON []byte

	err := r.db.QueryRow(ctx, query, id).Scan(
		&m.ID,
		&m.Name,
		&m.Type,
		&m.Provider,
		&apiKeyRef,
		&configJSON,
		&m.CreatedAt,
		&m.UpdatedAt,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("failed to get model: %w", err)
	}

	if apiKeyRef != nil {
		m.APIKeyRef = *apiKeyRef
	}

	return &m, nil
}

// GetByName retrieves a model by its name
func (r *PostgresModelRepository) GetByName(ctx context.Context, name string) (*model.Model, error) {
	query := `
		SELECT id, name, type, provider, api_key_ref, config, created_at, updated_at
		FROM models
		WHERE name = $1
	`

	var m model.Model
	var apiKeyRef *string
	var configJSON []byte

	err := r.db.QueryRow(ctx, query, name).Scan(
		&m.ID,
		&m.Name,
		&m.Type,
		&m.Provider,
		&apiKeyRef,
		&configJSON,
		&m.CreatedAt,
		&m.UpdatedAt,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("failed to get model by name: %w", err)
	}

	if apiKeyRef != nil {
		m.APIKeyRef = *apiKeyRef
	}

	return &m, nil
}

// List retrieves all models
func (r *PostgresModelRepository) List(ctx context.Context) ([]*model.Model, error) {
	query := `
		SELECT id, name, type, provider, api_key_ref, config, created_at, updated_at
		FROM models
		ORDER BY name
	`

	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to list models: %w", err)
	}
	defer rows.Close()

	var models []*model.Model
	for rows.Next() {
		var m model.Model
		var apiKeyRef *string
		var configJSON []byte

		err := rows.Scan(
			&m.ID,
			&m.Name,
			&m.Type,
			&m.Provider,
			&apiKeyRef,
			&configJSON,
			&m.CreatedAt,
			&m.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan model: %w", err)
		}

		if apiKeyRef != nil {
			m.APIKeyRef = *apiKeyRef
		}

		models = append(models, &m)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return models, nil
}
