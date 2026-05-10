package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/eval_llm/backend/internal/cache"
	"github.com/gin-gonic/gin"
)

// DatabaseHealthChecker defines the interface for database health checking
type DatabaseHealthChecker interface {
	Ping(ctx context.Context) error
}

// HealthHandler handles health check endpoints
type HealthHandler struct {
	db    DatabaseHealthChecker
	cache cache.StatusCache
}

// NewHealthHandler creates a new health handler
func NewHealthHandler(db DatabaseHealthChecker, cache cache.StatusCache) *HealthHandler {
	return &HealthHandler{
		db:    db,
		cache: cache,
	}
}

// HealthResponse represents the response for /health endpoint
type HealthResponse struct {
	Status string `json:"status"`
}

// ReadyResponse represents the response for /ready endpoint
type ReadyResponse struct {
	Status       string                      `json:"status"`
	Dependencies map[string]DependencyStatus `json:"dependencies"`
}

// DependencyStatus represents the status of a single dependency
type DependencyStatus struct {
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

// Health handles the liveness probe endpoint (/health)
// Returns 200 with {status: "healthy"} - minimal response
// Expected to respond in <100ms
func (h *HealthHandler) Health(c *gin.Context) {
	// Simple liveness check - service is running
	// This should always return 200 if the handler is reachable
	c.JSON(http.StatusOK, HealthResponse{
		Status: "healthy",
	})
}

// Ready handles the readiness probe endpoint (/ready)
// Returns 200 when all dependencies (DB, Redis) are connected
// Returns 503 when any dependency is unavailable
// Response includes individual dependency status
func (h *HealthHandler) Ready(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	dependencies := make(map[string]DependencyStatus)
	allHealthy := true

	// Check PostgreSQL connectivity
	dbStatus := h.checkDatabase(ctx)
	dependencies["database"] = dbStatus
	if dbStatus.Status != "healthy" {
		allHealthy = false
	}

	// Check Redis connectivity
	redisStatus := h.checkRedis(ctx)
	dependencies["redis"] = redisStatus
	if redisStatus.Status != "healthy" {
		allHealthy = false
	}

	if allHealthy {
		c.JSON(http.StatusOK, ReadyResponse{
			Status:       "ready",
			Dependencies: dependencies,
		})
	} else {
		c.JSON(http.StatusServiceUnavailable, ReadyResponse{
			Status:       "not_ready",
			Dependencies: dependencies,
		})
	}
}

// checkDatabase verifies PostgreSQL connectivity
func (h *HealthHandler) checkDatabase(ctx context.Context) DependencyStatus {
	if h.db == nil {
		return DependencyStatus{
			Status:  "unhealthy",
			Message: "database client not initialized",
		}
	}

	err := h.db.Ping(ctx)
	if err != nil {
		return DependencyStatus{
			Status:  "unhealthy",
			Message: err.Error(),
		}
	}

	return DependencyStatus{
		Status: "healthy",
	}
}

// checkRedis verifies Redis connectivity
func (h *HealthHandler) checkRedis(ctx context.Context) DependencyStatus {
	if h.cache == nil {
		return DependencyStatus{
			Status:  "unhealthy",
			Message: "redis client not initialized",
		}
	}

	err := h.cache.Ping(ctx)
	if err != nil {
		return DependencyStatus{
			Status:  "unhealthy",
			Message: err.Error(),
		}
	}

	return DependencyStatus{
		Status: "healthy",
	}
}
