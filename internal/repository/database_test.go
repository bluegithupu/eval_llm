package repository

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/eval_llm/backend/internal/config"
)

var testDB *Database

func TestMain(m *testing.M) {
	// Load test configuration
	cfg := &config.DatabaseConfig{
		Host:           getEnvOrDefault("DB_HOST", "localhost"),
		Port:           getEnvIntOrDefault("DB_PORT", 3105),
		Name:           getEnvOrDefault("DB_NAME", "evaluations"),
		User:           getEnvOrDefault("DB_USER", "eval_user"),
		Password:       getEnvOrDefault("DB_PASSWORD", "eval_pass"),
		MaxConnections: 25,
	}

	// Create database connection
	db, err := NewDatabase(cfg)
	if err != nil {
		fmt.Printf("Failed to connect to database: %v\n", err)
		os.Exit(1)
	}
	testDB = db

	// Run tests
	code := m.Run()

	// Cleanup
	db.Close()
	os.Exit(code)
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvIntOrDefault(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		var intVal int
		fmt.Sscanf(value, "%d", &intVal)
		return intVal
	}
	return defaultValue
}

// TestDatabaseConnection tests basic database connectivity
func TestDatabaseConnection(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := testDB.Ping(ctx)
	if err != nil {
		t.Fatalf("Failed to ping database: %v", err)
	}
}

// TestConnectionPoolMaxConnections tests that the pool respects max connections
func TestConnectionPoolMaxConnections(t *testing.T) {
	stats := testDB.Pool().Stat()
	if stats.MaxConns() != 25 {
		t.Errorf("Expected MaxConns to be 25, got %d", stats.MaxConns())
	}
}

// TestBeginTx tests transaction support
func TestBeginTx(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	tx, err := testDB.BeginTx(ctx)
	if err != nil {
		t.Fatalf("Failed to begin transaction: %v", err)
	}

	// Rollback (no changes made)
	err = tx.Rollback(ctx)
	if err != nil {
		t.Fatalf("Failed to rollback transaction: %v", err)
	}
}

// Helper function to get a valid model ID from the database
func getTestModelID(t *testing.T) string {
	ctx := context.Background()
	query := "SELECT id FROM models WHERE name = 'gpt-4' LIMIT 1"

	var modelID string
	err := testDB.Pool().QueryRow(ctx, query).Scan(&modelID)
	if err != nil {
		t.Fatalf("Failed to get test model ID: %v", err)
	}
	return modelID
}

// Helper function to get a valid dataset ID from the database
func getTestDatasetID(t *testing.T) string {
	ctx := context.Background()
	query := "SELECT id FROM datasets WHERE name = 'mmlu' LIMIT 1"

	var datasetID string
	err := testDB.Pool().QueryRow(ctx, query).Scan(&datasetID)
	if err != nil {
		t.Fatalf("Failed to get test dataset ID: %v", err)
	}
	return datasetID
}
