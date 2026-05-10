package model

import (
	"time"

	"github.com/google/uuid"
)

// EvaluationStatus represents the status of an evaluation task
type EvaluationStatus string

const (
	StatusPending   EvaluationStatus = "pending"
	StatusRunning   EvaluationStatus = "running"
	StatusCompleted EvaluationStatus = "completed"
	StatusFailed    EvaluationStatus = "failed"
	StatusCancelled EvaluationStatus = "cancelled"
)

// Evaluation represents an evaluation task
type Evaluation struct {
	ID           string           `json:"id"`
	ModelID      string           `json:"model_id"`
	DatasetIDs   []string         `json:"dataset_ids"`
	Config       map[string]any   `json:"config"`
	Status       EvaluationStatus `json:"status"`
	Progress     int              `json:"progress"`
	Version      int              `json:"version"` // Optimistic lock version
	ErrorMessage string           `json:"error_message,omitempty"`
	CreatedAt    time.Time        `json:"created_at"`
	UpdatedAt    time.Time        `json:"updated_at"`
	StartedAt    *time.Time       `json:"started_at,omitempty"`
	CompletedAt  *time.Time       `json:"completed_at,omitempty"`
}

// Model represents a supported model
type Model struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Type      string    `json:"type"`     // "api" or "local"
	Provider  string    `json:"provider"` // "openai", "anthropic", "dashscope"
	APIKeyRef string    `json:"api_key_ref,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Dataset represents a supported dataset
type Dataset struct {
	ID             string    `json:"id"`
	Name           string    `json:"name"`
	DisplayName    string    `json:"display_name"`
	Description    string    `json:"description"`
	ConfigTemplate string    `json:"config_template"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// Result represents evaluation results
type Result struct {
	EvaluationID string         `json:"evaluation_id"`
	Accuracy     float64        `json:"accuracy"`
	Metrics      map[string]any `json:"metrics"`
	Summary      string         `json:"summary"`
}

// NewEvaluation creates a new evaluation with generated UUID
func NewEvaluation(modelID string, datasetIDs []string) *Evaluation {
	return &Evaluation{
		ID:         uuid.New().String(),
		ModelID:    modelID,
		DatasetIDs: datasetIDs,
		Status:     StatusPending,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
}
