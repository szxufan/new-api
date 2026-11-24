package common

import "testing"

// 锁定 conditions 在 object/array 上的既有比较语义：
// 除数值比较外，字段值先被 Result.String() 转成文本再比较，
// 对象/数组的文本形式即其原始 JSON 原文。
func TestConditionCompareOnObjectAndArray(t *testing.T) {
	data := []byte(`{"obj":{"a":1,"b":"x"},"arr":[1,10],"num":10,"nil":null}`)

	cases := []struct {
		name    string
		cond    ConditionOperation
		want    bool
		wantErr bool
	}{
		{
			name: "object contains key name",
			cond: ConditionOperation{Path: "obj", Mode: "contains", Value: "a"},
			want: true,
		},
		{
			name: "object contains value text",
			cond: ConditionOperation{Path: "obj", Mode: "contains", Value: "x"},
			want: true,
		},
		{
			name: "object contains raw fragment",
			cond: ConditionOperation{Path: "obj", Mode: "contains", Value: `"a":1`},
			want: true,
		},
		{
			name: "object full same object",
			cond: ConditionOperation{Path: "obj", Mode: "full", Value: map[string]interface{}{"a": 1, "b": "x"}},
			want: true,
		},
		{
			name:    "object full with mismatched type fails request",
			cond:    ConditionOperation{Path: "obj", Mode: "full", Value: "x"},
			want:    false,
			wantErr: true,
		},
		{
			name: "array contains substring",
			cond: ConditionOperation{Path: "arr", Mode: "contains", Value: "0"},
			want: true,
		},
		{
			name: "array full same array",
			cond: ConditionOperation{Path: "arr", Mode: "full", Value: []interface{}{1, 10}},
			want: true,
		},
		{
			name: "number contains digit text",
			cond: ConditionOperation{Path: "num", Mode: "contains", Value: "0"},
			want: true,
		},
		{
			name: "null contains empty string",
			cond: ConditionOperation{Path: "nil", Mode: "contains", Value: ""},
			want: true,
		},
		{
			name: "missing path does not match",
			cond: ConditionOperation{Path: "missing", Mode: "contains", Value: "x"},
			want: false,
		},
		{
			name: "missing path passes when pass_missing_key",
			cond: ConditionOperation{Path: "missing", Mode: "contains", Value: "x", PassMissingKey: true},
			want: true,
		},
		{
			name:    "numeric mode with non-number field fails request",
			cond:    ConditionOperation{Path: "obj", Mode: "gt", Value: 1},
			want:    false,
			wantErr: true,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := checkSingleCondition(data, "", c.cond)
			if c.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil (matched=%v)", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != c.want {
				t.Fatalf("matched=%v, want %v", got, c.want)
			}
		})
	}
}

// full 对对象/数组按原始 JSON 文本比较，因此键顺序不同即不相等。
func TestConditionFullComparesRawJSONText(t *testing.T) {
	data := []byte(`{"obj":{"b":"x","a":1}}`)
	cond := ConditionOperation{
		Path:  "obj",
		Mode:  "full",
		Value: map[string]interface{}{"a": 1, "b": "x"},
	}

	matched, err := checkSingleCondition(data, "", cond)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if matched {
		t.Fatal("expected key order to matter for raw JSON text comparison")
	}
}
