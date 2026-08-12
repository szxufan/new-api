# 重试间隔策略：线性递增 + 随机抖动

## 背景

原实现中每次重试前都固定等待 `RetryIntervalMs` 毫秒（配置项），见
`common/constants.go` 的 `RetryIntervalMs`。固定间隔有两个缺点：

1. 高并发下多个请求在同一时刻重试会产生"惊群"效应，加重下游压力。
2. 间隔不随重试次数增长，持续失败时对下游的冲击保持不变。

## 目标

将固定重试间隔改为**线性递增 + 随机抖动**：

- **递增**：等待时间随重试次数线性增长。
- **随机**：叠加随机抖动，避免多个请求在同一时刻齐齐重试。

## 策略定义

对第 `attempt` 次重试（`attempt >= 1`，表示当前已完成的重试尝试次数）：

```
基础等待 = RetryIntervalMs * attempt   (毫秒)
随机抖动 = [0.8, 1.2]
实际等待 = 基础等待 * 随机抖动
```

约定：

- `RetryIntervalMs` 解释为**基准起点**：`attempt` 为 1 时即为第 1 次重试，
  等待约 `RetryIntervalMs * 1` 毫秒。
- 当 `RetryIntervalMs <= 0` 时保持原行为：**不等待，立即重试**。
- `attempt < 1` 时不等待（防御性校验）。

### 举例

假设 `RetryIntervalMs = 500`：

| 重试序号 | 基础等待 | 抖动后大致范围 |
| :---: | :---: | :---: |
| 第 1 次 | 500ms | 400 ~ 600ms |
| 第 2 次 | 1000ms | 800 ~ 1200ms |
| 第 3 次 | 1500ms | 1200 ~ 1800ms |
| ... | ... | ... |

## 实现

### 新增函数

在 `controller/relay.go` 中新增 `retryIntervalWait(c, attempt)`：

```go
func retryIntervalWait(c *gin.Context, attempt int) {
	if common.RetryIntervalMs <= 0 || attempt < 1 {
		return
	}
	base := time.Duration(common.RetryIntervalMs*attempt) * time.Millisecond
	jitter := 0.8 + rand.Float64()*0.4
	totalWait := time.Duration(float64(base) * jitter)
	retryKeepAliveSleep(c, totalWait)
}
```

`retryKeepAliveSleep` 复用现有实现：非流式请求直接 `Sleep`；流式请求通过
ticker 保持连接活跃直到总等待结束。

### 替换调用点

原有 3 处 `if common.RetryIntervalMs > 0 { retryKeepAliveSleep(...) }` 统一替换为：

```go
retryIntervalWait(c, attempt)
```

其中 `attempt` 取对应循环中已累计的总尝试次数 `totalAttempts`
（`tryChannelOnce` 中为 `*totalAttempts`，`RelayTask` 中为 `totalAttempts`）。

## 测试

- 新增 `controller/relay_retry_interval_test.go`，覆盖：
  - `RetryIntervalMs <= 0` 或 `attempt < 1` 时直接 return（不 sleep）；
  - 等待时间落在 `[base*0.8, base*1.2]` 范围内；
  - 递增性：`attempt` 越大，基础等待越大。
- 既有 `model/option_retry_interval_test.go` 继续保证配置解析行为不变。

## 兼容性

- 配置项 `RetryIntervalMs` 名称与含义（毫秒）不变，仅其在重试中的"解释"
  从"固定间隔"变为"基准起点"，无需迁移配置。
- `RetryIntervalMs = 0` 时行为保持不变（立即重试）。