# 上游 User-Agent 透传配置说明

该配置用于控制是否将入口请求的原始 `User-Agent`（UA）透传给上游渠道。默认情况下，new-api 向上游发送请求时使用固定的默认 UA（`hertz`），不会透传客户端 UA。

## 配置项

- 配置键：`general_setting.user_agent_passthrough`
- 配置位置：后台「系统设置 → 模型设置 → 全局配置」（上游 User-Agent 透传）
- 类型：多行文本，每行一个 UA 片段
- 默认值：空（表示不透传任何客户端 UA，行为与历史版本一致）

## 匹配语义

1. 每行填写一个 UA **片段**（子串），首尾空白会被忽略，空行跳过。
2. 入口请求的 `User-Agent` 中**包含**任一片段即命中（不区分大小写）。
3. 命中后，将入口请求的**完整原始 UA** 原样发送给上游；未命中则继续使用默认上游 UA。

## 生效范围与优先级

生效的请求类型：

- 普通 API 请求（`DoApiRequest`）
- 表单/文件类请求（`DoFormRequest`）
- WebSocket 实时请求（`DoWssRequest`）

不生效的场景：

- 异步任务类渠道（如 kling 等任务适配器自行设置 UA）
- 渠道测试请求：为避免把发起测试的管理员浏览器 UA 发送给上游，测试时不做透传

同一请求中 UA 的最终取值优先级（从高到低）：

1. 渠道 Header Override 显式配置的 `User-Agent`（最后应用，可覆盖一切）
2. 渠道适配器显式设置的 UA（如 kling 的 `kling-sdk/1.0`）
3. UA 透传名单命中 → 透传入口请求的原始客户端 UA
4. 默认上游 UA（`hertz`）

## 配置示例

```text
codex
claude-cli
Mozilla/5.0 (Windows NT 10.0)
```

含义：

- 客户端 UA 为 `codex-cli/1.0 (linux)` 时，命中片段 `codex`，上游收到的 `User-Agent` 为 `codex-cli/1.0 (linux)`；
- 客户端 UA 为普通浏览器 UA 且不含上述任一片段时，上游收到默认 UA `hertz`。

## 注意事项

- 片段建议尽量具体（如 `codex-cli` 而非单个字母），避免误命中。
- 若站点存在反向代理/网关改写入口 `User-Agent`，透传的是改写后的值。
- 修改配置保存后立即生效（全局配置运行时加载），无需重启。
