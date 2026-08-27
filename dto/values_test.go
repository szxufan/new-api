package dto

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
)

// TestIntValueUnmarshal 覆盖 dto.IntValue 的宽容解析：
// 整数、浮点数（如上游返回 10.0）、字符串整数、字符串浮点数均应成功，
// 非法值应报错。
func TestIntValueUnmarshal(t *testing.T) {
	tests := []struct {
		name    string
		json    string
		want    int
		wantErr bool
	}{
		{"int", `{"v": 10}`, 10, false},
		{"float", `{"v": 10.0}`, 10, false},
		{"float_fractional_truncates", `{"v": 9.9}`, 9, false},
		{"string_int", `{"v": "10"}`, 10, false},
		{"string_float", `{"v": "10.0"}`, 10, false},
		{"zero", `{"v": 0}`, 0, false},
		{"invalid_string", `{"v": "abc"}`, 0, true},
		{"invalid_object", `{"v": {}}`, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var payload struct {
				V IntValue `json:"v"`
			}
			err := common.Unmarshal([]byte(tt.json), &payload)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error for %s, got nil (value=%d)", tt.json, int(payload.V))
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error for %s: %v", tt.json, err)
			}
			if int(payload.V) != tt.want {
				t.Errorf("value mismatch for %s: got %d, want %d", tt.json, int(payload.V), tt.want)
			}
		})
	}
}

// TestIntValueMarshal 验证序列化仍输出整数
func TestIntValueMarshal(t *testing.T) {
	payload := struct {
		V IntValue `json:"v"`
	}{V: 10}
	data, err := common.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	if string(data) != `{"v":10}` {
		t.Errorf("marshal mismatch: got %s, want {\"v\":10}", string(data))
	}
}
