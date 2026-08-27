# 上游 User-Agent 配置说明

本文档说明两个与向上游渠道发送 `User-Agent`（UA）相关的全局配置：UA 透传名单与默认上游 UA。

## 配置项

| 配置键 | 界面名称 | 类型 | 默认值 |
| --- | --- | --- | --- |
| `general_setting.user_agent_passthrough` | UA 透传名单 | 多行文本，每行一个 UA 片段 | 空（不透传） |
| `general_setting.default_user_agent` | 默认上游 User-Agent | 单行文本 | 空（回退内置 `hertz`） |

配置位置：后台「系统设置 → 模型设置 → 全局配置」，两项位于同一区块。

## 一、UA 透传名单

控制是否将入口请求的原始 `User-Agent` 透传给上游渠道。

### 匹配语义

1. 每行填写一个 UA **片段**（子串），首尾空白会被忽略，空行跳过。
2. 入口请求的 `User-Agent` 中**包含**任一片段即命中（不区分大小写）。
3. 命中后，将入口请求的**完整原始 UA** 原样发送给上游；未命中则使用下文的默认上游 UA。

### 配置示例

```text
codex
claude-cli
Mozilla/5.0 (Windows NT 10.0)
```

含义：

- 客户端 UA 为 `codex-cli/1.0 (linux)` 时，命中片段 `codex`，上游收到的 `User-Agent` 为 `codex-cli/1.0 (linux)`；
- 客户端 UA 为普通浏览器 UA 且不含上述任一片段时，上游收到默认上游 UA。

### 注意事项

- 片段建议尽量具体（如 `codex-cli` 而非单个字母），避免误命中。
- 若站点存在反向代理/网关改写入口 `User-Agent`，透传的是改写后的值。

## 二、默认上游 User-Agent

未命中透传名单（或客户端没有 UA）时，向上游发送的 `User-Agent`。此前该值固定为 `hertz`，现在可以按部署需要自定义，例如某些上游要求识别特定客户端标识。

- 配置键：`general_setting.default_user_agent`
- 类型：单行文本，填写**完整 UA 字符串**（不是片段，不做子串匹配），首尾空白会被忽略
- 默认值：空 → 回退内置默认值 `hertz`，与历史版本行为一致

示例：

```text
my-gateway/1.0 (+https://example.com)
```

配置后，所有未命中透传名单的上游请求都会携带该 UA。

## 生效范围与优先级

生效的请求类型：

- 普通 API 请求（`DoApiRequest`）
- 表单/文件类请求（`DoFormRequest`）
- WebSocket 实时请求（`DoWssRequest`）

不生效的场景：

- 异步任务类渠道（如 kling 等任务适配器自行设置 UA，其显式值优先于默认 UA）
- 渠道测试请求：为避免把发起测试的管理员浏览器 UA 发送给上游，测试时不做**透传**（但仍使用配置的默认 UA）

同一请求中 UA 的最终取值优先级（从高到低）：

1. 渠道 Header Override 显式配置的 `User-Agent`（最后应用，可覆盖一切）
2. 渠道适配器显式设置的 UA（如 kling 的 `kling-sdk/1.0`）
3. UA 透传名单命中 → 透传入口请求的原始客户端 UA
4. 默认上游 UA（`general_setting.default_user_agent`，留空回退内置 `hertz`）

## 修改与生效

两项配置保存后立即生效（全局配置运行时加载），无需重启服务。
