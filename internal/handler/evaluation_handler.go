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
	evalRepo       repository.EvaluationRepository
	cache          cache.StatusCache
	modelRepo      repository.ModelRepository
	datasetRepo    repository.DatasetRepository
	resultRepo     repository.ResultRepository
	predictionRepo repository.PredictionRepository
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
	resultRepo repository.ResultRepository,
	predictionRepo repository.PredictionRepository,
) *EvaluationHandler {
	return &EvaluationHandler{
		evalRepo:       evalRepo,
		cache:          cache,
		modelRepo:      modelRepo,
		datasetRepo:    datasetRepo,
		resultRepo:     resultRepo,
		predictionRepo: predictionRepo,
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

// GetEvaluationStatusResponse represents the response for getting evaluation status
type GetEvaluationStatusResponse struct {
	ID          string                 `json:"id"`
	Status      model.EvaluationStatus `json:"status"`
	Progress    int                    `json:"progress"`
	Error       string                 `json:"error,omitempty"`
	CancelledAt string                 `json:"cancelled_at,omitempty"`
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

// GetEvaluationStatus handles GET /api/v1/evaluations/:id/status
// Returns current status (pending/running/completed/failed/cancelled) and progress (0-100)
// Handles not found (404), invalid UUID format (400)
func (h *EvaluationHandler) GetEvaluationStatus(c *gin.Context) {
	ctx := c.Request.Context()

	// Get ID from path parameter
	id := c.Param("id")

	// Validate UUID format (VAL-API-014 - invalid UUID returns 400)
	if _, err := uuid.Parse(id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid evaluation ID format: must be a valid UUID",
		})
		return
	}

	// Get evaluation from repository (VAL-API-014)
	eval, err := h.evalRepo.GetByID(ctx, id)
	if err != nil {
		if err == repository.ErrNotFound {
			// Task not found
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

	// Build response based on status (VAL-API-014, VAL-API-015, VAL-API-016, VAL-API-017, VAL-API-018, VAL-API-019)
	// Compute progress: completed status always returns 100, otherwise use stored progress
	progress := eval.Progress
	if eval.Status == model.StatusCompleted {
		progress = 100
	}

	response := GetEvaluationStatusResponse{
		ID:       eval.ID,
		Status:   eval.Status,
		Progress: progress,
	}

	// Add error field for failed status (VAL-API-018)
	if eval.Status == model.StatusFailed && eval.ErrorMessage != "" {
		response.Error = eval.ErrorMessage
	}

	// Add cancelled_at for cancelled status (VAL-API-019)
	if eval.Status == model.StatusCancelled && eval.CompletedAt != nil {
		response.CancelledAt = eval.CompletedAt.Format("2006-01-02T15:04:05Z07:00")
	}

	c.JSON(http.StatusOK, response)
}

// GetResultsResponse represents the response for getting evaluation results
type GetResultsResponse struct {
	Results     []*EvaluationResultItem `json:"results"`
	Predictions *PredictionsPage        `json:"predictions,omitempty"`
	Page        int                     `json:"page"`
	Limit       int                     `json:"limit"`
	Total       int                     `json:"total"`
	Pages       int                     `json:"pages"`
}

// EvaluationResultItem represents a single result item in the results response
type EvaluationResultItem struct {
	DatasetID   string         `json:"dataset_id"`
	DatasetName string         `json:"dataset_name"`
	Accuracy    float64        `json:"accuracy"`
	SampleCount int            `json:"sample_count"`
	Metrics     map[string]any `json:"metrics,omitempty"`
	Summary     string         `json:"summary,omitempty"`
}

// PredictionsPage represents paginated predictions
type PredictionsPage struct {
	Predictions []*PredictionItem `json:"predictions"`
	Total       int               `json:"total"`
	Page        int               `json:"page"`
	Limit       int               `json:"limit"`
	Pages       int               `json:"pages"`
}

// PredictionItem represents a single prediction item
type PredictionItem struct {
	ID         int    `json:"id"`
	Question   string `json:"question"`
	Prediction string `json:"prediction"`
	Answer     string `json:"answer"`
	Correct    bool   `json:"correct"`
}

// GetResults handles GET /api/v1/evaluations/:id/results
// Returns evaluation results for completed tasks
// Handles not ready (409/425 for pending/running), not found (404)
// Supports pagination for large prediction sets
func (h *EvaluationHandler) GetResults(c *gin.Context) {
	ctx := c.Request.Context()

	// Get ID from path parameter
	id := c.Param("id")

	// Validate UUID format
	if _, err := uuid.Parse(id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid evaluation ID format: must be a valid UUID",
		})
		return
	}

	// Get evaluation from repository
	eval, err := h.evalRepo.GetByID(ctx, id)
	if err != nil {
		if err == repository.ErrNotFound {
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

	// Check if evaluation is completed (VAL-API-021: 409/425 for pending/running)
	if eval.Status != model.StatusCompleted {
		c.JSON(http.StatusConflict, gin.H{
			"error": "results not available: evaluation is " + string(eval.Status),
		})
		return
	}

	// Get results for this evaluation
	results, err := h.resultRepo.GetByEvaluationID(ctx, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to retrieve results",
		})
		return
	}

	// Convert results to response items
	resultItems := make([]*EvaluationResultItem, 0, len(results))
	for _, result := range results {
		// Get dataset name if available
		datasetName := ""
		if len(eval.DatasetIDs) > 0 {
			if datasetEntity, err := h.datasetRepo.GetByID(ctx, result.DatasetID); err == nil {
				datasetName = datasetEntity.Name
			}
		}

		item := &EvaluationResultItem{
			DatasetID:   result.DatasetID,
			DatasetName: datasetName,
			Accuracy:    result.Accuracy,
			SampleCount: result.SampleCount,
			Metrics:     result.Metrics,
			Summary:     result.Summary,
		}
		resultItems = append(resultItems, item)
	}

	// Parse pagination parameters for predictions (default: page=1, limit=100)
	page := 1
	limit := 100

	if pageStr := c.Query("predictions_page"); pageStr != "" {
		var parsedPage int
		if _, err := fmt.Sscanf(pageStr, "%d", &parsedPage); err != nil || parsedPage < 1 {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "invalid predictions_page parameter: must be a positive integer",
			})
			return
		}
		page = parsedPage
	}

	if limitStr := c.Query("predictions_limit"); limitStr != "" {
		var parsedLimit int
		if _, err := fmt.Sscanf(limitStr, "%d", &parsedLimit); err != nil || parsedLimit < 1 {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "invalid predictions_limit parameter: must be a positive integer",
			})
			return
		}
		limit = parsedLimit
	}

	// Get predictions for this evaluation with pagination
	predictions, total, err := h.predictionRepo.GetByEvaluationID(ctx, id, page, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to retrieve predictions",
		})
		return
	}

	// Convert predictions to response items
	predictionItems := make([]*PredictionItem, 0, len(predictions))
	for _, pred := range predictions {
		item := &PredictionItem{
			ID:         pred.QuestionIndex,
			Question:   pred.Question,
			Prediction: pred.Prediction,
			Answer:     pred.Answer,
			Correct:    pred.Correct,
		}
		predictionItems = append(predictionItems, item)
	}

	// Calculate total pages for predictions
	predPages := 0
	if total > 0 && limit > 0 {
		predPages = (total + limit - 1) / limit // ceil(total / limit)
	}

	// Build response
	response := GetResultsResponse{
		Results: resultItems,
	}

	// Include predictions only if there are any
	if total > 0 {
		response.Predictions = &PredictionsPage{
			Predictions: predictionItems,
			Total:       total,
			Page:        page,
			Limit:       limit,
			Pages:       predPages,
		}
	}

	c.JSON(http.StatusOK, response)
}

// CancelEvaluationResponse represents the response for cancelling an evaluation
type CancelEvaluationResponse struct {
	ID     string                 `json:"id"`
	Status model.EvaluationStatus `json:"status"`
}

// CancelEvaluation handles DELETE /api/v1/evaluations/:id
// Cancels a pending or running evaluation task
// Returns 200/204 on success, 409 for completed tasks, 404 for not found, 400 for invalid UUID
func (h *EvaluationHandler) CancelEvaluation(c *gin.Context) {
	ctx := c.Request.Context()

	// Get ID from path parameter
	id := c.Param("id")

	// Validate UUID format (VAL-API-023 - invalid UUID returns 400)
	if _, err := uuid.Parse(id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid evaluation ID format: must be a valid UUID",
		})
		return
	}

	// Get evaluation from repository
	eval, err := h.evalRepo.GetByID(ctx, id)
	if err != nil {
		if err == repository.ErrNotFound {
			// Task not found (VAL-API-023 - 404 case)
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

	// Check if task is already completed (VAL-API-024: 409 for completed tasks)
	if eval.Status == model.StatusCompleted {
		c.JSON(http.StatusConflict, gin.H{
			"error": "cannot cancel completed evaluation",
		})
		return
	}

	// Check if task is already cancelled (VAL-API-023 - idempotent: return success)
	if eval.Status == model.StatusCancelled {
		// Return success for idempotent behavior
		c.JSON(http.StatusOK, CancelEvaluationResponse{
			ID:     eval.ID,
			Status: model.StatusCancelled,
		})
		return
	}

	// Cancel the evaluation by updating status to cancelled
	if err := h.evalRepo.Cancel(ctx, id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to cancel evaluation",
		})
		return
	}

	// Update Redis status to cancelled
	if err := h.cache.SetStatus(ctx, id, string(model.StatusCancelled)); err != nil {
		// Log error but don't fail - DB is authoritative
	}

	// Return success (VAL-API-023: 200/204 for pending/running)
	c.JSON(http.StatusOK, CancelEvaluationResponse{
		ID:     id,
		Status: model.StatusCancelled,
	})
}
