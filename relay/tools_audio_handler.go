package relay

import (
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

// ToolsAudioHelper 处理 DeepSeek 原生语音工具中继请求（/v1/tools/tts、/v1/tools/asr）。
// 与 OpenAI 兼容路径（AudioHelper）不同：响应原样返回（TTS 为音频字节流、ASR 为
// {request_id, text, duration_ms}），不做 OpenAI 格式归一化。上游请求体不透传原始
// body：客户端 body/form 中可选的 model 字段仅用于网关路由与计费，不得发送到上游
// （上游 TTS/ASR 请求 schema 均无 model 字段，ASR 仅接受 file 字段），因此 TTS 按
// DTO 序列化（Model 为 json:"-"）、ASR 重建 multipart 仅保留 file 字段。
// 预扣费/退款/重试由上层 Relay 统一处理；本函数在成功后按音频时长结算
// （复用 AudioHelper 的计费分支：AudioTokens > 0 走 PostAudioConsumeQuota）。
func ToolsAudioHelper(c *gin.Context, info *relaycommon.RelayInfo) (newAPIError *types.NewAPIError) {
	info.InitChannelMeta(c)

	var request dto.Request
	switch info.RelayMode {
	case relayconstant.RelayModeToolsTTS:
		ttsReq, ok := info.Request.(*dto.ToolsTTSRequest)
		if !ok {
			return types.NewErrorWithStatusCode(fmt.Errorf("invalid request type, expected dto.ToolsTTSRequest, got %T", info.Request), types.ErrorCodeInvalidRequest, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
		}
		copied, err := common.DeepCopy(ttsReq)
		if err != nil {
			return types.NewError(fmt.Errorf("failed to copy tools tts request: %w", err), types.ErrorCodeInvalidRequest, types.ErrOptionWithSkipRetry())
		}
		request = copied
	case relayconstant.RelayModeToolsASR:
		asrReq, ok := info.Request.(*dto.ToolsASRRequest)
		if !ok {
			return types.NewErrorWithStatusCode(fmt.Errorf("invalid request type, expected dto.ToolsASRRequest, got %T", info.Request), types.ErrorCodeInvalidRequest, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
		}
		copied, err := common.DeepCopy(asrReq)
		if err != nil {
			return types.NewError(fmt.Errorf("failed to copy tools asr request: %w", err), types.ErrorCodeInvalidRequest, types.ErrOptionWithSkipRetry())
		}
		request = copied
	default:
		return types.NewErrorWithStatusCode(fmt.Errorf("unsupported tools audio relay mode: %d", info.RelayMode), types.ErrorCodeInvalidRequest, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
	}

	if err := helper.ModelMappedHelper(c, info, request); err != nil {
		return types.NewError(err, types.ErrorCodeChannelModelMappedError, types.ErrOptionWithSkipRetry())
	}

	adaptor := GetAdaptor(info.ApiType)
	if adaptor == nil {
		return types.NewError(fmt.Errorf("invalid api type: %d", info.ApiType), types.ErrorCodeInvalidApiType, types.ErrOptionWithSkipRetry())
	}
	adaptor.Init(info)

	// 构建上游请求体（剥离路由/计费用的 model 字段，见函数注释）：
	// - TTS：按原生 DTO 序列化为 JSON
	// - ASR：复用 DeepSeek 适配器的 multipart 重建逻辑，仅保留 file 字段
	//   （空 AudioRequest 仅做占位：ResponseFormat 为空跳过兼容路径的格式校验）
	var body io.Reader
	switch info.RelayMode {
	case relayconstant.RelayModeToolsTTS:
		jsonData, err := common.Marshal(request)
		if err != nil {
			return types.NewError(fmt.Errorf("failed to marshal tools tts request: %w", err), types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
		}
		jsonBody, size, closer, err := relaycommon.NewOutboundJSONBody(jsonData)
		if err != nil {
			return types.NewError(fmt.Errorf("failed to build tools tts request body: %w", err), types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
		}
		defer closer.Close()
		info.UpstreamRequestBodySize = size
		body = jsonBody
	case relayconstant.RelayModeToolsASR:
		multipartBody, err := adaptor.ConvertAudioRequest(c, info, dto.AudioRequest{})
		if err != nil {
			return types.NewError(fmt.Errorf("failed to build tools asr request body: %w", err), types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
		}
		body = multipartBody
	}

	resp, err := adaptor.DoRequest(c, info, body)
	if err != nil {
		return types.NewOpenAIError(err, types.ErrorCodeDoRequestFailed, http.StatusInternalServerError)
	}
	statusCodeMappingStr := c.GetString("status_code_mapping")

	var httpResp *http.Response
	if resp != nil {
		var ok bool
		httpResp, ok = resp.(*http.Response)
		if !ok || httpResp == nil {
			return types.NewError(errors.New("unexpected response type from upstream"), types.ErrorCodeDoRequestFailed, types.ErrOptionWithSkipRetry())
		}
		if httpResp.StatusCode != http.StatusOK {
			newAPIError = service.RelayErrorHandler(c.Request.Context(), httpResp, false)
			service.ResetStatusCode(newAPIError, statusCodeMappingStr)
			return newAPIError
		}
	}

	usage, newAPIError := adaptor.DoResponse(c, httpResp, info)
	if newAPIError != nil {
		service.ResetStatusCode(newAPIError, statusCodeMappingStr)
		return newAPIError
	}
	if usage.(*dto.Usage).CompletionTokenDetails.AudioTokens > 0 || usage.(*dto.Usage).PromptTokensDetails.AudioTokens > 0 {
		service.PostAudioConsumeQuota(c, info, usage.(*dto.Usage), "")
	} else {
		service.PostTextConsumeQuota(c, info, usage.(*dto.Usage), nil)
	}

	return nil
}
