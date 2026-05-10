package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

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

func setupEvalTestRouter(handler *EvaluationHandler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/api/v1/evaluations", handler.CreateEvaluation)
	router.GET("/api/v1/evaluations", handler.ListEvaluations)
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

	handler := NewEvaluationHandler(mockEvalRepo, mockCache, mockModelRepo, mockDatasetRepo)
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

	handler := NewEvaluationHandler(mockEvalRepo, mockCache, mockModelRepo, mockDatasetRepo)
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

	handler := NewEvaluationHandler(mockEvalRepo, mockCache, mockModelRepo, mockDatasetRepo)
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

	handler := NewEvaluationHandler(mockEvalRepo, mockCache, mockModelRepo, mockDatasetRepo)
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

	handler := NewEvaluationHandler(mockEvalRepo, mockCache, mockModelRepo, mockDatasetRepo)
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

	handler := NewEvaluationHandler(mockEvalRepo, mockCache, mockModelRepo, mockDatasetRepo)
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

	handler := NewEvaluationHandler(mockEvalRepo, mockCache, mockModelRepo, mockDatasetRepo)
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

	handler := NewEvaluationHandler(mockEvalRepo, mockCache, mockModelRepo, mockDatasetRepo)
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

	handler := NewEvaluationHandler(mockEvalRepo, mockCache, mockModelRepo, mockDatasetRepo)
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

	handler := NewEvaluationHandler(mockEvalRepo, mockCache, mockModelRepo, mockDatasetRepo)
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

	handler := NewEvaluationHandler(mockEvalRepo, mockCache, mockModelRepo, mockDatasetRepo)
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

	handler := NewEvaluationHandler(mockEvalRepo, mockCache, mockModelRepo, mockDatasetRepo)
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

	handler := NewEvaluationHandler(mockEvalRepo, mockCache, mockModelRepo, mockDatasetRepo)
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
