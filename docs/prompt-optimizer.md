# /image-debug 提示词一键 AI 优化

## 功能说明

在「图像调试」（`/image-debug`）页面的图片生成与视频生成提示词输入框旁，提供「AI 优化」能力：

- 用户从下拉框选择一个**优化模型**（候选为管理员在模型元数据中标记「可用于 AI 优化提示词」的模型）；
- 点击「AI 优化」后，系统将当前提示词发送给该模型进行优化；
- 优化结果**直接替换**输入框中的提示词内容（可 Ctrl+Z 撤销）。

图片与视频两个 Tab 使用不同的优化指令：

- **图片**：润色为更适合图像生成的提示词（构图/光影/风格等）；
- **视频**：根据表单填写的时长生成**以 5 秒为单位的分镜头脚本**（镜头序号、时间区间、画面内容、镜头运动）；若表单上传了输入图片，图片会以多模态消息一并传给优化模型作为参考。

## 候选模型配置（管理员）

优化模型候选由管理员在「模型管理 → 模型元数据」（`/models/metadata`）中配置：

- 编辑模型时开启「可用于 AI 优化提示词」开关（后端字段 `models.prompt_optimize`，int 0/1，默认 0）；
- 标记记录按元数据的**名称规则**（精确/前缀/后缀/包含）匹配渠道中实际启用的模型名（与模型定价的元数据匹配方式一致）。例如以「前缀」规则标记 `qwen3.8`，则渠道中所有 `qwen3.8-*` 模型均可作为优化模型；
- 普通用户侧通过 `GET /api/user/models?prompt_optimize=true` 仅返回命中标记的模型（与用户可用模型、`?endpoint=` 过滤取交集）；
- 未标记任何模型时，/image-debug 优化模型下拉显示「暂无可用的优化模型」。

建议标记支持视觉（vision）能力的文本模型，以便视频优化时理解参考图片。

## 实现方式

- 调用端点：`POST /pg/chat/completions`（非流式），复用 playground 中继管线
  - 认证：与 /image-debug 其他请求一致，走 Cookie 会话认证（`UserAuth`）
  - 分组：请求体携带表单当前选择的分组（`group` 字段），后端经 `GroupInUserUsableGroups` 校验后使用
  - 计费：走 playground 常规计费流程（预扣费 + 结算），消费日志 `token_name` 显示为 `playground-<分组>`
  - 多模态：视频优化携带图片时 `content` 为「文本 + image_url」数组（dataURL 直传），后端 `dto.Message.ParseContent` 原生支持
- 后端文件：
  - `model/model_meta.go`：`Model.PromptOptimize` 字段（`Update()` Select 白名单已包含）与 `GetPromptOptimizeModels()` 查询
  - `controller/endpoint_filter.go`：`filterPromptOptimizeModels` / `matchesPromptOptimizeModel`（按元数据名称规则匹配实际模型名）
  - `controller/user.go`：`GetUserModels` 支持 `?prompt_optimize=true`（测试见 `controller/endpoint_filter_test.go`）
- 前端文件：
  - `web/default/src/features/models/components/drawers/model-mutate-drawer.tsx`：元数据编辑表单的「可用于 AI 优化提示词」开关
  - `web/default/src/features/image-debug/constants.ts`：`CHAT_COMPLETIONS` 端点、`TEXT_MODEL_ENDPOINTS`、`PROMPT_OPTIMIZE_MODEL_STORAGE_KEY` 与图像/视频优化系统提示词
  - `web/default/src/features/image-debug/types.ts`：`PromptOptimizeType` / `OptimizeContentPart` / `OptimizePromptRequest` / `OptimizePromptResponse`
  - `web/default/src/features/image-debug/api.ts`：`optimizePrompt(payload)`、`getUserModels(endpoints, promptOptimize)`
  - `web/default/src/features/image-debug/lib/prompt-optimizer.ts`：`buildOptimizePayload`（视频时长分镜说明 + 图片多模态）/ `extractOptimizedPrompt` / `getOptimizeErrorMessage` / `sortModelOptions` / localStorage 记忆相关纯函数，见 `lib/prompt-optimizer.test.ts`
  - `web/default/src/features/image-debug/components/prompt-optimizer.tsx`：共享 UI 组件（模型下拉 + 优化按钮）；候选 = 管理员标记 ∩ 用户可用 ∩ 支持 openai 端点，按名称排序；上次选择保存在 localStorage（键 `image-debug-prompt-optimize-model`）

## 使用说明

1. 管理员在 `/models/metadata` 为可供优化的模型开启开关；
2. 进入「图像调试」页面，在图片或视频 Tab 中填写提示词；
3. 在提示词标签旁的「优化模型」下拉框中选择模型；
4. 点击「AI 优化」按钮（提示词为空、无候选模型或生成任务提交中时按钮不可用）；
5. 优化完成后输入框内容被替换，并弹出成功提示；失败时展示后端返回的错误信息。

## 注意事项

- 视频优化的分镜数由前端按时长计算（`ceil(时长/5)`）写入请求；时长为 -1（智能时长）时由优化模型自行设计总时长；时长 ≤0 按默认 5 秒处理。
- 视频表单的输入图片（≤3 张 dataURL）随优化请求以 `image_url` 传给优化模型；优化模型需具备视觉能力才能理解图片。
- 上次使用的优化模型保存在浏览器 localStorage，再次进入页面时作为默认选择；若该模型已不在候选中，自动回退到候选第一个。
- 优化调用复用表单当前选择的分组；若该分组下所选模型没有可用渠道，后端返回的错误会展示在 toast 中，可切换分组或更换模型。
- 系统提示词要求模型保持与原提示词相同的语言输出，仅输出优化后文本/分镜脚本（无解释、无代码块）。
