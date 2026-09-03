# 渠道自动禁用日志（Auto Disable System Log）

## 功能概述

渠道被自动禁用时（请求错误触发禁用、余额耗尽触发禁用），系统会在**普通日志页**（`/usage-logs/common`）写入一条**系统类型**日志，记录禁用原因，方便管理员在统一的日志界面追溯禁用事件，而不需要查看服务端控制台输出或依赖通知邮件。

## 行为说明

- 触发路径：
  - 请求错误自动禁用：`controller/relay.go` 的 `processChannelError` → `service.DisableChannel`
  - 余额耗尽自动禁用：`service.DisableChannelIfBalanceDepleted`（消费扣减、余额查询等路径）→ 内部调用 `service.DisableChannel`
- 记录时机：`service.DisableChannel` 中 `model.UpdateChannelStatus` 返回 `true`（即状态确实发生了变化）之后。
- 日志归属：归属 **root 用户**（`model.GetRootUser()`），在日志页用户名为 `root`。
- 日志类型：`LogTypeSystem`（值为 `4`），日志页类型筛选中的"System"。
- 日志内容：`通道「{渠道名}」（#{渠道ID}）已被禁用，原因：{reason}`，`reason` 为上游错误详情（含状态码）或 `余额耗尽（自动扣减）` 等。

## 不记录日志的情况

| 场景 | 是否记录 |
|---|---|
| 渠道 `AutoBan` 未开启（跳过禁用） | 否 |
| 渠道已是目标状态（`UpdateChannelStatus` 返回 `false`） | 否 |
| root 用户不存在 | 否（不影响禁用本身，仅跳过日志） |

## 多Key渠道

多Key渠道按Key禁用时（`handlerMultiKeyUpdate`），只要渠道整体状态发生变化（`UpdateChannelStatus` 返回 `true`）同样会记录一条日志；单个Key状态变化但渠道整体状态不变时不记录。

## 相关代码

| 位置 | 说明 |
|---|---|
| `service/channel.go` | `DisableChannel`：禁用成功后调用 `model.RecordLog(rootUser.Id, model.LogTypeSystem, content)` |
| `model/log.go` | `RecordLog` / `LogTypeSystem` 常量 |
| `model/channel.go` | `UpdateChannelStatus`：返回值表示状态是否实际变更 |
| `web/default/src/features/usage-logs` | 普通日志页（common），`type=4` 显示为 "System" |

## 测试

`service/channel_disable_log_test.go`：

- `TestDisableChannel_RecordsSystemLog`：禁用成功后存在 System 日志，归属 root，内容包含渠道ID与原因
- `TestDisableChannel_AutoBanOff_NoLog`：AutoBan 关闭时不产生日志
- `TestDisableChannelIfBalanceDepleted_RecordsSystemLog`：余额耗尽路径同样记录日志
- `TestDisableChannelIfBalanceDepleted_NoRootUserNoPanic`：root 用户缺失时不 panic，禁用仍生效

运行方式：

```bash
go test ./service/ -run 'TestDisableChannel|TestDisableChannelIfBalanceDepleted' -v
```
