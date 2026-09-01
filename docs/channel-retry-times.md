# 渠道级独立重试次数（Channel Retry Times）

## 功能概述

允许管理员为**特定渠道**单独设置重试次数，覆盖全局重试次数设置（系统设置 → 运营设置 → 行为配置中的 `RetryTimes`）。

典型场景：

- 某渠道不稳定，希望失败时多试几次（覆盖为更大的重试次数）
- 某渠道昂贵或不允许重复请求，希望失败后立即切换下一个渠道（禁止重试）

## 取值语义

渠道字段 `retry_times`（`channels.retry_times`，整型，默认 `0`）：

| 取值 | 行为 |
|---|---|
| `0`（默认） | 跟随全局重试次数，行为与未启用该功能时完全一致 |
| `-1` | 禁止在该渠道上重试：该渠道仅尝试一次，失败后立即切换下一个渠道 |
| `N > 0` | 该渠道重试 `N` 次，即最多尝试 `N+1` 次（覆盖全局值） |

## 各分支行为

重试逻辑位于 `controller/relay.go`，渠道覆盖值通过 `resolveChannelAttempts` 统一解析：

| 分支 | 行为 |
|---|---|
| 轮次循环（普通请求） | 每个渠道按自身覆盖值尝试；`-1` 渠道仅尝试一次并输出日志 `channel #N has retry disabled` |
| fallback 渠道（429/自动禁用触发） | 每个 fallback 渠道同样按自身覆盖值尝试 |
| 亲和性锁定分支 | `-1` 渠道仅尝试一次即退出锁定（fall through 轮次循环切换其他渠道）；`N>0` 渠道按覆盖值重试且本分支循环上限相应放宽 |
| 响应内容检测命中 | 渠道 `retry_times > 0` 时覆盖全局重试上限（与渠道错误重试语义一致） |
| 指定渠道请求（`specific_channel_id`） | 不受影响，维持现状（指定渠道本就不重试） |

### 请求级预算补偿

原实现中，整个请求的总尝试上限 `maxAttempts = RetryTimes + 1`。为使覆盖渠道能获得精确的额外尝试次数而不挤占其他渠道的预算，首轮获取渠道列表后：

```text
maxAttempts += Σ max(0, 渠道尝试次数 - 全局默认尝试次数)
```

- 所有渠道均无覆盖（全为 `0` 或 `-1`）时，补偿为 `0`，总尝试上限与原行为**完全一致**；
- 渠道尝试次数小于全局默认（理论上覆盖只会更大，此处防御性处理）时不扣减预算。

## API

字段随渠道对象一起提交（`POST /api/channel/`、`PUT /api/channel/`）：

```json
{
  "name": "my-channel",
  "retry_times": 3
}
```

- 后端校验 `retry_times >= -1`（`controller/channel.go` 的 `validateChannel`，创建与更新共用）；
- 字段为指针类型（`*int`）：更新请求不传该字段时不会覆盖数据库已有值，传 `0` / `-1` 可显式写回；
- 数据库迁移由 GORM AutoMigrate 自动完成（`model/main.go`），SQLite / MySQL / PostgreSQL 三库兼容。

## 前端

渠道编辑抽屉 → 高级设置 → Routing & Overrides 分区新增「渠道重试次数」数字输入（范围 `-1 ~ 50`，对齐全局 `RetryTimes` 上限）：

- 占位提示：`0 = 使用全局重试次数`
- 说明文案：`该渠道的重试次数（0 = 使用全局设置，-1 = 禁止在该渠道重试）`

相关代码：

- 表单 schema/转换：`web/default/src/features/channels/lib/channel-form.ts`
- 表单 UI：`web/default/src/features/channels/components/drawers/channel-mutate-drawer.tsx`
- 渠道类型：`web/default/src/features/channels/types.ts`（`retry_times: number | null | undefined`）

## 实现位置索引

| 位置 | 说明 |
|---|---|
| `model/channel.go` | `Channel.RetryTimes` 字段与 `GetRetryTimes()` |
| `constant/context_key.go` | `ContextKeyChannelRetryTimes` |
| `middleware/distributor.go` | `SetupContextForSelectedChannel` 注入 context（每次重试切换渠道自动刷新） |
| `controller/relay.go` | `resolveChannelAttempts` / `computeExtraRetryBudget` / 各分支改造 / 检测命中上限 |
| `controller/channel.go` | `validateChannel` 校验 |
| 测试 | `controller/relay_channel_retry_test.go`、`channel-form.test.ts`（retry_times 用例） |
