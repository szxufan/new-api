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
	"github.com/QuantumNous/new-api/relay"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
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
		Description: "Generate an image from a text prompt using the configured image model for the current group",
		InputSchema: generateImageInputSchema(),
	}, handleMCPGenerateImage)
}

// generateImageInputSchema 返回 generate_image 工具的 JSON Schema
func generateImageInputSchema() json.RawMessage {
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"prompt": map[string]any{
				"type":        "string",
				"description": "The text description of the image to generate",
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
func handleMCPGenerateImage(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// 从 context 中获取 gin context（在路由层注入）
	ginCtx, ok := ctx.Value(mcpGinContextKey).(*gin.Context)
	if !ok || ginCtx == nil {
		logger.LogError(nil, "MCP: failed to get gin context from MCP tool handler context")
		return newMCPErrorResult("internal error: failed to get request context"), nil
	}

	// 解析工具参数
	var args struct {
		Prompt string `json:"prompt"`
	}
	if err := common.Unmarshal(req.Params.Arguments, &args); err != nil {
		return newMCPErrorResult(fmt.Sprintf("invalid arguments: %s", err.Error())), nil
	}
	if args.Prompt == "" {
		return newMCPErrorResult("prompt is required"), nil
	}

	// 从配置获取该分组的文生图模型
	group := common.GetContextKeyString(ginCtx, constant.ContextKeyUsingGroup)
	if group == "" {
		group = "default"
	}
	model := mcp_setting.GetGroupImageModel(group)
	if model == "" {
		return newMCPErrorResult(fmt.Sprintf("no image model configured for group: %s", group)), nil
	}

	// 构造 ImageRequest（仅 prompt 和 model，其他参数由上游默认值决定）
	imageReq := &dto.ImageRequest{
		Model:  model,
		Prompt: args.Prompt,
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

	// 覆盖 RequestURLPath 为正确的文生图路径
	// GenRelayInfo 从原始请求 URL 读取路径（/v1/mcp），
	// 但上游需要的是 /v1/images/generations
	relayInfo.RequestURLPath = "/v1/images/generations"

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
