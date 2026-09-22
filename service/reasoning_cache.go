package service

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/pkg/cachex"
	"github.com/samber/hot"

	"github.com/gin-gonic/gin"
)

const (
	reasoningCacheNamespace = "new-api:reasoning_content:v1"
	reasoningCacheTTL       = 30 * time.Minute
	reasoningCacheCapacity  = 50_000
)

type ReasoningCacheEntry struct {
	ReasoningContent string `json:"reasoning_content"`
}

var (
	reasoningCacheOnce sync.Once
	reasoningCache     *cachex.HybridCache[ReasoningCacheEntry]
)

func getReasoningCache() *cachex.HybridCache[ReasoningCacheEntry] {
	reasoningCacheOnce.Do(func() {
		reasoningCache = cachex.NewHybridCache(cachex.HybridCacheConfig[ReasoningCacheEntry]{
			Namespace: cachex.Namespace(reasoningCacheNamespace),
			Redis:     common.RDB,
			RedisEnabled: func() bool {
				return common.RedisEnabled && common.RDB != nil
			},
			RedisCodec: cachex.JSONCodec[ReasoningCacheEntry]{},
			Memory: func() *hot.HotCache[string, ReasoningCacheEntry] {
				return hot.NewHotCache[string, ReasoningCacheEntry](hot.LRU, reasoningCacheCapacity).
					WithTTL(reasoningCacheTTL).
					WithJanitor().
					Build()
			},
		})
	})
	return reasoningCache
}

func reasoningCacheKey(tokenKey string, content string, toolCalls json.RawMessage) string {
	h := sha256.New()
	h.Write([]byte(content))
	h.Write(toolCalls)
	hash := hex.EncodeToString(h.Sum(nil))
	return fmt.Sprintf("rc:%s:%s", tokenKey, hash)
}

func StoreReasoningContent(tokenKey string, content string, toolCalls json.RawMessage, reasoningContent string) {
	if tokenKey == "" || reasoningContent == "" {
		return
	}
	cache := getReasoningCache()
	key := reasoningCacheKey(tokenKey, content, toolCalls)
	entry := ReasoningCacheEntry{ReasoningContent: reasoningContent}
	_ = cache.SetWithTTL(key, entry, reasoningCacheTTL)
}

func LookupReasoningContent(tokenKey string, content string, toolCalls json.RawMessage) (string, bool) {
	if tokenKey == "" {
		return "", false
	}
	cache := getReasoningCache()
	key := reasoningCacheKey(tokenKey, content, toolCalls)
	entry, found, err := cache.Get(key)
	if err != nil || !found {
		return "", false
	}
	return entry.ReasoningContent, true
}

// ReasoningFillStats 记录一次请求中 reasoning_content 回填的条数与来源，
// 由各入口的 fillReasoningContent* 写入，经 GenerateTextOtherInfo 落到日志 other.reasoning_fill。
type ReasoningFillStats struct {
	Filled    int `json:"filled"`
	FromTag   int `json:"from_tag"`
	FromCache int `json:"from_cache"`
	FromEmpty int `json:"from_empty"`
}

// RecordReasoningFillStats 将回填统计写入 gin context（多次调用以最后一次为准）。
func RecordReasoningFillStats(c *gin.Context, stats ReasoningFillStats) {
	if c == nil || stats.Filled <= 0 {
		return
	}
	common.SetContextKey(c, constant.ContextKeyReasoningFillStats, stats)
}

func GetReasoningFillStats(c *gin.Context) (ReasoningFillStats, bool) {
	if c == nil {
		return ReasoningFillStats{}, false
	}
	v, ok := common.GetContextKey(c, constant.ContextKeyReasoningFillStats)
	if !ok {
		return ReasoningFillStats{}, false
	}
	stats, ok := v.(ReasoningFillStats)
	return stats, ok
}
