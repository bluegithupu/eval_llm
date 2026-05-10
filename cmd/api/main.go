package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/eval_llm/backend/internal/cache"
	"github.com/eval_llm/backend/internal/config"
	"github.com/eval_llm/backend/internal/handler"
	"github.com/eval_llm/backend/internal/repository"
)

func main() {
	// Load configuration
	cfg := config.Load()

	// Set Gin to release mode for production
	gin.SetMode(gin.ReleaseMode)

	// Create Gin router
	r := gin.New()
	r.Use(gin.Recovery())

	// Initialize database connection
	db, err := repository.NewDatabase(&cfg.Database)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()
	log.Printf("Connected to PostgreSQL on %s:%d", cfg.Database.Host, cfg.Database.Port)

	// Initialize Redis client
	redisClient, err := cache.NewRedisClient(&cfg.Redis)
	if err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}
	defer redisClient.Close()
	log.Printf("Connected to Redis on %s:%d", cfg.Redis.Host, cfg.Redis.Port)

	// Create repositories
	evalRepo := repository.NewEvaluationRepository(db.Pool())
	modelRepo := repository.NewModelRepository(db.Pool())
	datasetRepo := repository.NewDatasetRepository(db.Pool())

	// Create health handler
	healthHandler := handler.NewHealthHandler(db, redisClient)

	// Create evaluation handler
	evalHandler := handler.NewEvaluationHandler(evalRepo, redisClient, modelRepo, datasetRepo)

	// Health endpoints (liveness and readiness)
	r.GET("/health", healthHandler.Health)
	r.GET("/ready", healthHandler.Ready)

	// API v1 routes group
	v1 := r.Group("/api/v1")
	{
		// Evaluation endpoints
		v1.POST("/evaluations", evalHandler.CreateEvaluation)
		v1.GET("/evaluations", evalHandler.ListEvaluations)
		v1.GET("/evaluations/:id", evalHandler.GetEvaluation)
		v1.GET("/evaluations/:id/status", evalHandler.GetEvaluationStatus)
		// TODO: Add remaining evaluation endpoints
		// GET /evaluations/:id/results - Get evaluation results
		// DELETE /evaluations/:id - Cancel evaluation

		v1.GET("/models", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"models": []gin.H{
				{"id": "gpt-4", "name": "GPT-4", "provider": "openai"},
				{"id": "claude", "name": "Claude", "provider": "anthropic"},
				{"id": "qwen", "name": "Qwen", "provider": "dashscope"},
			}})
		})

		v1.GET("/datasets", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"datasets": []gin.H{
				{"id": "mmlu", "name": "MMLU", "description": "Massive Multitask Language Understanding"},
				{"id": "hellaswag", "name": "HellaSwag", "description": "Commonsense NLI tasks"},
			}})
		})
	}

	// Start server on configured port
	port := cfg.Server.Port
	log.Printf("Starting API server on port %d", port)

	// Validate UUID generation is working (will be used for task IDs)
	_ = uuid.New()

	if err := r.Run(fmt.Sprintf(":%d", port)); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
