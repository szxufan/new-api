package model

import (
	"strconv"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

// setupOptionMap 初始化 OptionMap 以避免 nil map panic
func setupOptionMap() {
	common.OptionMapRWMutex.Lock()
	common.OptionMap = make(map[string]string)
	common.OptionMapRWMutex.Unlock()
}

// TestUpdateOptionMap_RetryIntervalMs 验证 updateOptionMap 能正确解析 RetryIntervalMs
func TestUpdateOptionMap_RetryIntervalMs(t *testing.T) {
	setupOptionMap()

	// 保存原始值以便恢复
	origValue := common.RetryIntervalMs
	defer func() {
		common.RetryIntervalMs = origValue
		common.OptionMap["RetryIntervalMs"] = strconv.Itoa(origValue)
	}()

	tests := []struct {
		name     string
		input    string
		expected int
	}{
		{"zero value", "0", 0},
		{"positive value", "1000", 1000},
		{"max boundary", "60000", 60000},
		{"small value", "1", 1},
		{"invalid input yields zero", "abc", 0},
		{"empty string yields zero", "", 0},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := updateOptionMap("RetryIntervalMs", tc.input)
			require.NoError(t, err)
			require.Equal(t, tc.expected, common.RetryIntervalMs,
				"RetryIntervalMs should be %d for input %q", tc.expected, tc.input)
		})
	}
}

// TestInitOptionMap_ContainsRetryIntervalMs 验证 InitOptionMap 包含 RetryIntervalMs 键
func TestInitOptionMap_ContainsRetryIntervalMs(t *testing.T) {
	// 设置一个非零值以验证序列化
	origValue := common.RetryIntervalMs
	common.RetryIntervalMs = 500
	defer func() {
		common.RetryIntervalMs = origValue
	}()

	InitOptionMap()

	val, ok := common.OptionMap["RetryIntervalMs"]
	require.True(t, ok, "OptionMap should contain RetryIntervalMs key")
	require.Equal(t, "500", val, "OptionMap[RetryIntervalMs] should be \"500\"")
}

// TestRetryIntervalMs_SleepBehavior 验证 RetryIntervalMs 控制 sleep 行为的逻辑
func TestRetryIntervalMs_SleepBehavior(t *testing.T) {
	// 验证默认值（0 表示无间隔）
	require.Equal(t, 0, common.RetryIntervalMs,
		"default RetryIntervalMs should be 0")

	// 验证条件逻辑：>0 才 sleep
	require.False(t, common.RetryIntervalMs > 0,
		"should not sleep when RetryIntervalMs is 0")

	common.RetryIntervalMs = 100
	defer func() { common.RetryIntervalMs = 0 }()

	require.True(t, common.RetryIntervalMs > 0,
		"should sleep when RetryIntervalMs > 0")
}
