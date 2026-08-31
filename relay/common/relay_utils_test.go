package common

import "testing"

// TestResolveRequestedDuration 锚定请求时长的解析优先级：
// duration > seconds > metadata.durationSeconds；-1（智能时长）原样返回。
func TestResolveRequestedDuration(t *testing.T) {
	tests := []struct {
		name string
		req  TaskSubmitReq
		want int
	}{
		{
			name: "duration 优先",
			req:  TaskSubmitReq{Duration: 5, Seconds: "8"},
			want: 5,
		},
		{
			name: "seconds 字符串兜底",
			req:  TaskSubmitReq{Seconds: "8"},
			want: 8,
		},
		{
			name: "seconds 非法值忽略",
			req:  TaskSubmitReq{Seconds: "abc", Duration: 0},
			want: 0,
		},
		{
			name: "metadata.durationSeconds 整数",
			req:  TaskSubmitReq{Metadata: map[string]any{"durationSeconds": 6}},
			want: 6,
		},
		{
			name: "metadata.durationSeconds JSON 数字（float64）",
			req:  TaskSubmitReq{Metadata: map[string]any{"durationSeconds": float64(8)}},
			want: 8,
		},
		{
			name: "metadata.durationSeconds 字符串",
			req:  TaskSubmitReq{Metadata: map[string]any{"durationSeconds": "7"}},
			want: 7,
		},
		{
			name: "智能时长 -1 原样返回",
			req:  TaskSubmitReq{Duration: -1},
			want: -1,
		},
		{
			name: "未指定返回 0",
			req:  TaskSubmitReq{},
			want: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ResolveRequestedDuration(tt.req); got != tt.want {
				t.Errorf("ResolveRequestedDuration() = %d, want %d", got, tt.want)
			}
		})
	}
}
