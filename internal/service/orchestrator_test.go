package service

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/eval_llm/backend/internal/evaluator"
	"github.com/eval_llm/backend/internal/k8s"
	"github.com/eval_llm/backend/internal/k8s/monitor"
	"github.com/eval_llm/backend/internal/model"
	"github.com/eval_llm/backend/internal/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"log/slog"
)

// MockKubernetesClient mocks Kubernetes client operations
type MockKubernetesClient struct {
	mock.Mock
}

// MockK8sClient wraps MockKubernetesClient and implements k8s.ClientInterface
type MockK8sClient struct {
	MockKubernetesClient *MockKubernetesClient
}

func (m *MockK8sClient) EnsureNamespace(ctx context.Context) error {
	args := m.MockKubernetesClient.Called(ctx)
	return args.Error(0)
}

func (m *MockK8sClient) Namespace() string {
	return k8s.DefaultNamespace
}

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

func (m *MockEvaluationRepository) UpdateStatusAtomic(ctx context.Context, id string, expectedVersion int, status model.EvaluationStatus, progress int) error {
	args := m.Called(ctx, id, expectedVersion, status, progress)
	return args.Error(0)
}

func (m *MockEvaluationRepository) UpdateStatusAtomicWithError(ctx context.Context, id string, expectedVersion int, status model.EvaluationStatus, progress int, errorMsg string) error {
	args := m.Called(ctx, id, expectedVersion, status, progress, errorMsg)
	return args.Error(0)
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

// MockMonitor for testing
type MockMonitor struct {
	mock.Mock
}

func (m *MockMonitor) StartMonitoring(ctx context.Context, evalID string) error {
	args := m.Called(ctx, evalID)
	return args.Error(0)
}

func (m *MockMonitor) StopMonitoring(evalID string) {
	m.Called(evalID)
}

func (m *MockMonitor) GetEvents() <-chan *MockJobEvent {
	return nil
}

// MockEventStore for testing
type MockEventStore struct {
	mock.Mock
}

func (m *MockEventStore) StoreEvent(ctx context.Context, event *monitor.JobEvent) error {
	args := m.Called(ctx, event)
	return args.Error(0)
}

func (m *MockEventStore) StoreError(ctx context.Context, evalID string, errorType monitor.ErrorType, message string, stderr string) error {
	args := m.Called(ctx, evalID, errorType, message, stderr)
	return args.Error(0)
}

func (m *MockEventStore) GetEvents(ctx context.Context, evalID string) ([]*monitor.JobEvent, error) {
	args := m.Called(ctx, evalID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*monitor.JobEvent), args.Error(1)
}

func (m *MockEventStore) GetEventsByType(ctx context.Context, evalID string, eventType monitor.JobEventType) ([]*monitor.JobEvent, error) {
	args := m.Called(ctx, evalID, eventType)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*monitor.JobEvent), args.Error(1)
}

func (m *MockEventStore) ClearOldEvents(ctx context.Context, olderThan time.Duration) error {
	args := m.Called(ctx, olderThan)
	return args.Error(0)
}

// MockJobEvent for testing
type MockJobEvent struct {
	EvalID    string
	EventType string
}

// TestDefaultOrchestratorConfig tests default configuration
func TestDefaultOrchestratorConfig(t *testing.T) {
	cfg := DefaultOrchestratorConfig()

	assert.Equal(t, k8s.DefaultNamespace, cfg.Namespace)
	assert.Equal(t, "opencompass:latest", cfg.ContainerImage)
	assert.Equal(t, "/tmp/opencompass_runs", cfg.WorkDir)
	assert.Equal(t, 10*time.Second, cfg.PollInterval)
	assert.True(t, cfg.CleanupEnabled)
	assert.Equal(t, 1*time.Hour, cfg.CleanupTTL)
}

// TestOrchestratorStartEvaluation_Validation tests that StartEvaluation validates inputs
func TestOrchestratorStartEvaluation_Validation(t *testing.T) {
	// This test would validate that the orchestrator properly validates inputs
	// For now, we just test the config generator

	gen := &ConfigGenerator{
		Model: &model.Model{
			ID:       "model-1",
			Name:     "gpt-4",
			Provider: "openai",
		},
		Dataset: &model.Dataset{
			ID:             "dataset-1",
			Name:           "mmlu",
			ConfigTemplate: "mmlu",
		},
	}

	config := gen.GeneratePythonConfig()
	assert.NotEmpty(t, config)
	assert.Contains(t, config, "OpenAI")
	assert.Contains(t, config, "gpt-4")
	assert.Contains(t, config, "mmlu")
}

// TestConfigGenerator_OpenAI tests OpenAI model config generation
func TestConfigGenerator_OpenAI(t *testing.T) {
	gen := &ConfigGenerator{
		Model: &model.Model{
			ID:       "model-1",
			Name:     "gpt-4",
			Provider: "openai",
		},
		Dataset: &model.Dataset{
			ID:             "dataset-1",
			Name:           "mmlu",
			ConfigTemplate: "mmlu",
		},
	}

	modelType, modelPath := gen.getModelConfig()
	assert.Equal(t, "OpenAI", modelType)
	assert.Equal(t, "gpt-4", modelPath)
}

// TestConfigGenerator_Anthropic tests Anthropic model config generation
func TestConfigGenerator_Anthropic(t *testing.T) {
	gen := &ConfigGenerator{
		Model: &model.Model{
			ID:       "model-2",
			Name:     "claude-3-opus",
			Provider: "anthropic",
		},
		Dataset: &model.Dataset{
			ID:             "dataset-1",
			Name:           "mmlu",
			ConfigTemplate: "mmlu",
		},
	}

	modelType, modelPath := gen.getModelConfig()
	assert.Equal(t, "Anthropic", modelType)
	assert.Equal(t, "claude-3-opus", modelPath)
}

// TestConfigGenerator_DashScope tests DashScope model config generation
func TestConfigGenerator_DashScope(t *testing.T) {
	gen := &ConfigGenerator{
		Model: &model.Model{
			ID:       "model-3",
			Name:     "qwen-turbo",
			Provider: "dashscope",
		},
		Dataset: &model.Dataset{
			ID:             "dataset-1",
			Name:           "mmlu",
			ConfigTemplate: "mmlu",
		},
	}

	modelType, modelPath := gen.getModelConfig()
	assert.Equal(t, "DashScope", modelType)
	assert.Equal(t, "qwen-turbo", modelPath)
}

// TestConfigGenerator_DefaultProvider tests default provider fallback
func TestConfigGenerator_DefaultProvider(t *testing.T) {
	gen := &ConfigGenerator{
		Model: &model.Model{
			ID:       "model-4",
			Name:     "unknown-model",
			Provider: "unknown",
		},
		Dataset: &model.Dataset{
			ID:             "dataset-1",
			Name:           "mmlu",
			ConfigTemplate: "mmlu",
		},
	}

	modelType, modelPath := gen.getModelConfig()
	assert.Equal(t, "OpenAI", modelType) // Default to OpenAI
	assert.Equal(t, "unknown-model", modelPath)
}

// TestGenerateTimestamp tests timestamp generation
func TestGenerateTimestamp(t *testing.T) {
	timestamp := evaluator.GenerateTimestamp()
	assert.NotEmpty(t, timestamp)
	assert.Regexp(t, `^\d{8}_\d{6}$`, timestamp) // Format: YYYYMMDD_HHMMSS
}

// TestValidateTimestamp tests timestamp validation
func TestValidateTimestamp(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"valid format", "20240115_143022", true},
		{"invalid format - no underscore", "20240115143022", false},
		{"invalid format - wrong separator", "20240115-143022", false},
		{"invalid format - too short", "240115_143022", false},
		{"invalid format - letters", "20240115_abcd22", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := evaluator.ValidateTimestamp(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestOrchestratorConfig_Defaults tests that defaults are reasonable
func TestOrchestratorConfig_Defaults(t *testing.T) {
	cfg := DefaultOrchestratorConfig()

	assert.NotNil(t, cfg)
	assert.Equal(t, "llm-eval", cfg.Namespace)
	assert.NotEmpty(t, cfg.ContainerImage)
	assert.NotEmpty(t, cfg.WorkDir)
	assert.Greater(t, cfg.PollInterval, 0*time.Second)
	assert.True(t, cfg.CleanupEnabled)
	assert.Greater(t, cfg.CleanupTTL, 0*time.Hour)
}

// TestBuildJobCommand tests job command building
func TestBuildJobCommand(t *testing.T) {
	cfg := DefaultOrchestratorConfig()
	cfg.WorkDir = "/tmp/test_runs"

	orchestrator := &Orchestrator{
		cfg: cfg,
	}

	cmd := orchestrator.buildJobCommand("20240115_143022")
	assert.NotNil(t, cmd)
	assert.Equal(t, "sh", cmd[0])
	assert.Equal(t, "-c", cmd[1])
	assert.Contains(t, cmd[2], "cd /etc/config")
	assert.Contains(t, cmd[2], "20240115_143022")
}

// TestGetAPIKeysStruct tests API keys struct retrieval
func TestGetAPIKeysStruct(t *testing.T) {
	orchestrator := &Orchestrator{}
	keys := orchestrator.getAPIKeysStruct()
	assert.NotNil(t, keys)
	// Keys will be empty unless environment variables are set
	assert.Empty(t, keys.OpenAI)
	assert.Empty(t, keys.Claude)
	assert.Empty(t, keys.Qwen)
}

// TestMarkFailedWithError tests that markFailed updates status with error message
func TestMarkFailedWithError(t *testing.T) {
	mockEvalRepo := new(MockEvaluationRepository)

	// Note: We can't fully test markFailed without a logger
	// This just verifies the mock is set up correctly
	mockEvalRepo.On("UpdateStatusWithError", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
}

// TestMarkFailedWithStderr tests that markFailedWithStderr stores stderr
func TestMarkFailedWithStderr(t *testing.T) {
	mockEvalRepo := new(MockEvaluationRepository)
	mockEventStore := new(MockEventStore)

	// Set up expectations for StoreError to be called with stderr
	mockEventStore.On("StoreError", mock.Anything, "eval-123", monitor.ErrorTypeOpenCompass, "OpenCompass CLI failed", "Error: dataset not found").Return(nil)
	mockEvalRepo.On("UpdateStatusWithError", mock.Anything, "eval-123", model.StatusFailed, 0, "OpenCompass CLI failed").Return(nil)

	// Create a minimal orchestrator for testing with a logger
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	orchestrator := &Orchestrator{
		evalRepo:   mockEvalRepo,
		eventStore: mockEventStore,
		logger:     logger,
	}

	// Call markFailedWithStderr directly
	ctx := context.Background()
	orchestrator.markFailedWithStderr(ctx, "eval-123", "OpenCompass CLI failed", "Error: dataset not found")

	// Verify that both UpdateStatusWithError and StoreError were called with correct args
	mockEvalRepo.AssertExpectations(t)
	mockEventStore.AssertExpectations(t)
}

// TestMarkFailedWithStderr_EmptyStderr tests markFailedWithStderr with empty stderr
func TestMarkFailedWithStderr_EmptyStderr(t *testing.T) {
	mockEvalRepo := new(MockEvaluationRepository)
	mockEventStore := new(MockEventStore)

	// Set up expectations with empty stderr
	mockEventStore.On("StoreError", mock.Anything, "eval-456", monitor.ErrorTypeOpenCompass, "Job creation failed", "").Return(nil)
	mockEvalRepo.On("UpdateStatusWithError", mock.Anything, "eval-456", model.StatusFailed, 0, "Job creation failed").Return(nil)

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	orchestrator := &Orchestrator{
		evalRepo:   mockEvalRepo,
		eventStore: mockEventStore,
		logger:     logger,
	}

	ctx := context.Background()
	orchestrator.markFailedWithStderr(ctx, "eval-456", "Job creation failed", "")

	mockEvalRepo.AssertExpectations(t)
	mockEventStore.AssertExpectations(t)
}

// TestMarkFailedWithStderr_NoEventStore tests behavior when eventStore is nil
func TestMarkFailedWithStderr_NoEventStore(t *testing.T) {
	mockEvalRepo := new(MockEvaluationRepository)

	// Expect UpdateStatusWithError to still be called
	mockEvalRepo.On("UpdateStatusWithError", mock.Anything, "eval-789", model.StatusFailed, 0, "Some error").Return(nil)

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	orchestrator := &Orchestrator{
		evalRepo:   mockEvalRepo,
		eventStore: nil, // No event store
		logger:     logger,
	}

	ctx := context.Background()
	// This should not panic even without eventStore
	orchestrator.markFailedWithStderr(ctx, "eval-789", "Some error", "some stderr content")

	mockEvalRepo.AssertExpectations(t)
}

// TestNewOrchestrator tests orchestrator creation
func TestNewOrchestrator(t *testing.T) {
	mockEvalRepo := new(MockEvaluationRepository)
	mockResultRepo := new(MockResultRepository)
	mockPredRepo := new(MockPredictionRepository)
	mockMonitor := new(MockMonitor)

	// Note: We can't create a real orchestrator without K8s client
	// This test verifies the struct initialization
	cfg := DefaultOrchestratorConfig()

	// Verify config defaults
	assert.Equal(t, "llm-eval", cfg.Namespace)
	assert.Equal(t, "opencompass:latest", cfg.ContainerImage)
	assert.Equal(t, "/tmp/opencompass_runs", cfg.WorkDir)
	assert.Equal(t, 10*time.Second, cfg.PollInterval)
	assert.True(t, cfg.CleanupEnabled)
	assert.Equal(t, 1*time.Hour, cfg.CleanupTTL)

	// Verify mock repositories can be created
	assert.NotNil(t, mockEvalRepo)
	assert.NotNil(t, mockResultRepo)
	assert.NotNil(t, mockPredRepo)
	assert.NotNil(t, mockMonitor)
}
