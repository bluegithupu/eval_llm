package handler

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// ModelsHandler handles model-related HTTP requests
type ModelsHandler struct{}

// NewModelsHandler creates a new models handler
func NewModelsHandler() *ModelsHandler {
	return &ModelsHandler{}
}

// ModelResponse represents a model in the response
type ModelResponse struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Provider string `json:"provider,omitempty"`
}

// ListModelsResponse represents the response for listing models
type ListModelsResponse struct {
	Models []ModelResponse `json:"models"`
}

// ListModels handles GET /api/v1/models
// Returns static list of supported models (gpt-4, claude, qwen)
// Includes cache headers for infrequent changes (max-age=3600, stale-while-revalidate=7200)
func (h *ModelsHandler) ListModels(c *gin.Context) {
	models := []ModelResponse{
		{ID: "gpt-4", Name: "GPT-4", Provider: "openai"},
		{ID: "claude", Name: "Claude", Provider: "anthropic"},
		{ID: "qwen", Name: "Qwen", Provider: "dashscope"},
	}

	// Set cache headers for infrequent changes
	// Cache-Control: max-age=3600 (1 hour), stale-while-revalidate=7200 (2 hours)
	c.Header("Cache-Control", "public, max-age=3600, stale-while-revalidate=7200")
	c.Header("ETag", `"models-v1"`)
	c.Header("Last-Modified", time.RFC1123)

	response := ListModelsResponse{
		Models: models,
	}

	c.JSON(http.StatusOK, response)
}
