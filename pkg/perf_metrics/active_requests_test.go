package perfmetrics

import (
	"sync"
	"testing"
	"time"
)

// newTestTracker 创建一个以当前时间初始化的测试用 tracker
func newTestTracker() *ActiveRequestTracker {
	return &ActiveRequestTracker{
		secondBuckets: make([]int64, secondBucketCount),
		bucketStart:   time.Now().Unix(),
	}
}

func TestOnRequestStart_IncrementsActiveCount(t *testing.T) {
	tracker := newTestTracker()
	tracker.OnRequestStart()
	tracker.OnRequestStart()
	tracker.OnRequestStart()
	if got := tracker.activeCount.Load(); got != 3 {
		t.Fatalf("expected activeCount=3, got %d", got)
	}
}

func TestOnRequestEnd_DecrementsActiveCount(t *testing.T) {
	tracker := newTestTracker()
	tracker.OnRequestStart()
	tracker.OnRequestStart()
	tracker.OnRequestEnd()
	if got := tracker.activeCount.Load(); got != 1 {
		t.Fatalf("expected activeCount=1, got %d", got)
	}
}

func TestOnRequestEnd_NeverNegative(t *testing.T) {
	// 即使未配对的 End 也不应变为负数
	tracker := newTestTracker()
	tracker.OnRequestEnd()
	if got := tracker.activeCount.Load(); got != -1 {
		t.Fatalf("expected activeCount=-1 (unpaired), got %d", got)
	}
}

func TestSnapshot_RequestsInWindow(t *testing.T) {
	tracker := newTestTracker()
	// 发送 5 个请求
	for i := 0; i < 5; i++ {
		tracker.OnRequestStart()
	}
	stats := tracker.Snapshot()
	if stats.Requests10m != 5 {
		t.Fatalf("expected Requests10m=5, got %d", stats.Requests10m)
	}
	if stats.Requests1h != 5 {
		t.Fatalf("expected Requests1h=5, got %d", stats.Requests1h)
	}
}

func TestSnapshot_ActiveRequests(t *testing.T) {
	tracker := newTestTracker()
	tracker.OnRequestStart()
	tracker.OnRequestStart()
	stats := tracker.Snapshot()
	if stats.ActiveRequests != 2 {
		t.Fatalf("expected ActiveRequests=2, got %d", stats.ActiveRequests)
	}
	tracker.OnRequestEnd()
	tracker.OnRequestEnd()
	stats = tracker.Snapshot()
	if stats.ActiveRequests != 0 {
		t.Fatalf("expected ActiveRequests=0, got %d", stats.ActiveRequests)
	}
}

func TestSnapshot_Empty(t *testing.T) {
	tracker := newTestTracker()
	stats := tracker.Snapshot()
	if stats.ActiveRequests != 0 || stats.Requests10m != 0 || stats.Requests1h != 0 {
		t.Fatalf("expected all zero, got %+v", stats)
	}
}

func TestEnsureBucket_ResetWhenOutOfRange(t *testing.T) {
	tracker := newTestTracker()
	// 写入一些数据
	tracker.OnRequestStart()
	// 模拟时间已超出 1 小时范围
	tracker.mu.Lock()
	tracker.bucketStart = time.Now().Unix() - secondBucketCount - 10
	tracker.mu.Unlock()
	// 调用 OnRequestStart 应触发重置
	tracker.OnRequestStart()
	tracker.mu.RLock()
	if tracker.secondBuckets[0] != 1 {
		t.Fatalf("expected bucket[0]=1 after reset, got %d", tracker.secondBuckets[0])
	}
	tracker.mu.RUnlock()
}

func TestSnapshot_PartialWindow(t *testing.T) {
	// 当数据时间不到 10 分钟时，应该返回已有的全部计数
	tracker := newTestTracker()
	tracker.OnRequestStart()
	time.Sleep(1100 * time.Millisecond)
	stats := tracker.Snapshot()
	if stats.Requests10m != 1 {
		t.Fatalf("expected Requests10m=1, got %d", stats.Requests10m)
	}
}

func TestConcurrentAccess(t *testing.T) {
	tracker := newTestTracker()
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			tracker.OnRequestStart()
			time.Sleep(time.Millisecond)
			tracker.OnRequestEnd()
		}()
	}
	wg.Wait()
	if got := tracker.activeCount.Load(); got != 0 {
		t.Fatalf("expected activeCount=0 after all done, got %d", got)
	}
	stats := tracker.Snapshot()
	if stats.Requests10m != 100 {
		t.Fatalf("expected Requests10m=100, got %d", stats.Requests10m)
	}
}
