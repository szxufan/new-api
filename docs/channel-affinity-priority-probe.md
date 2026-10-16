# 渠道亲和性优先级回探（Channel Affinity Priority Probe）

## 功能概述

解决亲和性（sticky routing）的一个固有缺陷：高优先级渠道**偶发失败**后，请求重试到低优先级渠道成功，亲和性缓存（`switch_on_success`）随即将该会话亲和键更新为低优先级渠道，导致后续请求在 TTL 过期前（默认 1 小时）一直被钉在低优先级渠道上，无法自动回到高优先级渠道。

回探机制在亲和性命中后增加一道判断：**当存在比亲和渠道优先级更高的可用渠道时，按概率忽略亲和性、重新按优先级选择**。探测请求成功后，`RecordChannelAffinity` 会将亲和性迁回高优先级渠道，流量逐步自愈。

## 配置项

系统设置 → 模型设置 → 渠道亲和性（Channel Affinity）：

| 配置项 | Option Key | 默认值 | 说明 |
|---|---|---|---|
| 启用优先级回探 | `channel_affinity_setting.probe_enabled` | `false` | 总开关，默认关闭（行为与之前完全一致） |
| 回探概率（%） | `channel_affinity_setting.probe_percent` | `10` | 0-100；仅在存在更高优先级渠道时生效 |
| 回探冷却时间（秒） | `channel_affinity_setting.probe_cooldown_seconds` | `300` | 同一亲和键在冷却窗口内最多回探一次，避免并发请求集中回探 |

## 触发条件

`service.ShouldProbeHigherPriorityChannel`（`service/channel_affinity.go`）在以下**全部满足**时返回 true：

1. 亲和性与回探机制均已启用；
2. 命中的规则**未开启** `skip_retry_on_failure`——严格钉住场景下回探失败无法回退到亲和渠道，会直接中断请求，因此跳过；
3. 当前分组（`auto` 时为所有候选分组）中存在比亲和渠道优先级更高、且未命中用户分组黑名单的可用渠道；
4. 按 `probe_percent` 概率命中；
5. 该亲和键不在回探冷却窗口内（命中后写入冷却标记，Redis/内存双模式均支持）。

## 工作流程

```text
请求命中亲和性 → ShouldProbeHigherPriorityChannel
   ├─ false → 沿用亲和渠道（原有逻辑）
   └─ true  → 忽略亲和性，走正常优先级选择（CacheGetRandomSatisfiedChannel）
                ├─ 高优先级渠道成功 → RecordChannelAffinity 将亲和性迁回高优先级渠道
                └─ 高优先级渠道仍失败 → 正常重试逻辑按优先级降序尝试后续渠道
                   （原亲和渠道仍在候选列表中，不牺牲可用性）
```

## 观测

- 回探发生时会输出 debug 日志：`channel affinity probe: bypass affinity channel N, reselect by priority`；
- 使用日志的管理信息中 `channel_affinity` 字段会记录 `"probe": true, "reason": "probe_higher_priority"` 及原亲和渠道 ID。

## 注意事项

- 回探**不牺牲可用性**：探测本质是把本次选择交还给优先级排序，原亲和渠道仍在候选列表中，高优先级渠道持续故障时请求仍会落到可用渠道；
- 与 `switch_on_success` 配合使用效果最好（探测成功后亲和性立即迁回）；即使关闭 `switch_on_success`，亲和性也会记录为探测选中的高优先级渠道；
- 冷却标记使用独立命名空间 `new-api:channel_affinity_probe:v1`，清空亲和性缓存不影响冷却计时；
- 高优先级渠道**持续**故障的场景不依赖回探恢复——仍由既有的自动禁用/限流机制接管（亲和渠道被禁用时 distributor 本就走后备/正常选择）。
