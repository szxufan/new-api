package common

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMapToJsonStr(t *testing.T) {
	tests := []struct {
		name string
		m    map[string]interface{}
		want string
	}{
		{
			name: "empty map",
			m:    map[string]interface{}{},
			want: "{}",
		},
		{
			name: "nil map",
			m:    nil,
			want: "null",
		},
		{
			name: "normal map",
			m: map[string]interface{}{
				"retry_count": 2,
				"node_name":   "node-1",
			},
			want: `{"node_name":"node-1","retry_count":2}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, MapToJsonStr(tt.m))
		})
	}
}

// TestMapToJsonStr_MarshalError 验证序列化失败（如含有不支持序列化的值）时返回合法 JSON "{}"，
// 而不是空字符串，避免 PostgreSQL 端 ::jsonb 转换报 SQLSTATE 22P02。
func TestMapToJsonStr_MarshalError(t *testing.T) {
	m := map[string]interface{}{
		"bad": make(chan int),
	}
	require.Equal(t, "{}", MapToJsonStr(m))
}

// TestIsSensitiveQueryParam 验证敏感查询参数名的判断规则：大小写不敏感的子串匹配。
func TestIsSensitiveQueryParam(t *testing.T) {
	tests := []struct {
		name        string
		key         string
		isSensitive bool
	}{
		{name: "empty", key: "", isSensitive: false},
		{name: "key", key: "key", isSensitive: true},
		{name: "access_key", key: "access_key", isSensitive: true},
		{name: "client_secret", key: "client_secret", isSensitive: true},
		{name: "AUTH_TOKEN", key: "AUTH_TOKEN", isSensitive: true},
		{name: "password", key: "password", isSensitive: true},
		{name: "signature", key: "signature", isSensitive: true},
		{name: "model", key: "model", isSensitive: false},
		{name: "page", key: "page", isSensitive: false},
		{name: "timestamp", key: "timestamp", isSensitive: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.isSensitive, isSensitiveQueryParam(tt.key))
		})
	}
}

// TestMaskSensitiveInfo 验证打码规则：
// URL 仅打码敏感查询参数（key/secret/token 等），host、path 与普通参数保留明文；
// 裸 IP 与 api_key:xxx 仍然打码；域名不再打码。
func TestMaskSensitiveInfo(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "sensitive query param masked, host and path kept",
			in:   "error: https://api.test.org/v1/users/123?key=secret please retry",
			want: "error: https://api.test.org/v1/users/123?key=*** please retry",
		},
		{
			name: "normal query params kept as-is",
			in:   "https://api.test.org/v1/chat?model=gpt-4&limit=10",
			want: "https://api.test.org/v1/chat?model=gpt-4&limit=10",
		},
		{
			name: "mixed params: only sensitive one masked",
			in:   "https://sub.domain.co.uk/path?id=1&token=abc",
			want: "https://sub.domain.co.uk/path?id=1&token=***",
		},
		{
			name: "url without query unchanged",
			in:   "post https://api.openai.com/v1/chat/completions failed",
			want: "post https://api.openai.com/v1/chat/completions failed",
		},
		{
			name: "domain without protocol unchanged",
			in:   "connect to openai.com failed",
			want: "connect to openai.com failed",
		},
		{
			name: "ip still masked",
			in:   "dial 192.168.1.1:443 refused",
			want: "dial ***.***.***.***:443 refused",
		},
		{
			name: "api_key still masked",
			in:   `api_key:AIzaSyAAAaUooTUni8AdaOkSRMda30n_Q4vrV70`,
			want: `api_key:***`,
		},
		{
			name: "empty string",
			in:   "",
			want: "",
		},
		{
			name: "unparseable query masked entirely",
			in:   "https://api.test.org/path?%%%",
			want: "https://api.test.org/path?***",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, MaskSensitiveInfo(tt.in))
		})
	}
}
