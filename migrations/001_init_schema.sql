-- LLM Evaluation Backend - Initial Schema Migration
-- Creates all required tables with proper constraints and indexes

-- Enable UUID extension for UUID generation
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- =============================================================================
-- Models Table
-- Stores supported LLM model definitions with provider information
-- =============================================================================
CREATE TABLE models (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(255) NOT NULL UNIQUE,
    type VARCHAR(50) NOT NULL CHECK (type IN ('api', 'local')),
    provider VARCHAR(50) NOT NULL CHECK (provider IN ('openai', 'anthropic', 'dashscope', 'qwen', 'custom')),
    api_key_ref VARCHAR(255),  -- Reference to Kubernetes Secret name
    config JSONB DEFAULT '{}',  -- Additional model configuration
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Insert default supported models
INSERT INTO models (name, type, provider, api_key_ref) VALUES
    ('gpt-4', 'api', 'openai', 'openai-api-key'),
    ('gpt-4-turbo', 'api', 'openai', 'openai-api-key'),
    ('gpt-3.5-turbo', 'api', 'openai', 'openai-api-key'),
    ('claude-3-opus', 'api', 'anthropic', 'anthropic-api-key'),
    ('claude-3-sonnet', 'api', 'anthropic', 'anthropic-api-key'),
    ('claude-3-haiku', 'api', 'anthropic', 'anthropic-api-key'),
    ('qwen-max', 'api', 'dashscope', 'dashscope-api-key'),
    ('qwen-plus', 'api', 'dashscope', 'dashscope-api-key'),
    ('qwen-turbo', 'api', 'dashscope', 'dashscope-api-key');

-- =============================================================================
-- Datasets Table
-- Stores supported evaluation dataset definitions
-- =============================================================================
CREATE TABLE datasets (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(255) NOT NULL UNIQUE,
    display_name VARCHAR(255) NOT NULL,
    description TEXT,
    config_template JSONB DEFAULT '{}',  -- MMEngine config template for dataset
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Insert default supported datasets
INSERT INTO datasets (name, display_name, description) VALUES
    ('mmlu', 'MMLU', 'Massive Multitask Language Understanding benchmark'),
    ('hellaswag', 'HellaSwag', 'Commonsense NLI tasks benchmark'),
    ('humaneval', 'HumanEval', 'Code generation benchmark'),
    ('gsm8k', 'GSM8K', 'Grade school math problems benchmark'),
    ('ceval', 'C-Eval', 'Chinese evaluation benchmark');

-- =============================================================================
-- Evaluations Table
-- Main task records for evaluation jobs
-- =============================================================================
CREATE TABLE evaluations (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    model_id UUID NOT NULL REFERENCES models(id) ON DELETE RESTRICT,
    dataset_ids UUID[] NOT NULL,  -- Array of dataset IDs for multi-dataset evaluation
    config JSONB DEFAULT '{}',  -- Evaluation parameters (batch_size, temperature, etc.)
    status VARCHAR(50) NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'running', 'completed', 'failed', 'cancelled')),
    progress INTEGER DEFAULT 0 CHECK (progress >= 0 AND progress <= 100),
    error_message TEXT,  -- Error details if status is 'failed'
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP NOT NULL,
    started_at TIMESTAMP WITH TIME ZONE,  -- Set when status becomes 'running'
    completed_at TIMESTAMP WITH TIME ZONE  -- Set when status becomes 'completed', 'failed', or 'cancelled'
);

-- Create indexes for evaluations table
CREATE INDEX idx_evaluations_status ON evaluations(status);
CREATE INDEX idx_evaluations_created_at ON evaluations(created_at DESC);
CREATE INDEX idx_evaluations_model_id ON evaluations(model_id);

-- =============================================================================
-- Results Table
-- Stores evaluation results with accuracy metrics
-- =============================================================================
CREATE TABLE results (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    evaluation_id UUID NOT NULL REFERENCES evaluations(id) ON DELETE CASCADE,
    dataset_id UUID NOT NULL REFERENCES datasets(id) ON DELETE RESTRICT,
    accuracy DECIMAL(5,4) NOT NULL CHECK (accuracy >= 0 AND accuracy <= 1),
    sample_count INTEGER NOT NULL DEFAULT 0,
    correct_count INTEGER NOT NULL DEFAULT 0,
    metrics JSONB DEFAULT '{}',  -- Additional metrics (precision, recall, f1, etc.)
    summary TEXT,  -- CSV summary or detailed report
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(evaluation_id, dataset_id)  -- One result per evaluation per dataset
);

-- =============================================================================
-- Predictions Table
-- Stores per-sample predictions from evaluation
-- =============================================================================
CREATE TABLE predictions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    evaluation_id UUID NOT NULL REFERENCES evaluations(id) ON DELETE CASCADE,
    dataset_id UUID NOT NULL REFERENCES datasets(id) ON DELETE RESTRICT,
    question_index INTEGER NOT NULL,  -- Index of question in dataset
    question TEXT NOT NULL,  -- The question/prompt text
    prediction TEXT NOT NULL,  -- Model's predicted answer
    answer TEXT NOT NULL,  -- Ground truth answer
    correct BOOLEAN NOT NULL,  -- Whether prediction matches answer
    reasoning TEXT,  -- Model's reasoning (if available)
    metadata JSONB DEFAULT '{}',  -- Additional prediction metadata
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Create indexes for predictions table
CREATE INDEX idx_predictions_evaluation_id ON predictions(evaluation_id);
CREATE INDEX idx_predictions_dataset_id ON predictions(dataset_id);

-- =============================================================================
-- Logs Table
-- Stores task execution logs for debugging and monitoring
-- =============================================================================
CREATE TABLE logs (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    evaluation_id UUID NOT NULL REFERENCES evaluations(id) ON DELETE CASCADE,
    timestamp TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP NOT NULL,
    level VARCHAR(20) NOT NULL CHECK (level IN ('debug', 'info', 'warn', 'error', 'fatal')),
    message TEXT NOT NULL,
    source VARCHAR(100),  -- Log source (api, k8s-job, opencompass, etc.)
    metadata JSONB DEFAULT '{}',  -- Additional structured log data
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Create indexes for logs table
CREATE INDEX idx_logs_evaluation_id ON logs(evaluation_id);
CREATE INDEX idx_logs_timestamp ON logs(timestamp DESC);
CREATE INDEX idx_logs_level ON logs(level);

-- =============================================================================
-- Functions and Triggers
-- =============================================================================

-- Function to automatically update updated_at timestamp
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Apply trigger to tables that need updated_at tracking
CREATE TRIGGER update_models_updated_at
    BEFORE UPDATE ON models
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_datasets_updated_at
    BEFORE UPDATE ON datasets
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_evaluations_updated_at
    BEFORE UPDATE ON evaluations
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

-- Function to set timestamps based on status changes
CREATE OR REPLACE FUNCTION update_evaluation_timestamps()
RETURNS TRIGGER AS $$
BEGIN
    -- Set started_at when status changes to 'running'
    IF NEW.status = 'running' AND OLD.status != 'running' THEN
        NEW.started_at = CURRENT_TIMESTAMP;
    END IF;
    
    -- Set completed_at when status changes to terminal state
    IF NEW.status IN ('completed', 'failed', 'cancelled') AND OLD.status NOT IN ('completed', 'failed', 'cancelled') THEN
        NEW.completed_at = CURRENT_TIMESTAMP;
    END IF;
    
    -- Reset progress if status changes back to pending
    IF NEW.status = 'pending' AND OLD.status != 'pending' THEN
        NEW.progress = 0;
        NEW.started_at = NULL;
        NEW.completed_at = NULL;
    END IF;
    
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Apply timestamp trigger to evaluations
CREATE TRIGGER update_evaluation_status_timestamps
    BEFORE UPDATE ON evaluations
    FOR EACH ROW
    EXECUTE FUNCTION update_evaluation_timestamps();

-- =============================================================================
-- Comments for Documentation
-- =============================================================================

COMMENT ON TABLE models IS 'Supported LLM models for evaluation (API and local models)';
COMMENT ON TABLE datasets IS 'Supported evaluation datasets with configuration templates';
COMMENT ON TABLE evaluations IS 'Evaluation task records with status tracking';
COMMENT ON TABLE results IS 'Evaluation results with accuracy and metrics per dataset';
COMMENT ON TABLE predictions IS 'Per-sample predictions from model evaluation';
COMMENT ON TABLE logs IS 'Task execution logs for debugging and monitoring';

COMMENT ON COLUMN evaluations.dataset_ids IS 'Array of dataset UUIDs for multi-dataset evaluation';
COMMENT ON COLUMN evaluations.config IS 'JSONB evaluation parameters (batch_size, temperature, etc.)';
COMMENT ON COLUMN results.metrics IS 'JSONB additional metrics (precision, recall, f1, latency, etc.)';
COMMENT ON COLUMN predictions.metadata IS 'JSONB additional prediction metadata (tokens, latency, etc.)';
COMMENT ON COLUMN logs.metadata IS 'JSONB structured log data for debugging';
