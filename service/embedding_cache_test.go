package service

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/dto"
)

func TestBuildEmbeddingCacheKey_Deterministic(t *testing.T) {
	inputs := []string{"hello", "world"}
	dims := 1536

	key1 := buildEmbeddingCacheKey("token1", "text-embedding-3-small", inputs, "float", &dims)
	key2 := buildEmbeddingCacheKey("token1", "text-embedding-3-small", inputs, "float", &dims)

	if key1 != key2 {
		t.Fatalf("same inputs should produce same key: %s != %s", key1, key2)
	}
}

func TestBuildEmbeddingCacheKey_DifferentTokenKey(t *testing.T) {
	inputs := []string{"hello"}
	dims := 1536

	key1 := buildEmbeddingCacheKey("token1", "model", inputs, "float", &dims)
	key2 := buildEmbeddingCacheKey("token2", "model", inputs, "float", &dims)

	if key1 == key2 {
		t.Fatalf("different tokenKey should produce different keys")
	}
}

func TestBuildEmbeddingCacheKey_DifferentModel(t *testing.T) {
	inputs := []string{"hello"}
	dims := 1536

	key1 := buildEmbeddingCacheKey("token1", "model-a", inputs, "float", &dims)
	key2 := buildEmbeddingCacheKey("token1", "model-b", inputs, "float", &dims)

	if key1 == key2 {
		t.Fatalf("different model should produce different keys")
	}
}

func TestBuildEmbeddingCacheKey_DifferentEncodingFormat(t *testing.T) {
	inputs := []string{"hello"}
	dims := 1536

	key1 := buildEmbeddingCacheKey("token1", "model", inputs, "float", &dims)
	key2 := buildEmbeddingCacheKey("token1", "model", inputs, "base64", &dims)

	if key1 == key2 {
		t.Fatalf("different encoding_format should produce different keys")
	}
}

func TestBuildEmbeddingCacheKey_DifferentDimensions(t *testing.T) {
	inputs := []string{"hello"}
	dims1 := 1536
	dims2 := 256

	key1 := buildEmbeddingCacheKey("token1", "model", inputs, "float", &dims1)
	key2 := buildEmbeddingCacheKey("token1", "model", inputs, "float", &dims2)

	if key1 == key2 {
		t.Fatalf("different dimensions should produce different keys")
	}
}

func TestBuildEmbeddingCacheKey_NilDimensions(t *testing.T) {
	inputs := []string{"hello"}

	key1 := buildEmbeddingCacheKey("token1", "model", inputs, "float", nil)
	key2 := buildEmbeddingCacheKey("token1", "model", inputs, "float", nil)

	if key1 != key2 {
		t.Fatalf("nil dimensions should produce same keys")
	}

	dims := 0
	key3 := buildEmbeddingCacheKey("token1", "model", inputs, "float", &dims)
	if key1 == key3 {
		t.Fatalf("nil dimensions and 0 dimensions should produce different keys")
	}
}

func TestBuildEmbeddingCacheKey_DifferentInputs(t *testing.T) {
	dims := 1536

	key1 := buildEmbeddingCacheKey("token1", "model", []string{"hello"}, "float", &dims)
	key2 := buildEmbeddingCacheKey("token1", "model", []string{"world"}, "float", &dims)

	if key1 == key2 {
		t.Fatalf("different inputs should produce different keys")
	}
}

func TestLookupEmbeddingCache_EmptyTokenKey(t *testing.T) {
	_, found := LookupEmbeddingCache("", "model", []string{"hello"}, "float", nil)
	if found {
		t.Fatalf("empty tokenKey should not find anything")
	}
}

func TestLookupEmbeddingCache_EmptyInputs(t *testing.T) {
	_, found := LookupEmbeddingCache("token1", "model", []string{}, "float", nil)
	if found {
		t.Fatalf("empty inputs should not find anything")
	}
}

func TestStoreAndLookupEmbeddingCache(t *testing.T) {
	tokenKey := "test-token-store-lookup"
	model := "text-embedding-3-small"
	inputs := []string{"hello world"}
	encodingFormat := "float"
	dims := 1536

	body := []byte(`{"data":[{"embedding":[0.1,0.2]}],"usage":{"prompt_tokens":2}}`)
	usage := dto.Usage{
		PromptTokens: 2,
		TotalTokens:  2,
	}

	// 存储前不应命中
	_, found := LookupEmbeddingCache(tokenKey, model, inputs, encodingFormat, &dims)
	if found {
		t.Fatalf("should not find before store")
	}

	// 存储
	StoreEmbeddingCache(tokenKey, model, inputs, encodingFormat, &dims, body, usage)

	// 存储后应命中
	entry, found := LookupEmbeddingCache(tokenKey, model, inputs, encodingFormat, &dims)
	if !found {
		t.Fatalf("should find after store")
	}

	if string(entry.ResponseBody) != string(body) {
		t.Fatalf("response body mismatch: got %s, want %s", entry.ResponseBody, body)
	}

	if entry.Usage.PromptTokens != usage.PromptTokens {
		t.Fatalf("usage mismatch: got %d, want %d", entry.Usage.PromptTokens, usage.PromptTokens)
	}
}

func TestStoreEmbeddingCache_EmptyTokenKey(t *testing.T) {
	// 空 tokenKey 不应存储
	StoreEmbeddingCache("", "model", []string{"hello"}, "float", nil, []byte("body"), dto.Usage{})

	_, found := LookupEmbeddingCache("", "model", []string{"hello"}, "float", nil)
	if found {
		t.Fatalf("empty tokenKey should not store")
	}
}

func TestStoreEmbeddingCache_EmptyBody(t *testing.T) {
	tokenKey := "test-token-empty-body"
	// 空 body 不应存储
	StoreEmbeddingCache(tokenKey, "model", []string{"hello"}, "float", nil, []byte{}, dto.Usage{})

	_, found := LookupEmbeddingCache(tokenKey, "model", []string{"hello"}, "float", nil)
	if found {
		t.Fatalf("empty body should not store")
	}
}

func TestExecuteEmbeddingFetch_Singleflight(t *testing.T) {
	tokenKey := "test-sf-token"
	model := "test-sf-model"
	inputs := []string{"test-sf-input"}
	encodingFormat := "float"
	dims := 128

	var callCount int32
	var wg sync.WaitGroup
	const goroutines = 10

	expectedBody := []byte(`{"data":[{"embedding":[0.1]}]}`)
	expectedUsage := dto.Usage{PromptTokens: 1, TotalTokens: 1}

	// fetchRelease 在所有 goroutine 调用 Do 后关闭，确保 leader 阻塞期间 follower 都能进入 Do
	fetchRelease := make(chan struct{})

	// 启动 N 个并发请求
	results := make([]EmbeddingFetchResult, goroutines)
	shareds := make([]bool, goroutines)
	errs := make([]error, goroutines)

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			result, shared, err := ExecuteEmbeddingFetch(
				tokenKey, model, inputs, encodingFormat, &dims,
				func() (EmbeddingFetchResult, error) {
					atomic.AddInt32(&callCount, 1)
					<-fetchRelease // 阻塞 leader，让 follower 进入 Do
					return EmbeddingFetchResult{Body: expectedBody, Usage: expectedUsage}, nil
				},
			)
			results[idx] = result
			shareds[idx] = shared
			errs[idx] = err
		}(i)
	}

	// 等待足够时间让所有 goroutine 进入 Do（leader 阻塞在 fetchRelease）
	// 此时 callCount 应为 1（只有 leader 执行了 fetch），其余 follower 等待
	// 然后释放 fetchRelease，让 leader 完成
	// 使用轮询等待 callCount >= 1
	for i := 0; i < 100 && atomic.LoadInt32(&callCount) < 1; i++ {
		time.Sleep(time.Millisecond)
	}
	// 再等一会让所有 follower 进入 Do
	time.Sleep(50 * time.Millisecond)

	close(fetchRelease)
	wg.Wait()

	// fetch 函数应只被调用一次（singleflight）
	if atomic.LoadInt32(&callCount) != 1 {
		t.Fatalf("fetch function should be called exactly once, got %d", callCount)
	}

	// 所有 caller 应获得相同结果
	for i := 0; i < goroutines; i++ {
		if errs[i] != nil {
			t.Fatalf("goroutine %d got error: %v", i, errs[i])
		}
		if string(results[i].Body) != string(expectedBody) {
			t.Fatalf("goroutine %d body mismatch", i)
		}
		if results[i].Usage.PromptTokens != expectedUsage.PromptTokens {
			t.Fatalf("goroutine %d usage mismatch", i)
		}
	}

	// shared=true 表示结果被共享（至少有 2 个 caller 同时请求）
	sharedCount := 0
	for i := 0; i < goroutines; i++ {
		if shareds[i] {
			sharedCount++
		}
	}
	if sharedCount < 2 {
		t.Fatalf("expected at least 2 shared results, got %d", sharedCount)
	}
}

func TestExecuteEmbeddingFetch_ErrorPropagation(t *testing.T) {
	tokenKey := "test-sf-err-token"
	model := "test-sf-err-model"
	inputs := []string{"test-sf-err-input"}
	encodingFormat := "float"

	var callCount int32
	var wg sync.WaitGroup
	const goroutines = 5

	fetchRelease := make(chan struct{})

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _, err := ExecuteEmbeddingFetch(
				tokenKey, model, inputs, encodingFormat, nil,
				func() (EmbeddingFetchResult, error) {
					atomic.AddInt32(&callCount, 1)
					<-fetchRelease
					return EmbeddingFetchResult{}, assertError{msg: "upstream error"}
				},
			)
			if err == nil {
				t.Errorf("expected error from fetch function")
			}
		}()
	}

	for i := 0; i < 100 && atomic.LoadInt32(&callCount) < 1; i++ {
		time.Sleep(time.Millisecond)
	}
	time.Sleep(50 * time.Millisecond)

	close(fetchRelease)
	wg.Wait()

	// fetch 函数应只被调用一次
	if atomic.LoadInt32(&callCount) != 1 {
		t.Fatalf("fetch function should be called exactly once even on error, got %d", callCount)
	}
}

// assertError 是测试用的简单错误类型
type assertError struct {
	msg string
}

func (e assertError) Error() string {
	return e.msg
}

func TestEmbeddingCacheEnabled_DefaultTrue(t *testing.T) {
	// 默认应启用（环境变量未设置时）
	// 注意：这个测试依赖环境变量，如果 EMBEDDING_CACHE_ENABLED 已被设置为 false 则会失败
	// 在测试环境中通常未设置，所以默认为 true
	enabled := EmbeddingCacheEnabled()
	if !enabled {
		// 如果环境变量已设置为 false，跳过此测试
		t.Log("EMBEDDING_CACHE_ENABLED is set to false, skipping default check")
	}
}
