package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestDistribute_DashScopeInferenceDefersMissingModel(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(Distribute())
	called := false
	router.GET("/api-ws/v1/inference", func(c *gin.Context) {
		called = true
		c.Status(http.StatusNoContent)
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api-ws/v1/inference", nil)
	router.ServeHTTP(recorder, request)

	assert.True(t, called, "missing query model should defer rather than reject before the first frame")
	assert.Equal(t, http.StatusNoContent, recorder.Code)
}

func TestAbortWithOpenAiMessage_DashScopeWebSocketWriter(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/api-ws/v1/inference", nil)

	var captured *types.NewAPIError
	c.Set("dashscope_inference_error_writer", func(apiErr *types.NewAPIError) {
		captured = apiErr
	})

	abortWithOpenAiMessage(c, http.StatusServiceUnavailable, "no available channel", types.ErrorCodeModelNotFound)

	assert.True(t, c.IsAborted())
	assert.NotNil(t, captured)
	assert.Equal(t, http.StatusServiceUnavailable, captured.StatusCode)
	assert.Equal(t, types.ErrorCodeModelNotFound, captured.GetErrorCode())
	assert.Contains(t, captured.Error(), "no available channel")
	assert.Empty(t, recorder.Body.String(), "upgraded WebSocket requests must not receive an HTTP JSON error")
}
