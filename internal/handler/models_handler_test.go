package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestListModels_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handler := NewModelsHandler()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/models", nil)

	handler.ListModels(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response ListModelsResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Len(t, response.Models, 3)

	// Verify model fields (VAL-API-025)
	modelIDs := make([]string, len(response.Models))
	for i, m := range response.Models {
		modelIDs[i] = m.ID
		assert.NotEmpty(t, m.ID)
		assert.NotEmpty(t, m.Name)
	}
	assert.Contains(t, modelIDs, "gpt-4")
	assert.Contains(t, modelIDs, "claude")
	assert.Contains(t, modelIDs, "qwen")
}

func TestListModels_CacheHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handler := NewModelsHandler()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/models", nil)

	handler.ListModels(c)

	// Verify cache headers present
	assert.NotEmpty(t, w.Header().Get("Cache-Control"))
	assert.Contains(t, w.Header().Get("Cache-Control"), "max-age=3600")
	assert.Contains(t, w.Header().Get("Cache-Control"), "stale-while-revalidate=7200")
	assert.NotEmpty(t, w.Header().Get("ETag"))
	assert.NotEmpty(t, w.Header().Get("Last-Modified"))
}

func TestListModels_ResponseFormat(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handler := NewModelsHandler()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/models", nil)

	handler.ListModels(c)

	var response map[string][]map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Contains(t, response, "models")
}
