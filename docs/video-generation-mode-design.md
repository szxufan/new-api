# 视频生成「模式」统一契约设计

> **状态：阶段 -1 / 0 / 1 已实施，阶段 2 / 3 仍为提案。** 本文档描述接口设计，供评审后再决定后续实施范围。
> **调用方文档**：面向使用者的接口说明见 [video-api.md](./video-api.md)。
> 文中「现状」章节描述的是当前代码的真实行为（已逐条对照代码核实），可直接作为排障参考。
> 实施落点：统一解析层位于 [relay/common/video_media_plan.go](file:///home/xufan/trea_project/new-api/relay/common/video_media_plan.go)（`MediaPlan` / `BuildMediaPlan` / `MediaLimits` / `ExclusivePolicy`），各渠道接入见 §8 标注。

## 1. 问题陈述

`/v1/videos` 体系下，多数视频模型支持多种生成模式（文生视频、首帧生视频、首尾帧插值、参考生视频），但**客户端没有任何统一手段指定使用哪一种**。模式判定依赖 `images` 数组长度这一隐式启发式，而各渠道对同一长度的语义解释互相冲突。

### 1.1 现状：模式判定的三条互不兼容路径

| 路径                                    | 使用渠道                                                                                  | 问题                                                            |
| --------------------------------------- | ----------------------------------------------------------------------------------------- | --------------------------------------------------------------- |
| `info.Action`（`constant.TaskAction*`） | kling、vidu 用它选上游端点；hailuo v2 以它作协议版本标记供轮询分发（见 §3.5）；其余仅落库 | 只有 vidu 允许客户端经 `metadata.action` 指定，其余渠道完全不读 |
| `images` 数组长度（隐式推导）           | vidu、jimeng、hailuo v2、ali(wan3/minimax)                                                | **语义不一致**，见 §1.2                                         |
| 渠道私有 metadata 具名键                | kling、hailuo v1/v2、doubao、ali                                                          | 字段名各家不同，等于没有统一接口                                |

### 1.2 核心冲突：同一输入在不同渠道产生不同模式

| 输入    | vidu   | hailuo v2 | ali minimax | **ali wan3**   | jimeng |
| ------- | ------ | --------- | ----------- | -------------- | ------ |
| 1 张图  | 首帧   | 首帧      | 首帧        | 首帧           | 首帧   |
| 2 张图  | 首尾帧 | 首尾帧    | 首尾帧      | **2 个参考图** | 首尾帧 |
| ≥3 张图 | 参考   | 参考      | 参考        | 参考           | 首尾帧 |

代码位置：[vidu/adaptor.go#L98-L105](file:///home/xufan/trea_project/new-api/relay/channel/task/vidu/adaptor.go#L98-L105)、[hailuo/adaptor_v2.go#L103-L108](file:///home/xufan/trea_project/new-api/relay/channel/task/hailuo/adaptor_v2.go#L103-L108)、[ali/adaptor.go#L745-L752](file:///home/xufan/trea_project/new-api/relay/channel/task/ali/adaptor.go#L745-L752)、[ali/adaptor.go#L624-L634](file:///home/xufan/trea_project/new-api/relay/channel/task/ali/adaptor.go#L624-L634)、[jimeng/adaptor.go#L413-L418](file:///home/xufan/trea_project/new-api/relay/channel/task/jimeng/adaptor.go#L413-L418)。

### 1.3 根因

1. **无契约**：`TaskSubmitReq`（[relay_info.go#L708-L719](file:///home/xufan/trea_project/new-api/relay/common/relay_info.go#L708-L719)）只有 `Image` / `Images` / `InputReference` 三个无角色语义的素材字段。
2. **无能力声明**：适配器不对外暴露自己支持哪些模式，客户端只能试错。
3. **静默降级**：模式猜错、素材被丢弃时不报错（见 §7 缺陷清单），问题被掩盖。
4. **`Mode` 字段已被占用**：`TaskSubmitReq.Mode` 的唯一读取点是 [kling/adaptor.go#L270](file:///home/xufan/trea_project/new-api/relay/channel/task/kling/adaptor.go#L270)，映射为可灵的 `std`/`pro` 清晰度档位，**不可复用**为生成模式。

## 2. 设计目标与非目标

### 目标

- 客户端可用**一套跨渠道统一的字段**精确指定模式与素材角色
- 现有请求（不传新字段）**行为完全不变**，零回归
- 渠道能力可被探测，不支持的模式给出明确错误而非静默降级
- 消除各渠道重复实现的互斥降级 / 数量截断逻辑（目前至少 3 份拷贝）

### 非目标

- 不追求 OpenAI 官方规范兼容 —— `/v1/videos` 官方仅有 `input_reference` 单图，任何多素材表达都是本项目的扩展
- 不改变计费模型（`OtherRatios` 机制保持原样）
- 不改变异步任务轮询链路（`task.Action` 的持久化与回传机制保持）

## 3. 对外契约

### 3.1 新增字段

```jsonc
{
  "model": "MiniMax/MiniMax-H3",
  "prompt": "镜头缓慢推进……",

  // 可选。缺省时回落 §3.3 的推导链，行为与现状一致
  "generation_mode": "first_last_frame",

  // 可选。带角色语义的素材数组，与 images / input_reference 并存时优先生效
  "media": [
    { "type": "first_frame", "url": "https://example.com/a.png" },
    { "type": "last_frame", "url": "https://example.com/b.png" },
  ],
}
```

### 3.2 `generation_mode` 枚举

对外采用业界通用命名，对内映射到已有的 `constant.TaskAction*`，**不新造内部枚举**。

| 对外值             | 内部常量                      | 语义                                                        |
| ------------------ | ----------------------------- | ----------------------------------------------------------- |
| `text2video`       | `TaskActionTextGenerate`      | 纯文本提示词                                                |
| `image2video`      | `TaskActionGenerate`          | 首帧生视频                                                  |
| `first_last_frame` | `TaskActionFirstTailGenerate` | 首尾帧插值                                                  |
| `reference2video`  | `TaskActionReferenceGenerate` | 多模态参考生视频                                            |
| `remix`            | `TaskActionRemix`             | 仅由 `/v1/videos/{id}/remix` 路径触发，**不接受客户端指定** |

注意：`constant.TaskActionVideoV2Generate` 不属于模式枚举 —— 它是 hailuo v2 的协议版本标记（供后台轮询选择 v2 查询端点），与生成模式正交，见 §3.5。

`media[].type` 取值：`first_frame` / `last_frame` / `reference_image` / `reference_video` / `reference_audio`。

选择 `media[{type,url}]` 作为标准结构，是因为它与四家上游形态同构，映射损耗最小：阿里 `input.media[]`、豆包 `content[]`、MiniMax v2 `content[]`、可灵 `image` + `image_tail`。

### 3.3 解析优先级

```
逃生通道（metadata.input.media / metadata.content 整体覆盖）
  → 命中即跳过 MediaPlan 构建，原样透传，不做模式校验（现状语义）
generation_mode（显式）
  → media[] 的角色组合（半显式）
    → images 数量推导（隐式，现状逻辑）
      → input_reference / image（隐式，现状逻辑）
        → text2video
```

逃生通道的定位是「高级用户完全接管请求体」，刻意置于所有结构化字段之上：与显式字段并存时逃生通道生效并记 `SysLog`（与现有代码行为一致，见 §6）。

### 3.4 校验策略：显式严格，隐式收敛

这是本设计与现状最关键的区别：

| 来源                             | 参数不足 / 冲突时的行为                                   |
| -------------------------------- | --------------------------------------------------------- |
| **显式** `generation_mode`       | **返回 400**，说明缺哪个角色、该模式需要几个素材          |
| **显式** `media[]`               | **返回 400**（未知 type、角色数量越界、首尾帧与参考混用） |
| **隐式** 推导（`images` 数量等） | 沿用现状：收敛到合法值 + 系统日志，不报错                 |

理由：客户端显式表达意图后，静默降级会掩盖调用方 bug 并造成"钱花了、结果不对"。隐式路径保留收敛是为了向后兼容。

例外：逃生通道（§3.3 第 0 级）不经上述校验 —— 其语义即绕过抽象层，校验责任移交上游。

### 3.5 Mode 与 `info.Action` 的关系

`MediaPlan.Mode` 是提交期中间产物，**不统一回写 `info.Action`**。`info.Action` 保持各渠道现状语义：

| 渠道         | `info.Action` 的处理                                                                                                                                                                                                               |
| ------------ | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| vidu / kling | 由 `MediaPlan.Mode` 推导（与现状逻辑等价，仅推导来源统一化）                                                                                                                                                                       |
| hailuo v2    | 保持 `TaskActionVideoV2Generate` 不变 —— 它是后台轮询选择 v2 查询端点所依赖的协议版本标记（[hailuo/adaptor.go#L158](file:///home/xufan/trea_project/new-api/relay/channel/task/hailuo/adaptor.go#L158)）；Mode 只指导 content 组装 |
| 其余         | 现状不变                                                                                                                                                                                                                           |

若需要模式落库（供管理端过滤，并根治 D3 的失真），**不加数据库列**：在 task 的 `Properties` 结构（[model/task.go#L79](file:///home/xufan/trea_project/new-api/model/task.go#L79)，JSON 列存储）中新增字段即可，零迁移成本、三库兼容。

## 4. 内部模型：`MediaPlan`

### 4.1 数据结构

新增 `relay/common/video_media_plan.go`：

```go
// VideoGenerationMode 生成模式，取值为 constant.TaskAction* 字符串。
type VideoGenerationMode = string

// MediaPlan 是跨渠道统一的「模式 + 素材角色」标准化结果。
// 由各 adaptor 的 ValidateRequestAndSetAction 统一构造，
// 再由各渠道映射为自己的上游私有结构。
type MediaPlan struct {
	Mode            VideoGenerationMode
	Explicit        bool     // 是否来自客户端显式指定（决定校验严格度）
	FirstFrame      string
	LastFrame       string
	ReferenceImages []string
	ReferenceVideos []string
	ReferenceAudios []string

	// Limits 为该渠道声明的素材上限，解析时按此截断或（显式时）报错
	Limits MediaLimits
}

type MediaLimits struct {
	MaxFirstFrame     int
	MaxLastFrame      int
	MaxReferenceImage int
	MaxReferenceVideo int
	MaxReferenceAudio int
	MutualExclusive   bool // 首尾帧与参考素材是否互斥（阿里 MiniMax/万相为 true，可灵为 false）

	// 隐式路径下首尾帧与参考素材并存时的行为（仅 MutualExclusive 时生效）。
	// 显式路径不受此策略影响，一律 400。
	OnExclusiveConflict ExclusivePolicy
}

type ExclusivePolicy int

const (
	// DowngradeFramesToReference 首尾帧降级为参考图，保留素材 —— hailuo v2 现状
	DowngradeFramesToReference ExclusivePolicy = iota
	// DropFrames 丢弃首尾帧 —— ali minimax 现状
	DropFrames
)
```

### 4.2 解析流程

```
BuildMediaPlan(req TaskSubmitReq, caps ChannelCapabilities) (MediaPlan, error)
  1. 读 generation_mode → 命中则 Explicit=true
  2. 读 media[] → 按 type 归入对应角色桶（未知 type：Explicit 报错，隐式忽略）
  3. media[] 为空 → 回落 images / input_reference / image 数量推导
  4. 互斥处理：
     - MutualExclusive && 同时存在首尾帧与参考素材
         Explicit → 400；隐式 → 按渠道 OnExclusiveConflict 策略执行
         （hailuo v2 降级为参考图 / ali minimax 丢弃首尾帧，见 §4.1）+ SysLog
  5. 数量收敛：
     - Explicit → 超限即 400；隐式 → 按顺序截断保留靠前者
  6. 模式一致性校验：Mode 与素材组合矛盾时（如 mode=first_last_frame 但只有 1 张图）
     - Explicit → 400；隐式 → 按素材实际组合修正 Mode
```

### 4.3 复用现有实现

`buildV2Content` + `sanitizeV2Content`（[hailuo/adaptor_v2.go#L75-L193](file:///home/xufan/trea_project/new-api/relay/channel/task/hailuo/adaptor_v2.go#L75-L193)）已经是全仓最完整的实现（具名键 + 数量兜底 + 互斥降级 + 上限截断），本设计将其**上移为公共实现**，然后：

- `hailuo/adaptor_v2.go` 改为消费 `MediaPlan`（行为不变，删除私有实现）
- `ali/adaptor.go` 的 `buildMiniMaxMedia` 改为消费 `MediaPlan`（删除现存的重复实现）
- `metadataStringSlice`（[ali/adaptor.go#L361](file:///home/xufan/trea_project/new-api/relay/channel/task/ali/adaptor.go#L361)）与 `metadataStrings`（[hailuo/adaptor_v2.go#L551](file:///home/xufan/trea_project/new-api/relay/channel/task/hailuo/adaptor_v2.go#L551)）合并为 `taskcommon` 公共函数

### 4.4 能力声明接口

```go
// 可选接口，适配器按需实现；未实现视为「能力未知」，跳过校验保持现状行为
type ModeCapability interface {
	SupportedModes() []VideoGenerationMode
	MediaLimits() MediaLimits
}
```

`relay/relay_adaptor.go` 的 `GetTaskAdaptor` 返回后做一次类型断言即可，不破坏 `TaskAdaptor` 现有契约。

能力可进一步经渠道元数据接口暴露给前端，使视频调试台能按所选渠道动态渲染模式选项（当前前端注释明确写着「后端按 1 张/多张自动映射」，见 [video-payload.ts#L26](file:///home/xufan/trea_project/new-api/web/default/src/features/image-debug/lib/video-payload.ts#L26)）。

## 5. 渠道能力矩阵与映射

「待核实」= 上游 API 能力需查官方文档确认，本文档不做推测。
「互斥」列指**上游约束是否存在**；本地是否强制由各渠道决定：hailuo v2 与 ali minimax 有本地强制逻辑，hailuo v1 仅透传 metadata、依赖上游校验。

| 渠道                | 文生 | 首帧   | 尾帧   | 首尾帧     | 参考图 | 参考视频 | 参考音频 | 模式载体                                                       | 互斥   |
| ------------------- | ---- | ------ | ------ | ---------- | ------ | -------- | -------- | -------------------------------------------------------------- | ------ |
| kling (50)          | ✅   | ✅     | ✅     | ✅         | ❌     | ❌       | ❌       | `text2video`/`image2video` 端点 + `image`/`image_tail`         | 否     |
| vidu (52)           | ✅   | ✅     | 待核实 | ✅         | ✅     | 待核实   | 待核实   | action → 四个端点                                              | 待核实 |
| jimeng (51)         | ✅   | ✅     | ❌     | ✅         | ❌     | ❌       | ❌       | `req_key` 切换                                                 | 是     |
| doubao (54)         | ✅   | 待核实 | 待核实 | 待核实     | 待核实 | ✅       | ✅       | `content[].role`（后端当前不赋值）                             | 待核实 |
| hailuo v1 (35)      | ✅   | ✅     | ✅     | ✅         | ✅     | ❌       | ❌       | metadata 具名字段                                              | 是     |
| hailuo v2 (35)      | ✅   | ✅     | ✅     | ✅         | ✅     | ✅       | ✅       | `content[]` + `role`                                           | 是     |
| ali minimax (17)    | ✅   | ✅     | ✅     | ✅         | ✅     | ✅       | ✅       | `input.media[]`                                                | 是     |
| ali wan3 (17)       | ✅   | ✅     | 待核实 | **待核实** | ✅     | ✅       | ✅       | `input.media[]`                                                | 是     |
| ali wan2.x (17)     | ✅   | ✅     | ❌     | ❌         | ❌     | ❌       | ❌       | `input.img_url`                                                | —      |
| ali happyhorse (17) | ✅   | ✅     | 待核实 | 待核实     | ✅     | 待核实   | 待核实   | `input.media[]`                                                | 是     |
| gemini / vertex     | ✅   | ✅     | ❌     | ❌         | ❌     | ❌       | ❌       | `instance.image` 单图（`lastFrame`/`referenceImages` 为 TODO） | —      |
| sora (55/1)         | ✅   | ✅     | ❌     | ❌         | ❌     | ❌       | ❌       | 原样透传                                                       | —      |

### 5.1 `MediaPlan` → 上游映射示例

**ali minimax**（`input.media[]`，type 枚举与万相不同名）：

```
FirstFrame      → {type:"first_frame"}
LastFrame       → {type:"last_frame"}
ReferenceImages → {type:"image_url"}
ReferenceVideos → {type:"feature"}
ReferenceAudios → {type:"driving_audio"}
```

**hailuo v2**（`content[]` + `role`）：`FirstFrame` → `{type:"image_url", role:"first_frame"}`，以此类推。

**kling**（扁平双字段）：`FirstFrame` → `image`，`LastFrame` → `image_tail`，`ReferenceImages` → **不支持，显式指定时报 400**。

**vidu**：`Mode` 直接决定端点，素材按端点取用。

## 6. 兼容性：为什么零回归

| 现有请求形态                           | 新逻辑下的行为                                                                                                                                                                            |
| -------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 只传 `prompt`                          | `generation_mode` 缺省 → 无素材 → `text2video`，与现状一致                                                                                                                                |
| 传 `input_reference`                   | 无 `media[]` → 回落 images 推导 → 首帧，与现状一致                                                                                                                                        |
| 传 `images`（1/2/N 张）                | 无显式字段 → 走各渠道**原有**数量推导，未改动                                                                                                                                             |
| 传 `metadata.action`（vidu）           | 保留读取，优先级降为「与 `generation_mode` 冲突时以后者为准」                                                                                                                             |
| 传 `metadata.input.media`（ali）       | metadata 覆盖发生在 `convertToAliRequest` 末尾（[ali/adaptor.go#L414-L424](file:///home/xufan/trea_project/new-api/relay/channel/task/ali/adaptor.go#L414-L424)），仍为最高优先级逃生通道 |
| 传 `metadata.content`（hailuo/doubao） | 同上，原样透传通道保留                                                                                                                                                                    |

**关键约束**：`MediaPlan` 的隐式推导分支必须逐渠道复刻现有语义，不得"顺手统一"。ali wan3 的「2 张图 → 参考图」与互斥冲突的隐式处理分叉（hailuo v2 降级为参考图、ali minimax 丢弃首尾帧，见 §4.1 `ExclusivePolicy`）均属此类 —— 见 §9。

## 7. 现存缺陷清单

这些缺陷独立于本设计，是"无法指定模式"的实际伤害点，建议**先于**契约改造修复。
**实施状态：D1、D2、D3、D5、D6、D7 已修复**（随阶段 0 / 阶段 1 落地，均有回归测试）；D4 待单独评估。

| #   | 渠道          | 缺陷                                                                                                                                                                                                                  | 位置                                                                                                                                                                                                                        |
| --- | ------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| D1  | kling         | JSON 只传 `images` 数组（不传 `image`）→ 判成文生视频**且图片被丢弃**。`convertToRequestPayload` 读 `req.Image`，而 `ValidateBasicTaskRequest` 的兜底方向是反的（`Image`→`Images[0]`，不会 `Images[0]`→`Image`）      | [kling/adaptor.go#L269](file:///home/xufan/trea_project/new-api/relay/channel/task/kling/adaptor.go#L269) vs [relay_utils.go#L217-L220](file:///home/xufan/trea_project/new-api/relay/common/relay_utils.go#L217-L220)      |
| D2  | vidu          | `metadata.action` 无枚举白名单；非字符串值经 `meatAction.(string)` 得 `""` → 静默降级为文生视频                                                                                                                       | [vidu/adaptor.go#L95-L96](file:///home/xufan/trea_project/new-api/relay/channel/task/vidu/adaptor.go#L95-L96)                                                                                                               |
| D3  | jimeng        | action 与真实模式解耦：JSON 传 3 张图时 `task.Action` 仍为 `generate`，但 `req_key` 已转 `i2v_first_tail` → 落库、日志、管理端过滤全部失真；multipart 路径（#L134-L141）反而会按文件数设置 action，两条路径行为不一致 | [jimeng/adaptor.go#L100-L102](file:///home/xufan/trea_project/new-api/relay/channel/task/jimeng/adaptor.go#L100-L102)、[#L408-L423](file:///home/xufan/trea_project/new-api/relay/channel/task/jimeng/adaptor.go#L408-L423) |
| D4  | sora          | `-F input_reference="@file.jpg"`（官方推荐用法）不会填充 `req.InputReference`，因 `parseMultipartFormData` 只读 `form.Value` 不读 `form.File` → action 恒为 `textGenerate`。请求本身仍正确发出，仅落库与日志不准      | [common/gin.go#L363-L370](file:///home/xufan/trea_project/new-api/common/gin.go#L363-L370)                                                                                                                                  |
| D5  | gemini/vertex | 首尾帧与参考图未实现（`VeoInstance` 内两条 TODO）；multipart 图片 >20MB 时静默返回 nil → 退化为文生视频且无任何提示                                                                                                   | [gemini/dto.go#L11-L16](file:///home/xufan/trea_project/new-api/relay/channel/task/gemini/dto.go#L11-L16)、[gemini/image.go#L28-L30](file:///home/xufan/trea_project/new-api/relay/channel/task/gemini/image.go#L28-L30)    |
| D6  | doubao        | `ContentItem.Role` 字段存在但后端从不赋值，仅在客户端整体覆盖 `metadata.content` 时可用作透传                                                                                                                         | [doubao/adaptor.go#L36](file:///home/xufan/trea_project/new-api/relay/channel/task/doubao/adaptor.go#L36)                                                                                                                   |
| D7  | 公共          | `validateMultipartTaskRequest` 的 `action` 与 `info` 两个形参在函数体内均未使用                                                                                                                                       | [relay_utils.go#L81](file:///home/xufan/trea_project/new-api/relay/common/relay_utils.go#L81)                                                                                                                               |

其中 D4 涉及 `common/gin.go` 公共路径，改动影响面需单独评估（不只影响视频）。

### 修复方向（建议，均不动对外接口）

- **D1**：kling 适配器内部兜底 —— `Image` 为空时取 `Images[0]`；不修改 `ValidateBasicTaskRequest` 公共路径
- **D2**：`metadata.action` 增加四个合法动作的白名单校验；非法值 / 非字符串 → 400
- **D3**：jimeng 在 `req_key` 转换处按 `imageLen` 回填 `info.Action`（0 张→`textGenerate`、1 张→`generate`、≥2 张→`firstTailGenerate`）；kling 在 `DoRequest` 阶段回填 action 已有先例（[kling/adaptor.go#L183-L188](file:///home/xufan/trea_project/new-api/relay/channel/task/kling/adaptor.go#L183-L188)），落库发生在其后，安全
- **D5**：>20MB 改为返回明确错误而非静默 `nil`；`lastFrame` / `referenceImages` 的补齐归入阶段 3
- **D6**：由阶段 1 接入 `MediaPlan` 时一并解决（为 `ContentItem.Role` 赋值）
- **D7**：删除未使用的 `action` 与 `info` 形参

## 8. 分阶段实施计划

### 阶段 -1：回归测试锚定（后续所有阶段的前置）

零回归承诺需要测试锚点：每渠道 × 每输入形态（0/1/2/3 张图、`input_reference`、metadata 组合）编写 table-driven 黄金测试，断言生成的上游 payload 与 `info.Action` 与现状行为一致。[hailuo/adaptor_v2_test.go](file:///home/xufan/trea_project/new-api/relay/channel/task/hailuo/adaptor_v2_test.go) 可作模板。没有这层锚定，D1-D7 的修复正确性与 `BuildMediaPlan` 渠道覆写钩子（见 §9）都无法验证。

### 阶段 0：缺陷修复（独立可交付）✅ 已实施

D1、D2、D3、D5、D6、D7。不动接口，纯行为修正（方向见 §7 末尾）。D4 单独评估。

### 阶段 1：统一解析层（推荐起点）✅ 已实施

- 新增 `relay/common/video_media_plan.go`：`MediaPlan` + `BuildMediaPlan` + `MediaLimits`
- 上移 hailuo v2 的 `buildV2Content` / `sanitizeV2Content` 为公共实现
- 统一 metadata 具名键：`first_frame_image` / `last_frame_image` / `reference_images` / `reference_videos` / `reference_audios`
- 接入 ali、doubao、kling、vidu
- **不动** `TaskSubmitReq` 结构、不动 OpenAPI、不动前端
- 客户端立刻获得显式指定能力（经 metadata），零回归

实施备注：

- `metadataStringSlice`（ali）与 `metadataStrings`（hailuo）已合并为 `relaycommon.MetadataStrings`（置于 `relay/common` 而非 `taskcommon`，因 `relay/common` 为任务适配器的公共依赖层，反向引用会产生循环依赖）
- hailuo v2 的 `buildV2Request` 双调用点（`EstimateBilling` / `BuildRequestBody`）：`BuildMediaPlan` 是 req 的纯函数，两次调用结果必然一致，无需缓存
- kling 隐式路径仅用首张图（尾帧须经 `last_frame_image` 显式指定）；doubao 隐式路径保持无 role 项（上游 role 取值域待核实，§10），显式路径经 `MediaPlan` 赋 role（D6）
- wan3 经 `ImplicitImages` 钩子保留「单图首帧 / 多图参考图」语义（§9 决策），测试锚点见 `ali/adaptor_test.go` 的 `TestConvertWan3Request_TwoImagesAreReferences`

### 阶段 2：顶层契约字段

- `TaskSubmitReq` 新增 `GenerationMode` + `Media []MediaItem`
- `ModeCapability` 接口 + 各渠道能力声明
- 显式路径严格校验（400）
- multipart 表单支持：`generation_mode` 字段 + `media` JSON 串（注意 `isKnownTaskField` 白名单需同步，否则新字段会被自动降级进 metadata —— 见 [relay_utils.go#L107-L117](file:///home/xufan/trea_project/new-api/relay/common/relay_utils.go#L107-L117) 与 [L184-L196](file:///home/xufan/trea_project/new-api/relay/common/relay_utils.go#L184-L196)）
- `controller/swag_video.go` OpenAPI 注解同步

### 阶段 3：前端与能力探测

- 渠道能力接口暴露 `supported_modes`
- 视频调试台按渠道动态渲染模式选择器（替换现有"自动映射"提示）
- gemini/vertex 补齐 `lastFrame` / `referenceImages`（含 HTTP URL 图片下载转 base64）

**部分实施**：视频调试台已上线**静态**模式选择器（自动 / 首帧 / 首尾帧 / 参考生视频，
见 [video-debug-form.tsx](file:///home/xufan/trea_project/new-api/web/default/src/features/image-debug/components/video-debug-form.tsx)
与 [video-payload.ts](file:///home/xufan/trea_project/new-api/web/default/src/features/image-debug/lib/video-payload.ts)），
显式模式经统一具名键下发，`auto` 保持现状。尚未按渠道能力动态过滤选项 ——
对不支持某模式的渠道（如万相3.0 尾帧），显式指定会按「能力未知 → 宽松」策略静默降级，
动态渲染需等待渠道能力接口落地。

## 9. 已确认决策

| 决策                                   | 内容                                                                                                                                                                   |
| -------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 实施节奏                               | 先出设计文档评审，暂不动代码                                                                                                                                           |
| **ali wan3 的「2 张图 → 参考图」语义** | **保留现状不改**。该歧义继续存在，换取零回归风险；由阶段 2 的显式 `generation_mode` 提供规避手段                                                                       |
| 内部枚举                               | 复用 `constant.TaskAction*`，不新造（`TaskActionVideoV2Generate` 为协议标记，不属模式枚举，见 §3.5）                                                                   |
| 校验策略                               | 显式严格（400）、隐式收敛（现状）                                                                                                                                      |
| `Mode` 字段                            | 不复用（已被可灵清晰度档位占用）                                                                                                                                       |
| Mode 与 `info.Action`                  | `MediaPlan.Mode` 不统一回写 `info.Action`；hailuo v2 保持 `TaskActionVideoV2Generate` 协议标记（见 §3.5）                                                              |
| 模式落库载体                           | 不加数据库列；在 task `Properties` 结构（JSON 列）新增字段                                                                                                             |
| `generation_mode` 与 `media[]` 并存    | 两者都提供：`generation_mode` 是无素材时表达 `text2video` 的唯一手段，也是前端选择器的渲染依据；`media[]` 是素材事实来源。两者同时给出但互相矛盾 → 400（显式冲突不猜） |
| 能力未知的渠道                         | 跳过校验、保持现状行为（宽松），但记 `SysLog` 使其可观测                                                                                                               |
| 错误响应格式                           | 复用 `dto.TaskError`（`createTaskError`），与现有任务错误一致                                                                                                          |
| 互斥冲突的隐式行为                     | 渠道可声明 `ExclusivePolicy`：hailuo v2 降级为参考图、ali minimax 丢弃首尾帧（见 §4.1）                                                                                |
| 逃生通道优先级                         | 最高（§3.3 第 0 级）：命中即跳过 `MediaPlan` 构建与模式校验                                                                                                            |

> 由于 wan3 的隐式语义保留，**阶段 1 的 `BuildMediaPlan` 必须允许渠道覆写隐式推导规则**（例如通过 `MediaLimits` 之外的 hook，或允许 wan3 分支在 `MediaPlan` 之后重映射），否则统一解析层会意外改变 wan3 的现有行为。这是实施阶段 1 时的首要技术约束。
>
> 另一实施约束：hailuo v2 的 `buildV2Request` 会被调用两次 —— [EstimateBilling](file:///home/xufan/trea_project/new-api/relay/channel/task/hailuo/adaptor_v2.go#L314-L324)（计费需经 content 推导分辨率倍率）与 `BuildRequestBody` 各一次。阶段 1 改为消费 `MediaPlan` 后必须保证两处构建一致：`MediaPlan` 应构建一次并缓存于 gin context，否则计费与实际请求可能不一致。

## 10. 待评审的开放问题

原问题「`generation_mode` 与 `media[]` 取舍」「能力未知渠道的处理」「错误响应格式」已形成初步结论，见 §9「已确认决策」。剩余问题：

1. **`metadata.action` 的去留。** 阶段 2 后它与 `generation_mode` 语义重叠，建议保留读取但标记 deprecated；两者冲突时以 `generation_mode` 为准（见 §6）。
2. **D4（sora multipart 文件上传）是否修。** 涉及 `common/gin.go` 公共解析路径，影响面超出视频。
3. **上游能力核实。** §5 矩阵中 12 处「待核实」需对照各家官方文档确认后才能定稿 `MediaLimits`，尤其是可灵是否支持多主体参考（`image_list`，当前 `requestPayload` 完全无此字段）、豆包 `content[].role` 的合法取值域。
