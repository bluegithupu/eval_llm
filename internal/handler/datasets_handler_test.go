package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestListDatasets_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handler := NewDatasetsHandler()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/datasets", nil)

	handler.ListDatasets(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response ListDatasetsResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Len(t, response.Datasets, 2)

	// Verify dataset fields (VAL-API-026)
	datasetIDs := make([]string, len(response.Datasets))
	for i, d := range response.Datasets {
		datasetIDs[i] = d.ID
		assert.NotEmpty(t, d.ID)
		assert.NotEmpty(t, d.Name)
		assert.NotEmpty(t, d.Description)
	}
	assert.Contains(t, datasetIDs, "mmlu")
	assert.Contains(t, datasetIDs, "hellaswag")
}

func TestListDatasets_CacheHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handler := NewDatasetsHandler()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/datasets", nil)

	handler.ListDatasets(c)

	// Verify cache headers present
	assert.NotEmpty(t, w.Header().Get("Cache-Control"))
	assert.Contains(t, w.Header().Get("Cache-Control"), "max-age=3600")
	assert.Contains(t, w.Header().Get("Cache-Control"), "stale-while-revalidate=7200")
	assert.NotEmpty(t, w.Header().Get("ETag"))
	assert.NotEmpty(t, w.Header().Get("Last-Modified"))
}

func TestListDatasets_ResponseFormat(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handler := NewDatasetsHandler()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/datasets", nil)

	handler.ListDatasets(c)

	var response map[string][]map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Contains(t, response, "datasets")
}
