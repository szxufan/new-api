package perfmetrics

import (
	"sync"
	"sync/atomic"
	"time"
)

const (
	// secondBucketCount 覆盖 1 小时的秒级桶数量
	secondBucketCount = 3600
	// tenMinuteSeconds 10 分钟对应的秒数
	tenMinuteSeconds = 600
)

// ActiveRequestStats 活跃请求统计快照
type ActiveRequestStats struct {
	ActiveRequests int64 `json:"active_requests"` // 当前正在执行的请求数
	Requests10m    int64 `json:"requests_10m"`    // 最近 10 分钟请求数
	Requests1h     int64 `json:"requests_1h"`     // 最近 1 小时请求数
}

// ActiveRequestTracker 实时请求计数器，使用环形缓冲区按秒级粒度记录请求数。
type ActiveRequestTracker struct {
	activeCount atomic.Int64 // 当前活跃请求数
	mu          sync.RWMutex
	// secondBuckets 环形缓冲区，每秒一个桶，共 3600 个桶覆盖 1 小时
	secondBuckets []int64
	// bucketStart secondBuckets[0] 对应的 Unix 时间戳（秒）
	bucketStart int64
}

// ActiveTracker 全局活跃请求计数器单例
var ActiveTracker = &ActiveRequestTracker{
	secondBuckets: make([]int64, secondBucketCount),
	bucketStart:   time.Now().Unix(),
}

// OnRequestStart 请求开始时调用：活跃计数 +1，当前秒桶 +1
func (t *ActiveRequestTracker) OnRequestStart() {
	t.activeCount.Add(1)
	now := time.Now().Unix()
	t.mu.Lock()
	t.ensureBucket(now)
	idx := int((now - t.bucketStart) % secondBucketCount)
	t.secondBuckets[idx]++
	t.mu.Unlock()
}

// OnRequestEnd 请求结束时调用：活跃计数 -1
func (t *ActiveRequestTracker) OnRequestEnd() {
	t.activeCount.Add(-1)
}

// Snapshot 返回三个指标的当前快照
func (t *ActiveRequestTracker) Snapshot() ActiveRequestStats {
	now := time.Now().Unix()
	t.mu.RLock()
	// 只在缓冲区覆盖的时间范围内（0 ~ 3600 秒）累加数据
	var req10m, req1h int64
	if elapsed := now - t.bucketStart; elapsed >= 0 && elapsed < secondBucketCount {
		// 累加最近 10 分钟（600 秒）的桶
		req10m = t.sumBuckets(now, tenMinuteSeconds)
		// 累加最近 1 小时（3600 秒）的桶
		req1h = t.sumBuckets(now, secondBucketCount)
	}
	t.mu.RUnlock()
	return ActiveRequestStats{
		ActiveRequests: t.activeCount.Load(),
		Requests10m:    req10m,
		Requests1h:     req1h,
	}
}

// sumBuckets 累加从 now 往前 windowSeconds 秒的桶计数（包含当前秒）。
// 调用方需持有读锁。
func (t *ActiveRequestTracker) sumBuckets(now int64, windowSeconds int) int64 {
	elapsed := now - t.bucketStart
	if elapsed < 0 {
		return 0
	}
	// elapsed+1 表示从 bucketStart 到 now 共 elapsed+1 个秒桶（包含两端）
	available := int(elapsed) + 1
	// 如果可用秒数超过缓冲区容量，只能取最近 secondBucketCount 秒的数据
	if available > secondBucketCount {
		available = secondBucketCount
	}
	if available < windowSeconds {
		windowSeconds = available
	}
	var sum int64
	for i := 0; i < windowSeconds; i++ {
		// 从 now 往前数 i 秒
		ts := now - int64(i)
		if ts < t.bucketStart {
			break
		}
		idx := int((ts - t.bucketStart) % secondBucketCount)
		sum += t.secondBuckets[idx]
	}
	return sum
}

// ensureBucket 确保当前秒对应的桶可用，必要时重置缓冲区。
// 调用方需持有写锁。
func (t *ActiveRequestTracker) ensureBucket(now int64) {
	// 如果当前时间在缓冲区覆盖范围内（now - bucketStart < 3600），直接使用
	if now-t.bucketStart >= 0 && now-t.bucketStart < secondBucketCount {
		return
	}
	// 如果超出范围，重置缓冲区（以当前秒为新起点）
	for i := range t.secondBuckets {
		t.secondBuckets[i] = 0
	}
	t.bucketStart = now
}
