# 渠道价格系数（Channel Price Factors）

## 功能概述

允许管理员为**特定渠道**设置三类可独立配置、最终**累乘**生效的价格系数：

1. **总价格系数**：作用于该渠道全部计费结果的统一乘数。
2. **单项系数**：分别调节 输入 / 补全 / 缓存读 / 缓存写 / 图像输入 / 音频输入 / 音频输出 七个计费项。
3. **时间段系数**：为特定时间段配置价格系数，支持跨天时段（如 `22:00 - 08:00`）。

典型场景：

- 某上游渠道成本更低，按比例打折（总价格系数 `0.8`）
- 某渠道输出价格偏高，单独上调补全系数而不影响输入
- 夜间时段上游有折扣/免费额度，按时间段设置更低的价格系数（如 `22:00 - 08:00` 系数 `0.5`）

## 计算规则

最终价格在常规计算（模型倍率/价格 × 分组倍率）之外，按以下顺序累乘：

```text
最终价格 = 基础价格 × 单项系数（按计费项） × 总价格系数 × 命中时段系数
```

以按 Token 计费为例，内部按计费项分别应用单项系数后汇总，再乘通用系数：

```text
quota = [ baseTokens × input          + completionTokens × completionRatio × completion
        + cacheReadTokens × cacheRatio × cacheRead
        + cacheWriteTokens × cacheWriteRatio × cacheWrite
        + imageTokens × imageRatio    × imageInput
        + audioInput 部分              × audioInput
        + audioOutput 部分             × audioOutput ] × modelRatio × groupRatio
        × 总价格系数 × 命中时段系数
```

- 工具调用附加费（Web Search / File Search 等）与 Gemini 独立音频输入价同样受 **总价格系数 × 时段系数** 影响；
- 系数与 `OtherRatios`（时长/分辨率档位等）相互独立、同为乘数关系。

### 系数取值语义

| 配置 | 含义 |
|---|---|
| 留空（未配置，JSON 中无该字段） | 等价于 `1`，不调整价格 |
| `0` | 该计费项/整单免费 |
| `N > 0` | 按 `N` 倍计价 |

> 注意：显式 `0` 会被完整保留（存储结构使用指针类型区分「未配置」与「0」）。

## 配置结构

系数存储在渠道 `setting` JSON 的 `price` 子对象中（无需新增数据库列，随渠道设置一起缓存与下发）：

```json
{
  "setting": {
    "force_format": false,
    "price": {
      "total": 0.9,
      "input": 0.5,
      "completion": 1.2,
      "cache_read": 0.2,
      "cache_write": 1.1,
      "image_input": 2,
      "audio_input": 3,
      "audio_output": 4,
      "time_windows": [
        { "start": "22:00", "end": "08:00", "ratio": 0.5 },
        { "start": "12:00", "end": "14:00", "ratio": 0.8 }
      ]
    }
  }
}
```

| 字段 | 含义 |
|---|---|
| `price.total` | 总价格系数（作用于整单） |
| `price.input` | 输入 token 系数 |
| `price.completion` | 补全（输出）token 系数 |
| `price.cache_read` | 缓存读取 token 系数 |
| `price.cache_write` | 缓存写入 token 系数（含 Claude 5m/1h 拆分） |
| `price.image_input` | 图像输入 token 系数 |
| `price.audio_input` | 音频输入 token 系数 |
| `price.audio_output` | 音频输出 token 系数 |
| `price.time_windows[]` | 时间段系数列表（`start`/`end` 为 `HH:mm`，`ratio` 为该时段系数） |

### 时间段规则

- **跨天**：`end` 早于 `start` 表示跨天时段，如 `22:00 - 08:00`，覆盖 `[22:00, 24:00) ∪ [00:00, 08:00)`；`start == end` 为非法配置。
- **重叠**：请求时间同时命中多条时段时，**按配置顺序取第一条**命中的系数（不做累乘）。
- **时间基准**：以**请求开始时间**为准（`relayInfo.StartTime`）；异步任务差额结算以任务**提交时间**为准。
- **时区**：使用**服务器本地时间**（与渠道「定时开启时段」一致）。流式长请求跨越时段边界时，仍按请求开始时刻判定。
- 单渠道最多配置 20 条时段（与「定时开启时段」上限一致）。

## 生效范围

| 计费路径 | 生效系数 |
|---|---|
| 按 Token 计费（文本 / Claude / Gemini / Responses / Embedding / Rerank / 图像与音频同步接口） | 单项系数 + 总价格系数 + 时段系数（完整三类） |
| 按次计费（任务/视频、MJ、图片 `usePrice`） | 总价格系数 + 时段系数 |
| 表达式计费（`tiered_expr`） | 总价格系数 + 时段系数（乘在表达式最终结果之外，不改变表达式语义） |
| 实时（WebSocket）按量计费 | 单项系数 + 总价格系数 + 时段系数 |

### 预扣费与结算

- **预扣费**同步应用系数：估算输入/输出 token 分别乘对应单项系数，再乘「总价格系数 × 时段系数」；未配置系数时公式与旧版完全一致。
- 预扣使用**初始选定渠道**的系数，结算使用**最终成功渠道**（重试切换渠道后）的系数，差额通过多退少补处理。
- `0` 系数可能导致「预扣 > 0 而结算为 0」（结算时全额退还差额），属正常行为。

## 校验规则

`validateChannel`（创建/更新共用）会解析 `setting` 并校验 `price`：

- 各系数必须 `>= 0`；
- 时间段格式必须为合法 `HH:mm` 且 `start != end`；
- 时段数量不超过 20；
- 时段 `ratio` 必须 `>= 0`。

## 消费日志

当系数非全 1 时，消费日志的 `other.channel_price_factors` 中会记录生效的非 1 系数（键名与配置字段一致，命中时段系数记为 `time_window`），便于排查计费差异：

```json
{
  "channel_price_factors": { "total": 0.9, "input": 0.5, "time_window": 0.8 }
}
```

## 示例

渠道配置：`total = 0.9`、`input = 0.5`、`cache_read = 0.2`，时段 `22:00 - 08:00` 系数 `0.8`，模型倍率 `1`、分组倍率 `1`。

一次请求（22:30 发起）：输入 1000 tokens（其中缓存读 400）、输出 500 tokens、输出倍率 `2`、缓存读倍率 `0.1`：

```text
输入部分  = (1000 - 400) × 1 × 0.5 = 300
缓存读部分 = 400 × 0.1 × 0.2   = 8
输出部分  = 500 × 2 × 1        = 1000
小计      = 1308
最终      = 1308 × 1 × 1 × (0.9 × 0.8) = 941.76 → 942
```

## 实现位置

| 层 | 文件 |
|---|---|
| 结构与解析 | `dto/channel_price_settings.go`（`ChannelPriceSettings` / `PriceFactors` / `Validate` / `Resolve`） |
| 渠道校验接入 | `model/channel.go` `ValidateSettings` |
| 请求期解析 | `relay/common/channel_price.go` `ResolveChannelPriceFactors` |
| 预扣费 | `relay/helper/price.go`（量计费 / usePrice / follow / PerCall / tiered 五个分支） |
| 文本结算 | `service/text_quota.go` `calculateTextQuotaSummary` |
| 音频/实时结算 | `service/quota.go` `calculateAudioQuota` |
| 表达式结算 | `service/tiered_settle.go` `TryTieredSettle` |
| 任务异步结算 | `service/task_billing.go`、`service/task_polling.go`（`service/channel_price.go` 提供按渠道 ID 解析） |
| 前端表单 | `web/default/src/features/channels/lib/channel-form.ts` |
| 前端 UI | `web/default/src/features/channels/components/drawers/channel-mutate-drawer.tsx`、`price-time-windows-editor.tsx` |