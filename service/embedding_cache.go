package service

import (
	"crypto/sha256"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/pkg/cachex"
	"github.com/samber/hot"
	"golang.org/x/sync/singleflight"
)

const (
	embeddingCacheNamespace = "new-api:embedding:v1"
	embeddingCacheTTL       = 600 * time.Second
	embeddingCacheCapacity  = 10_000
)

// EmbeddingCacheEntry 是 embedding 缓存的值，存储 DoResponse 输出的响应体字节和 usage
type EmbeddingCacheEntry struct {
	ResponseBody []byte    `json:"response_body"` // DoResponse 输出的响应体字节（发给客户端的实际内容）
	Usage        dto.Usage `json:"usage"`         // 用于后扣费
}

// EmbeddingFetchResult 是 singleflight 的返回值
type EmbeddingFetchResult struct {
	Body  []byte    // DoResponse 输出的响应体字节
	Usage dto.Usage // 上游返回的 usage
}

var (
	embeddingCacheOnce sync.Once
	embeddingCache     *cachex.HybridCache[EmbeddingCacheEntry]
	embeddingSFGroup   singleflight.Group
)

// EmbeddingCacheEnabled 返回是否启用 embedding 缓存
func EmbeddingCacheEnabled() bool {
	return os.Getenv("EMBEDDING_CACHE_ENABLED") != "false"
}

func getEmbeddingCache() *cachex.HybridCache[EmbeddingCacheEntry] {
	embeddingCacheOnce.Do(func() {
		embeddingCache = cachex.NewHybridCache(cachex.HybridCacheConfig[EmbeddingCacheEntry]{
			Namespace: cachex.Namespace(embeddingCacheNamespace),
			Redis:     common.RDB,
			RedisEnabled: func() bool {
				return common.RedisEnabled && common.RDB != nil
			},
			RedisCodec: cachex.JSONCodec[EmbeddingCacheEntry]{},
			Memory: func() *hot.HotCache[string, EmbeddingCacheEntry] {
				return hot.NewHotCache[string, EmbeddingCacheEntry](hot.LRU, embeddingCacheCapacity).
					WithTTL(embeddingCacheTTL).
					WithJanitor().
					Build()
			},
		})
	})
	return embeddingCache
}

// buildEmbeddingCacheKey 构建缓存键
// key 包含 token_key + model + input + encoding_format + dimensions
func buildEmbeddingCacheKey(tokenKey, model string, inputs []string, encodingFormat string, dimensions *int) string {
	dimsStr := ""
	if dimensions != nil {
		dimsStr = strconv.Itoa(*dimensions)
	}
	raw := fmt.Sprintf("%s:%s:%s:%s:%s", tokenKey, model, strings.Join(inputs, "\n"), encodingFormat, dimsStr)
	h := sha256.Sum256([]byte(raw))
	return fmt.Sprintf("emb:%x", h)
}

// LookupEmbeddingCache 查询缓存
func LookupEmbeddingCache(tokenKey, model string, inputs []string, encodingFormat string, dimensions *int) (entry EmbeddingCacheEntry, found bool) {
	if tokenKey == "" || len(inputs) == 0 {
		return
	}
	cache := getEmbeddingCache()
	key := buildEmbeddingCacheKey(tokenKey, model, inputs, encodingFormat, dimensions)
	var err error
	entry, found, err = cache.Get(key)
	if err != nil {
		common.SysError(fmt.Sprintf("embedding cache lookup error: %v", err))
	}
	return
}

// StoreEmbeddingCache 存入缓存
func StoreEmbeddingCache(tokenKey, model string, inputs []string, encodingFormat string, dimensions *int, body []byte, usage dto.Usage) {
	if tokenKey == "" || len(body) == 0 {
		return
	}
	cache := getEmbeddingCache()
	key := buildEmbeddingCacheKey(tokenKey, model, inputs, encodingFormat, dimensions)
	entry := EmbeddingCacheEntry{ResponseBody: body, Usage: usage}
	if err := cache.SetWithTTL(key, entry, embeddingCacheTTL); err != nil {
		common.SysError(fmt.Sprintf("embedding cache store error: %v", err))
	}
}

// ExecuteEmbeddingFetch 使用 singleflight 执行上游调用，防止并发击穿
// fetch 函数由调用方实现，包含 DoRequest + DoResponse + 缓存存储逻辑
// 返回值 shared=true 表示当前 caller 是 follower（结果来自 leader）
func ExecuteEmbeddingFetch(tokenKey, model string, inputs []string, encodingFormat string, dimensions *int, fetch func() (EmbeddingFetchResult, error)) (EmbeddingFetchResult, bool, error) {
	key := buildEmbeddingCacheKey(tokenKey, model, inputs, encodingFormat, dimensions)
	v, err, shared := embeddingSFGroup.Do(key, func() (interface{}, error) {
		return fetch()
	})
	if err != nil {
		return EmbeddingFetchResult{}, false, err
	}
	return v.(EmbeddingFetchResult), shared, nil
}
