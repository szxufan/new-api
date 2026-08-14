# 重试防缓存（Retry Anti-Cache）

## 背景

部分渠道上游（或上游前的网关/CDN）会对**相同请求体**的失败响应做缓存，导致网关重试时返回与首次请求完全相同的错误码，重试失去意义。

本功能允许在渠道上配置"追加内容"，开启后每次重试都会在请求的**最后一条 user 消息末尾**追加该内容，使重试请求体与首次请求不同，从而绕过上游错误缓存。

## 配置

渠道编辑界面（新增/编辑渠道 → 额外设置）中提供两个配置项：

| 配置项 | 类型 | 说明 |
|---|---|---|
| 重试防缓存（Retry Anti-Cache） | 开关 | 是否在重试时追加内容 |
| 防缓存追加内容（Anti-Cache Retry Content） | 文本 | 追加的内容，例如 `快做！` |

追加规则：**第 N 次重试追加 N 个配置内容**。例如配置内容为 `快做！`：

- 首次请求（第 1 次尝试）：不追加；
- 第 1 次重试（第 2 次尝试）：最后一条 user 消息末尾追加 `快做！`；
- 第 2 次重试（第 3 次尝试）：末尾追加 `快做！快做！`；
- 第 N 次重试：末尾追加 N 个 `快做！`。

每次重试都从原始请求重新追加（不会 1+2+3 累加）。换渠道重试同样生效。

## 支持范围

- **生效格式**：OpenAI 兼容 Chat Completions、Claude Messages、OpenAI Responses（/v1/responses）三种文本请求格式。
- **内容形态**：最后一条 user 消息的 content 为字符串时直接追加；为数组（多模态）时在末尾追加一个 text 块，不破坏已有的 image/audio 等结构。
- **首次请求不追加**：仅重试（RetryIndex > 1）时生效，正常请求不受任何影响。

## 限制

- **透传模式（Pass Through Body）不生效**：启用"透传请求体"的渠道会直接把原始请求体发送给上游，无法安全改写消息，此功能自动失效。
- **Gemini 格式暂不支持**：Gemini 渠道（gemini 格式请求）暂不适用。
- **计费影响**：追加内容会略微增加请求 token 数，按实际发送内容计费。

## 实现位置（后端）

- 配置字段：`dto.ChannelSettings.AntiCacheRetryEnabled / AntiCacheRetryContent`（存于渠道 `setting` JSON 列，无需数据库迁移）。
- 追加逻辑：`relay/anti_cache_retry.go`（`applyRetryAntiCacheOpenAI` / `applyRetryAntiCacheClaude` / `applyRetryAntiCacheResponses`）。
- 调用入口：`relay/compatible_handler.go`（TextHelper）、`relay/claude_handler.go`（ClaudeHelper）、`relay/responses_handler.go`（ResponsesHelper），均在请求深拷贝之后、发送上游之前生效。
- 重试次数依据 `RelayInfo.RetryIndex`（首次尝试为 1）。

## 测试

`relay/anti_cache_retry_test.go` 覆盖：未重试/未开启/内容为空不修改、第 N 次重试追加 N 个、字符串与数组型 content、最后一条非 user 消息回退、无 user 消息不 panic、非法 JSON 不报错等场景。

```bash
go test ./relay/ -run 'TestApplyRetryAntiCache' -v
```
