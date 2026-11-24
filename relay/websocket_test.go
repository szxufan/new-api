package relay

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestWssHelper_RejectsHTTPAdaptorResponseWithoutPanic(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service.InitHttpClient()
	upstreamCalled := false
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamCalled = true
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{}`))
	}))
	defer upstream.Close()

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "/api-ws/v1/inference?model=sambert-zhichu-v1", nil)
	common.SetContextKey(c, constant.ContextKeyChannelType, constant.ChannelTypeOpenAI)
	common.SetContextKey(c, constant.ContextKeyChannelBaseUrl, upstream.URL)
	common.SetContextKey(c, constant.ContextKeyChannelKey, "test-key")
	common.SetContextKey(c, constant.ContextKeyOriginalModel, "sambert-zhichu-v1")

	info := &relaycommon.RelayInfo{
		RelayMode:      relayconstant.RelayModeDashScopeInference,
		RelayFormat:    types.RelayFormatDashScopeInference,
		RequestURLPath: c.Request.URL.Path,
	}

	var apiErr *types.NewAPIError
	assert.NotPanics(t, func() {
		apiErr = WssHelper(c, info)
	})
	assert.True(t, upstreamCalled, "the non-WebSocket adaptor should return an HTTP response before rejection")
	assert.NotNil(t, apiErr)
	assert.Contains(t, apiErr.Error(), "websocket relay")
}
