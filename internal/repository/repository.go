package repository

import (
	"context"

	"github.com/eval_llm/backend/internal/model"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository interfaces will be implemented in future features
// This placeholder imports pgx to ensure dependency is tracked

// EvaluationRepository interface for evaluation data access
type EvaluationRepository interface {
	Create(ctx context.Context, eval *model.Evaluation) error
	GetByID(ctx context.Context, id string) (*model.Evaluation, error)
}

// PostgresEvaluationRepository placeholder implementation
type PostgresEvaluationRepository struct {
	db *pgxpool.Pool
}

// Ensure pgx types are referenced for go mod
var _ pgx.Row
