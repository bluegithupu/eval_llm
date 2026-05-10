package repository

import (
	"context"
	"fmt"

	"github.com/eval_llm/backend/internal/model"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// DatasetRepository interface for dataset data access
type DatasetRepository interface {
	GetByID(ctx context.Context, id string) (*model.Dataset, error)
	GetByName(ctx context.Context, name string) (*model.Dataset, error)
	List(ctx context.Context) ([]*model.Dataset, error)
}

// PostgresDatasetRepository implements DatasetRepository using PostgreSQL
type PostgresDatasetRepository struct {
	db *pgxpool.Pool
}

// NewDatasetRepository creates a new dataset repository
func NewDatasetRepository(db *pgxpool.Pool) DatasetRepository {
	return &PostgresDatasetRepository{db: db}
}

// GetByID retrieves a dataset by its ID
func (r *PostgresDatasetRepository) GetByID(ctx context.Context, id string) (*model.Dataset, error) {
	query := `
		SELECT id, name, display_name, description, config_template, created_at, updated_at
		FROM datasets
		WHERE id = $1
	`

	var d model.Dataset
	var description *string
	var configJSON []byte

	err := r.db.QueryRow(ctx, query, id).Scan(
		&d.ID,
		&d.Name,
		&d.DisplayName,
		&description,
		&configJSON,
		&d.CreatedAt,
		&d.UpdatedAt,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("failed to get dataset: %w", err)
	}

	if description != nil {
		d.Description = *description
	}

	return &d, nil
}

// GetByName retrieves a dataset by its name
func (r *PostgresDatasetRepository) GetByName(ctx context.Context, name string) (*model.Dataset, error) {
	query := `
		SELECT id, name, display_name, description, config_template, created_at, updated_at
		FROM datasets
		WHERE name = $1
	`

	var d model.Dataset
	var description *string
	var configJSON []byte

	err := r.db.QueryRow(ctx, query, name).Scan(
		&d.ID,
		&d.Name,
		&d.DisplayName,
		&description,
		&configJSON,
		&d.CreatedAt,
		&d.UpdatedAt,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("failed to get dataset by name: %w", err)
	}

	if description != nil {
		d.Description = *description
	}

	return &d, nil
}

// List retrieves all datasets
func (r *PostgresDatasetRepository) List(ctx context.Context) ([]*model.Dataset, error) {
	query := `
		SELECT id, name, display_name, description, config_template, created_at, updated_at
		FROM datasets
		ORDER BY name
	`

	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to list datasets: %w", err)
	}
	defer rows.Close()

	var datasets []*model.Dataset
	for rows.Next() {
		var d model.Dataset
		var description *string
		var configJSON []byte

		err := rows.Scan(
			&d.ID,
			&d.Name,
			&d.DisplayName,
			&description,
			&configJSON,
			&d.CreatedAt,
			&d.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan dataset: %w", err)
		}

		if description != nil {
			d.Description = *description
		}

		datasets = append(datasets, &d)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return datasets, nil
}
