package service

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/eval_llm/backend/internal/evaluator"
	"github.com/eval_llm/backend/internal/k8s"
	"github.com/eval_llm/backend/internal/k8s/configmap"
	"github.com/eval_llm/backend/internal/k8s/job"
	"github.com/eval_llm/backend/internal/k8s/monitor"
	"github.com/eval_llm/backend/internal/k8s/secret"
	"github.com/eval_llm/backend/internal/model"
	"github.com/eval_llm/backend/internal/repository"
)

// OrchestratorConfig holds configuration for the evaluation orchestrator
type OrchestratorConfig struct {
	Namespace      string
	ContainerImage string
	WorkDir        string
	PollInterval   time.Duration
	CleanupEnabled bool
	CleanupTTL     time.Duration
}

// DefaultOrchestratorConfig returns default orchestrator configuration
func DefaultOrchestratorConfig() *OrchestratorConfig {
	return &OrchestratorConfig{
		Namespace:      k8s.DefaultNamespace,
		ContainerImage: "opencompass:latest",
		WorkDir:        "/tmp/opencompass_runs",
		PollInterval:   10 * time.Second,
		CleanupEnabled: true,
		CleanupTTL:     1 * time.Hour,
	}
}

// Orchestrator manages the lifecycle of evaluation jobs
type Orchestrator struct {
	cfg           *OrchestratorConfig
	client        *k8s.Client
	evalRepo      repository.EvaluationRepository
	resultRepo    repository.ResultRepository
	predRepo      repository.PredictionRepository
	monitor       *monitor.Monitor
	parser        *evaluator.Parser
	logger        *slog.Logger
	configGen     *configmap.OpenCompassConfigGenerator
	secretManager *secret.SecretManager

	// Channel for completed evaluations
	doneChan chan<- string

	// Event store for error logging
	eventStore monitor.EventStore
}

// NewOrchestrator creates a new evaluation orchestrator
func NewOrchestrator(
	cfg *OrchestratorConfig,
	k8sClient *k8s.Client,
	evalRepo repository.EvaluationRepository,
	resultRepo repository.ResultRepository,
	predRepo repository.PredictionRepository,
	monitor *monitor.Monitor,
	logger *slog.Logger,
	eventStore monitor.EventStore,
) *Orchestrator {
	if cfg == nil {
		cfg = DefaultOrchestratorConfig()
	}

	// Create parser with work directory
	parser := evaluator.NewParser(cfg.WorkDir)

	// Create config and secret generators
	configGen := configmap.NewOpenCompassConfigGenerator(k8sClient.Clientset())
	secretMgr := secret.NewSecretManager(k8sClient.Clientset())

	return &Orchestrator{
		cfg:           cfg,
		client:        k8sClient,
		evalRepo:      evalRepo,
		resultRepo:    resultRepo,
		predRepo:      predRepo,
		monitor:       monitor,
		parser:        parser,
		logger:        logger,
		configGen:     configGen,
		secretManager: secretMgr,
		eventStore:    eventStore,
	}
}

// StartEvaluation starts a new evaluation job
// It creates the ConfigMap, Secret, and Job, then starts monitoring
func (o *Orchestrator) StartEvaluation(ctx context.Context, eval *model.Evaluation, modelEntity *model.Model, datasetEntity *model.Dataset) error {
	o.logger.Info("starting evaluation",
		"eval_id", eval.ID,
		"model", modelEntity.Name,
		"dataset", datasetEntity.Name,
	)

	// Ensure namespace exists
	if err := o.client.EnsureNamespace(ctx); err != nil {
		return fmt.Errorf("failed to ensure namespace: %w", err)
	}

	// Generate timestamp for this run
	timestamp := evaluator.GenerateTimestamp()

	// 1. Create ConfigMap with OpenCompass configuration
	configData := o.generateConfigDataForJob(eval, modelEntity, datasetEntity)
	configMapName := configmap.ConfigMapName(eval.ID)

	if _, createErr := o.configGen.CreateConfigMap(ctx, o.cfg.Namespace, configData); createErr != nil {
		o.markFailed(ctx, eval.ID, fmt.Sprintf("failed to create ConfigMap: %v", createErr))
		return fmt.Errorf("failed to create ConfigMap: %w", createErr)
	}
	o.logger.Info("created ConfigMap", "name", configMapName, "eval_id", eval.ID)

	// 2. Create Secret with API keys
	secretName := secret.SecretName(eval.ID)
	secretData := &secret.SecretData{
		EvalID:    eval.ID,
		ModelName: modelEntity.Name,
		Dataset:   datasetEntity.Name,
		Keys:      o.getAPIKeysStruct(),
	}

	if _, createErr := o.secretManager.CreateSecret(ctx, o.cfg.Namespace, secretData, "", ""); createErr != nil {
		o.markFailed(ctx, eval.ID, fmt.Sprintf("failed to create Secret: %v", createErr))
		return fmt.Errorf("failed to create Secret: %w", createErr)
	}
	o.logger.Info("created Secret", "name", secretName, "eval_id", eval.ID)

	// 3. Create the Kubernetes Job
	jobSpec := &job.JobSpec{
		EvalID:         eval.ID,
		ModelName:      modelEntity.Name,
		Dataset:        datasetEntity.Name,
		ContainerImage: o.cfg.ContainerImage,
		WorkingDir:     o.cfg.WorkDir + "/" + timestamp,
		Command:        o.buildJobCommand(timestamp),
	}

	createdJob, err := job.CreateJob(ctx, o.client.Clientset(), o.cfg.Namespace, jobSpec)
	if err != nil {
		o.markFailed(ctx, eval.ID, fmt.Sprintf("failed to create Job: %v", err))
		return fmt.Errorf("failed to create Job: %w", err)
	}
	o.logger.Info("created Job", "name", createdJob.Name, "eval_id", eval.ID)

	// 4. Start monitoring the Job
	if err := o.monitor.StartMonitoring(ctx, eval.ID); err != nil {
		o.logger.Warn("failed to start monitoring", "eval_id", eval.ID, "error", err)
		// Don't fail the evaluation, just log warning
	}

	// 5. Update status to running
	if err := o.evalRepo.UpdateStatus(ctx, eval.ID, model.StatusRunning, 10); err != nil {
		o.logger.Warn("failed to update status to running", "eval_id", eval.ID, "error", err)
	}

	o.logger.Info("evaluation started successfully", "eval_id", eval.ID)
	return nil
}

// CollectResults collects results from a completed evaluation
func (o *Orchestrator) CollectResults(ctx context.Context, evalID string) error {
	o.logger.Info("collecting results", "eval_id", evalID)

	// Find the timestamp directory
	timestamp, err := o.parser.FindTimestampDir()
	if err != nil {
		o.logger.Warn("could not find timestamp directory", "eval_id", evalID, "error", err)
		// Try common patterns or use a default
		timestamp = o.findTimestampForEval(evalID)
	}

	if timestamp == "" {
		o.logger.Error("no timestamp found for evaluation", "eval_id", evalID)
		return fmt.Errorf("no output directory found for evaluation %s", evalID)
	}

	// Parse the output directory
	output, err := o.parser.ParseOutputDir(timestamp)
	if err != nil {
		o.logger.Error("failed to parse output directory", "eval_id", evalID, "error", err)
		return fmt.Errorf("failed to parse output: %w", err)
	}

	// Get evaluation to access dataset IDs
	eval, err := o.evalRepo.GetByID(ctx, evalID)
	if err != nil {
		return fmt.Errorf("failed to get evaluation: %w", err)
	}

	// 1. Store results
	for _, result := range output.Results {
		resultRecord := &repository.Result{
			EvaluationID: evalID,
			DatasetID:    o.getDatasetIDFromName(eval, result.Dataset),
			Accuracy:     result.Value,
			Metrics:      map[string]any{"metric": result.Metric},
			Summary:      fmt.Sprintf("%s: %.2f%%", result.Dataset, result.Value*100),
		}

		// Count predictions for this dataset
		sampleCount := 0
		correctCount := 0
		for _, pred := range output.Predictions {
			if pred.DatasetID == result.Dataset || result.Dataset == "" {
				sampleCount++
				// Check if prediction matches answer
				if pred.Prediction == pred.Answer {
					correctCount++
				}
			}
		}
		resultRecord.SampleCount = sampleCount
		resultRecord.CorrectCount = correctCount

		if err := o.resultRepo.Create(ctx, resultRecord); err != nil {
			o.logger.Error("failed to store result", "eval_id", evalID, "error", err)
			// Continue with other results
		}
	}

	// 2. Store predictions
	predictions := make([]*repository.Prediction, 0, len(output.Predictions))
	for i, pred := range output.Predictions {
		predRecord := &repository.Prediction{
			EvaluationID:  evalID,
			DatasetID:     o.getDatasetIDFromName(eval, pred.DatasetID),
			QuestionIndex: i,
			Question:      pred.Question,
			Prediction:    pred.Prediction,
			Answer:        pred.Answer,
			Correct:       pred.Prediction == pred.Answer,
		}
		predictions = append(predictions, predRecord)
	}

	if len(predictions) > 0 {
		if err := o.predRepo.BatchInsert(ctx, predictions); err != nil {
			o.logger.Error("failed to batch insert predictions", "eval_id", evalID, "error", err)
		}
	}

	o.logger.Info("results collected successfully",
		"eval_id", evalID,
		"results_count", len(output.Results),
		"predictions_count", len(output.Predictions),
	)

	// 3. Cleanup output directory if enabled
	if o.cfg.CleanupEnabled {
		if err := o.parser.CleanupOutputDir(timestamp); err != nil {
			o.logger.Warn("failed to cleanup output directory", "eval_id", evalID, "error", err)
		} else {
			o.logger.Info("cleaned up output directory", "eval_id", evalID)
		}
	}

	return nil
}

// CleanupEvaluation removes Kubernetes resources after evaluation completes
func (o *Orchestrator) CleanupEvaluation(ctx context.Context, evalID string) error {
	o.logger.Info("cleaning up evaluation resources", "eval_id", evalID)

	// Delete Job
	if err := job.DeleteJob(ctx, o.client.Clientset(), o.cfg.Namespace, evalID); err != nil {
		o.logger.Warn("failed to delete Job", "eval_id", evalID, "error", err)
	}

	// Delete ConfigMap
	if err := o.configGen.DeleteConfigMap(ctx, o.cfg.Namespace, evalID); err != nil {
		o.logger.Warn("failed to delete ConfigMap", "eval_id", evalID, "error", err)
	}

	// Delete Secret
	if err := o.secretManager.DeleteSecret(ctx, o.cfg.Namespace, evalID); err != nil {
		o.logger.Warn("failed to delete Secret", "eval_id", evalID, "error", err)
	}

	o.logger.Info("cleanup completed", "eval_id", evalID)
	return nil
}

// CancelEvaluation cancels a running evaluation
func (o *Orchestrator) CancelEvaluation(ctx context.Context, evalID string) error {
	o.logger.Info("cancelling evaluation", "eval_id", evalID)

	// Stop monitoring
	o.monitor.StopMonitoring(evalID)

	// Delete the Job (which will stop the pod)
	if err := job.DeleteJob(ctx, o.client.Clientset(), o.cfg.Namespace, evalID); err != nil {
		o.logger.Warn("failed to delete Job", "eval_id", evalID, "error", err)
	}

	// Update status to cancelled in DB
	if err := o.evalRepo.UpdateStatus(ctx, evalID, model.StatusCancelled, 0); err != nil {
		return fmt.Errorf("failed to update status to cancelled: %w", err)
	}

	o.logger.Info("evaluation cancelled", "eval_id", evalID)
	return nil
}

// markFailed marks an evaluation as failed with an error message
// It stores the error in the DB and logs it to the event store
func (o *Orchestrator) markFailed(ctx context.Context, evalID string, errorMsg string) {
	// Update status with error message in DB
	if err := o.evalRepo.UpdateStatusWithError(ctx, evalID, model.StatusFailed, 0, errorMsg); err != nil {
		o.logger.Error("failed to update status to failed", "eval_id", evalID, "error", err)
	}

	// Log error to event store (for stderr capture)
	if o.eventStore != nil {
		if storeErr := o.eventStore.StoreError(ctx, evalID, monitor.ErrorTypeOpenCompass, errorMsg, ""); storeErr != nil {
			o.logger.Error("failed to store error event", "eval_id", evalID, "error", storeErr)
		}
	}

	o.logger.Error("evaluation failed", "eval_id", evalID, "error", errorMsg)
}

// markFailedWithStderr marks an evaluation as failed with an error message and stderr output
// This is used to capture OpenCompass stderr and store it in the logs table
func (o *Orchestrator) markFailedWithStderr(ctx context.Context, evalID string, errorMsg string, stderr string) {
	// Update status with error message in DB
	if err := o.evalRepo.UpdateStatusWithError(ctx, evalID, model.StatusFailed, 0, errorMsg); err != nil {
		o.logger.Error("failed to update status to failed", "eval_id", evalID, "error", err)
	}

	// Log error to event store with stderr for debugging
	if o.eventStore != nil {
		if storeErr := o.eventStore.StoreError(ctx, evalID, monitor.ErrorTypeOpenCompass, errorMsg, stderr); storeErr != nil {
			o.logger.Error("failed to store error event", "eval_id", evalID, "error", storeErr)
		}
	}

	o.logger.Error("evaluation failed", "eval_id", evalID, "error", errorMsg, "stderr", stderr)
}

// generateConfigDataForJob generates ConfigData for creating a ConfigMap via the generator
func (o *Orchestrator) generateConfigDataForJob(eval *model.Evaluation, modelEntity *model.Model, datasetEntity *model.Dataset) *configmap.ConfigData {
	// Determine model type based on provider
	modelType := "openai"
	switch modelEntity.Provider {
	case "anthropic":
		modelType = "anthropic"
	case "dashscope":
		modelType = "dashscope"
	}

	return &configmap.ConfigData{
		EvalID:      eval.ID,
		ModelName:   modelEntity.Name,
		ModelType:   modelType,
		ModelPath:   modelEntity.Name,
		DatasetName: datasetEntity.Name,
		DatasetPath: datasetEntity.ConfigTemplate,
		WorkDir:     o.cfg.WorkDir,
		MaxSeqLen:   2048,
		MaxOutLen:   100,
		BatchSize:   8,
	}
}

// generateConfigData generates the OpenCompass configuration (legacy method for compatibility)
func (o *Orchestrator) generateConfigData(eval *model.Evaluation, modelEntity *model.Model, datasetEntity *model.Dataset) map[string]string {
	// Import config generator
	cfgGen := o.newConfigGenerator(modelEntity, datasetEntity)
	pythonConfig := cfgGen.GeneratePythonConfig()

	return map[string]string{
		"config.py": pythonConfig,
	}
}

// newConfigGenerator creates a config generator for the model and dataset
func (o *Orchestrator) newConfigGenerator(modelEntity *model.Model, datasetEntity *model.Dataset) ConfigGenerator {
	return ConfigGenerator{
		Model:   modelEntity,
		Dataset: datasetEntity,
	}
}

// ConfigGenerator generates OpenCompass configuration
type ConfigGenerator struct {
	Model   *model.Model
	Dataset *model.Dataset
}

// GeneratePythonConfig generates the Python configuration string
func (g *ConfigGenerator) GeneratePythonConfig() string {
	// Build model config based on provider
	modelType, modelPath := g.getModelConfig()

	// Build dataset config
	datasetConfig := g.Dataset.ConfigTemplate
	if datasetConfig == "" {
		datasetConfig = fmt.Sprintf("[%s]", g.Dataset.Name)
	}

	return fmt.Sprintf(`# OpenCompass Configuration
# Generated by LLM Evaluation Backend

from opencompass.models import %s

models = [
    dict(
        type=%s,
        path='%s',
        key='${OPENAI_API_KEY}',
        max_seq_len=2048,
        max_out_len=100,
        batch_size=8,
        run_cfg=dict(num_gpus=0),
    )
]

datasets = %s

work_dir = '/tmp/opencompass_runs'
`, modelType, modelType, modelPath, datasetConfig)
}

// getModelConfig returns the model type and path based on the provider
func (g *ConfigGenerator) getModelConfig() (modelType string, modelPath string) {
	switch g.Model.Provider {
	case "openai":
		return "OpenAI", g.Model.Name
	case "anthropic":
		return "Anthropic", g.Model.Name
	case "dashscope":
		return "DashScope", g.Model.Name
	default:
		return "OpenAI", g.Model.Name
	}
}

// getAPIKeys returns the API keys from environment (returns map for legacy use)
func (o *Orchestrator) getAPIKeys() map[string]string {
	keys := make(map[string]string)

	// These would be read from environment or configuration
	// In production, these should be provided when creating the orchestrator
	if openAIKey := os.Getenv("OPENAI_API_KEY"); openAIKey != "" {
		keys["OPENAI_API_KEY"] = openAIKey
	}
	if anthropicKey := os.Getenv("ANTHROPIC_API_KEY"); anthropicKey != "" {
		keys["ANTHROPIC_API_KEY"] = anthropicKey
	}
	if dashscopeKey := os.Getenv("DASHSCOPE_API_KEY"); dashscopeKey != "" {
		keys["DASHSCOPE_API_KEY"] = dashscopeKey
	}

	return keys
}

// getAPIKeysStruct returns the API keys as a SecretManager.APIKeys struct
func (o *Orchestrator) getAPIKeysStruct() secret.APIKeys {
	return secret.APIKeys{
		OpenAI: os.Getenv("OPENAI_API_KEY"),
		Claude: os.Getenv("ANTHROPIC_API_KEY"),
		Qwen:   os.Getenv("DASHSCOPE_API_KEY"),
	}
}

// buildJobCommand builds the command to run inside the Job pod
func (o *Orchestrator) buildJobCommand(timestamp string) []string {
	return []string{
		"sh", "-c",
		fmt.Sprintf("cd /etc/config && python -c \"from run import run_evaluation; run_evaluation('/tmp/opencompass_runs/%s')\" || python config.py", timestamp),
	}
}

// findTimestampForEval finds the timestamp directory for an evaluation
func (o *Orchestrator) findTimestampForEval(evalID string) string {
	// This would need to be implemented based on how timestamps are stored
	// For now, return empty
	return ""
}

// getDatasetIDFromName finds dataset ID from name
func (o *Orchestrator) getDatasetIDFromName(eval *model.Evaluation, datasetName string) string {
	// For now, return the first dataset ID if available
	if len(eval.DatasetIDs) > 0 {
		return eval.DatasetIDs[0]
	}
	return ""
}

// StartResultCollector starts a background goroutine that collects results for completed evaluations
func (o *Orchestrator) StartResultCollector(ctx context.Context) {
	go func() {
		// Subscribe to events from the monitor
		events := o.monitor.GetEvents()

		for {
			select {
			case <-ctx.Done():
				o.logger.Info("result collector shutting down")
				return
			case event := <-events:
				if event.EventType == monitor.EventJobCompleted {
					// Collect results for completed evaluation
					if err := o.CollectResults(ctx, event.EvalID); err != nil {
						o.logger.Error("failed to collect results",
							"eval_id", event.EvalID,
							"error", err,
						)
					}

					// Cleanup resources
					if err := o.CleanupEvaluation(ctx, event.EvalID); err != nil {
						o.logger.Error("failed to cleanup",
							"eval_id", event.EvalID,
							"error", err,
						)
					}
				}
			}
		}
	}()
}
