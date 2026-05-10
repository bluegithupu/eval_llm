package handler

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// DatasetsHandler handles dataset-related HTTP requests
type DatasetsHandler struct{}

// NewDatasetsHandler creates a new datasets handler
func NewDatasetsHandler() *DatasetsHandler {
	return &DatasetsHandler{}
}

// DatasetResponse represents a dataset in the response
type DatasetResponse struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// ListDatasetsResponse represents the response for listing datasets
type ListDatasetsResponse struct {
	Datasets []DatasetResponse `json:"datasets"`
}

// ListDatasets handles GET /api/v1/datasets
// Returns static list of supported datasets (MMLU, HellaSwag)
// Includes cache headers for infrequent changes (max-age=3600, stale-while-revalidate=7200)
func (h *DatasetsHandler) ListDatasets(c *gin.Context) {
	datasets := []DatasetResponse{
		{ID: "mmlu", Name: "MMLU", Description: "Massive Multitask Language Understanding"},
		{ID: "hellaswag", Name: "HellaSwag", Description: "Commonsense NLI tasks"},
	}

	// Set cache headers for infrequent changes
	// Cache-Control: max-age=3600 (1 hour), stale-while-revalidate=7200 (2 hours)
	c.Header("Cache-Control", "public, max-age=3600, stale-while-revalidate=7200")
	c.Header("ETag", `"datasets-v1"`)
	c.Header("Last-Modified", time.RFC1123)

	response := ListDatasetsResponse{
		Datasets: datasets,
	}

	c.JSON(http.StatusOK, response)
}
