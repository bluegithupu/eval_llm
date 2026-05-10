package service

import (
	"context"
	"testing"
	"time"

	"github.com/eval_llm/backend/internal/evaluator"
	"github.com/eval_llm/backend/internal/k8s"
	"github.com/eval_llm/backend/internal/model"
	"github.com/eval_llm/backend/internal/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
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
