package configmap

import (
	"context"
	"fmt"
	"strings"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/eval_llm/backend/internal/k8s"
)

// ConfigData holds the configuration data for an OpenCompass evaluation
type ConfigData struct {
	EvalID      string
	ModelName   string
	ModelType   string // "openai", "anthropic", "dashscope"
	ModelPath   string // e.g., "gpt-4", "claude-3-opus", "qwen-turbo"
	DatasetName string
	DatasetPath string
	WorkDir     string
	MaxSeqLen   int
	MaxOutLen   int
	BatchSize   int
}

// OpenCompassConfigGenerator generates OpenCompass MMEngine configuration files
type OpenCompassConfigGenerator struct {
	client *kubernetes.Clientset
}

// NewOpenCompassConfigGenerator creates a new ConfigMap generator
func NewOpenCompassConfigGenerator(client *kubernetes.Clientset) *OpenCompassConfigGenerator {
	return &OpenCompassConfigGenerator{
		client: client,
	}
}

// GenerateConfig generates an OpenCompass MMEngine Python config file content
func (g *OpenCompassConfigGenerator) GenerateConfig(cfg *ConfigData) (string, error) {
	if err := validateConfig(cfg); err != nil {
		return "", fmt.Errorf("invalid config: %w", err)
	}

	modelConfig := g.generateModelConfig(cfg)
	datasetConfig := g.generateDatasetConfig(cfg)
	inferencerConfig := g.generateInferencerConfig(cfg)
	datasetList := g.generateDatasetList(cfg)

	config := fmt.Sprintf(`# OpenCompass Configuration for Evaluation %s
# Generated automatically - DO NOT EDIT

from opencompass.summarizers import MedianSummarizer
from opencompass.runners import LocalLauncher
from opencompass.tasks import OpenICLEvalTask

%s

%s

datasets = [
    %s
]

%s

work_dir = '%s'`, cfg.EvalID, modelConfig, datasetConfig, datasetList, inferencerConfig, cfg.WorkDir)

	return config, nil
}

// generateModelConfig generates the model configuration section
func (g *OpenCompassConfigGenerator) generateModelConfig(cfg *ConfigData) string {
	switch cfg.ModelType {
	case "openai":
		return g.openAIModelConfig(cfg)
	case "anthropic":
		return g.anthropicModelConfig(cfg)
	case "dashscope":
		return g.dashscopeModelConfig(cfg)
	default:
		return g.openAIModelConfig(cfg)
	}
}

// openAIModelConfig generates OpenAI model configuration
func (g *OpenCompassConfigGenerator) openAIModelConfig(cfg *ConfigData) string {
	maxSeqLen := cfg.MaxSeqLen
	if maxSeqLen == 0 {
		maxSeqLen = 4096
	}
	maxOutLen := cfg.MaxOutLen
	if maxOutLen == 0 {
		maxOutLen = 2048
	}
	batchSize := cfg.BatchSize
	if batchSize == 0 {
		batchSize = 1
	}

	return fmt.Sprintf(`# Model Configuration for OpenAI %s
models = [
    dict(
        abbr='%s',
        type='OpenAI',
        path='%s',
        key='env://OPENAI_API_KEY',
        meta_template=dict(
            openai_api_base='https://api.openai.com/v1',
            max_seq_len=%d,
            max_out_len=%d,
        ),
        batch_size=%d,
        run_cfg=dict(num_gpus=0),
    )
]`, cfg.ModelPath, cfg.ModelName, cfg.ModelPath, maxSeqLen, maxOutLen, batchSize)
}

// anthropicModelConfig generates Anthropic Claude model configuration
func (g *OpenCompassConfigGenerator) anthropicModelConfig(cfg *ConfigData) string {
	maxSeqLen := cfg.MaxSeqLen
	if maxSeqLen == 0 {
		maxSeqLen = 8192
	}
	maxOutLen := cfg.MaxOutLen
	if maxOutLen == 0 {
		maxOutLen = 4096
	}
	batchSize := cfg.BatchSize
	if batchSize == 0 {
		batchSize = 1
	}

	return fmt.Sprintf(`# Model Configuration for Anthropic %s
models = [
    dict(
        abbr='%s',
        type='Anthropic',
        path='%s',
        key='env://ANTHROPIC_API_KEY',
        meta_template=dict(
            anthropic_api_base='https://api.anthropic.com',
            max_seq_len=%d,
            max_out_len=%d,
        ),
        batch_size=%d,
        run_cfg=dict(num_gpus=0),
    )
]`, cfg.ModelPath, cfg.ModelName, cfg.ModelPath, maxSeqLen, maxOutLen, batchSize)
}

// dashscopeModelConfig generates DashScope Qwen model configuration
func (g *OpenCompassConfigGenerator) dashscopeModelConfig(cfg *ConfigData) string {
	maxSeqLen := cfg.MaxSeqLen
	if maxSeqLen == 0 {
		maxSeqLen = 8192
	}
	maxOutLen := cfg.MaxOutLen
	if maxOutLen == 0 {
		maxOutLen = 2048
	}
	batchSize := cfg.BatchSize
	if batchSize == 0 {
		batchSize = 1
	}

	return fmt.Sprintf(`# Model Configuration for DashScope %s
models = [
    dict(
        abbr='%s',
        type='DashScope',
        path='%s',
        key='env://DASHSCOPE_API_KEY',
        meta_template=dict(
            dashscope_api_base='https://dashscope.aliyuncs.com/api/v1',
            max_seq_len=%d,
            max_out_len=%d,
        ),
        batch_size=%d,
        run_cfg=dict(num_gpus=0),
    )
]`, cfg.ModelPath, cfg.ModelName, cfg.ModelPath, maxSeqLen, maxOutLen, batchSize)
}

// generateDatasetConfig generates the dataset configuration section
func (g *OpenCompassConfigGenerator) generateDatasetConfig(cfg *ConfigData) string {
	return fmt.Sprintf(`# Dataset Configuration
# %s dataset`, cfg.DatasetName)
}

// generateInferencerConfig generates the inferencer configuration section
func (g *OpenCompassConfigGenerator) generateInferencerConfig(cfg *ConfigData) string {
	return fmt.Sprintf(`# Inferencer Configuration
inferencer = dict(
    type='OpenICLInferencer',
    batch_size=%d,
)`, cfg.BatchSize)
}

// generateDatasetList generates the dataset list section
func (g *OpenCompassConfigGenerator) generateDatasetList(cfg *ConfigData) string {
	return fmt.Sprintf(`dict(
        dataset_path='%s',
        dataset_name='%s',
        abbr='%s',
        type='OpenICLEvalTask',
    )`, cfg.DatasetPath, cfg.DatasetName, cfg.DatasetName)
}

// validateConfig validates the configuration data
func validateConfig(cfg *ConfigData) error {
	if cfg == nil {
		return fmt.Errorf("config is nil")
	}
	if cfg.EvalID == "" {
		return fmt.Errorf("evalID is required")
	}
	if cfg.ModelName == "" {
		return fmt.Errorf("modelName is required")
	}
	if cfg.ModelType == "" {
		return fmt.Errorf("modelType is required")
	}
	if cfg.ModelPath == "" {
		return fmt.Errorf("modelPath is required")
	}
	if cfg.DatasetName == "" {
		return fmt.Errorf("datasetName is required")
	}
	return nil
}

// GenerateConfig generates a complete MMEngine config string from ConfigData
// This is a convenience method that creates a temporary generator
func GenerateConfig(cfg *ConfigData) (string, error) {
	g := &OpenCompassConfigGenerator{}
	return g.GenerateConfig(cfg)
}

// CreateConfigMap creates a Kubernetes ConfigMap with the OpenCompass configuration
func (g *OpenCompassConfigGenerator) CreateConfigMap(ctx context.Context, namespace string, cfg *ConfigData) (*v1.ConfigMap, error) {
	configContent, err := g.GenerateConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to generate config: %w", err)
	}

	labels := k8s.Labels(cfg.EvalID, cfg.ModelName, cfg.DatasetName)

	configMap := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ConfigMapName(cfg.EvalID),
			Namespace: namespace,
			Labels:    labels,
		},
		Data: map[string]string{
			"config.py": configContent,
		},
	}

	result, err := g.client.CoreV1().ConfigMaps(namespace).Create(ctx, configMap, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to create ConfigMap: %w", err)
	}

	return result, nil
}

// ConfigMapName returns the standard name for an evaluation ConfigMap
func ConfigMapName(evalID string) string {
	return fmt.Sprintf("eval-config-%s", evalID)
}

// GetConfigMap retrieves a ConfigMap by evaluation ID
func (g *OpenCompassConfigGenerator) GetConfigMap(ctx context.Context, namespace, evalID string) (*v1.ConfigMap, error) {
	return g.client.CoreV1().ConfigMaps(namespace).Get(ctx, ConfigMapName(evalID), metav1.GetOptions{})
}

// DeleteConfigMap deletes a ConfigMap by evaluation ID
func (g *OpenCompassConfigGenerator) DeleteConfigMap(ctx context.Context, namespace, evalID string) error {
	return g.client.CoreV1().ConfigMaps(namespace).Delete(ctx, ConfigMapName(evalID), metav1.DeleteOptions{})
}

// ValidateConfigSyntax validates that the generated config is valid Python syntax
// This is a simple check that doesn't execute the code
func ValidateConfigSyntax(config string) error {
	// Basic syntax validation
	// Check for balanced parentheses
	openCount := 0
	for _, ch := range config {
		switch ch {
		case '(':
			openCount++
		case ')':
			openCount--
		}
	}
	if openCount != 0 {
		return fmt.Errorf("unbalanced parentheses")
	}

	// Check for balanced brackets
	squareCount := 0
	for _, ch := range config {
		switch ch {
		case '[':
			squareCount++
		case ']':
			squareCount--
		}
	}
	if squareCount != 0 {
		return fmt.Errorf("unbalanced square brackets")
	}

	// Check for balanced braces
	braceCount := 0
	for _, ch := range config {
		switch ch {
		case '{':
			braceCount++
		case '}':
			braceCount--
		}
	}
	if braceCount != 0 {
		return fmt.Errorf("unbalanced braces")
	}

	// Check that config.py key exists
	if !strings.Contains(config, "config.py") {
		return fmt.Errorf("config must reference config.py")
	}

	return nil
}
