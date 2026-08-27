# 前端开发规范

本文档定义前端项目的开发规范与最佳实践，供开发与 AI 助手共同遵循。具体依赖与脚本以 `package.json` 为准。

---

## 一、项目概览

### 技术栈

| 类别     | 技术 |
|----------|------|
| 包管理   | Bun |
| 框架     | React 19、TypeScript |
| 数据与请求 | @tanstack/react-query、axios、Zustand |
| 路由     | @tanstack/react-router |
| 表格与列表 | @tanstack/react-table、@tanstack/react-virtual |
| 国际化   | i18next、react-i18next、i18next-browser-languagedetector |
| 日期     | Day.js |
| UI 与样式 | Base UI、Hugeicons、Tailwind CSS、clsx / class-variance-authority |
| 表单     | React Hook Form、Zod |
| 图表     | @visactor/vchart、@visactor/react-vchart |
| 工具     | qrcode.react、prettier、eslint、vitest（可选）|

优先选用成熟、维护良好的开源库；仅在现有库无法满足或需特殊适配时自行实现，并评估可维护性与通用性。

---

## 二、目录

- [一、项目概览](#一项目概览)
- [二、目录](#二目录)
- [三、开发规范](#三开发规范)
  - [3.1 国际化](#31-国际化)
  - [3.2 代码风格与类型](#32-代码风格与类型)
  - [3.3 组件](#33-组件)
  - [3.4 性能](#34-性能)
  - [3.5 状态管理](#35-状态管理)
  - [3.6 API 请求](#36-api-请求)
  - [3.7 表单](#37-表单)
  - [3.8 路由](#38-路由)
  - [3.9 错误处理](#39-错误处理)
  - [3.10 样式](#310-样式)
  - [3.11 文件组织](#311-文件组织)
  - [3.12 可访问性](#312-可访问性)
  - [3.13 安全](#313-安全)
  - [3.14 测试](#314-测试)
  - [3.15 依赖管理](#315-依赖管理)
  - [3.16 构建与部署](#316-构建与部署)
- [四、协作与提交](#四协作与提交)
- [更新日志](#更新日志)

---

## 三、开发规范

### 3.1 国际化

- **页面文本**：所有面向用户的文案均需支持 i18n，使用 `useTranslation()` 的 `t()` 进行翻译。
- **使用场景**  
  - **React 组件**：必须使用 `const { t } = useTranslation()`，以保证语言切换时组件会重新渲染。  
  - **非 React 环境**（工具函数、常量、类方法）：可使用 `import { t } from 'i18next'`；此类用法不会随语言切换自动更新，仅在不依赖响应式更新的场景使用。  
  - 即使父组件已使用 `useTranslation()`，子组件仍应自行使用，以保证独立性。
- **专有名词**：品牌、产品、技术术语等可保留英文（如 API、React、TypeScript）；若有约定俗成的译法则使用翻译。
- **翻译键**：使用有层级、语义清晰的键名，如 `dashboard.overview.title`，并保持命名一致。

- **枚举与文案（常量中的 i18n）**  
  各 feature 的 `constants.ts` 中常出现「枚举/状态 + 展示文案」或「成功/错误消息」，须统一约定以免遗漏 i18n、用法混乱：  
  - **成功/错误/提示类消息**（如 `SUCCESS_MESSAGES`、`ERROR_MESSAGES`）：常量值仅表示 **i18n 键**（与英文 fallback 同字面量）。展示时**必须**通过 `t()` 使用，例如 `toast.success(t(SUCCESS_MESSAGES.API_KEY_CREATED))`、`toast.error(t(ERROR_MESSAGES.UNEXPECTED))`，**禁止**直接 `toast.success(SUCCESS_MESSAGES.xxx)` 当作最终文案。  
  - **状态/选项的 label**：在常量中统一用 **labelKey**（字符串，即 i18n 键），组件中通过 `t(config.labelKey)` 渲染；或约定用 `label` 存与 en 一致的 key 字符串，组件用 `t(config.label)`。同一 feature 内只采用一种方式，避免混用。  
  - **新增此类常量时**：同步在 `src/i18n/static-keys.ts` 中登记对应 key（若项目用其做提取），或确保文案以 `t('...')` 字面量形式出现以便扫描，避免遗漏翻译。

### 3.2 代码风格与类型

- **表达式**：禁止 2 层及以上嵌套三元表达式；改用 `if-else`、提前返回或抽取函数。单层三元可保留，但需简洁。
- **可读性**：控制函数圈复杂度，复杂逻辑拆成小函数；变量与函数命名需有意义，遵循驼峰等常规约定。
- **TypeScript**：避免 `any`，优先具体类型或 `unknown`；为参数与返回值显式标注类型；仅类型用途的导入使用 `import type { X } from '...'`。
- **类型检查**：每次改动 TypeScript 或 TSX 代码后都要执行类型检查（如 `bun run typecheck`）；若出现类型错误，须修复至无错误为止，不得遗留。
- **解构**：对象非必要不要进行解构，特别是组件的 props；直接使用 `props.xxx` 更清晰，避免不必要的解构增加代码复杂度。

### 3.3 组件

- 使用函数式组件与 Hooks，单一职责；组件 props 须有明确类型（接口或类型别名）。
- **Props 使用**：组件 props 非必要不要解构，直接使用 `props.xxx` 访问属性，保持代码清晰（详见 [3.2 代码风格与类型](#32-代码风格与类型)）。
- 单文件超过约 200 行时考虑拆分子组件或将逻辑抽到自定义 Hooks；类型定义可与组件同文件或放在同模块的 `types` 中。

### 3.4 性能

- **React**：合理使用 `useMemo`、`useCallback` 减少无效重渲染；避免在渲染路径中创建新对象/数组；必要时使用 `React.memo`。
- **代码分割**：使用 `React.lazy` 与动态 `import` 做按需加载，控制首屏与路由体积。
- **资源**：图片选用合适格式与尺寸，大列表考虑虚拟滚动（如 @tanstack/react-virtual），大量图片考虑懒加载。

### 3.5 状态管理

- 使用 Zustand 的 `create` 定义 store，并为 state 与 actions 定义清晰类型。
- 组件内优先用选择器订阅，避免整 store 订阅导致多余渲染，例如：`const user = useAuthStore((s) => s.auth.user)`。
- 需持久化的状态在 store 内读写 localStorage，并在初始化时恢复。
- Store 按功能放在 `src/stores/`，单文件职责清晰，命名表意明确。

### 3.6 API 请求

- **React Query**：数据获取用 `useQuery`，变更用 `useMutation`；为每个查询配置唯一 `queryKey`（建议数组形式、层级一致）；在 `onSuccess` 中对相关 query 做 `invalidateQueries`，可配合乐观更新。服务端错误统一通过 `handleServerError` 处理（详见 [3.9 错误处理](#39-错误处理)）。
- **Axios**：使用项目统一的 `api` 实例（含 `baseURL`、`headers`、`withCredentials: true`）；GET 默认请求去重，特殊请求可通过配置关闭；认证与通用错误在拦截器中处理。

### 3.7 表单

- 使用 React Hook Form + Zod：在功能模块的 `lib/` 下定义 schema，并用 `z.infer` 导出表单类型；`useForm` 配合 `@hookform/resolvers/zod` 做校验。
- 提交逻辑放在 `onSubmit`，展示加载与错误状态；成功后视场景重置表单或关闭弹窗。服务端校验错误映射到对应字段并展示（字段级错误展示方式见 [3.9 错误处理](#39-错误处理)）。

### 3.8 路由

- 使用 TanStack Router，路由文件位于 `src/routes/`，通过 `createFileRoute` 定义；搜索参数用 Zod schema + `validateSearch` 校验。
- 在 `beforeLoad` 中做认证与重定向，避免不必要的请求；嵌套结构用布局路由与 `_authenticated` 等前缀，子路由通过 `<Outlet />` 渲染。
- 导航使用 `useNavigate` 或 `Link`，保持类型安全，避免直接操作 `window.location`。

### 3.9 错误处理

- **服务端错误**：统一使用 `handleServerError`，在 React Query 全局配置与拦截器中接入；按 HTTP 状态码给出合适提示，文案使用 i18n。
- **展示**：使用 `toast.error` 等统一方式；路由级错误由 `errorComponent` 承接，提供友好错误页并记录便于排查的信息。
- **表单**：校验与服务端错误映射到字段后，在字段下方展示；使用 `form.setError` 等与表单库一致的方式。

### 3.10 样式

- 以 Tailwind 工具类为主，动态类名用 `cn()` 合并；非动态场景避免内联样式。
- 响应式采用移动优先与 Tailwind 断点（`sm:`、`md:`、`lg:` 等）；主题与暗色用 CSS 变量与 `dark:`，自定义样式集中在 `src/styles/`，组件内尽量少写自定义 CSS。

### 3.11 文件组织

- **功能模块**：置于 `src/features/<feature>/`，内含 `components/`、`lib/`、`hooks/`，以及按需的 `api.ts`、`types.ts`、`constants.ts`、入口组件等。
- **通用**：通用组件放 `src/components/`，通用工具与类型放 `src/lib/`；组件文件 PascalCase，工具/类型文件 kebab-case 或 `types.ts`，类型使用 PascalCase 命名并 `export type`。

### 3.12 可访问性

- 使用语义化 HTML（如 `header`、`nav`、`main`、`footer`），表单用 `label` 关联输入。
- 保证键盘可操作与焦点顺序合理；必要时使用 ARIA（如 `aria-label`、`aria-expanded`、`aria-hidden`）；装饰性图标加 `aria-hidden="true"`，重要信息提供文本等价。
- 对比度满足 WCAG 2.1 AA（正文至少 4.5:1）。

### 3.13 安全

- 认证与权限在路由与接口层校验；敏感操作增加二次确认等。
- 前后端均做数据校验（如 Zod），不信任仅前端校验；敏感信息不落前端存储，配置用环境变量，禁止硬编码密钥。
- 依赖 React 默认转义，慎用 `dangerouslySetInnerHTML`；跨域与 Cookie 使用 `withCredentials` 并按后端要求处理 CSRF。

### 3.14 测试

- 工具函数与纯逻辑优先单元测试（Vitest），测试文件 `*.test.ts`；组件用 React Testing Library 测交互与行为，避免测实现细节。
- 关键流程补充集成与 E2E（如 MSW 模拟 API、Playwright/Cypress）；核心功能目标覆盖率 80% 以上，关注业务路径与关键分支。

### 3.15 依赖管理

- 使用 **Bun**：`bun install`、`bun add <pkg>`、`bun add -d <pkg>`、`bun remove <pkg>`、`bun pm ls`、`bun update` 等。
- 新增依赖前评估维护情况、体积与许可；生产与开发依赖区分清楚，版本用 `^`/`~` 控制，定期更新以获取安全修复。

### 3.16 构建与部署

- 使用 Rsbuild，配置见 `rsbuild.config.ts`；脚本以 `package.json` 为准（如 `bun run dev`、`bun run build`、`bun run typecheck`、`bun run lint`、`bun run format`），包管理见 [3.15 依赖管理](#315-依赖管理)。
- 代码分割与懒加载策略见 [3.4 性能](#34-性能)；资源使用合适格式与压缩，环境变量用 `.env` 且以 `VITE_` 前缀，不在代码中硬编码。
- **发布前**：执行 typecheck、lint、format 检查，完成生产构建并检查产物体积与环境变量配置。

---

## 四、协作与提交

- 提交信息清晰、符合项目约定，描述变更内容与原因，中英文统一即可。
- 变更需经过代码审查，符合本文档规范，并关注质量、性能与安全。
- 重大功能或规范变更时更新相关文档与 `AGENTS.md`。

---

## 更新日志

- **2026-01-28**：初始版本（国际化、代码、组件、类型等基础规范）。
- **2026-01-28**：补充状态管理、API、表单、路由、错误处理、样式、文件组织、可访问性、安全、测试、依赖与构建部署规范。
- **2026-01-29**：重组文档结构，合并重复内容，明确主次与交叉引用。
- **2026-01-31**：在 3.2 中补充「类型检查」要求：改动 TS/TSX 后须执行 typecheck 并修复至无错。
- **2026-08-25**：游乐场实现「上传文件/上传照片/截图/拍照」四功能：复用 `ai-elements/prompt-input` 附件能力；图片压缩为 JPEG dataURL 并以 `image_url` 发送；文本类文件内容内联进消息文本；截图用 `getDisplayMedia`、拍照用 `getUserMedia` 弹窗捕获；附件仅内存保存（localStorage 中剥离 url）。新增 `MessageAttachment` 类型、`attachment-utils.ts`、`camera-capture-dialog.tsx`、`message-attachments.tsx` 及对应单测。
- **2026-08-26**：模型元数据编辑侧栏（/models/metadata）移除价格配置区块，改为跳转「系统设置 → 计费 → 模型定价」的链接（`MODEL_PRICING_SETTINGS_URL`）。原因：该区块通过 `/api/option/`（需 RootAuth）写入全局倍率配置，与计费设置页写入同一批 key，但对非 root 管理员读写均静默失效，且不支持 `CreateCacheRatio` 与表达式计费，易造成"配置不生效"的误解。表单 schema 剔除 7 个价格字段并导出 `modelFormSchema` 供测试；新增对应单测与 6 语言 i18n 文案。
- **2026-08-26**：/image-debug 页面的模型/分组下拉只显示有效选项：后端 `GET /api/user/models`、`GET /api/user/self/groups` 新增可选 `endpoint` 查询参数（逗号分隔端点类型，缺省行为不变），仅返回支持对应端点（如 `image-generation`/`image-edit`，依据 `model.GetModelSupportEndpointTypes`）的模型及包含它们的分组；前端新增 `getModeEndpoints`/`resolveSelection` 纯函数（`features/image-debug/lib/selection.ts` + 单测），按文生图/图生图模式动态传参并自动修正失效选中值。后端配套 `controller/endpoint_filter.go` 及集成测试。
- **2026-08-26**：模型编辑端点模板（`features/models/constants.ts` 的 `ENDPOINT_TEMPLATES`）新增 `openai-video`（`POST /v1/videos`），便于为异步视频生成模型（如阿里万相3.0）配置 OpenAI 兼容视频任务端点。
- **2026-08-26**：/image-debug 页面新增「视频」调试 Tab（`openai-video` 端点）：支持文生视频/首帧生视频/参考生视频（可传 1-3 张图转 Base64 dataURL）、宽高比/分辨率/时长（2-30 秒或 -1 智能时长）参数，提交 `POST /pg/videos` 后轮询 `GET /pg/videos/:task_id` 展示进度与结果视频；新增 `video-debug-tab`/`video-debug-form`/`video-result` 组件、`lib/video-payload.ts`/`lib/file-utils.ts` 及单测；后端配套 `/pg/videos` 路由（Playground → RelayTask 流程）与 `resolveVideoRelayMode` 路径识别（distributor）及单测；i18n 六语言新增 18 个文案。
- **2026-08-26**：修复视频调试无限转圈：`GET /pg/videos/:task_id` 现在与 `/v1/videos/:task_id` 一样返回 OpenAI 兼容视频格式（`relay/relay_task.go` 的 `isOpenAIVideoAPIPath` 同时识别 `/pg/videos/` 前缀）；阿里 `ConvertToOpenAIVideo` 改为从数据库实时字段（`task.Status`/`task.GetResultURL()`）读取状态与视频地址，消除 stuck-queued 问题。
- **2026-08-26**：模型定价「添加模型定价」（/system-settings/billing/model-pricing）的模型名称字段支持「输入 + 下拉选择」：候选来自 `GET /api/channel/models_enabled`（启用渠道配置的模型 + 启用虚拟模型），并排除当前定价草稿中已设置定价的模型；仍支持手动输入任意模型名。新增 `buildModelNameOptions` 纯函数（`features/system-settings/models/model-name-options.ts` + 单测），`ModelPricingEditorPanel`/`ModelPricingSheet` 新增 `modelNameOptions` 属性并复用现有 `Combobox`（`allowCustomValue`）；i18n 六语言新增 1 个文案。
- **2026-08-26**：/image-debug 页面提示词一键 AI 优化：图片/视频表单提示词区新增「优化模型」下拉 + 「AI 优化」按钮，复用 `POST /pg/chat/completions`（非流式、携带当前分组、走 playground 计费）以用户选择的文本模型润色提示词并直接替换输入框内容；新增 `PromptOptimizer` 共享组件、`lib/prompt-optimizer.ts` 纯函数（构建请求体/解析响应/错误提取）+ 单测、文档 `docs/prompt-optimizer.md`；i18n 六语言新增 6 个文案。
- **2026-08-26**：优化模型候选列表支持筛选与排序：候选经 `GET /api/user/models?endpoint=openai` 仅保留支持 OpenAI 聊天端点的文本模型（`TEXT_MODEL_ENDPOINTS` 常量），排除图片/视频/精选/向量化等非文本模型，并按名称不区分大小写自然序排序（`sortModelOptions` 纯函数 + 单测）。
- **2026-08-26**：优化模型选择支持记忆：上次使用的优化模型保存到浏览器 localStorage（键 `image-debug-prompt-optimize-model`，`PROMPT_OPTIMIZE_MODEL_STORAGE_KEY` 常量），再次进入 /image-debug 时作为默认选择；候选加载后校验存储值仍在列表中，失效则回退第一个（`getStoredOptimizeModel`/`saveStoredOptimizeModel`/`resolveStoredModel` 纯函数 + 单测）。
- **2026-08-27**：/usage-logs/task 任务日志新增详情弹窗与结果下载：Details 列新增眼睛图标入口，所有人可打开任务详情弹窗（基础信息 + 输入 + 失败原因；渠道/用户行仅管理员）；管理员额外可见「最后一次上游轮询」区块（后端新增 `poll_record` 字段记录每轮轮询的方法/URL/状态码/请求体/响应体，仅 `GET /api/task/` 管理员接口下发）；弹窗内「下载生成结果」按钮仅对本人提交的 SUCCESS 任务显示（Suno 下载 `audio_url` 音频，其余走 `/v1/videos/:task_id/content` 代理，fetch→blob 失败回退新标签页）。新增 `task-details-dialog.tsx`、`lib/download.ts`（`resolveTaskDownloads`/`canDownloadTaskResult`/`extFromContentType` 纯函数 + 单测）；`TaskLog` 类型补充 `result_url`/`properties`/`poll_record`/`quota`/`start_time` 字段；i18n 六语言新增 11 个文案。
