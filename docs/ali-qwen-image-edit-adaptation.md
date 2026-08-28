# 阿里百炼千问图像（qwen-image）多图编辑适配说明

相关代码：

- `relay/channel/ali/adaptor.go` — 端点分流、请求头、`response_format` 上下文写入
- `relay/channel/ali/image.go` — OpenAI 请求体 → DashScope 请求体转换
- `relay/channel/ali/dto.go` — `AliImageRequest` / `AliImageParameters`
- `relay/helper/valid_request.go` — `/v1/images/edits` 入参解析
- `setting/model_setting/qwen.go` — 同步/异步图像模型白名单

上游接口参考：<https://help.aliyun.com/zh/model-studio/qwen-image-generation-and-editing-api-reference>

## 1. 背景

`qwen-image-3.0` / `qwen-image-3.0-pro` 同时支持文生图（T2I）与图生图/图像编辑（I2I）。
I2I 场景要求 DashScope 请求体的 `input.messages[0].content` 中包含 **1–3 个 `{"image": ...}` 对象**
和 **1 个 `{"text": ...}` 对象**：

```json
{
  "model": "qwen-image-3.0-pro",
  "input": {
    "messages": [
      {
        "role": "user",
        "content": [
          { "image": "data:image/png;base64,iVBORw0KGgo..." },
          { "image": "data:image/png;base64,/9j/4AAQSk..." },
          { "text": "把源图放到参考图中的场景里" }
        ]
      }
    ]
  },
  "parameters": { "n": 1, "watermark": false }
}
```

而调用方通常以 OpenAI 兼容格式请求 `/v1/images/edits`：

```json
{
  "model": "qwen-image-3.0",
  "prompt": "编辑指令",
  "image": ["data:image/png;base64,...", "data:image/png;base64,..."],
  "response_format": "b64_json"
}
```

## 2. 历史缺陷

| 缺陷 | 表现 |
| --- | --- |
| JSON 请求体下的 `image` / `images` 字段从未被读取 | 参考图整体丢失，`content` 退化为仅含 `text`，I2I 变成 T2I，生成结果与源图无关 |
| `response_format` 被当作顶层字段透传给 DashScope | DashScope 请求体只接受 `model` / `input` / `parameters`，多余字段有被拒风险 |
| `aliImageHandler` 通过 `c.GetString("response_format")` 读取响应格式，但 ali 渠道从未写入该上下文键 | 恒为空串，`b64_json` 请求拿不到 base64 数据，只有 `url` |
| multipart 表单分支未解析 `response_format` | 表单方式请求 `b64_json` 同样不生效 |
| `oaiFormEdit2AliImageEdit` 丢弃表单里已解析的 `size` | 前端调试页设置的输出分辨率不生效 |

## 3. 当前映射规则

### 3.1 端点与请求头

`RelayModeImagesEdits` 下按模型名分流（`GetRequestURL`）：

| 模型 | 端点 | `X-DashScope-Async` |
| --- | --- | --- |
| 老万相（含 `wan`，排除 `wan2.6` / `wan2.7`） | `image2image/image-synthesis` | `enable` |
| `wan2.6` / `wan2.7` | `image-generation/generation` | `enable` |
| 其余（含全部 `qwen-image*`） | `multimodal-generation/generation` | 不设置（同步） |

同步/异步由 `model_setting.IsSyncImageModel` 判定，白名单在运营设置 `qwen.sync_image_models` 中可配，
默认包含 `qwen-image`，因此 `qwen-image-3.0` / `qwen-image-3.0-pro` 均命中同步分支。

### 3.2 输入图像来源

`ConvertImageRequest` 按 `Content-Type` 分两条路径，两条路径最终都通过
`buildAliSyncContent` 生成 `content` 数组（图像在前、文本在后，保持数组顺序）：

- `multipart/form-data` → `getImageBase64sFromForm`：读取 `image` / `image[]` / `image[*]`
  全部文件，逐个 `http.DetectContentType` + base64 编码为 data URL。
- `application/json` → `getImageInputsFromJSON`：读取 `image` 字段，`image` 缺失时回落到 `images`
  字段，值为字符串或字符串数组。

`normalizeImageInputs` 对每个输入做归一化：

| 输入形态 | 处理方式 |
| --- | --- |
| `http://` / `https://` 开头 | 原样传递（DashScope 支持公网 URL） |
| `data:` 开头 | 原样传递（DashScope 支持 `data:{MIME};base64,{data}`） |
| 裸 base64 字符串 | 通过 `service.DecodeBase64FileData` 探测 MIME 后补全为 data URL |
| 空字符串 / 纯空白 | 丢弃 |

### 3.3 数量限制

`isQwenImageModel`（模型名包含 `qwen-image`，大小写不敏感）时，输入图像超过
`qwenImageMaxInputImages = 3` 张直接返回错误，避免把必然被上游拒绝的请求发出去。
万相系列不做该限制，由上游校验。

### 3.4 显式 `input` 优先

若请求体携带阿里原生 `input` 字段，`dto.ImageRequest` 会把它收进 `Extra`，
转换时直接反序列化使用，**不会**再被 `prompt` / `image` 的转换结果覆盖。
`parameters` 同理，用于透传 `negative_prompt`、`bbox_list` 等高级参数。

### 3.5 `response_format`

`response_format` 不再出现在发往 DashScope 的请求体中（`AliImageRequest` 已移除该字段）。
`ConvertImageRequest` 会执行 `c.Set("response_format", request.ResponseFormat)`，
`aliImageHandler` 在响应阶段读取该值：

- `b64_json`：`ChoicesToOpenAIImageDate` 通过 `service.GetImageFromUrl` 下载 DashScope 返回的图片
  URL（有效期 24 小时）并转换为 base64 填入 `data[].b64_json`。
- 其他值：仅返回 `data[].url`。

### 3.6 参数映射

| OpenAI 字段 | DashScope 字段 | 说明 |
| --- | --- | --- |
| `prompt` | `input.messages[0].content[].text` | 同步模型；异步模型映射到 `input.prompt` |
| `image` / `images` | `input.messages[0].content[].image` | 仅同步模型 |
| `size` | `parameters.size` | `1024x1024` → `1024*1024` |
| `n` | `parameters.n` | 同时写入 `PriceData.OtherRatios["n"]` 参与计费 |
| `watermark` | `parameters.watermark` | |
| `response_format` | — | 本地处理，见 3.5 |

`AliImageParameters` 覆盖 qwen-image-3.0 的 `prompt_extend`、`prompt_extend_mode`、
`enable_thinking`、`n`、`size`、`negative_prompt`（经 `input`/`parameters` 透传）、`seed`、`watermark`，
以及 qwen-image-edit 系列的 `bbox_list`、`color_palette`、`enable_sequential`、`thinking_mode`。
未在本结构体声明的键会在 `common.Unmarshal` 时被丢弃，新增上游参数时需同步补充字段。

## 4. 计费注意

qwen-image-3.0 的用量字段为 `usage.output_image_count` / `input_image_count` /
`output_image_type`（按像素面积区分 `qima_*_1k` 与 `qima_*_2k` 档位），
而 `AliUsage` 目前只解析 `image_count`。同步模型下 `image_count` 恒为 0，
计费回落到 `len(response.Data)`，即按实际返回图片数量计 `n` 倍率，
**分辨率档位与输入图数量不参与计费**。如需按档位差异化定价，需要扩展 `AliUsage`
并在 `aliImageHandler` 中补充倍率逻辑。

## 5. 测试

`relay/channel/ali/image_test.go` 覆盖：

- JSON 多图 / 单图字符串 / `images` 字段 → `content` 顺序与内容
- 裸 base64 归一化为 data URL
- 文生图仅含 `text`
- 显式 `input` 不被覆盖
- qwen-image 超过 3 图报错、恰好 3 图通过
- 请求体序列化后不含 `response_format`
- `prompt_extend_mode` / `enable_thinking` 透传
- 异步模型仍使用 `input.prompt` 且 `size` 完成 `x` → `*` 转换
- multipart 多文件 → 多个 `image` 内容项 + `size` 生效
