package service

import (
	"fmt"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/pkg/cachex"

	"github.com/samber/hot"
)

// 虚拟模型速度模式"胜者抢跑"记录：
// 按 Token+虚拟模型名 粒度记录上次竞速胜出的模型+渠道，
// 下次请求优先只调用该分支，超时后再并发其余分支（见 relay/virtual_handler.go）。

const (
	virtualModelWinnerCacheNamespace = "new-api:virtual_model_winner:v1"
	virtualModelWinnerTTLSeconds     = 3600 // 每次竞速覆盖刷新，TTL 兜底 1 小时
	virtualModelWinnerCacheCapacity  = 100_000
)

// VirtualWinnerRecord 一次竞速的胜出记录
type VirtualWinnerRecord struct {
	Model     string `json:"model"`
	ChannelId int    `json:"channel_id"`
}

var (
	virtualModelWinnerCache     *cachex.HybridCache[VirtualWinnerRecord]
	virtualModelWinnerCacheOnce sync.Once
)

func getVirtualModelWinnerCache() *cachex.HybridCache[VirtualWinnerRecord] {
	virtualModelWinnerCacheOnce.Do(func() {
		virtualModelWinnerCache = cachex.NewHybridCache(cachex.HybridCacheConfig[VirtualWinnerRecord]{
			Namespace: cachex.Namespace(virtualModelWinnerCacheNamespace),
			Redis:     common.RDB,
			RedisEnabled: func() bool {
				return common.RedisEnabled && common.RDB != nil
			},
			RedisCodec: cachex.JSONCodec[VirtualWinnerRecord]{},
			Memory: func() *hot.HotCache[string, VirtualWinnerRecord] {
				return hot.NewHotCache[string, VirtualWinnerRecord](hot.LRU, virtualModelWinnerCacheCapacity).
					WithTTL(time.Duration(virtualModelWinnerTTLSeconds) * time.Second).
					WithJanitor().
					Build()
			},
		})
	})
	return virtualModelWinnerCache
}

// virtualModelWinnerKey 缓存键后缀（统一由 HybridCache 内部 FullKey 加前缀）。
// tokenId==0（内部/旁路场景）退化为仅按虚拟模型名共享记录。
func virtualModelWinnerKey(tokenId int, vmName string) string {
	return fmt.Sprintf("%d:%s", tokenId, vmName)
}

// GetVirtualModelWinner 读取胜者记录。对外保持两值：缓存故障（如 Redis 不可用）
// 吞掉 error 降级为"未命中"，调用方回退正常竞速，不影响主流程。
func GetVirtualModelWinner(tokenId int, vmName string) (*VirtualWinnerRecord, bool) {
	rec, found, err := getVirtualModelWinnerCache().Get(virtualModelWinnerKey(tokenId, vmName))
	if err != nil || !found {
		return nil, false
	}
	return &rec, true
}

// RecordVirtualModelWinner 覆盖写入胜者记录并刷新 TTL。
// 异步调用场景（gopool）使用 context.Background()，不得使用请求级 context。
func RecordVirtualModelWinner(tokenId int, vmName, modelName string, channelId int) {
	_ = getVirtualModelWinnerCache().SetWithTTL(
		virtualModelWinnerKey(tokenId, vmName),
		VirtualWinnerRecord{Model: modelName, ChannelId: channelId},
		time.Duration(virtualModelWinnerTTLSeconds)*time.Second,
	)
}

// DeleteVirtualModelWinner 删除胜者记录（记录渠道失效时清理）
func DeleteVirtualModelWinner(tokenId int, vmName string) {
	_, _ = getVirtualModelWinnerCache().DeleteMany([]string{virtualModelWinnerKey(tokenId, vmName)})
}
