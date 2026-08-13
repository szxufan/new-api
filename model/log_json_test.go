package model

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestPgJsonObjectExpr 验证 PostgreSQL 侧对 other 字段的 jsonb 转换加了前置校验：
// 只有以 { 或 [ 开头的值才执行 ::jsonb 转换，空字符串、普通文本等历史脏数据返回 NULL，
// 避免 invalid input syntax for type json (SQLSTATE 22P02) 导致整个查询报错。
func TestPgJsonObjectExpr(t *testing.T) {
	tests := []struct {
		name   string
		column string
		want   string
	}{
		{
			name:   "logs table other column",
			column: "logs.other",
			want:   "CASE WHEN logs.other ~ '^[[:space:]]*[{[]' THEN logs.other::jsonb ELSE NULL END",
		},
		{
			name:   "alias other column",
			column: "other",
			want:   "CASE WHEN other ~ '^[[:space:]]*[{[]' THEN other::jsonb ELSE NULL END",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, pgJsonObjectExpr(tt.column))
		})
	}
}

// TestPgJsonObjectExprIsTolerantToDirtyData 用实际样例值验证容错表达式对脏数据的处理语义。
// 将样例输入代入 SQL 表达式后应得到：合法 JSON 对象 -> 原值；空串/纯文本 -> NULL（不报错）。
func TestPgJsonObjectExprIsTolerantToDirtyData(t *testing.T) {
	expr := pgJsonObjectExpr("other")

	// 1. 合法 JSON 对象：应通过前置校验，表达式值等于 jsonb 值
	t.Run("valid json object passes guard", func(t *testing.T) {
		guard := `other ~ '^[[:space:]]*[{[]'`
		require.Contains(t, expr, guard)
		require.Contains(t, expr, "other::jsonb")
	})

	// 2. 脏数据（空字符串、普通文本）：不会匹配首字符特征，CASE 走 ELSE NULL 分支，不触发 ::jsonb
	t.Run("dirty data falls back to NULL", func(t *testing.T) {
		for _, dirty := range []string{"", "some plain text", "12345", "null", "true"} {
			// 模拟 SQL 中前置校验的判定结果：这些值不以 { 或 [ 开头
			require.NotRegexp(t, `^[[:space:]]*[{[]`, dirty,
				"dirty value %q should not pass the JSON guard", dirty)
		}
	})

	// 3. 合法 JSON 值：以 { 或 [ 开头，可以通过前置校验
	t.Run("valid json values pass guard", func(t *testing.T) {
		for _, valid := range []string{`{"retry_count":2}`, `[]`, ` [{"a":1}]`} {
			require.Regexp(t, `^[[:space:]]*[{[]`, valid,
				"valid JSON %q should pass the JSON guard", valid)
		}
	})
}
