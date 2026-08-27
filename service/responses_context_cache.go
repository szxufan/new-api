package service

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/pkg/cachex"
	"github.com/samber/hot"
)

const (
	responsesContextNamespace = "new-api:responses_context:v1"
	responsesContextTTL       = 7 * 24 * time.Hour // 7 天
	responsesContextCapacity  = 10_000
)

var (
	responsesContextOnce  sync.Once
	responsesContextCache *cachex.HybridCache[dto.ResponsesContextEntry]
)

func getResponsesContextCache() *cachex.HybridCache[dto.ResponsesContextEntry] {
	responsesContextOnce.Do(func() {
		responsesContextCache = cachex.NewHybridCache(cachex.HybridCacheConfig[dto.ResponsesContextEntry]{
			Namespace: cachex.Namespace(responsesContextNamespace),
			Redis:     common.RDB,
			RedisEnabled: func() bool {
				return common.RedisEnabled && common.RDB != nil
			},
			RedisCodec: cachex.JSONCodec[dto.ResponsesContextEntry]{},
			Memory: func() *hot.HotCache[string, dto.ResponsesContextEntry] {
				return hot.NewHotCache[string, dto.ResponsesContextEntry](hot.LRU, responsesContextCapacity).
					WithTTL(responsesContextTTL).
					WithJanitor().
					Build()
			},
		})
	})
	return responsesContextCache
}

// StoreResponsesContext 存储响应上下文（仅当 store=true 时调用）
// responseID: 响应 ID，客户端后续请求中作为 previous_response_id 使用
// entry: 包含模型、指令、输入、输出和工具的上下文信息
func StoreResponsesContext(responseID string, entry *dto.ResponsesContextEntry) {
	if responseID == "" || entry == nil {
		return
	}
	cache := getResponsesContextCache()
	_ = cache.SetWithTTL(responseID, *entry, responsesContextTTL)
}

// LookupResponsesContext 查询响应上下文
// responseID: 之前响应的 ID（即 previous_response_id）
// 返回: 上下文条目和是否找到
func LookupResponsesContext(responseID string) (*dto.ResponsesContextEntry, bool) {
	if responseID == "" {
		return nil, false
	}
	cache := getResponsesContextCache()
	entry, found, err := cache.Get(responseID)
	if err != nil || !found {
		return nil, false
	}
	return &entry, true
}

// ParseStoreField 解析请求中的 store 字段
// storeRaw: 请求中的 store 字段（json.RawMessage 类型）
// 返回: 是否应该存储上下文
func ParseStoreField(storeRaw json.RawMessage) bool {
	if len(storeRaw) == 0 {
		return false // 默认不缓存
	}
	var store bool
	if err := json.Unmarshal(storeRaw, &store); err != nil {
		return false
	}
	return store
}