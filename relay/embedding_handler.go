package relay

import (
	"bytes"
	"fmt"
	"io"
	"net/http"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

// teeWriter 包装 gin.ResponseWriter，在写入客户端的同时捕获输出到 buffer
type teeWriter struct {
	gin.ResponseWriter
	buf *bytes.Buffer
}

func (w *teeWriter) Write(b []byte) (int, error) {
	w.buf.Write(b)
	return w.ResponseWriter.Write(b)
}

func EmbeddingHelper(c *gin.Context, info *relaycommon.RelayInfo) (newAPIError *types.NewAPIError) {
	info.InitChannelMeta(c)

	embeddingReq, ok := info.Request.(*dto.EmbeddingRequest)
	if !ok {
		return types.NewErrorWithStatusCode(fmt.Errorf("invalid request type, expected *dto.EmbeddingRequest, got %T", info.Request), types.ErrorCodeInvalidRequest, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
	}

	// === 缓存读取：在 DoRequest 之前 ===
	tokenKey := common.GetContextKeyString(c, constant.ContextKeyTokenKey)
	inputs := embeddingReq.ParseInput()
	cacheEnabled := service.EmbeddingCacheEnabled() && tokenKey != "" && len(inputs) > 0

	if cacheEnabled {
		if entry, found := service.LookupEmbeddingCache(tokenKey, embeddingReq.Model, inputs, embeddingReq.EncodingFormat, embeddingReq.Dimensions); found {
			// 缓存命中：直接写客户端
			info.CacheHit = true
			c.Header("Content-Type", "application/json")
			c.Status(http.StatusOK)
			_, _ = c.Writer.Write(entry.ResponseBody)
			service.PostTextConsumeQuota(c, info, &entry.Usage, nil)
			return nil
		}
	}
	// === 缓存读取结束 ===

	request, err := common.DeepCopy(embeddingReq)
	if err != nil {
		return types.NewError(fmt.Errorf("failed to copy request to EmbeddingRequest: %w", err), types.ErrorCodeInvalidRequest, types.ErrOptionWithSkipRetry())
	}

	err = helper.ModelMappedHelper(c, info, request)
	if err != nil {
		return types.NewError(err, types.ErrorCodeChannelModelMappedError, types.ErrOptionWithSkipRetry())
	}

	adaptor := GetAdaptor(info.ApiType)
	if adaptor == nil {
		return types.NewError(fmt.Errorf("invalid api type: %d", info.ApiType), types.ErrorCodeInvalidApiType, types.ErrOptionWithSkipRetry())
	}
	adaptor.Init(info)

	convertedRequest, err := adaptor.ConvertEmbeddingRequest(c, info, *request)
	if err != nil {
		return types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
	}
	relaycommon.AppendRequestConversionFromRequest(info, convertedRequest)
	jsonData, err := common.Marshal(convertedRequest)
	if err != nil {
		return types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
	}

	if len(info.ParamOverride) > 0 {
		jsonData, err = relaycommon.ApplyParamOverrideWithRelayInfo(jsonData, info)
		if err != nil {
			return newAPIErrorFromParamOverride(err)
		}
	}

	logger.LogDebug(c, "converted embedding request body: %s", jsonData)
	body, size, closer, err := relaycommon.NewOutboundJSONBody(jsonData)
	if err != nil {
		return types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
	}
	defer closer.Close()
	jsonData = nil
	info.UpstreamRequestBodySize = size
	var requestBody io.Reader = body
	statusCodeMappingStr := c.GetString("status_code_mapping")

	// === singleflight 包裹上游调用（缓存启用时） ===
	if cacheEnabled {
		result, shared, sfErr := service.ExecuteEmbeddingFetch(
			tokenKey, embeddingReq.Model, inputs, embeddingReq.EncodingFormat, embeddingReq.Dimensions,
			func() (service.EmbeddingFetchResult, error) {
				// === Leader: 执行上游调用 ===
				resp, err := adaptor.DoRequest(c, info, requestBody)
				if err != nil {
					return service.EmbeddingFetchResult{}, types.NewOpenAIError(err, types.ErrorCodeDoRequestFailed, http.StatusInternalServerError)
				}

				var httpResp *http.Response
				if resp != nil {
					httpResp = resp.(*http.Response)
					if httpResp.StatusCode != http.StatusOK {
						newAPIError = service.RelayErrorHandler(c.Request.Context(), httpResp, false)
						service.ResetStatusCode(newAPIError, statusCodeMappingStr)
						return service.EmbeddingFetchResult{}, newAPIError
					}
				}

				// 读取 body 字节，替换 body 供 DoResponse 使用
				respBodyBytes, err := io.ReadAll(httpResp.Body)
				if err != nil {
					return service.EmbeddingFetchResult{}, types.NewError(err, types.ErrorCodeReadResponseBodyFailed, types.ErrOptionWithSkipRetry())
				}
				httpResp.Body.Close()
				httpResp.Body = io.NopCloser(bytes.NewReader(respBodyBytes))

				// 安装 tee writer 捕获 DoResponse 输出
				buf := &bytes.Buffer{}
				origWriter := c.Writer
				c.Writer = &teeWriter{ResponseWriter: origWriter, buf: buf}

				usage, doRespErr := adaptor.DoResponse(c, httpResp, info)

				// 恢复原 writer
				c.Writer = origWriter

				if doRespErr != nil {
					service.ResetStatusCode(doRespErr, statusCodeMappingStr)
					return service.EmbeddingFetchResult{}, doRespErr
				}

				actualUsage := *usage.(*dto.Usage)

				// 缓存: 存储 DoResponse 输出
				service.StoreEmbeddingCache(tokenKey, embeddingReq.Model, inputs, embeddingReq.EncodingFormat, embeddingReq.Dimensions, buf.Bytes(), actualUsage)

				return service.EmbeddingFetchResult{Body: buf.Bytes(), Usage: actualUsage}, nil
			},
		)

		if sfErr != nil {
			if apiErr, ok := sfErr.(*types.NewAPIError); ok {
				return apiErr
			}
			return types.NewError(sfErr, types.ErrorCodeDoRequestFailed, types.ErrOptionWithSkipRetry())
		}

		// Follower: 写 body 到自己的客户端
		// Leader (shared=false): DoResponse 已通过 tee writer 写入客户端，无需再写
		if shared {
			c.Header("Content-Type", "application/json")
			c.Status(http.StatusOK)
			_, _ = c.Writer.Write(result.Body)
		}

		// 所有 caller 各自执行后扣费
		service.PostTextConsumeQuota(c, info, &result.Usage, nil)
		return nil
	}
	// === singleflight 结束 ===

	// === 缓存未启用时的原有逻辑 ===
	resp, err := adaptor.DoRequest(c, info, requestBody)
	if err != nil {
		return types.NewOpenAIError(err, types.ErrorCodeDoRequestFailed, http.StatusInternalServerError)
	}

	var httpResp *http.Response
	if resp != nil {
		httpResp = resp.(*http.Response)
		if httpResp.StatusCode != http.StatusOK {
			newAPIError = service.RelayErrorHandler(c.Request.Context(), httpResp, false)
			// reset status code 重置状态码
			service.ResetStatusCode(newAPIError, statusCodeMappingStr)
			return newAPIError
		}
	}

	usage, newAPIError := adaptor.DoResponse(c, httpResp, info)
	if newAPIError != nil {
		// reset status code 重置状态码
		service.ResetStatusCode(newAPIError, statusCodeMappingStr)
		return newAPIError
	}
	service.PostTextConsumeQuota(c, info, usage.(*dto.Usage), nil)
	return nil
}
