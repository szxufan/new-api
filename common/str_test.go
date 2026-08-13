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
