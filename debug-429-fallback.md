# Debug Session: 429-fallback
- **Status**: [OPEN]
- **Issue**: 上游渠道返回 429 时，配置的 fallback（备用）渠道未生效，请求没有切换到 fallback 渠道重试。
- **Debug Server**: TBD (启动后填写)
- **Log File**: .dbg/trae-debug-log-429-fallback.ndjson

## Reproduction Steps
1. 配置一个主渠道 A，并在 A 的 `fallback_channel_ids` 中配置备用渠道 B
2. 使上游对渠道 A 返回 429（例如触发限流）
3. 期望：请求自动切换到 fallback 渠道 B 重试
4. 实际：fallback 渠道未生效，请求直接失败或走常规重试

## 关键代码路径
- `controller/relay.go:242` - `isFallbackEligibleError(newAPIError)` 判断是否触发 fallback
- `controller/relay.go:461` - `tryFallbackChannel` 查找可用 fallback 渠道
- `service/error.go:86` - `RelayErrorHandler` 将上游 429 转为 NewAPIError
- `service/error.go:131` - `ResetStatusCode` 可能改变 StatusCode
- `model/channel_cache.go:440` - `CacheGetFallbackChannel` 查找 fallback 渠道
- `relay/compatible_handler.go:206` - 上游非 200 状态码处理入口

## Hypotheses & Verification
| ID | Hypothesis | Likelihood | Effort | Evidence |
|----|------------|------------|--------|----------|
| A | `ResetStatusCode` 将 429 映射为其他状态码，导致 `isFallbackEligibleError` 判定失败 | High | Low | Pending |
| B | `tryFallbackChannel` 中 `retryParam.ModelName` 与 fallback 渠道配置的模型不匹配 | Medium | Low | Pending |
| C | fallback 渠道状态非 `ChannelStatusEnabled`（被禁用/限流） | Medium | Low | Pending |
| D | `NewOpenAIError` 的 `errors.As` 分支保留旧 StatusCode，覆盖新传入的 429 | Medium | Low | Pending |
| E | 主渠道未正确配置 `fallback_channel_ids`（OtherInfo 为空） | Low | Low | Pending |

## Log Evidence
[待收集]

## Verification Conclusion
[待分析]
