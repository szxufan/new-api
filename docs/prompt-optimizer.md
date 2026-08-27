# /image-debug 提示词一键 AI 优化

## 功能说明

在「图像调试」（`/image-debug`）页面的图片生成与视频生成提示词输入框旁，提供「AI 优化」能力：

- 用户从下拉框选择一个**文本模型**（候选为用户全部可用模型）；
- 点击「AI 优化」后，系统将当前提示词发送给该模型，要求其润色为更适合图像/视频生成的提示词；
- 优化结果**直接替换**输入框中的提示词内容（可 Ctrl+Z 撤销）。

图片（文生图/图生图）与视频两个 Tab 的提示词区均提供该功能，分别使用不同的优化指令（图像侧强调构图/光影/风格，视频侧额外强调镜头运动与主体随时间的变化）。

## 实现方式

纯前端实现，无后端改动：

- 调用端点：`POST /pg/chat/completions`（非流式），复用 playground 中继管线
  - 认证：与 /image-debug 其他请求一致，走 Cookie 会话认证（`UserAuth`）
  - 分组：请求体携带表单当前选择的分组（`group` 字段），后端经 `GroupInUserUsableGroups` 校验后使用
  - 计费：走 playground 常规计费流程（预扣费 + 结算），消费日志 `token_name` 显示为 `playground-<分组>`
- 前端文件（`web/default/src/features/image-debug/`）：
  - `constants.ts`：新增 `CHAT_COMPLETIONS` 端点、`TEXT_MODEL_ENDPOINTS` 端点过滤与图像/视频优化系统提示词常量
  - `types.ts`：`PromptOptimizeType` / `OptimizePromptRequest` / `OptimizePromptResponse` 等类型
  - `api.ts`：`optimizePrompt(payload)` 请求封装
  - `lib/prompt-optimizer.ts`：`buildOptimizePayload`（构建请求体）/ `extractOptimizedPrompt`（解析响应，剥离 markdown 代码块围栏）/ `getOptimizeErrorMessage`（错误文案提取）/ `sortModelOptions`（候选排序）/ `getStoredOptimizeModel`/`saveStoredOptimizeModel`/`resolveStoredModel`（模型记忆持久化），纯函数，见 `lib/prompt-optimizer.test.ts`
  - `components/prompt-optimizer.tsx`：共享 UI 组件（模型下拉 + 优化按钮），图片/视频表单复用；模型候选通过 `GET /api/user/models?endpoint=openai` 获取（仅保留支持 OpenAI 聊天端点的文本模型，排除图片/视频/精选/向量化等非文本模型），并按名称不区分大小写排序；上次选择的模型保存在 localStorage（键 `image-debug-prompt-optimize-model`），再次进入页面时作为默认选择；两个 Tab 共用同一 queryKey 缓存

## 使用说明

1. 进入「图像调试」页面，在图片或视频 Tab 中填写提示词；
2. 在提示词标签旁的「优化模型」下拉框中选择文本模型；
3. 点击「AI 优化」按钮（提示词为空、无可用模型或生成任务提交中时按钮不可用）；
4. 优化完成后输入框内容被替换，并弹出成功提示；失败时展示后端返回的错误信息。

## 注意事项

- 优化模型候选仅包含支持 OpenAI 聊天端点（`endpoint=openai`）的文本模型，图片/视频/精选/向量化等模型不会出现在列表中；列表按名称不区分大小写排序。
- 上次使用的优化模型会保存在浏览器 localStorage（键 `image-debug-prompt-optimize-model`），再次进入页面时作为默认选择；若该模型已不可用（如被移除），自动回退到候选列表第一个。
- 优化调用复用表单当前选择的分组；若该分组下选择的文本模型没有可用渠道，后端会返回错误并展示在 toast 中，此时可切换分组或更换模型。
- 优化模型候选默认取列表中第一个，未选择时自动回退到第一个。
- 优化请求不携带任何图片内容，仅对提示词文本进行润色。
- 系统提示词要求模型保持与原提示词相同的语言输出，仅输出优化后文本（无解释、无代码块）。