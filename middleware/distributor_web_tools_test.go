package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// TestGetModelRequestWebToolsDefaults 测试网页工具端点的默认虚拟模型解析：
// body 未指定 model 时按端点回填 deepseek-search / deepseek-fetch，显式指定时优先
func TestGetModelRequestWebToolsDefaults(t *testing.T) {
	cases := []struct {
		name      string
		path      string
		body      string
		wantModel string
	}{
		{name: "search 无 model 默认", path: "/v1/tools/search", body: `{"query":"golang"}`, wantModel: "deepseek-search"},
		{name: "fetch 无 model 默认", path: "/v1/tools/fetch", body: `{"url":"https://example.com"}`, wantModel: "deepseek-fetch"},
		{name: "search 显式 model", path: "/v1/tools/search", body: `{"model":"tavily-search","query":"golang"}`, wantModel: "tavily-search"},
		{name: "fetch 显式 model", path: "/v1/tools/fetch", body: `{"model":"jina-fetch","url":"https://example.com"}`, wantModel: "jina-fetch"},
		{name: "playground search 默认", path: "/pg/tools/search", body: `{"query":"golang"}`, wantModel: "deepseek-search"},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			req := httptest.NewRequest(http.MethodPost, tt.path, strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			c.Request = req

			modelRequest, shouldSelectChannel, err := getModelRequest(c)
			require.NoError(t, err)
			require.True(t, shouldSelectChannel)
			require.Equal(t, tt.wantModel, modelRequest.Model)
		})
	}
}
