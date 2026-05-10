package main

import (
	"log"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func main() {
	// Set Gin to release mode for production
	gin.SetMode(gin.ReleaseMode)

	// Create Gin router
	r := gin.New()
	r.Use(gin.Recovery())

	// Health endpoints (liveness and readiness)
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "healthy"})
	})

	r.GET("/ready", func(c *gin.Context) {
		// TODO: Add database and Redis connectivity checks
		c.JSON(200, gin.H{"status": "ready", "dependencies": gin.H{
			"database": "not_checked",
			"redis":    "not_checked",
		}})
	})

	// API v1 routes group
	v1 := r.Group("/api/v1")
	{
		// TODO: Add evaluation endpoints
		// POST /evaluations - Create evaluation task
		// GET /evaluations - List evaluations (paginated)
		// GET /evaluations/:id - Get evaluation details
		// GET /evaluations/:id/status - Get evaluation status
		// GET /evaluations/:id/results - Get evaluation results
		// DELETE /evaluations/:id - Cancel evaluation
		// GET /models - List supported models
		// GET /datasets - List supported datasets

		v1.GET("/models", func(c *gin.Context) {
			c.JSON(200, gin.H{"models": []gin.H{
				{"id": "gpt-4", "name": "GPT-4", "provider": "openai"},
				{"id": "claude", "name": "Claude", "provider": "anthropic"},
				{"id": "qwen", "name": "Qwen", "provider": "dashscope"},
			}})
		})

		v1.GET("/datasets", func(c *gin.Context) {
			c.JSON(200, gin.H{"datasets": []gin.H{
				{"id": "mmlu", "name": "MMLU", "description": "Massive Multitask Language Understanding"},
				{"id": "hellaswag", "name": "HellaSwag", "description": "Commonsense NLI tasks"},
			}})
		})
	}

	// Start server on port 3100
	port := "3100"
	log.Printf("Starting API server on port %s", port)
	
	// Validate UUID generation is working (will be used for task IDs)
	_ = uuid.New()
	
	if err := r.Run(":" + port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
