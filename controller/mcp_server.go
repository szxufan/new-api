package controller

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay"
	"github.com/QuantumNous/new-api/relay/channel/task/taskcommon"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/mcp_setting"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// MCPGinContextKey 用于在 HTTP request context 中传递 gin context
// 以便 MCP tool handler 能通过 context 获取认证信息（usingGroup 等）
type MCPGinContextKey struct{}

var mcpGinContextKey = MCPGinContextKey{}

// maxMCPReferenceImages MCP 图生图/参考图最多引用的临时图片数量
const maxMCPReferenceImages = 3

// mcpServer 是单例 MCP Server
var mcpServer *mcp.Server

func init() {
	mcpServer = mcp.NewServer(&mcp.Implementation{
		Name:    "new-api-mcp",
		Version: "1.0.0",
	}, nil)

	// 注册 generate_image 工具
	mcpServer.AddTool(&mcp.Tool{
		Name:        "generate_image",
		Description: "Generate an image from a text prompt, or edit/combine input images when image_ids is provided. Uses the image model configured for the current group",
		InputSchema: generateImageInputSchema(),
	}, handleMCPGenerateImage)

	// 注册文生视频工具
	mcpServer.AddTool(&mcp.Tool{
		Name:        "generate_video",
		Description: "Generate a video from a text prompt (async). Returns task_id immediately; poll get_video_task to check progress and get the result URL",
		InputSchema: generateVideoInputSchema(),
	}, handleMCPGenerateVideo)

	// 注册首帧/首尾帧生视频工具
	mcpServer.AddTool(&mcp.Tool{
		Name:        "generate_video_from_frames",
		Description: "Generate a video from a first frame image (and optionally a last frame image) uploaded via POST /v1/mcp-upload (async). Returns task_id immediately; poll get_video_task",
		InputSchema: generateVideoFromFramesInputSchema(),
	}, handleMCPGenerateVideoFromFrames)

	// 注册参考图生视频工具
	mcpServer.AddTool(&mcp.Tool{
		Name:        "generate_video_from_reference",
		Description: "Generate a video guided by 1-3 reference images uploaded via POST /v1/mcp-upload (async). Returns task_id immediately; poll get_video_task",
		InputSchema: generateVideoFromReferenceInputSchema(),
	}, handleMCPGenerateVideoFromReference)

	// 注册视频任务查询工具
	mcpServer.AddTool(&mcp.Tool{
		Name:        "get_video_task",
		Description: "Get the status, progress and result URL of a video generation task submitted by generate_video / generate_video_from_frames / generate_video_from_reference",
		InputSchema: getVideoTaskInputSchema(),
	}, handleMCPGetVideoTask)
}

// generateImageInputSchema 返回 generate_image 工具的 JSON Schema
func generateImageInputSchema() json.RawMessage {
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"prompt": map[string]any{
				"type":        "string",
				"description": "The text description of the image to generate, or the edit instruction when image_ids is provided",
			},
			"image_ids": map[string]any{
				"type":        "array",
				"items":       map[string]any{"type": "string"},
				"minItems":    1,
				"maxItems":    3,
				"description": "Optional. Temporary image IDs returned by POST /v1/mcp-upload (max 3). When provided, image-to-image (edit) is performed using these reference images",
			},
		},
		"required": []string{"prompt"},
	}
	data, _ := common.Marshal(schema)
	return json.RawMessage(data)
}

// handleMCPGenerateImage 是 generate_image 工具的 handler。
// 不使用 relay.ImageHelper（它会将响应写入 HTTP ResponseWriter），
// 而是直接调用适配器底层方法，将图片 URL 作为 MCP Tool Result 返回。
// 支持两种模式：
//   - 文生图：仅传 prompt，使用分组配置的文生图模型
//   - 图生图：传 prompt + image_ids（POST /v1/mcp-upload 返回的临时图片 ID，最多 3 张），
//     使用分组配置的图生图模型，走 /v1/images/edits 路径
func handleMCPGenerateImage(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// 从 context 中获取 gin context（在路由层注入）
	ginCtx, ok := ctx.Value(mcpGinContextKey).(*gin.Context)
	if !ok || ginCtx == nil {
		logger.LogError(ctx, "MCP: failed to get gin context from MCP tool handler context")
		return newMCPErrorResult("internal error: failed to get request context"), nil
	}

	// 解析工具参数
	var args struct {
		Prompt   string   `json:"prompt"`
		ImageIDs []string `json:"image_ids"`
	}
	if err := common.Unmarshal(req.Params.Arguments, &args); err != nil {
		return newMCPErrorResult(fmt.Sprintf("invalid arguments: %s", err.Error())), nil
	}
	if args.Prompt == "" {
		return newMCPErrorResult("prompt is required"), nil
	}
	if len(args.ImageIDs) > maxMCPReferenceImages {
		return newMCPErrorResult(fmt.Sprintf("image_ids supports at most %d images, got %d", maxMCPReferenceImages, len(args.ImageIDs))), nil
	}

	// 从配置获取该分组的文生图/图生图模型
	group := common.GetContextKeyString(ginCtx, constant.ContextKeyUsingGroup)
	if group == "" {
		group = "default"
	}
	isI2I := len(args.ImageIDs) > 0
	var model string
	if isI2I {
		model = mcp_setting.GetGroupI2IModel(group)
		if model == "" {
			return newMCPErrorResult(fmt.Sprintf("no image-to-image model configured for group: %s, please configure mcp_setting.group_i2i_models", group)), nil
		}
	} else {
		model = mcp_setting.GetGroupImageModel(group)
		if model == "" {
			return newMCPErrorResult(fmt.Sprintf("no image model configured for group: %s", group)), nil
		}
	}

	// 解析临时图片 ID 为本站代理 URL（仅图生图）
	var imageInputs []string
	if isI2I {
		var resolveErr error
		imageInputs, resolveErr = resolveMCPImageIDs(ginCtx, args.ImageIDs)
		if resolveErr != nil {
			return newMCPErrorResult(resolveErr.Error()), nil
		}
	}

	// 构造 ImageRequest
	imageReq := &dto.ImageRequest{
		Model:  model,
		Prompt: args.Prompt,
	}
	if isI2I {
		// OpenAI edits JSON 格式的 image 字段（字符串数组），上游适配器（如 ali qwen-image）会解析
		imagesJSON, marshalErr := common.Marshal(imageInputs)
		if marshalErr != nil {
			return newMCPErrorResult(fmt.Sprintf("failed to marshal image inputs: %s", marshalErr.Error())), nil
		}
		imageReq.Image = imagesJSON
	}

	// 设置 original_model，使 GenRelayInfo 能正确读取 OriginModelName
	// MCP 路由未走 Distribute 中间件，需要手动设置此键
	common.SetContextKey(ginCtx, constant.ContextKeyOriginalModel, model)

	// 构造 RelayInfo（从 gin context 读取用户/Token 信息）
	relayInfo, err := relaycommon.GenRelayInfo(ginCtx, types.RelayFormatOpenAIImage, imageReq, nil)
	if err != nil {
		logger.LogError(ginCtx, fmt.Sprintf("MCP: GenRelayInfo failed: %s", err.Error()))
		return newMCPErrorResult(fmt.Sprintf("failed to build relay info: %s", err.Error())), nil
	}

	// 覆盖 RequestURLPath 为正确的图片生成/编辑路径
	// GenRelayInfo 从原始请求 URL 读取路径（/v1/mcp），
	// 上游需要的是 /v1/images/generations（文生图）或 /v1/images/edits（图生图）
	if isI2I {
		relayInfo.RequestURLPath = "/v1/images/edits"
		relayInfo.RelayMode = relayconstant.RelayModeImagesEdits
	} else {
		relayInfo.RequestURLPath = "/v1/images/generations"
	}

	// 价格计算与预扣费
	meta := imageReq.GetTokenCountMeta()
	priceData, err := helper.ModelPriceHelper(ginCtx, relayInfo, 0, meta)
	if err != nil {
		logger.LogError(ginCtx, fmt.Sprintf("MCP: ModelPriceHelper failed: %s", err.Error()))
		return newMCPErrorResult(fmt.Sprintf("pricing error: %s", err.Error())), nil
	}

	if !priceData.FreeModel {
		if newAPIErr := service.PreConsumeBilling(ginCtx, priceData.QuotaToPreConsume, relayInfo); newAPIErr != nil {
			logger.LogError(ginCtx, fmt.Sprintf("MCP: PreConsumeBilling failed: %s", newAPIErr.Error()))
			return newMCPErrorResult(fmt.Sprintf("insufficient quota: %s", newAPIErr.Error())), nil
		}
	}

	// 失败退款
	var newAPIError *types.NewAPIError
	defer func() {
		if newAPIError != nil {
			if relayInfo.Billing != nil {
				relayInfo.Billing.Refund(ginCtx)
			}
		}
	}()

	// 选择渠道
	preferredTypes := types.GetPreferredChannelTypesByRelayFormat(types.RelayFormatOpenAIImage)
	channel, _, err := service.CacheGetRandomSatisfiedChannel(&service.RetryParam{
		Ctx:                   ginCtx,
		TokenGroup:            relayInfo.TokenGroup,
		ModelName:             relayInfo.OriginModelName,
		Retry:                 common.GetPointer(0),
		PreferredChannelTypes: preferredTypes,
	})
	if err != nil {
		newAPIError = types.NewError(fmt.Errorf("no available channel: %w", err), types.ErrorCodeGetChannelFailed)
		return newMCPErrorResult(fmt.Sprintf("no available channel: %s", err.Error())), nil
	}

	// 注入渠道元数据到 gin context
	if newAPIErr := middleware.SetupContextForSelectedChannel(ginCtx, channel, relayInfo.OriginModelName); newAPIErr != nil {
		newAPIError = newAPIErr
		return newMCPErrorResult(fmt.Sprintf("channel setup failed: %s", newAPIErr.Error())), nil
	}

	// 初始化渠道元数据（从 context 读取 ChannelType、ApiType 等）
	relayInfo.InitChannelMeta(ginCtx)

	// 获取适配器
	adaptor := relay.GetAdaptor(relayInfo.ApiType)
	if adaptor == nil {
		newAPIError = types.NewError(fmt.Errorf("unsupported channel type: %d", relayInfo.ChannelType), types.ErrorCodeInvalidApiType)
		return newMCPErrorResult(fmt.Sprintf("unsupported channel type: %d", relayInfo.ChannelType)), nil
	}
	adaptor.Init(relayInfo)

	// 模型映射
	if err := helper.ModelMappedHelper(ginCtx, relayInfo, imageReq); err != nil {
		newAPIError = types.NewError(err, types.ErrorCodeChannelModelMappedError)
		return newMCPErrorResult(fmt.Sprintf("model mapping failed: %s", err.Error())), nil
	}

	// 转换请求为上游格式
	convertedRequest, err := adaptor.ConvertImageRequest(ginCtx, relayInfo, *imageReq)
	if err != nil {
		newAPIError = types.NewError(err, types.ErrorCodeConvertRequestFailed)
		return newMCPErrorResult(fmt.Sprintf("request conversion failed: %s", err.Error())), nil
	}

	// 序列化请求体
	jsonData, err := common.Marshal(convertedRequest)
	if err != nil {
		newAPIError = types.NewError(err, types.ErrorCodeConvertRequestFailed)
		return newMCPErrorResult(fmt.Sprintf("request serialization failed: %s", err.Error())), nil
	}

	logger.LogDebug(ginCtx, "MCP image request body: %s", jsonData)

	// 发送请求到上游
	resp, err := adaptor.DoRequest(ginCtx, relayInfo, bytes.NewReader(jsonData))
	if err != nil {
		newAPIError = types.NewError(err, types.ErrorCodeDoRequestFailed)
		return newMCPErrorResult(fmt.Sprintf("upstream request failed: %s", err.Error())), nil
	}

	httpResp, ok := resp.(*http.Response)
	if !ok || httpResp == nil {
		newAPIError = types.NewError(fmt.Errorf("unexpected response type"), types.ErrorCodeDoRequestFailed)
		return newMCPErrorResult("unexpected response from upstream"), nil
	}
	defer httpResp.Body.Close()

	// 读取上游响应体
	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		newAPIError = types.NewError(err, types.ErrorCodeDoRequestFailed)
		return newMCPErrorResult(fmt.Sprintf("failed to read upstream response: %s", err.Error())), nil
	}

	if httpResp.StatusCode != http.StatusOK {
		newAPIError = service.RelayErrorHandler(ginCtx.Request.Context(), httpResp, false)
		return newMCPErrorResult(fmt.Sprintf("upstream returned status %d: %s", httpResp.StatusCode, string(respBody))), nil
	}

	// 解析上游图片响应，获取原始 URL 或 b64_json
	rawSources, err := extractImageSources(respBody)
	if err != nil {
		newAPIError = types.NewError(err, types.ErrorCodeBadResponseBody)
		return newMCPErrorResult(fmt.Sprintf("failed to parse image response: %s", err.Error())), nil
	}
	if len(rawSources) == 0 {
		return newMCPErrorResult("no image data in response"), nil
	}

	// 下载图片并缓存
	type cachedImage struct {
		imageID  string
		mimeType string
	}
	cached := make([]cachedImage, 0, len(rawSources))
	for _, src := range rawSources {
		var imageID string
		var mimeType string
		var cacheErr error

		if src.b64 != "" {
			imageID, mimeType, cacheErr = cacheBase64Image(src.b64)
		} else {
			imageID, mimeType, cacheErr = service.DownloadAndCacheImage(src.url, relayInfo.ChannelSetting.Proxy)
		}

		if cacheErr != nil {
			logger.LogWarn(ginCtx, fmt.Sprintf("MCP: failed to cache image: %s", cacheErr.Error()))
			continue
		}
		cached = append(cached, cachedImage{imageID, mimeType})
	}

	if len(cached) == 0 {
		newAPIError = types.NewError(fmt.Errorf("all images failed to cache"), types.ErrorCodeDoRequestFailed)
		return newMCPErrorResult("failed to download and cache any images"), nil
	}

	// 结算计费
	usage := &dto.Usage{
		TotalTokens:      1,
		PromptTokens:     1,
		CompletionTokens: 0,
	}
	service.PostTextConsumeQuota(ginCtx, relayInfo, usage, nil)

	// 构造 MCP 结果：每张图片返回 ImageContent（Agent 可渲染）+ TextContent（代理 URL）
	contents := make([]mcp.Content, 0, len(cached)*2)
	for _, img := range cached {
		entry, found := service.GetCachedImage(img.imageID)
		if !found {
			continue
		}
		// ImageContent：base64 编码的图片数据，Agent 可直接渲染
		contents = append(contents, &mcp.ImageContent{
			Data:     entry.Data,
			MIMEType: entry.MimeType,
		})
		// TextContent：代理 URL，供参考
		proxyURL := buildImageProxyURL(ginCtx, img.imageID, img.mimeType)
		contents = append(contents, &mcp.TextContent{Text: proxyURL})
	}
	return &mcp.CallToolResult{Content: contents}, nil
}

// newMCPErrorResult 构造一个 IsError=true 的 MCP 工具结果
func newMCPErrorResult(message string) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: message},
		},
		IsError: true,
	}
}

// resolveMCPImageIDs 将临时图片 ID 列表解析为本站代理 URL 列表。
// 任一 ID 不存在或已过期则返回错误（由 MCP 工具层提示 Agent 重新上传）。
func resolveMCPImageIDs(c *gin.Context, imageIDs []string) ([]string, error) {
	urls := make([]string, 0, len(imageIDs))
	for _, id := range imageIDs {
		id = strings.TrimSpace(id)
		if id == "" {
			return nil, fmt.Errorf("empty image id in image list")
		}
		// 兼容 Agent 直接传代理 URL 的情况
		if strings.HasPrefix(id, "http://") || strings.HasPrefix(id, "https://") {
			urls = append(urls, id)
			continue
		}
		entry, found := service.GetCachedImage(stripExt(id))
		if !found {
			return nil, fmt.Errorf("image not found or expired: %s (upload again via POST /v1/mcp-upload)", id)
		}
		urls = append(urls, buildImageProxyURL(c, stripExt(id), entry.MimeType))
	}
	return urls, nil
}

// ============================================================================
// 视频生成工具（异步任务）
// ============================================================================

// videoToolArgs 视频提交工具的公共参数
type videoToolArgs struct {
	Prompt   string `json:"prompt"`
	Duration int    `json:"duration"`
	Size     string `json:"size"`
}

// videoToolCommonSchema 返回 prompt/duration/size 公共属性
func videoToolCommonSchema(promptDesc string) map[string]any {
	return map[string]any{
		"prompt": map[string]any{
			"type":        "string",
			"description": promptDesc,
		},
		"duration": map[string]any{
			"type":        "integer",
			"description": "Optional. Video duration in seconds (model-dependent supported range)",
		},
		"size": map[string]any{
			"type":        "string",
			"description": "Optional. Resolution/aspect ratio (model-dependent, e.g. 1280x720, 720P, 16:9)",
		},
	}
}

// generateVideoInputSchema 文生视频工具的 JSON Schema
func generateVideoInputSchema() json.RawMessage {
	schema := map[string]any{
		"type":       "object",
		"properties": videoToolCommonSchema("The text description of the video to generate"),
		"required":   []string{"prompt"},
	}
	data, _ := common.Marshal(schema)
	return json.RawMessage(data)
}

// generateVideoFromFramesInputSchema 首帧/首尾帧生视频工具的 JSON Schema
func generateVideoFromFramesInputSchema() json.RawMessage {
	props := videoToolCommonSchema("The text description of the video to generate")
	props["first_frame_id"] = map[string]any{
		"type":        "string",
		"description": "Required. Temporary image ID (or proxy URL) of the first frame, returned by POST /v1/mcp-upload",
	}
	props["last_frame_id"] = map[string]any{
		"type":        "string",
		"description": "Optional. Temporary image ID (or proxy URL) of the last frame. Providing both first and last frames performs first-last frame interpolation",
	}
	schema := map[string]any{
		"type":       "object",
		"properties": props,
		"required":   []string{"prompt", "first_frame_id"},
	}
	data, _ := common.Marshal(schema)
	return json.RawMessage(data)
}

// generateVideoFromReferenceInputSchema 参考图生视频工具的 JSON Schema
func generateVideoFromReferenceInputSchema() json.RawMessage {
	props := videoToolCommonSchema("The text description of the video to generate")
	props["image_ids"] = map[string]any{
		"type":        "array",
		"items":       map[string]any{"type": "string"},
		"minItems":    1,
		"maxItems":    3,
		"description": "Required. 1-3 temporary image IDs (or proxy URLs) of reference images, returned by POST /v1/mcp-upload",
	}
	schema := map[string]any{
		"type":       "object",
		"properties": props,
		"required":   []string{"prompt", "image_ids"},
	}
	data, _ := common.Marshal(schema)
	return json.RawMessage(data)
}

// getVideoTaskInputSchema 视频任务查询工具的 JSON Schema
func getVideoTaskInputSchema() json.RawMessage {
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"task_id": map[string]any{
				"type":        "string",
				"description": "The task_id returned by a video generation tool",
			},
		},
		"required": []string{"task_id"},
	}
	data, _ := common.Marshal(schema)
	return json.RawMessage(data)
}

// submitMCPTask 提交 MCP 视频生成异步任务（复用 RelayTaskSubmit 完整链路：
// 请求校验 → 模型映射 → 按次计费 + OtherRatios → 预扣费 → 上游提交 → 提交后计费调整）。
// kind 为视频模型池类型（mcp_setting.VideoModelKind*），taskReq 为已构造的入站请求。
// 成功返回公开 task_id；失败已自动退款并返回错误信息。
func submitMCPTask(ginCtx *gin.Context, kind string, taskReq relaycommon.TaskSubmitReq) (string, *mcp.CallToolResult) {
	group := common.GetContextKeyString(ginCtx, constant.ContextKeyUsingGroup)
	if group == "" {
		group = "default"
	}
	videoModel := mcp_setting.GetGroupVideoModel(kind, group)
	if videoModel == "" {
		return "", newMCPErrorResult(fmt.Sprintf("no video model configured for group: %s (kind: %s), please configure the corresponding mcp_setting.group_video_*_models", group, kind))
	}
	taskReq.Model = videoModel

	// 设置 original_model（MCP 路由未走 Distribute 中间件）
	common.SetContextKey(ginCtx, constant.ContextKeyOriginalModel, videoModel)

	// 构造合成 HTTP 请求，让 ValidateRequestAndSetAction / UnmarshalBodyReusable 按标准流程解析
	bodyBytes, err := common.Marshal(taskReq)
	if err != nil {
		return "", newMCPErrorResult(fmt.Sprintf("failed to marshal task request: %s", err.Error()))
	}
	httpReq, err := http.NewRequest(http.MethodPost, "/v1/videos", bytes.NewReader(bodyBytes))
	if err != nil {
		return "", newMCPErrorResult(fmt.Sprintf("failed to build synthetic request: %s", err.Error()))
	}
	httpReq.Header.Set("Content-Type", "application/json")
	ginCtx.Request = httpReq

	relayInfo, err := relaycommon.GenRelayInfo(ginCtx, types.RelayFormatTask, nil, nil)
	if err != nil {
		return "", newMCPErrorResult(fmt.Sprintf("failed to build relay info: %s", err.Error()))
	}

	// 选择渠道并注入渠道元数据（按模型名路由，不限定渠道类型）
	channel, _, err := service.CacheGetRandomSatisfiedChannel(&service.RetryParam{
		Ctx:        ginCtx,
		TokenGroup: relayInfo.TokenGroup,
		ModelName:  videoModel,
		Retry:      common.GetPointer(0),
	})
	if err != nil {
		return "", newMCPErrorResult(fmt.Sprintf("no available channel: %s", err.Error()))
	}
	if channel == nil {
		return "", newMCPErrorResult(fmt.Sprintf("no available channel for model: %s", videoModel))
	}
	if newAPIErr := middleware.SetupContextForSelectedChannel(ginCtx, channel, videoModel); newAPIErr != nil {
		return "", newMCPErrorResult(fmt.Sprintf("channel setup failed: %s", newAPIErr.Error()))
	}

	// 提交任务（内部完成校验、价格计算、预扣费、上游请求、提交后计费调整）
	result, taskErr := relay.RelayTaskSubmit(ginCtx, relayInfo)
	if taskErr != nil {
		if relayInfo.Billing != nil {
			relayInfo.Billing.Refund(ginCtx)
		}
		return "", newMCPErrorResult(fmt.Sprintf("task submit failed: %s", taskErr.Message))
	}

	// 成功收尾：结算 + 消费日志 + 插入任务（后台轮询器负责后续状态更新与最终结算/退款）
	if settleErr := service.SettleBilling(ginCtx, relayInfo, result.Quota); settleErr != nil {
		common.SysError("MCP: settle task billing error: " + settleErr.Error())
	}
	service.LogTaskConsumption(ginCtx, relayInfo)

	task := model.InitTask(result.Platform, relayInfo)
	task.PrivateData.UpstreamTaskID = result.UpstreamTaskID
	task.PrivateData.BillingSource = relayInfo.BillingSource
	task.PrivateData.SubscriptionId = relayInfo.SubscriptionId
	task.PrivateData.TokenId = relayInfo.TokenId
	task.PrivateData.BillingContext = &model.TaskBillingContext{
		ModelPrice:      relayInfo.PriceData.ModelPrice,
		GroupRatio:      relayInfo.PriceData.GroupRatioInfo.GroupRatio,
		ModelRatio:      relayInfo.PriceData.ModelRatio,
		OtherRatios:     relayInfo.PriceData.OtherRatios,
		OriginModelName: relayInfo.OriginModelName,
		PerCallBilling:  common.StringsContains(constant.TaskPricePatches, relayInfo.OriginModelName) || relayInfo.PriceData.UsePrice,
	}
	task.Quota = result.Quota
	task.Data = result.TaskData
	task.Action = relayInfo.Action
	task.Properties.Duration = relaycommon.ResolveRequestedDuration(taskReq)
	if insertErr := task.Insert(); insertErr != nil {
		common.SysError("MCP: insert task error: " + insertErr.Error())
	}

	// 返回提交结果
	resultData, _ := common.Marshal(gin.H{
		"task_id":                task.TaskID,
		"status":                 "submitted",
		"model":                  videoModel,
		"message":                "Video generation task submitted. Poll get_video_task with task_id to check progress.",
		"estimated_wait_seconds": 60,
	})
	return task.TaskID, &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: string(resultData)},
		},
	}
}

// handleMCPGenerateVideo 文生视频工具 handler（t2v 模型池）
func handleMCPGenerateVideo(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	ginCtx, ok := ctx.Value(mcpGinContextKey).(*gin.Context)
	if !ok || ginCtx == nil {
		return newMCPErrorResult("internal error: failed to get request context"), nil
	}

	var args videoToolArgs
	if err := common.Unmarshal(req.Params.Arguments, &args); err != nil {
		return newMCPErrorResult(fmt.Sprintf("invalid arguments: %s", err.Error())), nil
	}
	if args.Prompt == "" {
		return newMCPErrorResult("prompt is required"), nil
	}

	taskReq := relaycommon.TaskSubmitReq{
		Prompt:   args.Prompt,
		Size:     args.Size,
		Duration: args.Duration,
	}
	_, result := submitMCPTask(ginCtx, mcp_setting.VideoModelKindT2V, taskReq)
	return result, nil
}

// handleMCPGenerateVideoFromFrames 首帧/首尾帧生视频工具 handler
// 仅传 first_frame_id → i2v 模型池；同时传 last_frame_id → kf2v 模型池
func handleMCPGenerateVideoFromFrames(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	ginCtx, ok := ctx.Value(mcpGinContextKey).(*gin.Context)
	if !ok || ginCtx == nil {
		return newMCPErrorResult("internal error: failed to get request context"), nil
	}

	var args struct {
		videoToolArgs
		FirstFrameID string `json:"first_frame_id"`
		LastFrameID  string `json:"last_frame_id"`
	}
	if err := common.Unmarshal(req.Params.Arguments, &args); err != nil {
		return newMCPErrorResult(fmt.Sprintf("invalid arguments: %s", err.Error())), nil
	}
	if args.Prompt == "" {
		return newMCPErrorResult("prompt is required"), nil
	}
	if args.FirstFrameID == "" {
		return newMCPErrorResult("first_frame_id is required"), nil
	}

	frameURLs, resolveErr := resolveMCPImageIDs(ginCtx, []string{args.FirstFrameID, args.LastFrameID})
	if resolveErr != nil {
		return newMCPErrorResult(resolveErr.Error()), nil
	}

	metadata := map[string]interface{}{
		relaycommon.MetadataKeyFirstFrame: frameURLs[0],
	}
	kind := mcp_setting.VideoModelKindI2V
	if args.LastFrameID != "" {
		metadata[relaycommon.MetadataKeyLastFrame] = frameURLs[1]
		kind = mcp_setting.VideoModelKindKF2V
	}

	taskReq := relaycommon.TaskSubmitReq{
		Prompt:   args.Prompt,
		Size:     args.Size,
		Duration: args.Duration,
		Metadata: metadata,
	}
	_, result := submitMCPTask(ginCtx, kind, taskReq)
	return result, nil
}

// handleMCPGenerateVideoFromReference 参考图生视频工具 handler（r2v 模型池，最多 3 张参考图）
func handleMCPGenerateVideoFromReference(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	ginCtx, ok := ctx.Value(mcpGinContextKey).(*gin.Context)
	if !ok || ginCtx == nil {
		return newMCPErrorResult("internal error: failed to get request context"), nil
	}

	var args struct {
		videoToolArgs
		ImageIDs []string `json:"image_ids"`
	}
	if err := common.Unmarshal(req.Params.Arguments, &args); err != nil {
		return newMCPErrorResult(fmt.Sprintf("invalid arguments: %s", err.Error())), nil
	}
	if args.Prompt == "" {
		return newMCPErrorResult("prompt is required"), nil
	}
	if len(args.ImageIDs) == 0 {
		return newMCPErrorResult("image_ids is required (1-3 reference image IDs)"), nil
	}
	if len(args.ImageIDs) > maxMCPReferenceImages {
		return newMCPErrorResult(fmt.Sprintf("image_ids supports at most %d images, got %d", maxMCPReferenceImages, len(args.ImageIDs))), nil
	}

	refURLs, resolveErr := resolveMCPImageIDs(ginCtx, args.ImageIDs)
	if resolveErr != nil {
		return newMCPErrorResult(resolveErr.Error()), nil
	}

	taskReq := relaycommon.TaskSubmitReq{
		Prompt:   args.Prompt,
		Size:     args.Size,
		Duration: args.Duration,
		Metadata: map[string]interface{}{
			relaycommon.MetadataKeyReferenceImages: refURLs,
		},
	}
	_, result := submitMCPTask(ginCtx, mcp_setting.VideoModelKindR2V, taskReq)
	return result, nil
}

// handleMCPGetVideoTask 视频任务查询工具 handler。
// 直读数据库（与 GET /v1/videos/:task_id 一致），进度由后台 TaskPollingLoop 更新。
func handleMCPGetVideoTask(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	ginCtx, ok := ctx.Value(mcpGinContextKey).(*gin.Context)
	if !ok || ginCtx == nil {
		return newMCPErrorResult("internal error: failed to get request context"), nil
	}

	var args struct {
		TaskID string `json:"task_id"`
	}
	if err := common.Unmarshal(req.Params.Arguments, &args); err != nil {
		return newMCPErrorResult(fmt.Sprintf("invalid arguments: %s", err.Error())), nil
	}
	if args.TaskID == "" {
		return newMCPErrorResult("task_id is required"), nil
	}

	// 归属校验：只能查询本人任务（与 REST 查询一致）
	task, exist, err := model.GetByTaskId(ginCtx.GetInt("id"), args.TaskID)
	if err != nil {
		return newMCPErrorResult(fmt.Sprintf("failed to get task: %s", err.Error())), nil
	}
	if !exist {
		return newMCPErrorResult(fmt.Sprintf("task not found: %s", args.TaskID)), nil
	}

	resultData, _ := common.Marshal(gin.H{
		"task_id":     task.TaskID,
		"status":      string(task.Status),
		"progress":    task.Progress,
		"fail_reason": task.FailReason,
		"result_url":  task.GetResultURL(),
		"proxy_url":   taskcommon.BuildProxyURL(task.TaskID),
	})
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: string(resultData)},
		},
	}, nil
}

// imageSource 表示一个图片来源（URL 或 base64）
type imageSource struct {
	url string
	b64 string
}

// extractImageSources 从上游响应中解析图片数据
// 支持 OpenAI 标准格式 {"data":[{"url":"..."}]} 和 b64_json 格式
func extractImageSources(respBody []byte) ([]imageSource, error) {
	var imageResp dto.ImageResponse
	if err := common.Unmarshal(respBody, &imageResp); err == nil && len(imageResp.Data) > 0 {
		sources := make([]imageSource, 0, len(imageResp.Data))
		for _, d := range imageResp.Data {
			if d.Url != "" {
				sources = append(sources, imageSource{url: d.Url})
			} else if d.B64Json != "" {
				sources = append(sources, imageSource{b64: d.B64Json})
			}
		}
		if len(sources) > 0 {
			return sources, nil
		}
	}

	text := string(respBody)
	if len(text) > 0 {
		return nil, fmt.Errorf("unrecognized image response format")
	}

	return nil, fmt.Errorf("empty response")
}

// cacheBase64Image 将 base64 编码的图片数据解码后存入缓存，返回缓存 ID
// 兼容以下格式：
//   - 纯 base64: "iVBORw0KGgo..."
//   - data URI: "data:image/png;base64,iVBORw0KGgo..."
func cacheBase64Image(b64Data string) (string, string, error) {
	// 去掉 data URI 前缀
	b64Data = stripBase64Prefix(b64Data)

	data, err := base64.StdEncoding.DecodeString(b64Data)
	if err != nil {
		return "", "", fmt.Errorf("failed to decode base64 image: %w", err)
	}

	imageID := generateImageCacheID(string(data))
	mimeType := http.DetectContentType(data)
	entry := service.MCPImageEntry{
		Data:     data,
		MimeType: mimeType,
		OrigSize: int64(len(data)),
	}
	if err := service.SetCachedImage(imageID, entry); err != nil {
		return "", "", fmt.Errorf("failed to cache base64 image: %w", err)
	}
	return imageID, mimeType, nil
}

// buildImageProxyURL 构造代理图片 URL（含扩展名）
// 使用请求的 Host 和 scheme，确保返回 Agent 可访问的公网地址
func buildImageProxyURL(c *gin.Context, imageID string, mimeType string) string {
	scheme := "https"
	if c.Request.TLS == nil && c.GetHeader("X-Forwarded-Proto") != "https" {
		scheme = "http"
	}
	ext := service.MimeTypeToExt(mimeType)
	return fmt.Sprintf("%s://%s/v1/mcp-image/%s%s", scheme, c.Request.Host, imageID, ext)
}

// generateImageCacheID 根据数据内容生成唯一缓存 ID
func generateImageCacheID(data string) string {
	h := sha256.New()
	h.Write([]byte(data))
	h.Write([]byte(time.Now().Format(time.RFC3339Nano)))
	return hex.EncodeToString(h.Sum(nil))[:16]
}

// stripBase64Prefix 去掉 data URI 前缀，返回纯 base64 数据
// 支持格式: "data:image/png;base64,iVBORw0KGgo..."
func stripBase64Prefix(s string) string {
	if idx := strings.Index(s, ";base64,"); idx != -1 {
		return s[idx+len(";base64,"):]
	}
	return s
}

// MCPServerHandler 返回 MCP StreamableHTTP handler
// 使用 Stateless 模式，每个请求独立处理，不维护会话状态
func MCPServerHandler() http.Handler {
	return mcp.NewStreamableHTTPHandler(func(r *http.Request) *mcp.Server {
		return mcpServer
	}, &mcp.StreamableHTTPOptions{
		Stateless: true,
	})
}

// ServeMCPImage 从缓存中获取图片并返回给客户端
// GET /v1/mcp/images/:imageId （支持可选的扩展名，如 /v1/mcp/images/abc123.png）
func ServeMCPImage(c *gin.Context) {
	imageID := c.Param("imageId")
	if imageID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "imageId is required"})
		return
	}

	// 去掉可选的扩展名（如 .png .jpg）
	imageID = stripExt(imageID)

	entry, found := service.GetCachedImage(imageID)
	if !found {
		c.JSON(http.StatusNotFound, gin.H{"error": "image not found or expired"})
		return
	}

	c.Header("Content-Type", entry.MimeType)
	c.Header("Content-Length", fmt.Sprintf("%d", entry.OrigSize))
	c.Header("Cache-Control", "public, max-age=1800")
	c.Data(http.StatusOK, entry.MimeType, entry.Data)
}

// stripExt 去掉文件扩展名，如 "abc123.png" → "abc123"
func stripExt(s string) string {
	if idx := strings.LastIndex(s, "."); idx != -1 {
		return s[:idx]
	}
	return s
}

// allowedUploadMimeTypes MCP 临时上传允许的图片 MIME 类型白名单
var allowedUploadMimeTypes = map[string]bool{
	"image/png":  true,
	"image/jpeg": true,
	"image/webp": true,
	"image/gif":  true,
}

// MCPUploadImage 处理 MCP 临时图片上传（POST /v1/mcp-upload，multipart/form-data）。
// 返回临时图片 ID（2 小时后自动删除），供 generate_image / generate_video_from_* 工具
// 以 image_ids / first_frame_id / last_frame_id 参数引用。
func MCPUploadImage(c *gin.Context) {
	fileHeader, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "file field is required (multipart/form-data)"})
		return
	}
	if fileHeader.Size > service.MCPUploadMaxSize {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": fmt.Sprintf("file too large: %d bytes, max %d bytes", fileHeader.Size, service.MCPUploadMaxSize)})
		return
	}

	file, err := fileHeader.Open()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": fmt.Sprintf("failed to open uploaded file: %s", err.Error())})
		return
	}
	defer file.Close()

	// 限制读取量并检测真实内容类型（防止扩展名伪造）
	data, err := io.ReadAll(io.LimitReader(file, service.MCPUploadMaxSize+1))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": fmt.Sprintf("failed to read uploaded file: %s", err.Error())})
		return
	}
	if len(data) > service.MCPUploadMaxSize {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": fmt.Sprintf("file too large, max %d bytes", service.MCPUploadMaxSize)})
		return
	}
	mimeType := http.DetectContentType(data)
	if !allowedUploadMimeTypes[mimeType] {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": fmt.Sprintf("unsupported image type: %s (allowed: png, jpeg, webp, gif)", mimeType)})
		return
	}

	imageID := generateImageCacheID(string(data))
	entry := service.MCPImageEntry{
		Data:     data,
		MimeType: mimeType,
		OrigSize: int64(len(data)),
	}
	if err := service.SetCachedImageWithTTL(imageID, entry, service.MCPUploadImageTTL); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": fmt.Sprintf("failed to cache image: %s", err.Error())})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"id":         imageID,
			"url":        buildImageProxyURL(c, imageID, mimeType),
			"mime_type":  mimeType,
			"size":       len(data),
			"expires_in": int(service.MCPUploadImageTTL.Seconds()),
		},
	})
}
