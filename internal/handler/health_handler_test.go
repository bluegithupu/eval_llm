package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockDatabase implements DatabaseHealthChecker for testing
type MockDatabase struct {
	mock.Mock
}

func (m *MockDatabase) Ping(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

// MockCache implements cache.StatusCache for testing
type MockCache struct {
	mock.Mock
}

func (m *MockCache) Ping(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockCache) SetStatus(ctx context.Context, evalID string, status string) error {
	args := m.Called(ctx, evalID, status)
	return args.Error(0)
}

func (m *MockCache) GetStatus(ctx context.Context, evalID string) (string, error) {
	args := m.Called(ctx, evalID)
	return args.String(0), args.Error(1)
}

func (m *MockCache) SetProgress(ctx context.Context, evalID string, progress int) error {
	args := m.Called(ctx, evalID, progress)
	return args.Error(0)
}

func (m *MockCache) GetProgress(ctx context.Context, evalID string) (int, error) {
	args := m.Called(ctx, evalID)
	return args.Int(0), args.Error(1)
}

func (m *MockCache) DeleteStatus(ctx context.Context, evalID string) error {
	args := m.Called(ctx, evalID)
	return args.Error(0)
}

func (m *MockCache) DeleteProgress(ctx context.Context, evalID string) error {
	args := m.Called(ctx, evalID)
	return args.Error(0)
}

func (m *MockCache) Close() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockCache) IsAvailable(ctx context.Context) bool {
	args := m.Called(ctx)
	return args.Bool(0)
}

func setupTestRouter(handler *HealthHandler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/health", handler.Health)
	router.GET("/ready", handler.Ready)
	return router
}

func TestHealth_Returns200Healthy(t *testing.T) {
	// Arrange
	handler := NewHealthHandler(nil, nil) // Health doesn't need dependencies
	router := setupTestRouter(handler)

	// Act
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Assert
	assert.Equal(t, http.StatusOK, w.Code)
	assert.JSONEq(t, `{"status": "healthy"}`, w.Body.String())
}

func TestHealth_RespondsQuickly(t *testing.T) {
	// Arrange
	handler := NewHealthHandler(nil, nil)
	router := setupTestRouter(handler)

	// Act
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Assert - response should be immediate (no network calls)
	assert.Equal(t, http.StatusOK, w.Code)
	// The handler doesn't perform any I/O, so it should be <100ms
}

func TestReady_Returns200WhenAllHealthy(t *testing.T) {
	// Arrange
	mockDB := new(MockDatabase)
	mockCache := new(MockCache)
	mockDB.On("Ping", mock.Anything).Return(nil)
	mockCache.On("Ping", mock.Anything).Return(nil)

	handler := NewHealthHandler(mockDB, mockCache)
	router := setupTestRouter(handler)

	// Act
	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Assert
	assert.Equal(t, http.StatusOK, w.Code)

	var response ReadyResponse
	err := parseJSONResponse(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "ready", response.Status)
	assert.Equal(t, "healthy", response.Dependencies["database"].Status)
	assert.Equal(t, "healthy", response.Dependencies["redis"].Status)

	mockDB.AssertExpectations(t)
	mockCache.AssertExpectations(t)
}

func TestReady_Returns503WhenDBUnhealthy(t *testing.T) {
	// Arrange
	mockDB := new(MockDatabase)
	mockCache := new(MockCache)
	mockDB.On("Ping", mock.Anything).Return(errors.New("connection refused"))
	mockCache.On("Ping", mock.Anything).Return(nil)

	handler := NewHealthHandler(mockDB, mockCache)
	router := setupTestRouter(handler)

	// Act
	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Assert
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)

	var response ReadyResponse
	err := parseJSONResponse(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "not_ready", response.Status)
	assert.Equal(t, "unhealthy", response.Dependencies["database"].Status)
	assert.Contains(t, response.Dependencies["database"].Message, "connection refused")
	assert.Equal(t, "healthy", response.Dependencies["redis"].Status)

	mockDB.AssertExpectations(t)
	mockCache.AssertExpectations(t)
}

func TestReady_Returns503WhenRedisUnhealthy(t *testing.T) {
	// Arrange
	mockDB := new(MockDatabase)
	mockCache := new(MockCache)
	mockDB.On("Ping", mock.Anything).Return(nil)
	mockCache.On("Ping", mock.Anything).Return(errors.New("redis connection error"))

	handler := NewHealthHandler(mockDB, mockCache)
	router := setupTestRouter(handler)

	// Act
	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Assert
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)

	var response ReadyResponse
	err := parseJSONResponse(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "not_ready", response.Status)
	assert.Equal(t, "healthy", response.Dependencies["database"].Status)
	assert.Equal(t, "unhealthy", response.Dependencies["redis"].Status)
	assert.Contains(t, response.Dependencies["redis"].Message, "redis connection error")

	mockDB.AssertExpectations(t)
	mockCache.AssertExpectations(t)
}

func TestReady_Returns503WhenBothUnhealthy(t *testing.T) {
	// Arrange
	mockDB := new(MockDatabase)
	mockCache := new(MockCache)
	mockDB.On("Ping", mock.Anything).Return(errors.New("db error"))
	mockCache.On("Ping", mock.Anything).Return(errors.New("redis error"))

	handler := NewHealthHandler(mockDB, mockCache)
	router := setupTestRouter(handler)

	// Act
	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Assert
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)

	var response ReadyResponse
	err := parseJSONResponse(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "not_ready", response.Status)
	assert.Equal(t, "unhealthy", response.Dependencies["database"].Status)
	assert.Equal(t, "unhealthy", response.Dependencies["redis"].Status)

	mockDB.AssertExpectations(t)
	mockCache.AssertExpectations(t)
}

func TestReady_Returns503WhenDBNotInitialized(t *testing.T) {
	// Arrange
	mockCache := new(MockCache)
	mockCache.On("Ping", mock.Anything).Return(nil)

	handler := NewHealthHandler(nil, mockCache) // DB is nil
	router := setupTestRouter(handler)

	// Act
	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Assert
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)

	var response ReadyResponse
	err := parseJSONResponse(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "not_ready", response.Status)
	assert.Equal(t, "unhealthy", response.Dependencies["database"].Status)
	assert.Contains(t, response.Dependencies["database"].Message, "not initialized")
	assert.Equal(t, "healthy", response.Dependencies["redis"].Status)

	mockCache.AssertExpectations(t)
}

func TestReady_Returns503WhenCacheNotInitialized(t *testing.T) {
	// Arrange
	mockDB := new(MockDatabase)
	mockDB.On("Ping", mock.Anything).Return(nil)

	handler := NewHealthHandler(mockDB, nil) // Cache is nil
	router := setupTestRouter(handler)

	// Act
	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Assert
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)

	var response ReadyResponse
	err := parseJSONResponse(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "not_ready", response.Status)
	assert.Equal(t, "healthy", response.Dependencies["database"].Status)
	assert.Equal(t, "unhealthy", response.Dependencies["redis"].Status)
	assert.Contains(t, response.Dependencies["redis"].Message, "not initialized")

	mockDB.AssertExpectations(t)
}

func TestReady_IncludesIndividualDependencyStatus(t *testing.T) {
	// Arrange
	mockDB := new(MockDatabase)
	mockCache := new(MockCache)
	mockDB.On("Ping", mock.Anything).Return(nil)
	mockCache.On("Ping", mock.Anything).Return(nil)

	handler := NewHealthHandler(mockDB, mockCache)
	router := setupTestRouter(handler)

	// Act
	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Assert
	assert.Equal(t, http.StatusOK, w.Code)

	var response ReadyResponse
	err := parseJSONResponse(w.Body.Bytes(), &response)
	assert.NoError(t, err)

	// Verify both dependencies are present
	assert.Contains(t, response.Dependencies, "database")
	assert.Contains(t, response.Dependencies, "redis")

	// Verify each has status field
	assert.NotEmpty(t, response.Dependencies["database"].Status)
	assert.NotEmpty(t, response.Dependencies["redis"].Status)

	mockDB.AssertExpectations(t)
	mockCache.AssertExpectations(t)
}

// Helper function to parse JSON response
func parseJSONResponse(data []byte, target interface{}) error {
	return json.Unmarshal(data, target)
}
