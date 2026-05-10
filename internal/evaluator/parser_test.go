package evaluator

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test for VAL-OC-009: JSON Predictions Parsing
func TestParser_ParsePredictions(t *testing.T) {
	// Create a temporary directory structure
	tempDir := t.TempDir()
	timestampDir := filepath.Join(tempDir, "output")
	err := os.MkdirAll(timestampDir, 0755)
	require.NoError(t, err)

	predictionsDir := filepath.Join(timestampDir, "predictions")
	err = os.MkdirAll(predictionsDir, 0755)
	require.NoError(t, err)

	// Create a sample prediction JSON file
	predictionJSON := `[
		{"question": "What is the capital of France?", "prediction": "Paris", "answer": "Paris"},
		{"question": "What is 2+2?", "prediction": "4", "answer": "4"},
		{"question": "Who wrote Hamlet?", "prediction": "Shakespeare", "answer": "William Shakespeare"}
	]`
	err = os.WriteFile(filepath.Join(predictionsDir, "test_predictions.json"), []byte(predictionJSON), 0644)
	require.NoError(t, err)

	parser := NewParser(tempDir)
	output, err := parser.ParseOutputDir("output")
	require.NoError(t, err)

	assert.Len(t, output.Predictions, 3)
	assert.Equal(t, "What is the capital of France?", output.Predictions[0].Question)
	assert.Equal(t, "Paris", output.Predictions[0].Prediction)
	assert.Equal(t, "Paris", output.Predictions[0].Answer)
}

func TestParser_ParsePredictionFile(t *testing.T) {
	// Create a temporary directory
	tempDir := t.TempDir()
	predictionsDir := filepath.Join(tempDir, "predictions")
	err := os.MkdirAll(predictionsDir, 0755)
	require.NoError(t, err)

	// Test with array format
	predictionJSON := `[
		{"question": "Q1", "prediction": "A1", "answer": "A1"},
		{"question": "Q2", "prediction": "A2", "answer": "A2"}
	]`
	jsonPath := filepath.Join(predictionsDir, "predictions_array.json")
	err = os.WriteFile(jsonPath, []byte(predictionJSON), 0644)
	require.NoError(t, err)

	parser := NewParser(tempDir)
	preds, err := parser.parsePredictionFile(jsonPath)
	require.NoError(t, err)
	assert.Len(t, preds, 2)

	// Test with single object format
	singleJSON := `{"question": "Single Q", "prediction": "Single P", "answer": "Single A"}`
	singlePath := filepath.Join(predictionsDir, "predictions_single.json")
	err = os.WriteFile(singlePath, []byte(singleJSON), 0644)
	require.NoError(t, err)

	preds, err = parser.parsePredictionFile(singlePath)
	require.NoError(t, err)
	assert.Len(t, preds, 1)
	assert.Equal(t, "Single Q", preds[0].Question)
}

func TestParser_ParsePredictionFile_MalformedJSON(t *testing.T) {
	tempDir := t.TempDir()
	predictionsDir := filepath.Join(tempDir, "predictions")
	err := os.MkdirAll(predictionsDir, 0755)
	require.NoError(t, err)

	// Create a malformed JSON file
	malformedJSON := `{invalid json}`
	err = os.WriteFile(filepath.Join(predictionsDir, "malformed.json"), []byte(malformedJSON), 0644)
	require.NoError(t, err)

	parser := NewParser(tempDir)
	_, err = parser.parsePredictionFile(filepath.Join(predictionsDir, "malformed.json"))
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrMalformedFile)
}

func TestParser_ParsePredictionFile_NonExistent(t *testing.T) {
	parser := NewParser("/nonexistent")
	_, err := parser.parsePredictionFile("/nonexistent/file.json")
	assert.Error(t, err)
}

// Test for VAL-OC-010: CSV Summary Extraction
func TestParser_ParseSummaryCSV(t *testing.T) {
	tempDir := t.TempDir()
	summaryDir := filepath.Join(tempDir, "summary")
	err := os.MkdirAll(summaryDir, 0755)
	require.NoError(t, err)

	// Create a sample CSV file
	csvContent := `dataset,model,metric,value
MMLU,gpt-4,accuracy,0.85
MMLU,gpt-4,precision,0.83
HellaSwag,gpt-4,accuracy,0.79
BBH,gpt-4,accuracy,0.72`
	err = os.WriteFile(filepath.Join(summaryDir, "summary.csv"), []byte(csvContent), 0644)
	require.NoError(t, err)

	parser := NewParser(tempDir)
	results, err := parser.parseSummaryCSV(tempDir)
	require.NoError(t, err)
	assert.Len(t, results, 4)

	// Check first result
	assert.Equal(t, "MMLU", results[0].Dataset)
	assert.Equal(t, "gpt-4", results[0].Model)
	assert.Equal(t, "accuracy", results[0].Metric)
	assert.Equal(t, 0.85, results[0].Value)
}

func TestParser_ParseSummaryCSV_AlternativeHeaders(t *testing.T) {
	tempDir := t.TempDir()
	summaryDir := filepath.Join(tempDir, "summary")
	err := os.MkdirAll(summaryDir, 0755)
	require.NoError(t, err)

	// Create a CSV with different header case
	csvContent := `Dataset,Model,Metric,Accuracy
MMLU,gpt-4,accuracy,0.85
HellaSwag,gpt-4,accuracy,0.79`
	err = os.WriteFile(filepath.Join(summaryDir, "summary.csv"), []byte(csvContent), 0644)
	require.NoError(t, err)

	parser := NewParser(tempDir)
	results, err := parser.parseSummaryCSV(tempDir)
	require.NoError(t, err)
	assert.Len(t, results, 2)
	assert.Equal(t, 0.85, results[0].Value)
}

func TestParser_ParseSummaryCSV_NoFile(t *testing.T) {
	parser := NewParser("/nonexistent")
	results, err := parser.parseSummaryCSV("/nonexistent")
	assert.NoError(t, err) // No file is acceptable, returns nil
	assert.Nil(t, results)
}

func TestParser_ParseSummaryCSV_MalformedCSV(t *testing.T) {
	tempDir := t.TempDir()
	summaryDir := filepath.Join(tempDir, "summary")
	err := os.MkdirAll(summaryDir, 0755)
	require.NoError(t, err)

	// Create a malformed CSV - missing numeric values in data rows
	malformedCSV := `dataset,model,metric,value
MMLU,gpt-4,accuracy,not-a-number`
	err = os.WriteFile(filepath.Join(summaryDir, "summary.csv"), []byte(malformedCSV), 0644)
	require.NoError(t, err)

	parser := NewParser(tempDir)
	results, err := parser.parseSummaryCSV(tempDir)
	require.NoError(t, err)
	// Should have 0 results since the value can't be parsed as float
	assert.Len(t, results, 0)
}

func TestParser_ParseSummaryCSV_EmptyFile(t *testing.T) {
	tempDir := t.TempDir()
	summaryDir := filepath.Join(tempDir, "summary")
	err := os.MkdirAll(summaryDir, 0755)
	require.NoError(t, err)

	// Create an empty CSV
	err = os.WriteFile(filepath.Join(summaryDir, "summary.csv"), []byte(""), 0644)
	require.NoError(t, err)

	parser := NewParser(tempDir)
	_, err = parser.parseSummaryCSV(tempDir)
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrNoResults)
}

// Test for VAL-OC-011: Timestamp-Based Output Directory Naming
func TestParser_FindTimestampDir(t *testing.T) {
	tempDir := t.TempDir()

	// Create timestamp directories
	os.Mkdir(filepath.Join(tempDir, "20230220_183030"), 0755)
	os.Mkdir(filepath.Join(tempDir, "other_dir"), 0755)
	os.Mkdir(filepath.Join(tempDir, "20230115_120000"), 0755)

	parser := NewParser(tempDir)
	timestamp, err := parser.FindTimestampDir()
	require.NoError(t, err)
	assert.True(t, timestamp == "20230220_183030" || timestamp == "20230115_120000")
}

func TestParser_FindTimestampDir_NoTimestamp(t *testing.T) {
	tempDir := t.TempDir()
	os.Mkdir(filepath.Join(tempDir, "other_dir"), 0755)

	parser := NewParser(tempDir)
	_, err := parser.FindTimestampDir()
	assert.Error(t, err)
}

func TestGenerateTimestamp(t *testing.T) {
	ts := GenerateTimestamp()
	assert.True(t, ValidateTimestamp(ts))
	assert.Len(t, ts, 15) // YYYYMMDD_HHMMSS = 8 + 1 + 6 = 15
}

func TestValidateTimestamp(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"valid timestamp", "20230220_183030", true},
		{"valid timestamp 2", "20230101_000000", true},
		{"invalid format - no underscore", "20230220183030", false},
		{"invalid format - wrong length", "2023022_183030", false},
		{"invalid format - letters", "20230220_abcde", false},
		{"empty string", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ValidateTimestamp(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// Test for VAL-OC-012: Output Directory Cleanup After Collection
func TestParser_CleanupOutputDir(t *testing.T) {
	tempDir := t.TempDir()
	timestampDir := filepath.Join(tempDir, "20230220_183030")
	err := os.MkdirAll(timestampDir, 0755)
	require.NoError(t, err)

	// Create some test files in the directory
	os.WriteFile(filepath.Join(timestampDir, "test.txt"), []byte("test"), 0644)
	os.Mkdir(filepath.Join(timestampDir, "subdir"), 0755)
	os.WriteFile(filepath.Join(timestampDir, "subdir", "nested.txt"), []byte("nested"), 0644)

	parser := NewParser(tempDir)
	err = parser.CleanupOutputDir("20230220_183030")
	require.NoError(t, err)

	// Verify directory is deleted
	_, err = os.Stat(timestampDir)
	assert.True(t, os.IsNotExist(err))
}

func TestParser_CleanupOutputDir_NonExistent(t *testing.T) {
	parser := NewParser("/nonexistent")
	err := parser.CleanupOutputDir("20230220_183030")
	assert.Error(t, err)
}

// Test for VAL-OC-013: Non-Zero Exit Code Capture
// This is handled by cli.go, but we verify the error types are properly defined
func TestParserErrors(t *testing.T) {
	// Verify error variables are properly defined
	assert.NotNil(t, ErrOutputDirNotFound)
	assert.NotNil(t, ErrMalformedFile)
	assert.NotNil(t, ErrNoResults)

	// Verify error messages
	assert.Contains(t, ErrOutputDirNotFound.Error(), "output directory not found")
	assert.Contains(t, ErrMalformedFile.Error(), "malformed file")
	assert.Contains(t, ErrNoResults.Error(), "no results found")
}

// Test ParseOutputDir with complete structure
func TestParser_ParseOutputDir_Complete(t *testing.T) {
	tempDir := t.TempDir()
	timestamp := "20230220_183030"
	outputDir := filepath.Join(tempDir, timestamp)

	// Create complete directory structure
	os.MkdirAll(filepath.Join(outputDir, "predictions"), 0755)
	os.MkdirAll(filepath.Join(outputDir, "results"), 0755)
	os.MkdirAll(filepath.Join(outputDir, "summary"), 0755)

	// Write prediction JSON
	predictionJSON := `[{"question": "Q1", "prediction": "P1", "answer": "A1"}]`
	os.WriteFile(filepath.Join(outputDir, "predictions", "test.json"), []byte(predictionJSON), 0644)

	// Write summary CSV
	csvContent := `dataset,model,metric,value
MMLU,gpt-4,accuracy,0.85`
	os.WriteFile(filepath.Join(outputDir, "summary", "summary.csv"), []byte(csvContent), 0644)

	parser := NewParser(tempDir)
	result, err := parser.ParseOutputDir(timestamp)
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Len(t, result.Predictions, 1)
	assert.Len(t, result.Results, 1)
	assert.Equal(t, "20230220_183030", result.Timestamp)
}

func TestParser_ParseOutputDir_MissingDir(t *testing.T) {
	parser := NewParser("/nonexistent")
	_, err := parser.ParseOutputDir("20230220_183030")
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrOutputDirNotFound)
}

func TestParser_ParseOutputDir_NoResults(t *testing.T) {
	tempDir := t.TempDir()
	timestamp := "20230220_183030"
	outputDir := filepath.Join(tempDir, timestamp)
	os.MkdirAll(outputDir, 0755)

	// Don't create any prediction or summary files
	parser := NewParser(tempDir)
	_, err := parser.ParseOutputDir(timestamp)
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrNoResults)
}

// Test ParsePredictionsFromJSON helper function
func TestParsePredictionsFromJSON(t *testing.T) {
	jsonData := `[{"question": "Q1", "prediction": "P1", "answer": "A1"}]`
	preds, err := ParsePredictionsFromJSON([]byte(jsonData))
	require.NoError(t, err)
	assert.Len(t, preds, 1)
}

func TestParsePredictionsFromJSON_Invalid(t *testing.T) {
	_, err := ParsePredictionsFromJSON([]byte("invalid"))
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrMalformedFile)
}

// Test ParseResultsFromCSV helper function
func TestParseResultsFromCSV(t *testing.T) {
	csvData := `dataset,value
MMLU,0.85
HellaSwag,0.79`
	results, err := ParseResultsFromCSV([]byte(csvData))
	require.NoError(t, err)
	assert.Len(t, results, 2)
	assert.Equal(t, "MMLU", results[0].Dataset)
	assert.Equal(t, 0.85, results[0].Value)
}

func TestParseResultsFromCSV_Invalid(t *testing.T) {
	_, err := ParseResultsFromCSV([]byte("not csv"))
	assert.Error(t, err)
}

func TestParseResultsFromCSV_Empty(t *testing.T) {
	_, err := ParseResultsFromCSV([]byte("header only"))
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrNoResults)
}

// Test NewParser
func TestNewParser(t *testing.T) {
	parser := NewParser("/test/path")
	assert.Equal(t, "/test/path", parser.baseDir)
}
