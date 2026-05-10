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

// ListEvaluationsResponse represents the response for listing evaluations
type ListEvaluationsResponse struct {
	Tasks []EvaluationListItem `json:"tasks"`
	Page  int                  `json:"page"`
	Limit int                  `json:"limit"`
	Total int                  `json:"total"`
	Pages int                  `json:"pages"`
}

// EvaluationListItem represents a single evaluation in the list response
type EvaluationListItem struct {
	ID        string                 `json:"id"`
	Model     string                 `json:"model"`
	Dataset   string                 `json:"dataset"`
	Status    model.EvaluationStatus `json:"status"`
	Progress  int                    `json:"progress"`
	CreatedAt string                 `json:"created_at"`
}

// GetEvaluationResponse represents the response for getting a single evaluation
type GetEvaluationResponse struct {
	ID        string                 `json:"id"`
	Model     string                 `json:"model"`
	Dataset   string                 `json:"dataset"`
	Status    model.EvaluationStatus `json:"status"`
	Config    map[string]any         `json:"config"`
	Progress  int                    `json:"progress"`
	CreatedAt string                 `json:"created_at"`
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

// ListEvaluations handles GET /api/v1/evaluations with pagination
// Supports page and limit query parameters (default: page=1, limit=10)
// Returns tasks array, total count, pages calculation
func (h *EvaluationHandler) ListEvaluations(c *gin.Context) {
	ctx := c.Request.Context()

	// Parse pagination parameters with defaults (VAL-API-008)
	page := 1
	limit := 10

	// Parse page parameter
	if pageStr := c.Query("page"); pageStr != "" {
		var parsedPage int
		if _, err := fmt.Sscanf(pageStr, "%d", &parsedPage); err != nil || parsedPage < 1 {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "invalid page parameter: must be a positive integer",
			})
			return
		}
		page = parsedPage
	}

	// Parse limit parameter
	if limitStr := c.Query("limit"); limitStr != "" {
		var parsedLimit int
		if _, err := fmt.Sscanf(limitStr, "%d", &parsedLimit); err != nil || parsedLimit < 1 {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "invalid limit parameter: must be a positive integer",
			})
			return
		}
		limit = parsedLimit
	}

	// Get evaluations with pagination from repository
	evaluations, total, err := h.evalRepo.List(ctx, page, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to retrieve evaluations",
		})
		return
	}

	// Calculate total pages (VAL-API-011)
	pages := 0
	if total > 0 && limit > 0 {
		pages = (total + limit - 1) / limit // ceil(total / limit)
	}

	// Convert evaluations to list items
	// For each evaluation, we need to get the model and dataset names
	tasks := make([]EvaluationListItem, 0, len(evaluations))
	for _, eval := range evaluations {
		// Get model name
		modelName := ""
		if eval.ModelID != "" {
			if modelEntity, err := h.modelRepo.GetByID(ctx, eval.ModelID); err == nil {
				modelName = modelEntity.Name
			}
		}

		// Get dataset name (first dataset ID)
		datasetName := ""
		if len(eval.DatasetIDs) > 0 && eval.DatasetIDs[0] != "" {
			if datasetEntity, err := h.datasetRepo.GetByID(ctx, eval.DatasetIDs[0]); err == nil {
				datasetName = datasetEntity.Name
			}
		}

		item := EvaluationListItem{
			ID:        eval.ID,
			Model:     modelName,
			Dataset:   datasetName,
			Status:    eval.Status,
			Progress:  eval.Progress,
			CreatedAt: eval.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		}
		tasks = append(tasks, item)
	}

	// Build response (VAL-API-008, VAL-API-009, VAL-API-010, VAL-API-011)
	response := ListEvaluationsResponse{
		Tasks: tasks,
		Page:  page,
		Limit: limit,
		Total: total,
		Pages: pages,
	}

	c.JSON(http.StatusOK, response)
}

// GetEvaluation handles GET /api/v1/evaluations/:id
// Returns task details including id, model, dataset, status, config, created_at
// Handles not found (404) and invalid UUID format (400)
func (h *EvaluationHandler) GetEvaluation(c *gin.Context) {
	ctx := c.Request.Context()

	// Get ID from path parameter
	id := c.Param("id")

	// Validate UUID format (VAL-API-013 - invalid UUID returns 400)
	if _, err := uuid.Parse(id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid evaluation ID format: must be a valid UUID",
		})
		return
	}

	// Get evaluation from repository (VAL-API-012)
	eval, err := h.evalRepo.GetByID(ctx, id)
	if err != nil {
		if err == repository.ErrNotFound {
			// Task not found (VAL-API-013)
			c.JSON(http.StatusNotFound, gin.H{
				"error": fmt.Sprintf("evaluation not found: %s", id),
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to retrieve evaluation",
		})
		return
	}

	// Get model name
	modelName := ""
	if eval.ModelID != "" {
		if modelEntity, err := h.modelRepo.GetByID(ctx, eval.ModelID); err == nil {
			modelName = modelEntity.Name
		}
	}

	// Get dataset name (first dataset ID)
	datasetName := ""
	if len(eval.DatasetIDs) > 0 && eval.DatasetIDs[0] != "" {
		if datasetEntity, err := h.datasetRepo.GetByID(ctx, eval.DatasetIDs[0]); err == nil {
			datasetName = datasetEntity.Name
		}
	}

	// Build response (VAL-API-012)
	response := GetEvaluationResponse{
		ID:        eval.ID,
		Model:     modelName,
		Dataset:   datasetName,
		Status:    eval.Status,
		Config:    eval.Config,
		Progress:  eval.Progress,
		CreatedAt: eval.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}

	c.JSON(http.StatusOK, response)
}
