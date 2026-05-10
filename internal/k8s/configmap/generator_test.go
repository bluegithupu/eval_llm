package configmap

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateConfig_ValidConfig(t *testing.T) {
	cfg := &ConfigData{
		EvalID:      "test-eval-123",
		ModelName:   "gpt-4",
		ModelType:   "openai",
		ModelPath:   "gpt-4",
		DatasetName: "MMLU",
		DatasetPath: "datasets/mmlu",
		WorkDir:     "/tmp/opencompass",
		MaxSeqLen:   4096,
		MaxOutLen:   2048,
		BatchSize:   1,
	}

	g := &OpenCompassConfigGenerator{}
	config, err := g.GenerateConfig(cfg)

	require.NoError(t, err)
	assert.NotEmpty(t, config)

	// Check that config contains required elements
	assert.Contains(t, config, "# OpenCompass Configuration for Evaluation test-eval-123")
	assert.Contains(t, config, "from opencompass")
	assert.Contains(t, config, "models = [")
	assert.Contains(t, config, "datasets = [")
	assert.Contains(t, config, "work_dir = '/tmp/opencompass'")
	assert.Contains(t, config, "OpenAI")
}

func TestGenerateConfig_AnthropicModel(t *testing.T) {
	cfg := &ConfigData{
		EvalID:      "test-eval-456",
		ModelName:   "claude-3-opus",
		ModelType:   "anthropic",
		ModelPath:   "claude-3-opus",
		DatasetName: "HellaSwag",
		DatasetPath: "datasets/hellaswag",
		WorkDir:     "/tmp/opencompass",
		MaxSeqLen:   8192,
		MaxOutLen:   4096,
		BatchSize:   1,
	}

	g := &OpenCompassConfigGenerator{}
	config, err := g.GenerateConfig(cfg)

	require.NoError(t, err)
	assert.Contains(t, config, "Anthropic")
	assert.Contains(t, config, "env://ANTHROPIC_API_KEY")
}

func TestGenerateConfig_DashscopeModel(t *testing.T) {
	cfg := &ConfigData{
		EvalID:      "test-eval-789",
		ModelName:   "qwen-turbo",
		ModelType:   "dashscope",
		ModelPath:   "qwen-turbo",
		DatasetName: "MMLU",
		DatasetPath: "datasets/mmlu",
		WorkDir:     "/tmp/opencompass",
		MaxSeqLen:   8192,
		MaxOutLen:   2048,
		BatchSize:   1,
	}

	g := &OpenCompassConfigGenerator{}
	config, err := g.GenerateConfig(cfg)

	require.NoError(t, err)
	assert.Contains(t, config, "DashScope")
	assert.Contains(t, config, "env://DASHSCOPE_API_KEY")
}

func TestGenerateConfig_DefaultValues(t *testing.T) {
	cfg := &ConfigData{
		EvalID:      "test-eval-defaults",
		ModelName:   "gpt-4",
		ModelType:   "openai",
		ModelPath:   "gpt-4",
		DatasetName: "MMLU",
		DatasetPath: "datasets/mmlu",
		WorkDir:     "/tmp/opencompass",
		// MaxSeqLen, MaxOutLen, BatchSize not set - should use defaults
	}

	g := &OpenCompassConfigGenerator{}
	config, err := g.GenerateConfig(cfg)

	require.NoError(t, err)
	assert.Contains(t, config, "max_seq_len=4096") // default for openai
	assert.Contains(t, config, "max_out_len=2048") // default for openai
	assert.Contains(t, config, "batch_size=1")     // default
}

func TestGenerateConfig_NilConfig(t *testing.T) {
	g := &OpenCompassConfigGenerator{}
	_, err := g.GenerateConfig(nil)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "config is nil")
}

func TestGenerateConfig_MissingEvalID(t *testing.T) {
	cfg := &ConfigData{
		EvalID:      "",
		ModelName:   "gpt-4",
		ModelType:   "openai",
		ModelPath:   "gpt-4",
		DatasetName: "MMLU",
		WorkDir:     "/tmp/opencompass",
	}

	g := &OpenCompassConfigGenerator{}
	_, err := g.GenerateConfig(cfg)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "evalID is required")
}

func TestGenerateConfig_MissingModelName(t *testing.T) {
	cfg := &ConfigData{
		EvalID:      "test-123",
		ModelName:   "",
		ModelType:   "openai",
		ModelPath:   "gpt-4",
		DatasetName: "MMLU",
		WorkDir:     "/tmp/opencompass",
	}

	g := &OpenCompassConfigGenerator{}
	_, err := g.GenerateConfig(cfg)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "modelName is required")
}

func TestGenerateConfig_MissingModelType(t *testing.T) {
	cfg := &ConfigData{
		EvalID:      "test-123",
		ModelName:   "gpt-4",
		ModelType:   "",
		ModelPath:   "gpt-4",
		DatasetName: "MMLU",
		WorkDir:     "/tmp/opencompass",
	}

	g := &OpenCompassConfigGenerator{}
	_, err := g.GenerateConfig(cfg)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "modelType is required")
}

func TestGenerateConfig_MissingModelPath(t *testing.T) {
	cfg := &ConfigData{
		EvalID:      "test-123",
		ModelName:   "gpt-4",
		ModelType:   "openai",
		ModelPath:   "",
		DatasetName: "MMLU",
		WorkDir:     "/tmp/opencompass",
	}

	g := &OpenCompassConfigGenerator{}
	_, err := g.GenerateConfig(cfg)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "modelPath is required")
}

func TestGenerateConfig_MissingDatasetName(t *testing.T) {
	cfg := &ConfigData{
		EvalID:      "test-123",
		ModelName:   "gpt-4",
		ModelType:   "openai",
		ModelPath:   "gpt-4",
		DatasetName: "",
		WorkDir:     "/tmp/opencompass",
	}

	g := &OpenCompassConfigGenerator{}
	_, err := g.GenerateConfig(cfg)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "datasetName is required")
}

func TestValidateConfigSyntax_ValidConfig(t *testing.T) {
	validConfig := `# OpenCompass Configuration for config.py
from opencompass.summarizers import MedianSummarizer

models = [
    dict(abbr='gpt-4', type='OpenAI', path='gpt-4', key='env://OPENAI_API_KEY')
]

datasets = [
    dict(dataset_path='datasets/mmlu', dataset_name='MMLU')
]

work_dir = '/tmp/opencompass'
`
	err := ValidateConfigSyntax(validConfig)
	assert.NoError(t, err)
}

func TestValidateConfigSyntax_UnbalancedParentheses(t *testing.T) {
	// Config with unbalanced parentheses (missing closing paren)
	invalidConfig := `# OpenCompass Configuration for config.py
models = [
    dict(abbr='gpt-4', type='OpenAI'
]

work_dir = '/tmp/opencompass'
`

	err := ValidateConfigSyntax(invalidConfig)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unbalanced parentheses")
}

func TestValidateConfigSyntax_UnbalancedBrackets(t *testing.T) {
	invalidConfig := `# OpenCompass Configuration
models = [
    dict(abbr='gpt-4', type='OpenAI'),
]`

	// Remove closing bracket to make it invalid
	invalidConfig = strings.Replace(invalidConfig, "]", "", 1)

	err := ValidateConfigSyntax(invalidConfig)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unbalanced square brackets")
}

func TestValidateConfigSyntax_UnbalancedBraces(t *testing.T) {
	// Config with unbalanced braces (extra opening brace, no closing)
	invalidConfig := `# OpenCompass Configuration for config.py
models = [
    dict(abbr='gpt-4', type='OpenAI'),
]

work_dir = '/tmp/opencompass'{`
	err := ValidateConfigSyntax(invalidConfig)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unbalanced braces")
}

func TestValidateConfigSyntax_MissingConfigPyReference(t *testing.T) {
	invalidConfig := `# OpenCompass Configuration
models = []
datasets = []
`

	err := ValidateConfigSyntax(invalidConfig)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "config must reference config.py")
}

func TestConfigMapName(t *testing.T) {
	evalID := "550e8400-e29b-41d4-a716-446655440000"
	name := ConfigMapName(evalID)
	assert.Equal(t, "eval-config-550e8400-e29b-41d4-a716-446655440000", name)
}

func TestGenerateConfig_ContainsRequiredModelFields(t *testing.T) {
	cfg := &ConfigData{
		EvalID:      "test-model-fields",
		ModelName:   "gpt-4",
		ModelType:   "openai",
		ModelPath:   "gpt-4",
		DatasetName: "MMLU",
		DatasetPath: "datasets/mmlu",
		WorkDir:     "/tmp/opencompass",
		MaxSeqLen:   4096,
		MaxOutLen:   2048,
		BatchSize:   1,
	}

	g := &OpenCompassConfigGenerator{}
	config, err := g.GenerateConfig(cfg)

	require.NoError(t, err)

	// Check for required model configuration fields as per VAL-OC-005
	assert.Contains(t, config, "type=")
	assert.Contains(t, config, "path=")
	assert.Contains(t, config, "max_seq_len=")
	assert.Contains(t, config, "max_out_len=")
	assert.Contains(t, config, "batch_size=")
	assert.Contains(t, config, "run_cfg=")
}

func TestGenerateConfig_OpenAIIncludesNumGPUs(t *testing.T) {
	cfg := &ConfigData{
		EvalID:      "test-openai-gpus",
		ModelName:   "gpt-4",
		ModelType:   "openai",
		ModelPath:   "gpt-4",
		DatasetName: "MMLU",
		DatasetPath: "datasets/mmlu",
		WorkDir:     "/tmp/opencompass",
	}

	g := &OpenCompassConfigGenerator{}
	config, err := g.GenerateConfig(cfg)

	require.NoError(t, err)
	// OpenAI model should have num_gpus=0 as per VAL-OC-006
	assert.Contains(t, config, "num_gpus=0")
}

func TestGenerateConfig_EnvironmentVariableReferences(t *testing.T) {
	testCases := []struct {
		modelType string
		envVar    string
	}{
		{"openai", "OPENAI_API_KEY"},
		{"anthropic", "ANTHROPIC_API_KEY"},
		{"dashscope", "DASHSCOPE_API_KEY"},
	}

	for _, tc := range testCases {
		t.Run(tc.modelType, func(t *testing.T) {
			cfg := &ConfigData{
				EvalID:      "test-env-var",
				ModelName:   tc.modelType + "-model",
				ModelType:   tc.modelType,
				ModelPath:   tc.modelType + "-model",
				DatasetName: "MMLU",
				DatasetPath: "datasets/mmlu",
				WorkDir:     "/tmp/opencompass",
			}

			g := &OpenCompassConfigGenerator{}
			config, err := g.GenerateConfig(cfg)

			require.NoError(t, err)
			assert.Contains(t, config, "env://"+tc.envVar)
		})
	}
}

func TestGenerateConfig_InferencerSettings(t *testing.T) {
	cfg := &ConfigData{
		EvalID:      "test-inferencer",
		ModelName:   "gpt-4",
		ModelType:   "openai",
		ModelPath:   "gpt-4",
		DatasetName: "MMLU",
		DatasetPath: "datasets/mmlu",
		WorkDir:     "/tmp/opencompass",
		BatchSize:   4,
	}

	g := &OpenCompassConfigGenerator{}
	config, err := g.GenerateConfig(cfg)

	require.NoError(t, err)

	// Check for inferencer configuration - MMLU uses PPLInferencer
	assert.Contains(t, config, "PPLInferencer")
	assert.Contains(t, config, "batch_size=4")
}

func TestGenerateConfig_HellaSwagInferencer(t *testing.T) {
	cfg := &ConfigData{
		EvalID:      "test-hellaswag-inferencer",
		ModelName:   "gpt-4",
		ModelType:   "openai",
		ModelPath:   "gpt-4",
		DatasetName: "HellaSwag",
		DatasetPath: "datasets/hellaswag",
		WorkDir:     "/tmp/opencompass",
		BatchSize:   2,
	}

	g := &OpenCompassConfigGenerator{}
	config, err := g.GenerateConfig(cfg)

	require.NoError(t, err)

	// HellaSwag uses GenInferencer for generation tasks
	assert.Contains(t, config, "GenInferencer")
	assert.Contains(t, config, "batch_size=2")
}

func TestGenerateConfig_DatasetConfig(t *testing.T) {
	cfg := &ConfigData{
		EvalID:      "test-dataset-config",
		ModelName:   "gpt-4",
		ModelType:   "openai",
		ModelPath:   "gpt-4",
		DatasetName: "MMLU",
		DatasetPath: "datasets/mmlu",
		WorkDir:     "/tmp/opencompass",
	}

	g := &OpenCompassConfigGenerator{}
	config, err := g.GenerateConfig(cfg)

	require.NoError(t, err)

	// Check dataset configuration is present
	assert.Contains(t, config, "dataset_path=")
	assert.Contains(t, config, "dataset_name=")
	assert.Contains(t, config, "abbr=")
}
