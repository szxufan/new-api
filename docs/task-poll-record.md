# 任务轮询记录（poll_record）与任务结果下载

## 功能说明

### 1. 任务详情弹窗与轮询记录

在「使用日志 → 任务日志」（`/usage-logs/task`）页面，每行 Details 列新增眼睛图标按钮，点击打开**任务详情弹窗**（所有用户均可打开自己可见的任务）：

- 基础信息：Task ID、平台、动作、状态、进度、额度、提交/开始/完成时间、输入（properties.input）、失败原因；
- 管理员额外可见：渠道、用户、**「最后一次上游轮询」区块**；
- 弹窗底部：符合条件的任务显示「下载生成结果」按钮（见下文）。

「最后一次上游轮询」区块展示后端任务轮询循环（`service.TaskPollingLoop`，每 15 秒一轮）**最近一次**向渠道上游查询任务状态时的：

| 字段 | 说明 |
|---|---|
| `time` | 轮询时间（unix 秒） |
| `method` / `url` | 实际发出的 HTTP 方法与上游 URL（取自 `resp.Request`；若发生重定向则为最后一次请求） |
| `status_code` | 上游返回的 HTTP 状态码 |
| `request` | 轮询请求体（视频任务为 `{"task_id","action"}`；Suno 为批量 `{"ids":[...]}`） |
| `response` | 上游响应体（视频任务经 `redactVideoResponseBody` 脱敏；Suno 为该任务对应的响应条目） |

### 2. 下载生成结果

详情弹窗内的「下载生成结果」按钮仅在 **任务状态为 SUCCESS 且任务由当前登录用户本人提交** 时显示（管理员查看他人任务时不显示）：

- Suno：逐个下载 `data` 中的 `audio_url` 音频；
- 其他平台：走同源代理 `GET /v1/videos/:task_id/content`（`controller.VideoProxy`），后端同样强制「仅本人 + SUCCESS」，并按渠道类型回源（Gemini/Vertex 使用私有 key、OpenAI 拼上游 content 路径、其余使用 `GetResultURL()`）。
- 前端优先 `fetch → blob` 下载（可控制文件名，按 Content-Type 修正扩展名）；跨域受限时回退为新标签页打开。

## 实现方式

### 后端

- `model/task.go`
  - 新增 `TaskPollRecord` 结构体（`Scan`/`Value` 走 `common.Marshal/Unmarshal`，与 `TaskPrivateData` 同模式）；
  - `Task` 新增 `PollRecord` 字段：`gorm:"column:poll_record;type:json"`，AutoMigrate 自动加列（SQLite/MySQL/PostgreSQL 均为 ADD COLUMN，兼容）；
  - `taskSnapshot` 纳入 `PollRecord`（`EqualPollRecord` 比较）：即使任务状态/进度无变化，轮询记录变化也会触发写库，保证记录始终反映最后一次轮询。
- `service/task_polling.go`
  - `newPollRecord(resp, request, response)`：从 `resp.Request` 捕获方法与 URL，构造记录；请求/响应各自截断到 64KB（`maxPollRecordPayloadBytes`，超长追加 `...(truncated)`）；
  - 视频路径 `updateVideoSingleTask`：每轮写入 `task.PollRecord`（响应为脱敏后的响应体）；上游返回 429 的提前返回分支单独以 `TaskBulkUpdateByID` 持久化该列；
  - Suno 路径 `updateSunoTasks`：遍历本渠道全部未完成任务（而非仅上游返回的条目），逐个写入记录并持久化；上游未返回的任务记录中响应为空。
- `dto/task.go` / `controller/task.go`
  - `TaskDto` 新增 `poll_record`（指针，`omitempty`）；
  - 仅管理员列表接口 `GET /api/task/`（`tasksToDto(items, true)`）填充；`GET /api/task/self` 与 `/v1/videos/:task_id` 等 fetch 端点一律不含（`relay.TaskModel2Dto` 不映射该字段）。

### 前端（`web/default/src/features/usage-logs/`）

- `types.ts`：`TaskLog` 新增 `result_url`、`properties`、`poll_record`、`quota`、`start_time`；新增 `TaskPollRecord`、`TaskLogProperties` 类型；
- `lib/download.ts`：`resolveTaskDownloads`（Suno 音频条目 / 视频代理 URL）、`canDownloadTaskResult`（本人 + SUCCESS）、`extFromContentType`、`downloadTaskResults`（fetch→blob，失败回退新标签页），纯函数见 `lib/download.test.ts`；
- `components/dialogs/task-details-dialog.tsx`：任务详情弹窗（含管理员专属轮询区块与下载按钮）；
- `components/columns/task-logs-columns.tsx`：Details 列追加眼睛图标入口，保留原有音频预览/视频链接/失败原因展示。

## 已知限制

- 轮询请求/响应仅在轮询循环成功发出 HTTP 请求并收到响应后落库；Suno 批量请求遇到非 200/解析失败等提前返回分支时，该轮不落记录（错误仅记日志）。
- 请求/响应各最大存储 64KB，超长截断。
- 旧任务（功能上线前）无轮询记录，弹窗显示「暂无轮询记录」。
- 用户主动 fetch（`tryRealtimeFetch`）不属于后端轮询，不写入轮询记录。
- Midjourney 任务走独立的绘图日志体系（`/usage-logs/drawing`），不在本功能范围内。
