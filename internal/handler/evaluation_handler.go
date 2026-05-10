package handler

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/eval_llm/backend/internal/cache"
	"github.com/eval_llm/backend/internal/model"
	"github.com/eval_llm/backend/internal/repository"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// CreateEvaluationRequest represents the request body for creating an evaluation
type CreateEvaluationRequest struct {
	Model   string `json:"model" binding:"required"`
	Dataset string `json:"dataset" binding:"required"`
}

// CreateEvaluationResponse represents the response for a created evaluation
type CreateEvaluationResponse struct {
	TaskID  string                 `json:"task_id"`
	Status  model.EvaluationStatus `json:"status"`
	Model   string                 `json:"model"`
	Dataset string                 `json:"dataset"`
}

// EvaluationHandler handles evaluation-related HTTP requests
type EvaluationHandler struct {
	evalRepo    repository.EvaluationRepository
	cache       cache.StatusCache
	modelRepo   repository.ModelRepository
	datasetRepo repository.DatasetRepository
}

// NewEvaluationHandler creates a new evaluation handler
func NewEvaluationHandler(
	evalRepo repository.EvaluationRepository,
	cache cache.StatusCache,
	modelRepo repository.ModelRepository,
	datasetRepo repository.DatasetRepository,
) *EvaluationHandler {
	return &EvaluationHandler{
		evalRepo:    evalRepo,
		cache:       cache,
		modelRepo:   modelRepo,
		datasetRepo: datasetRepo,
	}
}

// CreateEvaluation handles POST /api/v1/evaluations
// Validates request, creates DB record with status=pending, sets Redis status,
// returns 202 Accepted with task_id (UUID v4) and Location header
func (h *EvaluationHandler) CreateEvaluation(c *gin.Context) {
	ctx := c.Request.Context()

	var req CreateEvaluationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// Check if it's a validation error for required fields
		// Gin returns error like "Key: 'CreateEvaluationRequest.Model' Error:Field validation for 'Model' failed on the 'required' tag"
		errMsg := err.Error()
		if strings.Contains(errMsg, "Model") {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "model is required",
			})
			return
		}
		if strings.Contains(errMsg, "Dataset") {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "dataset is required",
			})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid request body",
		})
		return
	}

	// Validate required fields (VAL-API-003, VAL-API-004)
	if req.Model == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "model is required",
		})
		return
	}

	if req.Dataset == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "dataset is required",
		})
		return
	}

	// Validate model exists in supported list (VAL-API-005)
	modelEntity, err := h.modelRepo.GetByName(ctx, req.Model)
	if err != nil {
		if err == repository.ErrNotFound {
			// Get list of valid models
			validModels, listErr := h.modelRepo.List(ctx)
			if listErr != nil {
				c.JSON(http.StatusInternalServerError, gin.H{
					"error": "failed to retrieve valid models",
				})
				return
			}

			modelNames := make([]string, len(validModels))
			for i, m := range validModels {
				modelNames[i] = m.Name
			}

			c.JSON(http.StatusBadRequest, gin.H{
				"error":        fmt.Sprintf("invalid model: %s", req.Model),
				"valid_models": modelNames,
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to validate model",
		})
		return
	}

	// Validate dataset exists in supported list (VAL-API-006)
	datasetEntity, err := h.datasetRepo.GetByName(ctx, req.Dataset)
	if err != nil {
		if err == repository.ErrNotFound {
			// Get list of valid datasets
			validDatasets, listErr := h.datasetRepo.List(ctx)
			if listErr != nil {
				c.JSON(http.StatusInternalServerError, gin.H{
					"error": "failed to retrieve valid datasets",
				})
				return
			}

			datasetNames := make([]string, len(validDatasets))
			for i, d := range validDatasets {
				datasetNames[i] = d.Name
			}

			c.JSON(http.StatusBadRequest, gin.H{
				"error":          fmt.Sprintf("invalid dataset: %s", req.Dataset),
				"valid_datasets": datasetNames,
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to validate dataset",
		})
		return
	}

	// Create evaluation record with UUID v4 (VAL-API-007)
	taskID := uuid.New().String()
	evaluation := &model.Evaluation{
		ID:         taskID,
		ModelID:    modelEntity.ID,
		DatasetIDs: []string{datasetEntity.ID},
		Config:     map[string]any{},
		Status:     model.StatusPending,
		Progress:   0,
	}

	// Create DB record with status=pending (VAL-DB-005)
	if err := h.evalRepo.Create(ctx, evaluation); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to create evaluation",
		})
		return
	}

	// Set Redis status to pending
	if err := h.cache.SetStatus(ctx, taskID, "pending"); err != nil {
		// Log the error but don't fail - DB is authoritative
		// In production, you'd want to log this properly
	}

	// Build response (VAL-API-001, VAL-API-002)
	response := CreateEvaluationResponse{
		TaskID:  taskID,
		Status:  model.StatusPending,
		Model:   req.Model,
		Dataset: req.Dataset,
	}

	// Set Location header with task URL
	location := fmt.Sprintf("/api/v1/evaluations/%s", taskID)
	c.Header("Location", location)

	// Return 202 Accepted (not 201 Created) - async processing (VAL-API-002)
	c.JSON(http.StatusAccepted, response)
}
