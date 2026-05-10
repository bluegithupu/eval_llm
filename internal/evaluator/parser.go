package evaluator

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"time"
)

// ErrOutputDirNotFound indicates the output directory doesn't exist
var ErrOutputDirNotFound = errors.New("output directory not found")

// ErrMalformedFile indicates a file couldn't be parsed correctly
var ErrMalformedFile = errors.New("malformed file")

// ErrNoResults indicates no results were found in the output directory
var ErrNoResults = errors.New("no results found")

// ParsedPrediction represents a single prediction from OpenCompass JSON output
type ParsedPrediction struct {
	Question    string `json:"question"`
	Prediction  string `json:"prediction"`
	Answer      string `json:"answer"`
	DatasetID   string `json:"dataset_id,omitempty"`
	QuestionIdx int    `json:"question_idx,omitempty"`
}

// ParsedResult represents an evaluation result extracted from CSV summary
type ParsedResult struct {
	Dataset   string  `json:"dataset"`
	Model     string  `json:"model"`
	Metric    string  `json:"metric"`
	Value     float64 `json:"value"`
	Timestamp string  `json:"timestamp,omitempty"`
}

// Parser handles parsing of OpenCompass output files
type Parser struct {
	baseDir string
}

// NewParser creates a new Parser with the specified base directory
func NewParser(baseDir string) *Parser {
	return &Parser{
		baseDir: baseDir,
	}
}

// ParseOutputDir parses the output directory and returns predictions and results
// It handles timestamp-based directory naming and validates the output structure
func (p *Parser) ParseOutputDir(timestamp string) (*ParseOutput, error) {
	// Build the expected output path with timestamp
	outputDir := filepath.Join(p.baseDir, timestamp)

	// Validate output directory exists
	if _, err := os.Stat(outputDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("%w: %s", ErrOutputDirNotFound, outputDir)
	}

	// Parse predictions
	predictions, err := p.parsePredictions(outputDir)
	if err != nil {
		// Don't fail on prediction parse errors, just return empty
		predictions = []ParsedPrediction{}
	}

	// Parse summary CSV
	results, err := p.parseSummaryCSV(outputDir)
	if err != nil {
		// Don't fail on summary parse errors, just return empty
		results = []ParsedResult{}
	}

	// Verify we have some data
	if len(predictions) == 0 && len(results) == 0 {
		return nil, ErrNoResults
	}

	return &ParseOutput{
		Predictions: predictions,
		Results:     results,
		OutputDir:   outputDir,
		Timestamp:   timestamp,
	}, nil
}

// ParseOutput holds the parsed results from OpenCompass output
type ParseOutput struct {
	Predictions []ParsedPrediction `json:"predictions"`
	Results     []ParsedResult     `json:"results"`
	OutputDir   string             `json:"output_dir"`
	Timestamp   string             `json:"timestamp"`
}

// parsePredictions parses JSON prediction files from the predictions directory
func (p *Parser) parsePredictions(outputDir string) ([]ParsedPrediction, error) {
	predictionsDir := filepath.Join(outputDir, "predictions")

	// Check if predictions directory exists
	info, err := os.Stat(predictionsDir)
	if os.IsNotExist(err) {
		return nil, nil // No predictions directory is acceptable
	}
	if err != nil {
		return nil, fmt.Errorf("failed to stat predictions directory: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("predictions path is not a directory: %s", predictionsDir)
	}

	// Read all JSON files in the predictions directory
	entries, err := os.ReadDir(predictionsDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read predictions directory: %w", err)
	}

	var predictions []ParsedPrediction
	jsonFilePattern := regexp.MustCompile(`\.json$`)

	for _, entry := range entries {
		if entry.IsDir() || !jsonFilePattern.MatchString(entry.Name()) {
			continue
		}

		filePath := filepath.Join(predictionsDir, entry.Name())
		preds, err := p.parsePredictionFile(filePath)
		if err != nil {
			// Log error but continue parsing other files
			continue
		}
		predictions = append(predictions, preds...)
	}

	return predictions, nil
}

// parsePredictionFile parses a single JSON prediction file
func (p *Parser) parsePredictionFile(filePath string) ([]ParsedPrediction, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %w", filePath, err)
	}

	// Try to parse as JSON array
	var preds []ParsedPrediction
	if err := json.Unmarshal(content, &preds); err != nil {
		// Try to parse as a single prediction object
		var single PredictedPrediction
		if err2 := json.Unmarshal(content, &single); err2 == nil {
			return []ParsedPrediction{{
				Question:   single.Question,
				Prediction: single.Prediction,
				Answer:     single.Answer,
			}}, nil
		}
		return nil, fmt.Errorf("%w: failed to parse %s as JSON: %v", ErrMalformedFile, filePath, err)
	}

	// Convert to ParsedPrediction format
	result := make([]ParsedPrediction, len(preds))
	for i, pred := range preds {
		result[i] = ParsedPrediction{
			Question:   pred.Question,
			Prediction: pred.Prediction,
			Answer:     pred.Answer,
		}
	}

	return result, nil
}

// PredictedPrediction is used for parsing JSON with different field names
type PredictedPrediction struct {
	Question   string `json:"question"`
	Prediction string `json:"prediction"`
	Answer     string `json:"answer"`
}

// parseSummaryCSV parses the summary CSV file and extracts accuracy scores
func (p *Parser) parseSummaryCSV(outputDir string) ([]ParsedResult, error) {
	summaryDir := filepath.Join(outputDir, "summary")
	csvPath := filepath.Join(summaryDir, "summary.csv")

	// Check if summary CSV exists
	if _, err := os.Stat(csvPath); os.IsNotExist(err) {
		return nil, nil // No summary is acceptable
	}

	// Open the CSV file
	file, err := os.Open(csvPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open summary CSV: %w", err)
	}
	defer file.Close()

	// Parse CSV
	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("%w: failed to parse summary CSV: %v", ErrMalformedFile, err)
	}

	if len(records) < 2 {
		return nil, ErrNoResults
	}

	// Parse header to find column indices
	header := records[0]
	colIndex := make(map[string]int)
	for i, col := range header {
		colIndex[col] = i
	}

	// Required columns
	datasetCol, ok := colIndex["dataset"]
	if !ok {
		datasetCol, ok = colIndex["Dataset"]
	}
	modelCol, ok := colIndex["model"]
	if !ok {
		modelCol, ok = colIndex["Model"]
	}
	metricCol, ok := colIndex["metric"]
	if !ok {
		metricCol, ok = colIndex["Metric"]
	}
	valueCol, ok := colIndex["value"]
	if !ok {
		valueCol, ok = colIndex["Value"]
	}
	accuracyCol, ok := colIndex["accuracy"]
	if !ok {
		accuracyCol, ok = colIndex["Accuracy"]
	}

	// Parse data rows
	var results []ParsedResult
	for _, record := range records[1:] {
		// Ensure record has enough fields
		if len(record) < len(header) {
			continue
		}

		// Try to extract dataset name
		dataset := ""
		if datasetCol < len(record) {
			dataset = record[datasetCol]
		}

		// Try to extract model name
		model := ""
		if modelCol < len(record) {
			model = record[modelCol]
		}

		// Try to extract metric
		metric := ""
		if metricCol < len(record) {
			metric = record[metricCol]
		}

		// Try to extract value (either "value" or "accuracy" column)
		var value float64
		found := false

		if valueCol < len(record) && record[valueCol] != "" {
			if v, err := strconv.ParseFloat(record[valueCol], 64); err == nil {
				value = v
				found = true
			}
		}
		if !found && accuracyCol < len(record) && record[accuracyCol] != "" {
			if v, err := strconv.ParseFloat(record[accuracyCol], 64); err == nil {
				value = v
				found = true
			}
		}

		if found {
			results = append(results, ParsedResult{
				Dataset:   dataset,
				Model:     model,
				Metric:    metric,
				Value:     value,
				Timestamp: filepath.Base(filepath.Dir(csvPath)),
			})
		}
	}

	return results, nil
}

// FindTimestampDir finds a timestamp directory in the base directory matching the pattern
// Returns the timestamp string (e.g., "20230220_183030") or empty string if not found
func (p *Parser) FindTimestampDir() (string, error) {
	entries, err := os.ReadDir(p.baseDir)
	if err != nil {
		return "", fmt.Errorf("failed to read base directory: %w", err)
	}

	timestampPattern := regexp.MustCompile(`^\d{8}_\d{6}$`)

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		if timestampPattern.MatchString(name) {
			return name, nil
		}
	}

	return "", fmt.Errorf("no timestamp directory found in %s", p.baseDir)
}

// CleanupOutputDir removes the output directory and all its contents
// This should be called after successful result collection
func (p *Parser) CleanupOutputDir(timestamp string) error {
	outputDir := filepath.Join(p.baseDir, timestamp)

	// Verify directory exists before attempting cleanup
	if _, err := os.Stat(outputDir); os.IsNotExist(err) {
		return fmt.Errorf("output directory does not exist: %s", outputDir)
	}

	// Use os.RemoveAll to recursively delete
	if err := os.RemoveAll(outputDir); err != nil {
		return fmt.Errorf("failed to cleanup output directory: %w", err)
	}

	return nil
}

// GenerateTimestamp generates a timestamp string in the format YYYYMMDD_HHMMSS
func GenerateTimestamp() string {
	return time.Now().Format("20060102_150405")
}

// ValidateTimestamp checks if a string is a valid timestamp format
func ValidateTimestamp(ts string) bool {
	matched, _ := regexp.MatchString(`^\d{8}_\d{6}$`, ts)
	return matched
}

// ParsePredictionsFromJSON parses predictions from a JSON byte slice
func ParsePredictionsFromJSON(data []byte) ([]ParsedPrediction, error) {
	var preds []ParsedPrediction
	if err := json.Unmarshal(data, &preds); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrMalformedFile, err)
	}
	return preds, nil
}

// ParseResultsFromCSV parses results from a CSV byte slice
func ParseResultsFromCSV(data []byte) ([]ParsedResult, error) {
	reader := csv.NewReader(bytes.NewReader(data))
	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrMalformedFile, err)
	}

	if len(records) < 2 {
		return nil, ErrNoResults
	}

	// Simple CSV parsing without header assumption
	var results []ParsedResult
	for _, record := range records[1:] {
		if len(record) < 2 {
			continue
		}

		// Try to parse value from second column
		value, err := strconv.ParseFloat(record[1], 64)
		if err != nil {
			continue
		}

		results = append(results, ParsedResult{
			Dataset: record[0],
			Value:   value,
			Metric:  "accuracy",
		})
	}

	return results, nil
}
