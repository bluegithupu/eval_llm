package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/eval_llm/backend/internal/cache"
	"github.com/eval_llm/backend/internal/config"
	"github.com/eval_llm/backend/internal/handler"
	"github.com/eval_llm/backend/internal/k8s"
	"github.com/eval_llm/backend/internal/k8s/monitor"
	"github.com/eval_llm/backend/internal/repository"
	"github.com/eval_llm/backend/internal/service"
)

func main() {
	// Set up structured logger
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

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
	resultRepo := repository.NewResultRepository(db.Pool())
	predictionRepo := repository.NewPredictionRepository(db.Pool())

	// Initialize Kubernetes client (if available)
	var k8sClient *k8s.Client
	var jobMonitor *monitor.Monitor
	var orchestrator *service.Orchestrator

	k8sClient, err = k8s.NewClient(k8s.DefaultClientConfig())
	if err != nil {
		log.Printf("Warning: Failed to connect to Kubernetes: %v. Job execution will be disabled.", err)
	} else {
		log.Printf("Connected to Kubernetes namespace: %s", k8sClient.Namespace())

		// Create event store and monitor for Job status tracking
		eventStore := monitor.NewInMemoryEventStore()
		jobMonitor = monitor.NewMonitor(
			k8sClient.Clientset(),
			evalRepo,
			redisClient,
			eventStore,
			logger,
			monitor.DefaultMonitorConfig(),
		)

		// Create the orchestrator that ties together Job creation, execution, and result collection
		orchestratorCfg := service.DefaultOrchestratorConfig()
		orchestrator = service.NewOrchestrator(
			orchestratorCfg,
			k8sClient,
			evalRepo,
			resultRepo,
			predictionRepo,
			jobMonitor,
			logger,
			eventStore,
		)

		// Start result collector in background
		ctx := context.Background()
		orchestrator.StartResultCollector(ctx)

		log.Printf("Evaluation orchestrator initialized")
	}

	// Create health handler
	healthHandler := handler.NewHealthHandler(db, redisClient)

	// Create evaluation handler with orchestrator (may be nil if K8s unavailable)
	evalHandler := handler.NewEvaluationHandler(
		evalRepo,
		redisClient,
		modelRepo,
		datasetRepo,
		resultRepo,
		predictionRepo,
		orchestrator,
	)

	// Create models and datasets handlers
	modelsHandler := handler.NewModelsHandler()
	datasetsHandler := handler.NewDatasetsHandler()

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
		v1.GET("/evaluations/:id/results", evalHandler.GetResults)
		v1.DELETE("/evaluations/:id", evalHandler.CancelEvaluation)

		// Models and datasets endpoints
		v1.GET("/models", modelsHandler.ListModels)
		v1.GET("/datasets", datasetsHandler.ListDatasets)
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
