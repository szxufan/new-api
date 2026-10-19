package helper

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/dto"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func newWebToolsTestContext(path, body string) *gin.Context {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	c.Request = req
	return c
}

// TestGetAndValidateWebToolsRequest_Search 测试搜索端点请求校验
func TestGetAndValidateWebToolsRequest_Search(t *testing.T) {
	request, err := GetAndValidateWebToolsRequest(
		newWebToolsTestContext("/v1/tools/search", `{"query":"golang","count":3,"need_content":true}`),
		relayconstant.RelayModeWebSearch)
	require.NoError(t, err)
	searchReq, ok := request.(*dto.WebSearchRequest)
	require.True(t, ok)
	require.Equal(t, "golang", searchReq.Query)
	require.NotNil(t, searchReq.Count)
	require.Equal(t, 3, *searchReq.Count)
	require.NotNil(t, searchReq.NeedContent)
	require.True(t, *searchReq.NeedContent)
}

// TestGetAndValidateWebToolsRequest_Fetch 测试抓取端点请求校验
func TestGetAndValidateWebToolsRequest_Fetch(t *testing.T) {
	request, err := GetAndValidateWebToolsRequest(
		newWebToolsTestContext("/v1/tools/fetch", `{"url":"https://example.com/page"}`),
		relayconstant.RelayModeWebFetch)
	require.NoError(t, err)
	fetchReq, ok := request.(*dto.WebFetchRequest)
	require.True(t, ok)
	require.Equal(t, "https://example.com/page", fetchReq.Url)
}

// TestGetAndValidateWebToolsRequest_Invalid 测试非法请求被拒绝
func TestGetAndValidateWebToolsRequest_Invalid(t *testing.T) {
	cases := []struct {
		name      string
		relayMode int
		body      string
	}{
		{"search 缺 query", relayconstant.RelayModeWebSearch, `{"count":3}`},
		{"search query 超长", relayconstant.RelayModeWebSearch, `{"query":"` + strings.Repeat("a", 129) + `"}`},
		{"search count 越界", relayconstant.RelayModeWebSearch, `{"query":"go","count":51}`},
		{"fetch 缺 url", relayconstant.RelayModeWebFetch, `{}`},
		{"fetch url 非法", relayconstant.RelayModeWebFetch, `{"url":"ftp://example.com"}`},
		{"未知 relay mode", relayconstant.RelayModeUnknown, `{}`},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			_, err := GetAndValidateWebToolsRequest(newWebToolsTestContext("/v1/tools", tt.body), tt.relayMode)
			require.Error(t, err)
		})
	}
}
