# MCP Server 文档

> 面向使用方与管理员。实现代码位于 [controller/mcp_server.go](../controller/mcp_server.go)（工具注册与 handler）、
> [setting/mcp_setting/config.go](../setting/mcp_setting/config.go)（模型池配置）、
> [service/mcp_image_cache.go](../service/mcp_image_cache.go)（图片缓存）。

## 1. 端点总览

| 方法 | 路径 | 认证 | 说明 |
|---|---|---|---|
| POST/GET/DELETE | `/v1/mcp`（含 `/*path`） | TokenAuth | MCP StreamableHTTP 端点（Stateless，每个请求独立处理） |
| POST | `/v1/mcp-upload`（或 `/v1/mcp-upload/upload`） | TokenAuth | 临时图片上传（multipart，2 小时后自动删除） |
| GET | `/v1/mcp-image/:imageId` | 无 | 图片代理（生成结果与临时上传共用，支持可选扩展名如 `.png`） |

MCP 端点复用令牌（`sk-xxx`）认证：`Authorization: Bearer sk-xxx`。工具 handler 通过请求上下文获取
调用方令牌分组，决定使用哪个分组的模型配置。

## 2. 临时图片上传

`POST /v1/mcp-upload`，`Content-Type: multipart/form-data`，字段 `file`。

限制：

- 单文件最大 **20MB**
- 允许的图片类型（按文件内容嗅探，不信任扩展名）：`image/png`、`image/jpeg`、`image/webp`、`image/gif`
- 保留时长 **2 小时**，到期自动删除（内存 LRU + 可选 Redis 混合缓存）

响应：

```json
{
  "success": true,
  "message": "",
  "data": {
    "id": "abc123def4567890",
    "url": "https://your-server/v1/mcp-image/abc123def4567890.png",
    "mime_type": "image/png",
    "size": 123456,
    "expires_in": 7200
  }
}
```

返回的 `id` 用于 `generate_image` / `generate_video_from_frames` / `generate_video_from_reference`
工具的图片参数（`image_ids` / `first_frame_id` / `last_frame_id`）。工具参数同时也兼容直接传
`url`（代理 URL 或任意公网 URL）。

curl 示例：

```bash
curl -H "Authorization: Bearer sk-xxx" -F file=@source.png \
  https://your-server/v1/mcp-upload
```

## 3. 工具列表

### 3.1 `generate_image` — 文生图 / 图生图

| 参数 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `prompt` | string | 是 | 文生图描述；图生图时为编辑指令 |
| `image_ids` | string[]（1–3） | 否 | 临时图片 ID（`/v1/mcp-upload` 返回）。提供时执行图生图 |

- 仅 `prompt`：走 `/v1/images/generations`，使用 `mcp_setting.group_image_models` 配置的模型。
- 带 `image_ids`：走 `/v1/images/edits`（JSON 请求，`image` 为 URL 数组），使用
  `mcp_setting.group_i2i_models` 配置的模型。上游适配器（如阿里 qwen-image 系列）会按 1–3 张
  输入图构造多模态编辑请求；qwen-image 系列超过 3 张会被拒绝。
- 返回：每张图两条内容 — `ImageContent`（base64，可直接渲染）+ `TextContent`（本站代理 URL）。

### 3.2 `generate_video` — 文生视频（异步）

| 参数 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `prompt` | string | 是 | 视频描述 |
| `duration` | number | 否 | 时长（秒），支持范围依模型而定 |
| `size` | string | 否 | 分辨率/比例（如 `1280x720`、`720P`、`16:9`），依模型而定 |

使用 `mcp_setting.group_video_t2v_models` 配置的模型。

### 3.3 `generate_video_from_frames` — 首帧 / 首尾帧生视频（异步）

| 参数 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `prompt` | string | 是 | 视频描述 |
| `first_frame_id` | string | 是 | 首帧临时图片 ID |
| `last_frame_id` | string | 否 | 尾帧临时图片 ID |
| `duration` / `size` | 同上 | 否 | |

- 仅 `first_frame_id` → 使用 `mcp_setting.group_video_i2v_models`（图生视频，首帧驱动）。
- `first_frame_id` + `last_frame_id` → 使用 `mcp_setting.group_video_kf2v_models`（首尾帧插值）。

### 3.4 `generate_video_from_reference` — 参考图生视频（异步）

| 参数 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `prompt` | string | 是 | 视频描述 |
| `image_ids` | string[]（1–3） | 是 | 参考图临时图片 ID |
| `duration` / `size` | 同上 | 否 | |

使用 `mcp_setting.group_video_r2v_models` 配置的模型。

### 3.5 视频提交结果（3 个提交工具共用）

提交成功立即返回（不等待生成完成）：

```json
{
  "task_id": "task_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
  "status": "submitted",
  "model": "wan2.5-t2v-preview",
  "message": "Video generation task submitted. Poll get_video_task with task_id to check progress.",
  "estimated_wait_seconds": 60
}
```

### 3.6 `get_video_task` — 查询视频任务

| 参数 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `task_id` | string | 是 | 提交工具返回的任务 ID |

返回：

```json
{
  "task_id": "task_xxx",
  "status": "SUCCESS",
  "progress": "100%",
  "fail_reason": "",
  "result_url": "https://cdn-upstream/xxx.mp4",
  "proxy_url": "https://your-server/v1/videos/task_xxx/content"
}
```

- 状态枚举与 REST `GET /v1/videos/:task_id` 一致（`NOT_START` / `SUBMITTED` / `QUEUED` /
  `IN_PROGRESS` / `SUCCESS` / `FAILURE`）。进度由后台轮询器（每 15 秒）更新。
- 任务失败时已自动退款；`fail_reason` 为上游错误信息。
- 只能查询本人提交的任务。
- `proxy_url` 为本站视频内容代理，有效期与任务记录一致，适合 Agent 直接下载。

### 3.7 调用示例（JSON-RPC）

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "tools/call",
  "params": {
    "name": "generate_video_from_frames",
    "arguments": {
      "prompt": "镜头从日出的海面缓慢拉远",
      "first_frame_id": "abc123def4567890",
      "last_frame_id": "def456abc7890123",
      "duration": 5
    }
  }
}
```

## 4. 模型池配置

系统设置 → 模型设置 → **MCP Image Generation**（结构化编辑器：分组下拉 + 模型下拉，
均支持手动输入自定义值；新增/删除行立即生效，空分组行保存时自动忽略；点击保存
无条件提交全部 6 个配置项，不做"无更改则拦截"的判断，且编辑期间窗口聚焦触发的
数据刷新不会清空未保存的修改）。对应 6 个 option key（均为 `{"分组": "模型名"}` JSON）：

| 配置 key | 用途 | 工具 |
|---|---|---|
| `mcp_setting.group_image_models` | 文生图 | `generate_image` |
| `mcp_setting.group_i2i_models` | 图生图 | `generate_image` + `image_ids` |
| `mcp_setting.group_video_t2v_models` | 文生视频 | `generate_video` |
| `mcp_setting.group_video_i2v_models` | 图生视频（首帧） | `generate_video_from_frames` 仅首帧 |
| `mcp_setting.group_video_kf2v_models` | 首尾帧生视频 | `generate_video_from_frames` 首帧+尾帧 |
| `mcp_setting.group_video_r2v_models` | 参考图生视频 | `generate_video_from_reference` |

回退规则：调用方分组未配置时回退 `default` 分组的模型；`default` 也未配置时该工具返回
错误（提示缺少哪个配置项）。视频模型池默认全部为空，需按需配置。

## 5. 计费

- 图片（文生图/图生图）：按图片模型计费（价格/倍率体系与 `/v1/images/*` 一致），
  请求前预扣费，失败自动退款。
- 视频：按异步任务计费（`ModelPriceHelperPerCall`：优先按次价格，其次模型倍率预扣一半，
  再叠加渠道 OtherRatios 如时长/分辨率档位）。提交成功即结算；任务失败由后台轮询器自动退款。
- 临时图片上传与查询不消耗额度。

## 6. 与 REST 端点的对应关系

| MCP 工具 | 等价 REST | 差异 |
|---|---|---|
| `generate_image` | `POST /v1/images/generations` / `POST /v1/images/edits` | MCP 按 token 分组自动选模型；返回内容附带可渲染的 base64 |
| `generate_video*` | `POST /v1/videos`（`metadata.first_frame_image` / `last_frame_image` / `reference_images`） | MCP 先经 `/v1/mcp-upload` 上传图片，模型由分组配置注入 |
| `get_video_task` | `GET /v1/videos/:task_id` | 归属校验一致；返回精简 JSON |

## 7. 部署注意事项

- 图片以本站代理 URL（`{scheme}://{host}/v1/mcp-image/{id}`）传给上游渠道，
  要求站点可被上游公网访问（与生成图返回代理 URL 的既有行为一致）。
- `/v1/mcp-image/:id` 为无认证只读端点（设计上便于上游拉取），请勿将敏感内容上传。
- Redis 已启用时，临时图片与生成图缓存跨实例共享；未启用时仅存本实例内存，重启即失效。
