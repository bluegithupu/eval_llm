package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/eval_llm/backend/internal/model"
	"github.com/eval_llm/backend/internal/repository"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockEvaluationRepository for testing
type MockEvaluationRepository struct {
	mock.Mock
}

func (m *MockEvaluationRepository) Create(ctx context.Context, eval *model.Evaluation) error {
	args := m.Called(ctx, eval)
	return args.Error(0)
}

func (m *MockEvaluationRepository) GetByID(ctx context.Context, id string) (*model.Evaluation, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.Evaluation), args.Error(1)
}

func (m *MockEvaluationRepository) List(ctx context.Context, page, limit int) ([]*model.Evaluation, int, error) {
	args := m.Called(ctx, page, limit)
	return args.Get(0).([]*model.Evaluation), args.Int(1), args.Error(2)
}

func (m *MockEvaluationRepository) UpdateStatus(ctx context.Context, id string, status model.EvaluationStatus, progress int) error {
	args := m.Called(ctx, id, status, progress)
	return args.Error(0)
}

func (m *MockEvaluationRepository) UpdateStatusWithError(ctx context.Context, id string, status model.EvaluationStatus, progress int, errorMsg string) error {
	args := m.Called(ctx, id, status, progress, errorMsg)
	return args.Error(0)
}

func (m *MockEvaluationRepository) Cancel(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockEvaluationRepository) Delete(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockEvaluationRepository) Count(ctx context.Context) (int, error) {
	args := m.Called(ctx)
	return args.Int(0), args.Error(1)
}

func (m *MockEvaluationRepository) CountByStatus(ctx context.Context, status model.EvaluationStatus) (int, error) {
	args := m.Called(ctx, status)
	return args.Int(0), args.Error(1)
}

// MockStatusCache for testing
type MockStatusCache struct {
	mock.Mock
}

func (m *MockStatusCache) SetStatus(ctx context.Context, evalID string, status string) error {
	args := m.Called(ctx, evalID, status)
	return args.Error(0)
}

func (m *MockStatusCache) GetStatus(ctx context.Context, evalID string) (string, error) {
	args := m.Called(ctx, evalID)
	return args.String(0), args.Error(1)
}

func (m *MockStatusCache) SetProgress(ctx context.Context, evalID string, progress int) error {
	args := m.Called(ctx, evalID, progress)
	return args.Error(0)
}

func (m *MockStatusCache) GetProgress(ctx context.Context, evalID string) (int, error) {
	args := m.Called(ctx, evalID)
	return args.Int(0), args.Error(1)
}

func (m *MockStatusCache) DeleteStatus(ctx context.Context, evalID string) error {
	args := m.Called(ctx, evalID)
	return args.Error(0)
}

func (m *MockStatusCache) DeleteProgress(ctx context.Context, evalID string) error {
	args := m.Called(ctx, evalID)
	return args.Error(0)
}

func (m *MockStatusCache) Ping(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockStatusCache) Close() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockStatusCache) IsAvailable(ctx context.Context) bool {
	args := m.Called(ctx)
	return args.Bool(0)
}

// MockModelRepository for testing
type MockModelRepository struct {
	mock.Mock
}

func (m *MockModelRepository) GetByID(ctx context.Context, id string) (*model.Model, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.Model), args.Error(1)
}

func (m *MockModelRepository) GetByName(ctx context.Context, name string) (*model.Model, error) {
	args := m.Called(ctx, name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.Model), args.Error(1)
}

func (m *MockModelRepository) List(ctx context.Context) ([]*model.Model, error) {
	args := m.Called(ctx)
	return args.Get(0).([]*model.Model), args.Error(1)
}

// MockDatasetRepository for testing
type MockDatasetRepository struct {
	mock.Mock
}

func (m *MockDatasetRepository) GetByID(ctx context.Context, id string) (*model.Dataset, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.Dataset), args.Error(1)
}

func (m *MockDatasetRepository) GetByName(ctx context.Context, name string) (*model.Dataset, error) {
	args := m.Called(ctx, name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.Dataset), args.Error(1)
}

func (m *MockDatasetRepository) List(ctx context.Context) ([]*model.Dataset, error) {
	args := m.Called(ctx)
	return args.Get(0).([]*model.Dataset), args.Error(1)
}

// MockResultRepository for testing
type MockResultRepository struct {
	mock.Mock
}

func (m *MockResultRepository) Create(ctx context.Context, result *repository.Result) error {
	args := m.Called(ctx, result)
	return args.Error(0)
}

func (m *MockResultRepository) GetByEvaluationID(ctx context.Context, evaluationID string) ([]*repository.Result, error) {
	args := m.Called(ctx, evaluationID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*repository.Result), args.Error(1)
}

func (m *MockResultRepository) GetByEvaluationAndDataset(ctx context.Context, evaluationID, datasetID string) (*repository.Result, error) {
	args := m.Called(ctx, evaluationID, datasetID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*repository.Result), args.Error(1)
}

func (m *MockResultRepository) QueryByMetrics(ctx context.Context, key, value string) ([]*repository.Result, error) {
	args := m.Called(ctx, key, value)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*repository.Result), args.Error(1)
}

// MockPredictionRepository for testing
type MockPredictionRepository struct {
	mock.Mock
}

func (m *MockPredictionRepository) Create(ctx context.Context, prediction *repository.Prediction) error {
	args := m.Called(ctx, prediction)
	return args.Error(0)
}

func (m *MockPredictionRepository) BatchInsert(ctx context.Context, predictions []*repository.Prediction) error {
	args := m.Called(ctx, predictions)
	return args.Error(0)
}

func (m *MockPredictionRepository) GetByEvaluationID(ctx context.Context, evaluationID string, page, limit int) ([]*repository.Prediction, int, error) {
	args := m.Called(ctx, evaluationID, page, limit)
	return args.Get(0).([]*repository.Prediction), args.Int(1), args.Error(2)
}

func (m *MockPredictionRepository) CountByEvaluationID(ctx context.Context, evaluationID string) (int, error) {
	args := m.Called(ctx, evaluationID)
	return args.Int(0), args.Error(1)
}

func setupEvalTestRouter(handler *EvaluationHandler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/api/v1/evaluations", handler.CreateEvaluation)
	router.GET("/api/v1/evaluations", handler.ListEvaluations)
	router.GET("/api/v1/evaluations/:id", handler.GetEvaluation)
	router.GET("/api/v1/evaluations/:id/status", handler.GetEvaluationStatus)
	router.GET("/api/v1/evaluations/:id/results", handler.GetResults)
	router.DELETE("/api/v1/evaluations/:id", handler.CancelEvaluation)
	return router
}

// TestCreateEvaluation_ValidRequest tests VAL-API-001, VAL-API-002, VAL-API-007
func TestCreateEvaluation_ValidRequest(t *testing.T) {
	router := setupEvalTestRouter(nil)
	mockEvalRepo := new(MockEvaluationRepository)
	mockCache := new(MockStatusCache)
	mockModelRepo := new(MockModelRepository)
	mockDatasetRepo := new(MockDatasetRepository)

	// Test data
	modelID := uuid.New().String()
	datasetID := uuid.New().String()
	expectedTaskID := uuid.New().String()

	// Mock expectations
	mockModelRepo.On("GetByName", mock.Anything, "gpt-4").Return(&model.Model{ID: modelID, Name: "gpt-4"}, nil)
	mockDatasetRepo.On("GetByName", mock.Anything, "mmlu").Return(&model.Dataset{ID: datasetID, Name: "mmlu"}, nil)
	mockEvalRepo.On("Create", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		eval := args.Get(1).(*model.Evaluation)
		eval.ID = expectedTaskID
	}).Return(nil)
	mockCache.On("SetStatus", mock.Anything, mock.Anything, "pending").Return(nil)

	handler := NewEvaluationHandler(mockEvalRepo, mockCache, mockModelRepo, mockDatasetRepo, nil, nil, nil)
	router = setupEvalTestRouter(handler)

	// Request body
	body := map[string]string{
		"model":   "gpt-4",
		"dataset": "mmlu",
	}
	jsonBody, _ := json.Marshal(body)

	// Execute request
	req := httptest.NewRequest(http.MethodPost, "/api/v1/evaluations", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Verify response
	assert.Equal(t, http.StatusAccepted, w.Code)

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	assert.Contains(t, response, "task_id")

	// Verify UUID v4 format (VAL-API-007)
	taskID := response["task_id"].(string)
	_, err := uuid.Parse(taskID)
	assert.NoError(t, err, "task_id should be valid UUID format")

	// Verify Location header
	location := w.Header().Get("Location")
	assert.Contains(t, location, taskID)

	// Verify status is pending
	assert.Equal(t, "pending", response["status"])

	mockModelRepo.AssertExpectations(t)
	mockDatasetRepo.AssertExpectations(t)
	mockEvalRepo.AssertExpectations(t)
	mockCache.AssertExpectations(t)
}

// TestCreateEvaluation_MissingModel tests VAL-API-003
func TestCreateEvaluation_MissingModel(t *testing.T) {
	mockEvalRepo := new(MockEvaluationRepository)
	mockCache := new(MockStatusCache)
	mockModelRepo := new(MockModelRepository)
	mockDatasetRepo := new(MockDatasetRepository)

	handler := NewEvaluationHandler(mockEvalRepo, mockCache, mockModelRepo, mockDatasetRepo, nil, nil, nil)
	router := setupEvalTestRouter(handler)

	// Request body without model
	body := map[string]string{
		"dataset": "mmlu",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/evaluations", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Verify 400 response with error message
	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	assert.Contains(t, response, "error")
	// Error message says "model is required"
	assert.Equal(t, "model is required", response["error"])
}

// TestCreateEvaluation_MissingDataset tests VAL-API-004
func TestCreateEvaluation_MissingDataset(t *testing.T) {
	mockEvalRepo := new(MockEvaluationRepository)
	mockCache := new(MockStatusCache)
	mockModelRepo := new(MockModelRepository)
	mockDatasetRepo := new(MockDatasetRepository)

	handler := NewEvaluationHandler(mockEvalRepo, mockCache, mockModelRepo, mockDatasetRepo, nil, nil, nil)
	router := setupEvalTestRouter(handler)

	// Request body without dataset
	body := map[string]string{
		"model": "gpt-4",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/evaluations", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Verify 400 response with error message
	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	assert.Contains(t, response, "error")
	// Error message says "dataset is required"
	assert.Equal(t, "dataset is required", response["error"])
}

// TestCreateEvaluation_InvalidModel tests VAL-API-005
func TestCreateEvaluation_InvalidModel(t *testing.T) {
	mockEvalRepo := new(MockEvaluationRepository)
	mockCache := new(MockStatusCache)
	mockModelRepo := new(MockModelRepository)
	mockDatasetRepo := new(MockDatasetRepository)

	// Mock expectations - model not found
	mockModelRepo.On("GetByName", mock.Anything, "invalid-model").Return(nil, repository.ErrNotFound)
	mockModelRepo.On("List", mock.Anything).Return([]*model.Model{
		{ID: "1", Name: "gpt-4"},
		{ID: "2", Name: "claude-3-opus"},
		{ID: "3", Name: "qwen-max"},
	}, nil)

	handler := NewEvaluationHandler(mockEvalRepo, mockCache, mockModelRepo, mockDatasetRepo, nil, nil, nil)
	router := setupEvalTestRouter(handler)

	// Request body with invalid model
	body := map[string]string{
		"model":   "invalid-model",
		"dataset": "mmlu",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/evaluations", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Verify 400 response with valid models list
	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	assert.Contains(t, response, "error")
	assert.Contains(t, response["error"].(string), "model")

	// Verify valid models list is included
	assert.Contains(t, response, "valid_models")
	validModels := response["valid_models"].([]interface{})
	assert.Contains(t, validModels, "gpt-4")
	assert.Contains(t, validModels, "claude-3-opus")
	assert.Contains(t, validModels, "qwen-max")

	mockModelRepo.AssertExpectations(t)
}

// TestCreateEvaluation_InvalidDataset tests VAL-API-006
func TestCreateEvaluation_InvalidDataset(t *testing.T) {
	mockEvalRepo := new(MockEvaluationRepository)
	mockCache := new(MockStatusCache)
	mockModelRepo := new(MockModelRepository)
	mockDatasetRepo := new(MockDatasetRepository)

	modelID := uuid.New().String()

	// Mock expectations
	mockModelRepo.On("GetByName", mock.Anything, "gpt-4").Return(&model.Model{ID: modelID, Name: "gpt-4"}, nil)
	mockDatasetRepo.On("GetByName", mock.Anything, "invalid-dataset").Return(nil, repository.ErrNotFound)
	mockDatasetRepo.On("List", mock.Anything).Return([]*model.Dataset{
		{ID: "1", Name: "mmlu"},
		{ID: "2", Name: "hellaswag"},
		{ID: "3", Name: "humaneval"},
	}, nil)

	handler := NewEvaluationHandler(mockEvalRepo, mockCache, mockModelRepo, mockDatasetRepo, nil, nil, nil)
	router := setupEvalTestRouter(handler)

	// Request body with invalid dataset
	body := map[string]string{
		"model":   "gpt-4",
		"dataset": "invalid-dataset",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/evaluations", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Verify 400 response with valid datasets list
	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	assert.Contains(t, response, "error")
	assert.Contains(t, response["error"].(string), "dataset")

	// Verify valid datasets list is included
	assert.Contains(t, response, "valid_datasets")
	validDatasets := response["valid_datasets"].([]interface{})
	assert.Contains(t, validDatasets, "mmlu")
	assert.Contains(t, validDatasets, "hellaswag")

	mockModelRepo.AssertExpectations(t)
	mockDatasetRepo.AssertExpectations(t)
}

// TestListEvaluations_DefaultPagination tests VAL-API-008
func TestListEvaluations_DefaultPagination(t *testing.T) {
	mockEvalRepo := new(MockEvaluationRepository)
	mockCache := new(MockStatusCache)
	mockModelRepo := new(MockModelRepository)
	mockDatasetRepo := new(MockDatasetRepository)

	// Mock expectations - empty list
	mockEvalRepo.On("List", mock.Anything, 1, 10).Return([]*model.Evaluation{}, 0, nil)

	handler := NewEvaluationHandler(mockEvalRepo, mockCache, mockModelRepo, mockDatasetRepo, nil, nil, nil)
	router := setupEvalTestRouter(handler)

	// Request without pagination params (should use defaults)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/evaluations", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Verify response
	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)

	// Verify default pagination (VAL-API-008)
	assert.Equal(t, float64(1), response["page"])
	assert.Equal(t, float64(10), response["limit"])
	assert.Equal(t, float64(0), response["total"])
	assert.Equal(t, float64(0), response["pages"])
	assert.NotNil(t, response["tasks"])

	mockEvalRepo.AssertExpectations(t)
}

// TestListEvaluations_CustomPagination tests VAL-API-009
func TestListEvaluations_CustomPagination(t *testing.T) {
	mockEvalRepo := new(MockEvaluationRepository)
	mockCache := new(MockStatusCache)
	mockModelRepo := new(MockModelRepository)
	mockDatasetRepo := new(MockDatasetRepository)

	modelID := uuid.New().String()
	datasetID := uuid.New().String()
	evalID := uuid.New().String()

	// Mock expectations - list with one item
	mockEvalRepo.On("List", mock.Anything, 2, 5).Return([]*model.Evaluation{
		{
			ID:         evalID,
			ModelID:    modelID,
			DatasetIDs: []string{datasetID},
			Status:     model.StatusPending,
			Progress:   0,
		},
	}, 1, nil)
	mockModelRepo.On("GetByID", mock.Anything, modelID).Return(&model.Model{ID: modelID, Name: "gpt-4"}, nil)
	mockDatasetRepo.On("GetByID", mock.Anything, datasetID).Return(&model.Dataset{ID: datasetID, Name: "mmlu"}, nil)

	handler := NewEvaluationHandler(mockEvalRepo, mockCache, mockModelRepo, mockDatasetRepo, nil, nil, nil)
	router := setupEvalTestRouter(handler)

	// Request with custom pagination
	req := httptest.NewRequest(http.MethodGet, "/api/v1/evaluations?page=2&limit=5", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Verify response
	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)

	// Verify custom pagination (VAL-API-009)
	assert.Equal(t, float64(2), response["page"])
	assert.Equal(t, float64(5), response["limit"])
	assert.Equal(t, float64(1), response["total"])
	assert.Equal(t, float64(1), response["pages"]) // ceil(1/5) = 1

	// Verify task in response
	tasks := response["tasks"].([]interface{})
	assert.Len(t, tasks, 1)

	mockEvalRepo.AssertExpectations(t)
	mockModelRepo.AssertExpectations(t)
	mockDatasetRepo.AssertExpectations(t)
}

// TestListEvaluations_TotalCount tests VAL-API-010
func TestListEvaluations_TotalCount(t *testing.T) {
	mockEvalRepo := new(MockEvaluationRepository)
	mockCache := new(MockStatusCache)
	mockModelRepo := new(MockModelRepository)
	mockDatasetRepo := new(MockDatasetRepository)

	// Mock expectations - 25 total items, returning first page
	mockEvalRepo.On("List", mock.Anything, 1, 10).Return([]*model.Evaluation{
		{ID: uuid.New().String(), Status: model.StatusPending},
		{ID: uuid.New().String(), Status: model.StatusPending},
	}, 25, nil)

	handler := NewEvaluationHandler(mockEvalRepo, mockCache, mockModelRepo, mockDatasetRepo, nil, nil, nil)
	router := setupEvalTestRouter(handler)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/evaluations", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)

	// Verify total count (VAL-API-010)
	assert.Equal(t, float64(25), response["total"])

	mockEvalRepo.AssertExpectations(t)
}

// TestListEvaluations_PagesCalculation tests VAL-API-011
func TestListEvaluations_PagesCalculation(t *testing.T) {
	mockEvalRepo := new(MockEvaluationRepository)
	mockCache := new(MockStatusCache)
	mockModelRepo := new(MockModelRepository)
	mockDatasetRepo := new(MockDatasetRepository)

	// Test case: 25 total items, 10 per page = 3 pages (VAL-API-011)
	mockEvalRepo.On("List", mock.Anything, 1, 10).Return([]*model.Evaluation{}, 25, nil)

	handler := NewEvaluationHandler(mockEvalRepo, mockCache, mockModelRepo, mockDatasetRepo, nil, nil, nil)
	router := setupEvalTestRouter(handler)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/evaluations", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)

	// Verify pages calculation: ceil(25/10) = 3 (VAL-API-011)
	assert.Equal(t, float64(3), response["pages"])

	mockEvalRepo.AssertExpectations(t)
}

// TestListEvaluations_InvalidPage tests VAL-API-008 (invalid page returns 400)
func TestListEvaluations_InvalidPage(t *testing.T) {
	mockEvalRepo := new(MockEvaluationRepository)
	mockCache := new(MockStatusCache)
	mockModelRepo := new(MockModelRepository)
	mockDatasetRepo := new(MockDatasetRepository)

	handler := NewEvaluationHandler(mockEvalRepo, mockCache, mockModelRepo, mockDatasetRepo, nil, nil, nil)
	router := setupEvalTestRouter(handler)

	// Request with invalid page (negative)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/evaluations?page=-1", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Verify 400 response
	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	assert.Contains(t, response, "error")
	assert.Contains(t, response["error"].(string), "page")
}

// TestListEvaluations_InvalidLimit tests VAL-API-008 (invalid limit returns 400)
func TestListEvaluations_InvalidLimit(t *testing.T) {
	mockEvalRepo := new(MockEvaluationRepository)
	mockCache := new(MockStatusCache)
	mockModelRepo := new(MockModelRepository)
	mockDatasetRepo := new(MockDatasetRepository)

	handler := NewEvaluationHandler(mockEvalRepo, mockCache, mockModelRepo, mockDatasetRepo, nil, nil, nil)
	router := setupEvalTestRouter(handler)

	// Request with invalid limit (zero)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/evaluations?limit=0", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Verify 400 response
	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	assert.Contains(t, response, "error")
	assert.Contains(t, response["error"].(string), "limit")
}

// TestListEvaluations_EmptyList tests VAL-API-008 (empty list returns empty array)
func TestListEvaluations_EmptyList(t *testing.T) {
	mockEvalRepo := new(MockEvaluationRepository)
	mockCache := new(MockStatusCache)
	mockModelRepo := new(MockModelRepository)
	mockDatasetRepo := new(MockDatasetRepository)

	// Mock expectations - empty list
	mockEvalRepo.On("List", mock.Anything, 1, 10).Return([]*model.Evaluation{}, 0, nil)

	handler := NewEvaluationHandler(mockEvalRepo, mockCache, mockModelRepo, mockDatasetRepo, nil, nil, nil)
	router := setupEvalTestRouter(handler)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/evaluations", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)

	// Verify empty list response (VAL-API-008)
	tasks := response["tasks"].([]interface{})
	assert.Len(t, tasks, 0)
	assert.Equal(t, float64(0), response["total"])
	assert.Equal(t, float64(0), response["pages"])

	mockEvalRepo.AssertExpectations(t)
}

// TestListEvaluations_PageBeyondRange tests VAL-API-008 (page beyond range returns empty array)
func TestListEvaluations_PageBeyondRange(t *testing.T) {
	mockEvalRepo := new(MockEvaluationRepository)
	mockCache := new(MockStatusCache)
	mockModelRepo := new(MockModelRepository)
	mockDatasetRepo := new(MockDatasetRepository)

	// Mock expectations - page beyond range, returns empty but total is 10 (2 pages total)
	mockEvalRepo.On("List", mock.Anything, 100, 10).Return([]*model.Evaluation{}, 10, nil)

	handler := NewEvaluationHandler(mockEvalRepo, mockCache, mockModelRepo, mockDatasetRepo, nil, nil, nil)
	router := setupEvalTestRouter(handler)

	// Request with page way beyond range
	req := httptest.NewRequest(http.MethodGet, "/api/v1/evaluations?page=100", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)

	// Verify empty array for page beyond range (VAL-API-008)
	tasks := response["tasks"].([]interface{})
	assert.Len(t, tasks, 0)
	assert.Equal(t, float64(10), response["total"])
	assert.Equal(t, float64(1), response["pages"]) // ceil(10/10) = 1

	mockEvalRepo.AssertExpectations(t)
}

// TestGetEvaluation_ValidID tests VAL-API-012
func TestGetEvaluation_ValidID(t *testing.T) {
	mockEvalRepo := new(MockEvaluationRepository)
	mockCache := new(MockStatusCache)
	mockModelRepo := new(MockModelRepository)
	mockDatasetRepo := new(MockDatasetRepository)

	evalID := uuid.New().String()
	modelID := uuid.New().String()
	datasetID := uuid.New().String()

	eval := &model.Evaluation{
		ID:         evalID,
		ModelID:    modelID,
		DatasetIDs: []string{datasetID},
		Status:     model.StatusPending,
		Progress:   0,
		Config:     map[string]any{"temperature": 0.7},
	}

	mockEvalRepo.On("GetByID", mock.Anything, evalID).Return(eval, nil)
	mockModelRepo.On("GetByID", mock.Anything, modelID).Return(&model.Model{ID: modelID, Name: "gpt-4"}, nil)
	mockDatasetRepo.On("GetByID", mock.Anything, datasetID).Return(&model.Dataset{ID: datasetID, Name: "mmlu"}, nil)

	handler := NewEvaluationHandler(mockEvalRepo, mockCache, mockModelRepo, mockDatasetRepo, nil, nil, nil)
	router := setupEvalTestRouter(handler)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/evaluations/"+evalID, nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)

	// Verify response fields (VAL-API-012)
	assert.Equal(t, evalID, response["id"])
	assert.Equal(t, "gpt-4", response["model"])
	assert.Equal(t, "mmlu", response["dataset"])
	assert.Equal(t, "pending", response["status"])
	assert.NotNil(t, response["config"])
	assert.Equal(t, float64(0), response["progress"])
	assert.NotNil(t, response["created_at"])

	mockEvalRepo.AssertExpectations(t)
	mockModelRepo.AssertExpectations(t)
	mockDatasetRepo.AssertExpectations(t)
}

// TestGetEvaluation_NotFound tests VAL-API-013
func TestGetEvaluation_NotFound(t *testing.T) {
	mockEvalRepo := new(MockEvaluationRepository)
	mockCache := new(MockStatusCache)
	mockModelRepo := new(MockModelRepository)
	mockDatasetRepo := new(MockDatasetRepository)

	evalID := uuid.New().String()

	mockEvalRepo.On("GetByID", mock.Anything, evalID).Return(nil, repository.ErrNotFound)

	handler := NewEvaluationHandler(mockEvalRepo, mockCache, mockModelRepo, mockDatasetRepo, nil, nil, nil)
	router := setupEvalTestRouter(handler)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/evaluations/"+evalID, nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	assert.Contains(t, response, "error")
	assert.Contains(t, response["error"].(string), "not found")

	mockEvalRepo.AssertExpectations(t)
}

// TestGetEvaluation_InvalidUUID tests VAL-API-013 (invalid UUID returns 400)
func TestGetEvaluation_InvalidUUID(t *testing.T) {
	mockEvalRepo := new(MockEvaluationRepository)
	mockCache := new(MockStatusCache)
	mockModelRepo := new(MockModelRepository)
	mockDatasetRepo := new(MockDatasetRepository)

	handler := NewEvaluationHandler(mockEvalRepo, mockCache, mockModelRepo, mockDatasetRepo, nil, nil, nil)
	router := setupEvalTestRouter(handler)

	// Request with invalid UUID format
	req := httptest.NewRequest(http.MethodGet, "/api/v1/evaluations/invalid-uuid", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	assert.Contains(t, response, "error")
	assert.Contains(t, response["error"].(string), "invalid")
	assert.Contains(t, response["error"].(string), "UUID")
}

// TestGetEvaluation_AllStatusTypes tests VAL-API-012 (different status values)
func TestGetEvaluation_AllStatusTypes(t *testing.T) {
	testCases := []struct {
		name   string
		status model.EvaluationStatus
	}{
		{"pending", model.StatusPending},
		{"running", model.StatusRunning},
		{"completed", model.StatusCompleted},
		{"failed", model.StatusFailed},
		{"cancelled", model.StatusCancelled},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockEvalRepo := new(MockEvaluationRepository)
			mockCache := new(MockStatusCache)
			mockModelRepo := new(MockModelRepository)
			mockDatasetRepo := new(MockDatasetRepository)

			evalID := uuid.New().String()
			modelID := uuid.New().String()
			datasetID := uuid.New().String()

			eval := &model.Evaluation{
				ID:         evalID,
				ModelID:    modelID,
				DatasetIDs: []string{datasetID},
				Status:     tc.status,
				Progress:   50,
				Config:     map[string]any{},
			}

			mockEvalRepo.On("GetByID", mock.Anything, evalID).Return(eval, nil)
			mockModelRepo.On("GetByID", mock.Anything, modelID).Return(&model.Model{ID: modelID, Name: "gpt-4"}, nil)
			mockDatasetRepo.On("GetByID", mock.Anything, datasetID).Return(&model.Dataset{ID: datasetID, Name: "mmlu"}, nil)

			handler := NewEvaluationHandler(mockEvalRepo, mockCache, mockModelRepo, mockDatasetRepo, nil, nil, nil)
			router := setupEvalTestRouter(handler)

			req := httptest.NewRequest(http.MethodGet, "/api/v1/evaluations/"+evalID, nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code)

			var response map[string]interface{}
			json.Unmarshal(w.Body.Bytes(), &response)
			assert.Equal(t, string(tc.status), response["status"])

			mockEvalRepo.AssertExpectations(t)
			mockModelRepo.AssertExpectations(t)
			mockDatasetRepo.AssertExpectations(t)
		})
	}
}

// TestGetEvaluationStatus_ValidID tests VAL-API-014
func TestGetEvaluationStatus_ValidID(t *testing.T) {
	mockEvalRepo := new(MockEvaluationRepository)
	mockCache := new(MockStatusCache)
	mockModelRepo := new(MockModelRepository)
	mockDatasetRepo := new(MockDatasetRepository)

	evalID := uuid.New().String()

	eval := &model.Evaluation{
		ID:       evalID,
		Status:   model.StatusRunning,
		Progress: 50,
	}

	mockEvalRepo.On("GetByID", mock.Anything, evalID).Return(eval, nil)

	handler := NewEvaluationHandler(mockEvalRepo, mockCache, mockModelRepo, mockDatasetRepo, nil, nil, nil)
	router := setupEvalTestRouter(handler)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/evaluations/"+evalID+"/status", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)

	// Verify response has status and progress fields (VAL-API-014)
	assert.Equal(t, evalID, response["id"])
	assert.Equal(t, "running", response["status"])
	assert.Equal(t, float64(50), response["progress"])

	mockEvalRepo.AssertExpectations(t)
}

// TestGetEvaluationStatus_PendingState tests VAL-API-015
func TestGetEvaluationStatus_PendingState(t *testing.T) {
	mockEvalRepo := new(MockEvaluationRepository)
	mockCache := new(MockStatusCache)
	mockModelRepo := new(MockModelRepository)
	mockDatasetRepo := new(MockDatasetRepository)

	evalID := uuid.New().String()

	eval := &model.Evaluation{
		ID:       evalID,
		Status:   model.StatusPending,
		Progress: 0,
	}

	mockEvalRepo.On("GetByID", mock.Anything, evalID).Return(eval, nil)

	handler := NewEvaluationHandler(mockEvalRepo, mockCache, mockModelRepo, mockDatasetRepo, nil, nil, nil)
	router := setupEvalTestRouter(handler)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/evaluations/"+evalID+"/status", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)

	// Verify pending state returns status=pending, progress=0 (VAL-API-015)
	assert.Equal(t, "pending", response["status"])
	assert.Equal(t, float64(0), response["progress"])

	mockEvalRepo.AssertExpectations(t)
}

// TestGetEvaluationStatus_RunningState tests VAL-API-016
func TestGetEvaluationStatus_RunningState(t *testing.T) {
	testCases := []struct {
		name     string
		progress int
	}{
		{"low_progress", 1},
		{"mid_progress", 50},
		{"high_progress", 99},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockEvalRepo := new(MockEvaluationRepository)
			mockCache := new(MockStatusCache)
			mockModelRepo := new(MockModelRepository)
			mockDatasetRepo := new(MockDatasetRepository)

			evalID := uuid.New().String()

			eval := &model.Evaluation{
				ID:       evalID,
				Status:   model.StatusRunning,
				Progress: tc.progress,
			}

			mockEvalRepo.On("GetByID", mock.Anything, evalID).Return(eval, nil)

			handler := NewEvaluationHandler(mockEvalRepo, mockCache, mockModelRepo, mockDatasetRepo, nil, nil, nil)
			router := setupEvalTestRouter(handler)

			req := httptest.NewRequest(http.MethodGet, "/api/v1/evaluations/"+evalID+"/status", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code)

			var response map[string]interface{}
			json.Unmarshal(w.Body.Bytes(), &response)

			// Verify running state returns status=running, progress 1-99 (VAL-API-016)
			assert.Equal(t, "running", response["status"])
			assert.Equal(t, float64(tc.progress), response["progress"])

			mockEvalRepo.AssertExpectations(t)
		})
	}
}

// TestGetEvaluationStatus_CompletedState tests VAL-API-017
// This test verifies that progress is always 100 for completed status,
// regardless of the stored progress value (handles bug where DB stores progress=0)
func TestGetEvaluationStatus_CompletedState(t *testing.T) {
	mockEvalRepo := new(MockEvaluationRepository)
	mockCache := new(MockStatusCache)
	mockModelRepo := new(MockModelRepository)
	mockDatasetRepo := new(MockDatasetRepository)

	evalID := uuid.New().String()

	// Simulate bug scenario: DB stores progress=0 even though status is completed
	eval := &model.Evaluation{
		ID:       evalID,
		Status:   model.StatusCompleted,
		Progress: 0, // Bug scenario: DB has progress=0
	}

	mockEvalRepo.On("GetByID", mock.Anything, evalID).Return(eval, nil)

	handler := NewEvaluationHandler(mockEvalRepo, mockCache, mockModelRepo, mockDatasetRepo, nil, nil, nil)
	router := setupEvalTestRouter(handler)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/evaluations/"+evalID+"/status", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)

	// Verify completed state returns status=completed, progress=100 (VAL-API-017)
	// Handler must override progress to 100 for completed status
	assert.Equal(t, "completed", response["status"])
	assert.Equal(t, float64(100), response["progress"])

	mockEvalRepo.AssertExpectations(t)
}

// TestGetEvaluationStatus_FailedState tests VAL-API-018
func TestGetEvaluationStatus_FailedState(t *testing.T) {
	mockEvalRepo := new(MockEvaluationRepository)
	mockCache := new(MockStatusCache)
	mockModelRepo := new(MockModelRepository)
	mockDatasetRepo := new(MockDatasetRepository)

	evalID := uuid.New().String()
	errorMsg := "OpenCompass execution failed: out of memory"

	eval := &model.Evaluation{
		ID:           evalID,
		Status:       model.StatusFailed,
		Progress:     45,
		ErrorMessage: errorMsg,
	}

	mockEvalRepo.On("GetByID", mock.Anything, evalID).Return(eval, nil)

	handler := NewEvaluationHandler(mockEvalRepo, mockCache, mockModelRepo, mockDatasetRepo, nil, nil, nil)
	router := setupEvalTestRouter(handler)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/evaluations/"+evalID+"/status", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)

	// Verify failed state returns status=failed with error field (VAL-API-018)
	assert.Equal(t, "failed", response["status"])
	assert.Contains(t, response, "error")
	assert.Equal(t, errorMsg, response["error"])

	mockEvalRepo.AssertExpectations(t)
}

// TestGetEvaluationStatus_CancelledState tests VAL-API-019
func TestGetEvaluationStatus_CancelledState(t *testing.T) {
	mockEvalRepo := new(MockEvaluationRepository)
	mockCache := new(MockStatusCache)
	mockModelRepo := new(MockModelRepository)
	mockDatasetRepo := new(MockDatasetRepository)

	evalID := uuid.New().String()
	cancelledAt := time.Now().Add(-5 * time.Minute)

	eval := &model.Evaluation{
		ID:          evalID,
		Status:      model.StatusCancelled,
		Progress:    30,
		CompletedAt: &cancelledAt,
	}

	mockEvalRepo.On("GetByID", mock.Anything, evalID).Return(eval, nil)

	handler := NewEvaluationHandler(mockEvalRepo, mockCache, mockModelRepo, mockDatasetRepo, nil, nil, nil)
	router := setupEvalTestRouter(handler)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/evaluations/"+evalID+"/status", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)

	// Verify cancelled state returns status=cancelled with cancelled_at (VAL-API-019)
	assert.Equal(t, "cancelled", response["status"])
	assert.Contains(t, response, "cancelled_at")
	assert.NotEmpty(t, response["cancelled_at"])

	mockEvalRepo.AssertExpectations(t)
}

// TestGetEvaluationStatus_NotFound tests VAL-API-014 (404 case)
func TestGetEvaluationStatus_NotFound(t *testing.T) {
	mockEvalRepo := new(MockEvaluationRepository)
	mockCache := new(MockStatusCache)
	mockModelRepo := new(MockModelRepository)
	mockDatasetRepo := new(MockDatasetRepository)

	evalID := uuid.New().String()

	mockEvalRepo.On("GetByID", mock.Anything, evalID).Return(nil, repository.ErrNotFound)

	handler := NewEvaluationHandler(mockEvalRepo, mockCache, mockModelRepo, mockDatasetRepo, nil, nil, nil)
	router := setupEvalTestRouter(handler)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/evaluations/"+evalID+"/status", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	assert.Contains(t, response, "error")
	assert.Contains(t, response["error"].(string), "not found")

	mockEvalRepo.AssertExpectations(t)
}

// TestGetEvaluationStatus_InvalidUUID tests VAL-API-014 (400 case)
func TestGetEvaluationStatus_InvalidUUID(t *testing.T) {
	mockEvalRepo := new(MockEvaluationRepository)
	mockCache := new(MockStatusCache)
	mockModelRepo := new(MockModelRepository)
	mockDatasetRepo := new(MockDatasetRepository)

	handler := NewEvaluationHandler(mockEvalRepo, mockCache, mockModelRepo, mockDatasetRepo, nil, nil, nil)
	router := setupEvalTestRouter(handler)

	// Request with invalid UUID format
	req := httptest.NewRequest(http.MethodGet, "/api/v1/evaluations/invalid-uuid/status", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	assert.Contains(t, response, "error")
	assert.Contains(t, response["error"].(string), "invalid")
	assert.Contains(t, response["error"].(string), "UUID")

	mockEvalRepo.AssertExpectations(t)
}

// TestGetResults_CompletedTask tests VAL-API-020, VAL-API-022
func TestGetResults_CompletedTask(t *testing.T) {
	mockEvalRepo := new(MockEvaluationRepository)
	mockCache := new(MockStatusCache)
	mockModelRepo := new(MockModelRepository)
	mockDatasetRepo := new(MockDatasetRepository)
	mockResultRepo := new(MockResultRepository)
	mockPredRepo := new(MockPredictionRepository)

	evalID := uuid.New().String()
	modelID := uuid.New().String()
	datasetID := uuid.New().String()

	eval := &model.Evaluation{
		ID:         evalID,
		ModelID:    modelID,
		DatasetIDs: []string{datasetID},
		Status:     model.StatusCompleted,
		Progress:   100,
	}

	results := []*repository.Result{
		{
			ID:           uuid.New().String(),
			EvaluationID: evalID,
			DatasetID:    datasetID,
			Accuracy:     0.85,
			SampleCount:  100,
			CorrectCount: 85,
			Metrics:      map[string]any{"f1": 0.87, "precision": 0.86},
			Summary:      "Evaluation completed successfully",
		},
	}

	predictions := []*repository.Prediction{
		{
			ID:            uuid.New().String(),
			EvaluationID:  evalID,
			DatasetID:     datasetID,
			QuestionIndex: 0,
			Question:      "What is 2+2?",
			Prediction:    "4",
			Answer:        "4",
			Correct:       true,
		},
		{
			ID:            uuid.New().String(),
			EvaluationID:  evalID,
			DatasetID:     datasetID,
			QuestionIndex: 1,
			Question:      "What is 3+3?",
			Prediction:    "6",
			Answer:        "6",
			Correct:       true,
		},
	}

	mockEvalRepo.On("GetByID", mock.Anything, evalID).Return(eval, nil)
	mockResultRepo.On("GetByEvaluationID", mock.Anything, evalID).Return(results, nil)
	mockDatasetRepo.On("GetByID", mock.Anything, datasetID).Return(&model.Dataset{ID: datasetID, Name: "mmlu"}, nil)
	mockPredRepo.On("GetByEvaluationID", mock.Anything, evalID, 1, 100).Return(predictions, 2, nil)

	handler := NewEvaluationHandler(mockEvalRepo, mockCache, mockModelRepo, mockDatasetRepo, mockResultRepo, mockPredRepo, nil)
	router := setupEvalTestRouter(handler)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/evaluations/"+evalID+"/results", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)

	// Verify response has results (VAL-API-020, VAL-API-022)
	assert.Contains(t, response, "results")
	resultsList := response["results"].([]interface{})
	assert.Len(t, resultsList, 1)

	// Verify accuracy metric is included (VAL-API-022)
	resultItem := resultsList[0].(map[string]interface{})
	assert.Equal(t, 0.85, resultItem["accuracy"])
	assert.Equal(t, 100, int(resultItem["sample_count"].(float64)))

	// Verify predictions are included
	assert.Contains(t, response, "predictions")
	predictionsData := response["predictions"].(map[string]interface{})
	assert.Equal(t, 2, int(predictionsData["total"].(float64)))

	mockEvalRepo.AssertExpectations(t)
	mockResultRepo.AssertExpectations(t)
	mockDatasetRepo.AssertExpectations(t)
	mockPredRepo.AssertExpectations(t)
}

// TestGetResults_PendingTask tests VAL-API-021 (409 for pending)
func TestGetResults_PendingTask(t *testing.T) {
	mockEvalRepo := new(MockEvaluationRepository)
	mockCache := new(MockStatusCache)
	mockModelRepo := new(MockModelRepository)
	mockDatasetRepo := new(MockDatasetRepository)
	mockResultRepo := new(MockResultRepository)
	mockPredRepo := new(MockPredictionRepository)

	evalID := uuid.New().String()

	eval := &model.Evaluation{
		ID:       evalID,
		Status:   model.StatusPending,
		Progress: 0,
	}

	mockEvalRepo.On("GetByID", mock.Anything, evalID).Return(eval, nil)

	handler := NewEvaluationHandler(mockEvalRepo, mockCache, mockModelRepo, mockDatasetRepo, mockResultRepo, mockPredRepo, nil)
	router := setupEvalTestRouter(handler)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/evaluations/"+evalID+"/results", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Verify 409 Conflict for pending task (VAL-API-021)
	assert.Equal(t, http.StatusConflict, w.Code)

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	assert.Contains(t, response, "error")
	assert.Contains(t, response["error"].(string), "not available")
	assert.Contains(t, response["error"].(string), "pending")

	mockEvalRepo.AssertExpectations(t)
}

// TestGetResults_RunningTask tests VAL-API-021 (409 for running)
func TestGetResults_RunningTask(t *testing.T) {
	mockEvalRepo := new(MockEvaluationRepository)
	mockCache := new(MockStatusCache)
	mockModelRepo := new(MockModelRepository)
	mockDatasetRepo := new(MockDatasetRepository)
	mockResultRepo := new(MockResultRepository)
	mockPredRepo := new(MockPredictionRepository)

	evalID := uuid.New().String()

	eval := &model.Evaluation{
		ID:       evalID,
		Status:   model.StatusRunning,
		Progress: 50,
	}

	mockEvalRepo.On("GetByID", mock.Anything, evalID).Return(eval, nil)

	handler := NewEvaluationHandler(mockEvalRepo, mockCache, mockModelRepo, mockDatasetRepo, mockResultRepo, mockPredRepo, nil)
	router := setupEvalTestRouter(handler)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/evaluations/"+evalID+"/results", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Verify 409 Conflict for running task (VAL-API-021)
	assert.Equal(t, http.StatusConflict, w.Code)

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	assert.Contains(t, response, "error")
	assert.Contains(t, response["error"].(string), "not available")
	assert.Contains(t, response["error"].(string), "running")

	mockEvalRepo.AssertExpectations(t)
}

// TestGetResults_NotFound tests VAL-API-020 (404 case)
func TestGetResults_NotFound(t *testing.T) {
	mockEvalRepo := new(MockEvaluationRepository)
	mockCache := new(MockStatusCache)
	mockModelRepo := new(MockModelRepository)
	mockDatasetRepo := new(MockDatasetRepository)
	mockResultRepo := new(MockResultRepository)
	mockPredRepo := new(MockPredictionRepository)

	evalID := uuid.New().String()

	mockEvalRepo.On("GetByID", mock.Anything, evalID).Return(nil, repository.ErrNotFound)

	handler := NewEvaluationHandler(mockEvalRepo, mockCache, mockModelRepo, mockDatasetRepo, mockResultRepo, mockPredRepo, nil)
	router := setupEvalTestRouter(handler)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/evaluations/"+evalID+"/results", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	assert.Contains(t, response, "error")
	assert.Contains(t, response["error"].(string), "not found")

	mockEvalRepo.AssertExpectations(t)
}

// TestGetResults_InvalidUUID tests invalid UUID format
func TestGetResults_InvalidUUID(t *testing.T) {
	mockEvalRepo := new(MockEvaluationRepository)
	mockCache := new(MockStatusCache)
	mockModelRepo := new(MockModelRepository)
	mockDatasetRepo := new(MockDatasetRepository)
	mockResultRepo := new(MockResultRepository)
	mockPredRepo := new(MockPredictionRepository)

	handler := NewEvaluationHandler(mockEvalRepo, mockCache, mockModelRepo, mockDatasetRepo, mockResultRepo, mockPredRepo, nil)
	router := setupEvalTestRouter(handler)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/evaluations/invalid-uuid/results", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	assert.Contains(t, response, "error")
	assert.Contains(t, response["error"].(string), "invalid")
	assert.Contains(t, response["error"].(string), "UUID")
}

// TestGetResults_Pagination tests pagination for large prediction sets
func TestGetResults_Pagination(t *testing.T) {
	mockEvalRepo := new(MockEvaluationRepository)
	mockCache := new(MockStatusCache)
	mockModelRepo := new(MockModelRepository)
	mockDatasetRepo := new(MockDatasetRepository)
	mockResultRepo := new(MockResultRepository)
	mockPredRepo := new(MockPredictionRepository)

	evalID := uuid.New().String()
	modelID := uuid.New().String()
	datasetID := uuid.New().String()

	eval := &model.Evaluation{
		ID:         evalID,
		ModelID:    modelID,
		DatasetIDs: []string{datasetID},
		Status:     model.StatusCompleted,
		Progress:   100,
	}

	results := []*repository.Result{
		{
			ID:           uuid.New().String(),
			EvaluationID: evalID,
			DatasetID:    datasetID,
			Accuracy:     0.85,
			SampleCount:  10000,
			Metrics:      map[string]any{},
		},
	}

	// Return 100 predictions for page 2
	predictions := []*repository.Prediction{}
	for i := 0; i < 100; i++ {
		predictions = append(predictions, &repository.Prediction{
			ID:            uuid.New().String(),
			EvaluationID:  evalID,
			DatasetID:     datasetID,
			QuestionIndex: 100 + i,
			Question:      "Question",
			Prediction:    "Answer",
			Answer:        "Answer",
			Correct:       true,
		})
	}

	mockEvalRepo.On("GetByID", mock.Anything, evalID).Return(eval, nil)
	mockResultRepo.On("GetByEvaluationID", mock.Anything, evalID).Return(results, nil)
	mockDatasetRepo.On("GetByID", mock.Anything, datasetID).Return(&model.Dataset{ID: datasetID, Name: "mmlu"}, nil)
	mockPredRepo.On("GetByEvaluationID", mock.Anything, evalID, 2, 100).Return(predictions, 1000, nil)

	handler := NewEvaluationHandler(mockEvalRepo, mockCache, mockModelRepo, mockDatasetRepo, mockResultRepo, mockPredRepo, nil)
	router := setupEvalTestRouter(handler)

	// Request page 2 with limit 100
	req := httptest.NewRequest(http.MethodGet, "/api/v1/evaluations/"+evalID+"/results?predictions_page=2&predictions_limit=100", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)

	// Verify pagination in predictions
	assert.Contains(t, response, "predictions")
	predictionsData := response["predictions"].(map[string]interface{})
	assert.Equal(t, 2, int(predictionsData["page"].(float64)))
	assert.Equal(t, 100, int(predictionsData["limit"].(float64)))
	assert.Equal(t, 1000, int(predictionsData["total"].(float64)))
	assert.Equal(t, 10, int(predictionsData["pages"].(float64))) // ceil(1000/100)

	mockEvalRepo.AssertExpectations(t)
	mockResultRepo.AssertExpectations(t)
	mockDatasetRepo.AssertExpectations(t)
	mockPredRepo.AssertExpectations(t)
}

// TestCancelEvaluation_PendingTask tests VAL-API-023 (cancel pending task returns 200)
func TestCancelEvaluation_PendingTask(t *testing.T) {
	mockEvalRepo := new(MockEvaluationRepository)
	mockCache := new(MockStatusCache)
	mockModelRepo := new(MockModelRepository)
	mockDatasetRepo := new(MockDatasetRepository)

	evalID := uuid.New().String()

	eval := &model.Evaluation{
		ID:       evalID,
		Status:   model.StatusPending,
		Progress: 0,
	}

	mockEvalRepo.On("GetByID", mock.Anything, evalID).Return(eval, nil)
	mockEvalRepo.On("Cancel", mock.Anything, evalID).Return(nil)
	mockCache.On("SetStatus", mock.Anything, evalID, "cancelled").Return(nil)

	handler := NewEvaluationHandler(mockEvalRepo, mockCache, mockModelRepo, mockDatasetRepo, nil, nil, nil)
	router := setupEvalTestRouter(handler)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/evaluations/"+evalID, nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Verify 200 response (VAL-API-023)
	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)

	// Verify response has id and status=cancelled (VAL-API-023)
	assert.Equal(t, evalID, response["id"])
	assert.Equal(t, "cancelled", response["status"])

	mockEvalRepo.AssertExpectations(t)
	mockCache.AssertExpectations(t)
}

// TestCancelEvaluation_RunningTask tests VAL-API-023 (cancel running task returns 200)
func TestCancelEvaluation_RunningTask(t *testing.T) {
	mockEvalRepo := new(MockEvaluationRepository)
	mockCache := new(MockStatusCache)
	mockModelRepo := new(MockModelRepository)
	mockDatasetRepo := new(MockDatasetRepository)

	evalID := uuid.New().String()

	eval := &model.Evaluation{
		ID:       evalID,
		Status:   model.StatusRunning,
		Progress: 50,
	}

	mockEvalRepo.On("GetByID", mock.Anything, evalID).Return(eval, nil)
	mockEvalRepo.On("Cancel", mock.Anything, evalID).Return(nil)
	mockCache.On("SetStatus", mock.Anything, evalID, "cancelled").Return(nil)

	handler := NewEvaluationHandler(mockEvalRepo, mockCache, mockModelRepo, mockDatasetRepo, nil, nil, nil)
	router := setupEvalTestRouter(handler)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/evaluations/"+evalID, nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Verify 200 response (VAL-API-023)
	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)

	// Verify response has id and status=cancelled (VAL-API-023)
	assert.Equal(t, evalID, response["id"])
	assert.Equal(t, "cancelled", response["status"])

	mockEvalRepo.AssertExpectations(t)
	mockCache.AssertExpectations(t)
}

// TestCancelEvaluation_CompletedTask tests VAL-API-024 (cancel completed task returns 409)
func TestCancelEvaluation_CompletedTask(t *testing.T) {
	mockEvalRepo := new(MockEvaluationRepository)
	mockCache := new(MockStatusCache)
	mockModelRepo := new(MockModelRepository)
	mockDatasetRepo := new(MockDatasetRepository)

	evalID := uuid.New().String()

	eval := &model.Evaluation{
		ID:       evalID,
		Status:   model.StatusCompleted,
		Progress: 100,
	}

	mockEvalRepo.On("GetByID", mock.Anything, evalID).Return(eval, nil)

	handler := NewEvaluationHandler(mockEvalRepo, mockCache, mockModelRepo, mockDatasetRepo, nil, nil, nil)
	router := setupEvalTestRouter(handler)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/evaluations/"+evalID, nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Verify 409 Conflict for completed task (VAL-API-024)
	assert.Equal(t, http.StatusConflict, w.Code)

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	assert.Contains(t, response, "error")
	assert.Contains(t, response["error"].(string), "completed")

	mockEvalRepo.AssertExpectations(t)
}

// TestCancelEvaluation_AlreadyCancelled tests idempotent behavior (cancel already cancelled returns 200)
func TestCancelEvaluation_AlreadyCancelled(t *testing.T) {
	mockEvalRepo := new(MockEvaluationRepository)
	mockCache := new(MockStatusCache)
	mockModelRepo := new(MockModelRepository)
	mockDatasetRepo := new(MockDatasetRepository)

	evalID := uuid.New().String()

	eval := &model.Evaluation{
		ID:       evalID,
		Status:   model.StatusCancelled,
		Progress: 30,
	}

	mockEvalRepo.On("GetByID", mock.Anything, evalID).Return(eval, nil)

	handler := NewEvaluationHandler(mockEvalRepo, mockCache, mockModelRepo, mockDatasetRepo, nil, nil, nil)
	router := setupEvalTestRouter(handler)

	// First cancellation
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/evaluations/"+evalID, nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Verify 200 response for idempotent behavior
	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)

	// Verify response has id and status=cancelled
	assert.Equal(t, evalID, response["id"])
	assert.Equal(t, "cancelled", response["status"])

	mockEvalRepo.AssertExpectations(t)
}

// TestCancelEvaluation_NotFound tests 404 case
func TestCancelEvaluation_NotFound(t *testing.T) {
	mockEvalRepo := new(MockEvaluationRepository)
	mockCache := new(MockStatusCache)
	mockModelRepo := new(MockModelRepository)
	mockDatasetRepo := new(MockDatasetRepository)

	evalID := uuid.New().String()

	mockEvalRepo.On("GetByID", mock.Anything, evalID).Return(nil, repository.ErrNotFound)

	handler := NewEvaluationHandler(mockEvalRepo, mockCache, mockModelRepo, mockDatasetRepo, nil, nil, nil)
	router := setupEvalTestRouter(handler)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/evaluations/"+evalID, nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	assert.Contains(t, response, "error")
	assert.Contains(t, response["error"].(string), "not found")

	mockEvalRepo.AssertExpectations(t)
}

// TestCancelEvaluation_InvalidUUID tests invalid UUID format returns 400
func TestCancelEvaluation_InvalidUUID(t *testing.T) {
	mockEvalRepo := new(MockEvaluationRepository)
	mockCache := new(MockStatusCache)
	mockModelRepo := new(MockModelRepository)
	mockDatasetRepo := new(MockDatasetRepository)

	handler := NewEvaluationHandler(mockEvalRepo, mockCache, mockModelRepo, mockDatasetRepo, nil, nil, nil)
	router := setupEvalTestRouter(handler)

	// Request with invalid UUID format
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/evaluations/invalid-uuid", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	assert.Contains(t, response, "error")
	assert.Contains(t, response["error"].(string), "invalid")
	assert.Contains(t, response["error"].(string), "UUID")

	mockEvalRepo.AssertExpectations(t)
}
