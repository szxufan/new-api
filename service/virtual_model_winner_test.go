package service

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 内存模式（测试环境无 Redis）下的胜者记录读写
func TestVirtualModelWinnerRecordAndGet(t *testing.T) {
	vmName := "vm-winner-test-record"

	// 初始未命中
	_, ok := GetVirtualModelWinner(1001, vmName)
	assert.False(t, ok)

	// 记录后命中
	RecordVirtualModelWinner(1001, vmName, "gpt-4o", 42)
	rec, ok := GetVirtualModelWinner(1001, vmName)
	require.True(t, ok)
	assert.Equal(t, "gpt-4o", rec.Model)
	assert.Equal(t, 42, rec.ChannelId)

	// 覆盖写入
	RecordVirtualModelWinner(1001, vmName, "gpt-4o-mini", 43)
	rec, ok = GetVirtualModelWinner(1001, vmName)
	require.True(t, ok)
	assert.Equal(t, "gpt-4o-mini", rec.Model)
	assert.Equal(t, 43, rec.ChannelId)
}

// key 粒度：不同 tokenId / 不同虚拟模型名互不影响
func TestVirtualModelWinnerKeyIsolation(t *testing.T) {
	vmName := "vm-winner-test-isolation"
	RecordVirtualModelWinner(2001, vmName, "gpt-4o", 11)
	RecordVirtualModelWinner(2002, vmName, "gpt-4o-mini", 22)
	RecordVirtualModelWinner(2001, "vm-winner-test-isolation-2", "claude-3", 33)

	rec, ok := GetVirtualModelWinner(2001, vmName)
	require.True(t, ok)
	assert.Equal(t, 11, rec.ChannelId)

	rec, ok = GetVirtualModelWinner(2002, vmName)
	require.True(t, ok)
	assert.Equal(t, 22, rec.ChannelId)

	rec, ok = GetVirtualModelWinner(2001, "vm-winner-test-isolation-2")
	require.True(t, ok)
	assert.Equal(t, "claude-3", rec.Model)
}

// tokenId=0 降级：退化为仅按虚拟模型名的共享记录
func TestVirtualModelWinnerTokenIdZeroFallback(t *testing.T) {
	vmName := "vm-winner-test-zero"
	RecordVirtualModelWinner(0, vmName, "gpt-4o", 55)
	rec, ok := GetVirtualModelWinner(0, vmName)
	require.True(t, ok)
	assert.Equal(t, 55, rec.ChannelId)
}

// 删除后未命中
func TestVirtualModelWinnerDelete(t *testing.T) {
	vmName := "vm-winner-test-delete"
	RecordVirtualModelWinner(3001, vmName, "gpt-4o", 66)
	_, ok := GetVirtualModelWinner(3001, vmName)
	require.True(t, ok)

	DeleteVirtualModelWinner(3001, vmName)
	_, ok = GetVirtualModelWinner(3001, vmName)
	assert.False(t, ok)
}
