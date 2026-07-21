package controller

import (
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	perfmetrics "github.com/QuantumNous/new-api/pkg/perf_metrics"
	"github.com/QuantumNous/new-api/relay"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/types"

	"github.com/bytedance/gopkg/util/gopool"
	"github.com/samber/lo"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

func relayHandler(c *gin.Context, info *relaycommon.RelayInfo) *types.NewAPIError {
	var err *types.NewAPIError
	switch info.RelayMode {
	case relayconstant.RelayModeImagesGenerations, relayconstant.RelayModeImagesEdits:
		err = relay.ImageHelper(c, info)
	case relayconstant.RelayModeAudioSpeech:
		fallthrough
	case relayconstant.RelayModeAudioTranslation:
		fallthrough
	case relayconstant.RelayModeAudioTranscription:
		err = relay.AudioHelper(c, info)
	case relayconstant.RelayModeRerank:
		err = relay.RerankHelper(c, info)
	case relayconstant.RelayModeEmbeddings:
		err = relay.EmbeddingHelper(c, info)
	case relayconstant.RelayModeResponses, relayconstant.RelayModeResponsesCompact:
		err = relay.ResponsesHelper(c, info)
	default:
		err = relay.TextHelper(c, info)
	}
	return err
}

func geminiRelayHandler(c *gin.Context, info *relaycommon.RelayInfo) *types.NewAPIError {
	var err *types.NewAPIError
	if strings.Contains(c.Request.URL.Path, "embed") {
		err = relay.GeminiEmbeddingHandler(c, info)
	} else {
		err = relay.GeminiHelper(c, info)
	}
	return err
}

// executeRelayHandler 根据 relayFormat 分发到对应的 relay handler。
func executeRelayHandler(c *gin.Context, relayInfo *relaycommon.RelayInfo, relayFormat types.RelayFormat) *types.NewAPIError {
	switch relayFormat {
	case types.RelayFormatOpenAIRealtime:
		return relay.WssHelper(c, relayInfo)
	case types.RelayFormatClaude:
		return relay.ClaudeHelper(c, relayInfo)
	case types.RelayFormatGemini:
		return geminiRelayHandler(c, relayInfo)
	default:
		return relayHandler(c, relayInfo)
	}
}

// shouldRetryChannel 判断当前错误是否应该重试同一渠道。
func shouldRetryChannel(c *gin.Context, openaiErr *types.NewAPIError) bool {
	if openaiErr == nil {
		return false
	}
	// 检测命中总是允许重试（skipRetry 已在上游根据次数设置）
	if openaiErr.GetErrorCode() == types.ErrorCodeResponseDetectionHit {
		return true
	}
	if service.ShouldSkipRetryAfterChannelAffinityFailure(c) {
		return false
	}
	if types.IsChannelError(openaiErr) {
		return true
	}
	if types.IsSkipRetryError(openaiErr) {
		return false
	}
	if _, ok := c.Get("specific_channel_id"); ok {
		return false
	}
	code := openaiErr.StatusCode
	if code >= 200 && code < 300 {
		return false
	}
	if code < 100 || code > 599 {
		return true
	}
	if operation_setting.IsAlwaysSkipRetryCode(openaiErr.GetErrorCode()) {
		return false
	}
	return operation_setting.ShouldRetryByStatusCode(code)
}

// makeChannelError 从 channel 和 context 构造 ChannelError，供 processChannelError 使用。
func makeChannelError(channel *model.Channel, c *gin.Context) types.ChannelError {
	return *types.NewChannelError(
		channel.Id, channel.Type, channel.Name,
		channel.ChannelInfo.IsMultiKey,
		common.GetContextKeyString(c, constant.ContextKeyChannelKey),
		channel.GetAutoBan(),
		channel.GetDisable429Ban(),
	)
}

// logRelayResult 封装重试结束后的日志记录：use_channel 日志 + recordFinalErrorLog + perfmetrics。
func logRelayResult(c *gin.Context, relayInfo *relaycommon.RelayInfo, newAPIError *types.NewAPIError) {
	useChannel := c.GetStringSlice("use_channel")
	if len(useChannel) > 1 {
		retryLogStr := fmt.Sprintf("重试：%s", strings.Trim(strings.Join(strings.Fields(fmt.Sprint(useChannel)), "->"), "[]"))
		logger.LogInfo(c, retryLogStr)
	}
	if newAPIError != nil {
		recordFinalErrorLog(c, relayInfo, newAPIError)
		gopool.Go(func() {
			perfmetrics.RecordRelaySample(relayInfo, false, 0)
		})
	}
}

// tryChannelOnce 对单个渠道执行内层重试循环（不含 fallback 触发）。
// 主渠道和 fallback 渠道共用此函数。
// 429/自动禁用时立刻返回，不消耗剩余 perChannelAttempts，由调用方决定是否进 fallback。
// 返回 nil 表示成功，返回非 nil 表示失败。
func tryChannelOnce(
	c *gin.Context,
	relayInfo *relaycommon.RelayInfo,
	relayFormat types.RelayFormat,
	channel *model.Channel,
	perChannelAttempts int,
	totalAttempts *int,
	maxAttempts int,
) *types.NewAPIError {
	if setupErr := middleware.SetupContextForSelectedChannel(c, channel, relayInfo.OriginModelName); setupErr != nil {
		return setupErr
	}

	var lastError *types.NewAPIError
	for attempt := 0; attempt < perChannelAttempts; attempt++ {
		if *totalAttempts >= maxAttempts {
			return lastError
		}
		*totalAttempts++
		relayInfo.RetryIndex = *totalAttempts

		addUsedChannel(c, channel.Id)
		bodyStorage, bodyErr := common.GetBodyStorage(c)
		if bodyErr != nil {
			if common.IsRequestBodyTooLargeError(bodyErr) || errors.Is(bodyErr, common.ErrRequestBodyTooLarge) {
				return types.NewErrorWithStatusCode(bodyErr, types.ErrorCodeReadRequestBodyFailed, http.StatusRequestEntityTooLarge, types.ErrOptionWithSkipRetry())
			}
			return types.NewErrorWithStatusCode(bodyErr, types.ErrorCodeReadRequestBodyFailed, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
		}
		c.Request.Body = io.NopCloser(bodyStorage)

		lastError = executeRelayHandler(c, relayInfo, relayFormat)
		if lastError == nil {
			// HTTP 层面成功，检查是否有响应内容检测命中
			if relayInfo.DetectionHit {
				lastError = handleDetectionHit(relayInfo)
			}
		}
		if lastError == nil {
			return nil
		}
		// 检测命中错误不是渠道错误，跳过渠道错误处理（不应触发自动禁用/限流）
		if !helper.IsDetectionHitError(lastError) {
			lastError = service.NormalizeViolationFeeError(lastError)
			relayInfo.LastError = lastError
			processChannelError(c, makeChannelError(channel, c), lastError)
		}

		// 429/自动禁用 → 立刻尝试 fallback 渠道
		if isFallbackEligibleError(lastError, channel.GetDisable429Ban()) {
			fbError := tryFallbackChannels(c, relayInfo, relayFormat, channel, perChannelAttempts, totalAttempts, maxAttempts, channel.Id, lastError)
			if fbError == nil {
				return nil
			}
			lastError = fbError
			// fallback 也失败，继续重试当前渠道
			if !shouldRetryChannel(c, lastError) {
				return lastError
			}
			if common.RetryIntervalMs > 0 {
				retryKeepAliveSleep(c, time.Duration(common.RetryIntervalMs)*time.Millisecond)
			}
			continue
		}

		if !shouldRetryChannel(c, lastError) {
			break
		}
		if common.RetryIntervalMs > 0 {
			retryKeepAliveSleep(c, time.Duration(common.RetryIntervalMs)*time.Millisecond)
		}
	}
	return lastError
}

// handleDetectionHit 处理响应内容检测命中，构造检测错误并决定是否允许重试
func handleDetectionHit(relayInfo *relaycommon.RelayInfo) *types.NewAPIError {
	detection := relayInfo.ChannelMeta.ChannelSetting.ResponseDetection

	// OnHit == "abort" 时，直接返回错误不重试
	if detection != nil && detection.OnHit == "abort" {
		err := types.NewError(
			fmt.Errorf("response detection hit: %s", detectionHitMessage(relayInfo.DetectionHitKeywords)),
			types.ErrorCodeResponseDetectionHit,
			types.ErrOptionWithSkipRetry(),
		)
		relayInfo.ClearDetectionHit()
		return err
	}

	// 默认 retry 行为
	err := types.NewError(
		fmt.Errorf("response detection hit: %s", detectionHitMessage(relayInfo.DetectionHitKeywords)),
		types.ErrorCodeResponseDetectionHit,
	)

	// 检查检测重试次数限制
	maxRetries := common.RetryTimes
	if detection != nil && detection.MaxRetries > 0 {
		maxRetries = detection.MaxRetries
	}
	relayInfo.DetectionRetryCount++
	if relayInfo.DetectionRetryCount > maxRetries {
		// 超过检测重试上限，标记 skipRetry
		err = types.NewError(
			fmt.Errorf("response detection hit after %d retries: %s",
				relayInfo.DetectionRetryCount, detectionHitMessage(relayInfo.DetectionHitKeywords)),
			types.ErrorCodeResponseDetectionHit,
			types.ErrOptionWithSkipRetry(),
		)
	}

	relayInfo.ClearDetectionHit()
	return err
}

// detectionHitMessage 根据命中关键词生成检测命中的描述片段
// keywords 为 nil 表示空回复命中（AllowEmpty 场景）
func detectionHitMessage(keywords []string) string {
	if keywords == nil {
		return "empty response"
	}
	return fmt.Sprintf("keywords=%v", keywords)
}

// tryFallbackChannels 遍历 sourceChannel 的 fallback 渠道列表（内层子循环）。
// 对每个 fallback 渠道调用 tryChannelOnce（不递归，fallback 渠道 429 只当作普通失败）。
// triggerError 是触发本次 fallback 的原始错误，没有可用 fallback 时原样返回。
// 返回 nil 表示某个 fallback 成功，返回非 nil 表示所有 fallback 都失败。
func tryFallbackChannels(
	c *gin.Context,
	relayInfo *relaycommon.RelayInfo,
	relayFormat types.RelayFormat,
	sourceChannel *model.Channel,
	perChannelAttempts int,
	totalAttempts *int,
	maxAttempts int,
	originalChannelId int,
	triggerError *types.NewAPIError,
) *types.NewAPIError {
	fallbackIds := sourceChannel.GetFallbackChannelIDs()
	if len(fallbackIds) == 0 {
		return triggerError // 没有 fallback 渠道，原样返回触发错误
	}

	lastError := triggerError
	for _, fbId := range fallbackIds {
		fbChannel, err := model.CacheGetChannel(fbId)
		if err != nil || fbChannel == nil {
			continue
		}
		if fbChannel.Status != common.ChannelStatusEnabled {
			continue
		}
		if !model.IsChannelEnabledForAnyGroupModel(fbChannel.GetGroups(), relayInfo.OriginModelName, fbChannel.Id) {
			continue
		}

		common.SetContextKey(c, constant.ContextKeyFallbackFromChannelId, originalChannelId)
		common.SetContextKey(c, constant.ContextKeyFallbackToChannelId, fbChannel.Id)
		logger.LogInfo(c, fmt.Sprintf("using fallback channel #%d for original channel #%d", fbChannel.Id, originalChannelId))

		// fallback 渠道走 tryChannelOnce（不递归，429 也只当作失败）
		lastError = tryChannelOnce(c, relayInfo, relayFormat, fbChannel, perChannelAttempts, totalAttempts, maxAttempts)
		if lastError == nil {
			return nil
		}
		// lastError 非 nil：继续试下一个 fallback 渠道
	}
	return lastError
}

func Relay(c *gin.Context, relayFormat types.RelayFormat) {

	requestId := c.GetString(common.RequestIdKey)
	//group := common.GetContextKeyString(c, constant.ContextKeyUsingGroup)
	//originalModel := common.GetContextKeyString(c, constant.ContextKeyOriginalModel)

	perfmetrics.ActiveTracker.OnRequestStart()
	defer perfmetrics.ActiveTracker.OnRequestEnd()

	var (
		newAPIError *types.NewAPIError
		ws          *websocket.Conn
	)

	if relayFormat == types.RelayFormatOpenAIRealtime {
		var err error
		ws, err = upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			helper.WssError(c, ws, types.NewError(err, types.ErrorCodeGetChannelFailed, types.ErrOptionWithSkipRetry()).ToOpenAIError())
			return
		}
		defer ws.Close()
	}

	defer func() {
		if newAPIError != nil {
			logger.LogError(c, fmt.Sprintf("relay error: %s", newAPIError.Error()))
			newAPIError.SetMessage(common.MessageWithRequestId(newAPIError.Error(), requestId))
			switch relayFormat {
			case types.RelayFormatOpenAIRealtime:
				helper.WssError(c, ws, newAPIError.ToOpenAIError())
			case types.RelayFormatClaude:
				c.JSON(newAPIError.StatusCode, gin.H{
					"type":  "error",
					"error": newAPIError.ToClaudeError(),
				})
			default:
				c.JSON(newAPIError.StatusCode, gin.H{
					"error": newAPIError.ToOpenAIError(),
				})
			}
		}
	}()

	request, err := helper.GetAndValidateRequest(c, relayFormat)
	if err != nil {
		// Map "request body too large" to 413 so clients can handle it correctly
		if common.IsRequestBodyTooLargeError(err) || errors.Is(err, common.ErrRequestBodyTooLarge) {
			newAPIError = types.NewErrorWithStatusCode(err, types.ErrorCodeReadRequestBodyFailed, http.StatusRequestEntityTooLarge, types.ErrOptionWithSkipRetry())
		} else {
			newAPIError = types.NewError(err, types.ErrorCodeInvalidRequest)
		}
		return
	}

	relayInfo, err := relaycommon.GenRelayInfo(c, relayFormat, request, ws)
	if err != nil {
		newAPIError = types.NewError(err, types.ErrorCodeGenRelayInfoFailed)
		return
	}

	needSensitiveCheck := setting.ShouldCheckPromptSensitive()
	needCountToken := constant.CountToken
	// Avoid building huge CombineText (strings.Join) when token counting and sensitive check are both disabled.
	var meta *types.TokenCountMeta
	if needSensitiveCheck || needCountToken {
		meta = request.GetTokenCountMeta()
	} else {
		meta = fastTokenCountMetaForPricing(request)
	}

	if needSensitiveCheck && meta != nil {
		contains, words := service.CheckSensitiveText(meta.CombineText)
		if contains {
			logger.LogWarn(c, fmt.Sprintf("user sensitive words detected: %s", strings.Join(words, ", ")))
			newAPIError = types.NewError(err, types.ErrorCodeSensitiveWordsDetected)
			return
		}
	}

	tokens, err := service.EstimateRequestToken(c, meta, relayInfo)
	if err != nil {
		newAPIError = types.NewError(err, types.ErrorCodeCountTokenFailed)
		return
	}

	relayInfo.SetEstimatePromptTokens(tokens)

	priceData, err := helper.ModelPriceHelper(c, relayInfo, tokens, meta)
	if err != nil {
		newAPIError = types.NewError(err, types.ErrorCodeModelPriceError, types.ErrOptionWithStatusCode(http.StatusBadRequest))
		return
	}

	// common.SetContextKey(c, constant.ContextKeyTokenCountMeta, meta)

	if priceData.FreeModel {
		logger.LogInfo(c, fmt.Sprintf("模型 %s 免费，跳过预扣费", relayInfo.OriginModelName))
	} else {
		newAPIError = service.PreConsumeBilling(c, priceData.QuotaToPreConsume, relayInfo)
		if newAPIError != nil {
			return
		}
	}

	defer func() {
		// Only return quota if downstream failed and quota was actually pre-consumed
		if newAPIError != nil {
			newAPIError = service.NormalizeViolationFeeError(newAPIError)
			if relayInfo.Billing != nil {
				relayInfo.Billing.Refund(c)
			}
			service.ChargeViolationFeeIfNeeded(c, relayInfo, newAPIError)
		}
	}()

	// 根据请求路径（RelayFormat）推断优先渠道类型
	preferredTypes := types.GetPreferredChannelTypesByRelayFormat(relayFormat)

	retryParam := &service.RetryParam{
		Ctx:                   c,
		TokenGroup:            relayInfo.TokenGroup,
		ModelName:             relayInfo.OriginModelName,
		Retry:                 common.GetPointer(0),
		PreferredChannelTypes: preferredTypes,
	}
	perChannelAttempts := service.CalcPerChannelAttempts(common.RetryTimes)
	maxAttempts := common.RetryTimes + 1 // 总尝试次数 = 重试次数 + 1 次初始尝试

	relayInfo.RetryIndex = 0
	relayInfo.LastError = nil

	totalAttempts := 0
	var lastError *types.NewAPIError

	// specific_channel_id 短路：token 指定了特定渠道，不走轮次循环，只尝试该渠道一次
	if _, ok := c.Get("specific_channel_id"); ok {
		autoBan := c.GetBool("auto_ban")
		autoBanInt := 1
		if !autoBan {
			autoBanInt = 0
		}
		// 指定渠道不走 SetupContextForSelectedChannel（context 已由 distributor 设置）
		addUsedChannel(c, c.GetInt("channel_id"))
		bodyStorage, bodyErr := common.GetBodyStorage(c)
		if bodyErr != nil {
			if common.IsRequestBodyTooLargeError(bodyErr) || errors.Is(bodyErr, common.ErrRequestBodyTooLarge) {
				newAPIError = types.NewErrorWithStatusCode(bodyErr, types.ErrorCodeReadRequestBodyFailed, http.StatusRequestEntityTooLarge, types.ErrOptionWithSkipRetry())
			} else {
				newAPIError = types.NewErrorWithStatusCode(bodyErr, types.ErrorCodeReadRequestBodyFailed, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
			}
			logRelayResult(c, relayInfo, newAPIError)
			return
		}
		c.Request.Body = io.NopCloser(bodyStorage)
		relayInfo.RetryIndex = 1
		lastError := executeRelayHandler(c, relayInfo, relayFormat)
		if lastError == nil {
			relayInfo.LastError = nil
		} else {
			lastError = service.NormalizeViolationFeeError(lastError)
			relayInfo.LastError = lastError
			processChannelError(c, *types.NewChannelError(
				c.GetInt("channel_id"), c.GetInt("channel_type"), c.GetString("channel_name"),
				false, common.GetContextKeyString(c, constant.ContextKeyChannelKey),
				autoBanInt == 1,
				c.GetBool(string(constant.ContextKeyChannelDisable429Ban)),
			), lastError)
			newAPIError = lastError
		}
		logRelayResult(c, relayInfo, newAPIError)
		return
	}

	// 亲和性命中分支：distributor 已选好渠道，只重试该渠道及其 fallback
	// 渠道被关闭后 fallback 也失败 → 退出亲和性锁定，fall through 到轮次循环
	affinityChannelId := common.GetContextKeyInt(c, constant.ContextKeyOriginalChannelId)
	if affinityChannelId > 0 {
		affinityChannel, affinityErr := model.CacheGetChannel(affinityChannelId)
		if affinityErr == nil && affinityChannel != nil {
			// 亲和性锁定：只重试该渠道和它的 fallback，不换其他渠道
			for totalAttempts < maxAttempts {
				if affinityChannel.Status == common.ChannelStatusEnabled {
					// 清除可能残留的 fallback 上下文
					common.SetContextKey(c, constant.ContextKeyFallbackFromChannelId, 0)
					common.SetContextKey(c, constant.ContextKeyFallbackToChannelId, 0)

					lastError = tryChannelOnce(c, relayInfo, relayFormat, affinityChannel, 1, &totalAttempts, maxAttempts)
					if lastError == nil {
						relayInfo.LastError = nil
						logRelayResult(c, relayInfo, nil)
						return
					}

					// 429/自动禁用 → 立刻尝试 fallback，fallback 全失败则退出亲和性锁定
					if isFallbackEligibleError(lastError, affinityChannel.GetDisable429Ban()) {
						fbError := tryFallbackChannels(c, relayInfo, relayFormat, affinityChannel, 1, &totalAttempts, maxAttempts, affinityChannel.Id, lastError)
						if fbError == nil {
							relayInfo.LastError = nil
							logRelayResult(c, relayInfo, nil)
							return
						}
						lastError = fbError
						break // fallback 也失败，渠道被关闭，退出亲和性锁定
					}

					// 其他错误：亲和性 SkipRetry=true 时不重试，直接退出
					if !shouldRetryChannel(c, lastError) {
						break
					}
					// 渠道仍启用，继续重试同一渠道
				} else {
					// 渠道被关闭（429/自动禁用/手动禁用），先试 fallback
					fbError := tryFallbackChannels(c, relayInfo, relayFormat, affinityChannel, 1, &totalAttempts, maxAttempts, affinityChannel.Id, lastError)
					if fbError == nil {
						relayInfo.LastError = nil
						logRelayResult(c, relayInfo, nil)
						return
					}
					lastError = fbError
					break // fallback 也失败，退出亲和性锁定
				}
			}

			// 亲和性 SkipRetry=true → 不走轮次循环，直接返回
			if service.ShouldSkipRetryAfterChannelAffinityFailure(c) {
				newAPIError = lastError
				logRelayResult(c, relayInfo, newAPIError)
				return
			}
			// 否则 fall through 到轮次循环，换其他渠道继续尝试
		}
	}

	// 亲和性不命中 / 亲和性锁定失败后 fall through：走轮次循环
	for round := 0; ; round++ {
		attemptsBeforeRound := totalAttempts
		if round > 0 {
			retryParam.ResetChannelSequence()
		}
		channelList := service.GetSortedChannelList(retryParam)
		if len(channelList) == 0 {
			break
		}

		// 中层：遍历主渠道
		for _, channel := range channelList {
			// 跳过亲和性渠道已尝试过的（避免重复）
			if affinityChannelId > 0 && channel.Id == affinityChannelId {
				continue
			}

			// 手动禁用的渠道跳过，其他状态（429限流/自动禁用等）仍尝试
			if channel.Status == common.ChannelStatusManuallyDisabled {
				continue
			}

			// 清除可能残留的 fallback 上下文
			common.SetContextKey(c, constant.ContextKeyFallbackFromChannelId, 0)
			common.SetContextKey(c, constant.ContextKeyFallbackToChannelId, 0)

			// 内层：同渠道重试，429 时内部触发 fallback
			lastError = tryChannelOnce(c, relayInfo, relayFormat, channel, perChannelAttempts, &totalAttempts, maxAttempts)
			if lastError == nil {
				relayInfo.LastError = nil
				logRelayResult(c, relayInfo, nil)
				return
			}

			if totalAttempts >= maxAttempts {
				break
			}
		}

		if totalAttempts >= maxAttempts {
			break
		}
		if totalAttempts == attemptsBeforeRound {
			break
		}
	}

	newAPIError = lastError
	logRelayResult(c, relayInfo, newAPIError)
}

var upgrader = websocket.Upgrader{
	Subprotocols: []string{"realtime"}, // WS 握手支持的协议，如果有使用 Sec-WebSocket-Protocol，则必须在此声明对应的 Protocol TODO add other protocol
	CheckOrigin: func(r *http.Request) bool {
		return true // 允许跨域
	},
}

func addUsedChannel(c *gin.Context, channelId int) {
	useChannel := c.GetStringSlice("use_channel")
	// 如果使用了后备渠道，标记为 fallback
	if fallbackFromId := common.GetContextKeyInt(c, constant.ContextKeyFallbackFromChannelId); fallbackFromId > 0 {
		useChannel = append(useChannel, fmt.Sprintf("%d(fallback_from_%d)", channelId, fallbackFromId))
	} else {
		useChannel = append(useChannel, fmt.Sprintf("%d", channelId))
	}
	c.Set("use_channel", useChannel)
	// 每次添加渠道后更新重试信息到 context，供消费日志在 relay handler 内部生成时读取
	retryCount := len(useChannel) - 1
	if retryCount > 0 {
		common.SetContextKey(c, constant.ContextKeyRetryCount, retryCount)
		startTime := common.GetContextKeyTime(c, constant.ContextKeyRequestStartTime)
		if !startTime.IsZero() {
			retryDurationMs := time.Since(startTime).Milliseconds()
			common.SetContextKey(c, constant.ContextKeyRetryDurationMs, retryDurationMs)
		}
	}
}

func fastTokenCountMetaForPricing(request dto.Request) *types.TokenCountMeta {
	if request == nil {
		return &types.TokenCountMeta{}
	}
	meta := &types.TokenCountMeta{
		TokenType: types.TokenTypeTokenizer,
	}
	switch r := request.(type) {
	case *dto.GeneralOpenAIRequest:
		maxCompletionTokens := lo.FromPtrOr(r.MaxCompletionTokens, uint(0))
		maxTokens := lo.FromPtrOr(r.MaxTokens, uint(0))
		if maxCompletionTokens > maxTokens {
			meta.MaxTokens = int(maxCompletionTokens)
		} else {
			meta.MaxTokens = int(maxTokens)
		}
	case *dto.OpenAIResponsesRequest:
		meta.MaxTokens = int(lo.FromPtrOr(r.MaxOutputTokens, uint(0)))
	case *dto.ClaudeRequest:
		meta.MaxTokens = int(lo.FromPtr(r.MaxTokens))
	case *dto.ImageRequest:
		// Pricing for image requests depends on ImagePriceRatio; safe to compute even when CountToken is disabled.
		return r.GetTokenCountMeta()
	default:
		// Best-effort: leave CombineText empty to avoid large allocations.
	}
	return meta
}

// isFallbackEligibleError 判断错误是否适合走 Fallback 渠道重试
// 429 限流和自动禁用（由 ShouldDisableChannel 判定）属于临时不可用，应优先走 Fallback
// disable429Ban=true 时，429 不触发 fallback，改为走正常重试
func isFallbackEligibleError(apiErr *types.NewAPIError, disable429Ban bool) bool {
	if apiErr == nil {
		return false
	}
	// 429 限流
	if apiErr.StatusCode == 429 && !disable429Ban {
		return true
	}
	// 自动禁用类错误（如 401、403 等）
	if service.ShouldDisableChannel(apiErr) {
		return true
	}
	return false
}

func processChannelError(c *gin.Context, channelError types.ChannelError, err *types.NewAPIError) {
	logger.LogError(c, fmt.Sprintf("channel error (channel #%d, status code: %d): %s", channelError.ChannelId, err.StatusCode, err.Error()))
	// 不要使用context获取渠道信息，异步处理时可能会出现渠道信息不一致的情况
	// do not use context to get channel info, there may be inconsistent channel info when processing asynchronously

	// 429限流处理：优先于自动禁用
	// disable_429_ban=true 时跳过 429 自动限流，改为走正常重试
	if err.StatusCode == 429 && operation_setting.RateLimit429Enabled && channelError.AutoBan && !channelError.Disable429Ban {
		gopool.Go(func() {
			service.RateLimitChannelKey429(channelError)
		})
		// 429不触发自动禁用，只触发限流
	} else if service.ShouldDisableChannel(err) && channelError.AutoBan {
		gopool.Go(func() {
			service.DisableChannel(channelError, err.ErrorWithStatusCode())
		})
	}

	// 错误日志不再在此处记录，改为在重试循环结束后统一记录一条合并的错误日志
}

// recordFinalErrorLog 在重试循环全部结束后，统一记录一条合并的错误日志。
// 该日志包含重试次数和重试耗时，替代之前每次渠道失败都记录一条错误日志的行为。
func recordFinalErrorLog(c *gin.Context, relayInfo *relaycommon.RelayInfo, err *types.NewAPIError) {
	if !constant.ErrorLogEnabled || !types.IsRecordErrorLog(err) {
		return
	}
	userId := c.GetInt("id")
	tokenName := c.GetString("token_name")
	modelName := c.GetString("original_model")
	tokenId := c.GetInt("token_id")
	userGroup := c.GetString("group")
	channelId := c.GetInt("channel_id")

	other := make(map[string]interface{})
	if c.Request != nil && c.Request.URL != nil {
		other["request_path"] = c.Request.URL.Path
	}
	other["error_type"] = err.GetErrorType()
	other["error_code"] = err.GetErrorCode()
	other["status_code"] = err.StatusCode
	other["channel_id"] = channelId
	other["channel_name"] = c.GetString("channel_name")
	other["channel_type"] = c.GetInt("channel_type")

	// 写入重试次数和重试耗时
	retryCount := common.GetContextKeyInt(c, constant.ContextKeyRetryCount)
	if retryCount > 0 {
		other["retry_count"] = retryCount
		other["retry_duration_ms"] = common.GetContextKeyInt64(c, constant.ContextKeyRetryDurationMs)
	}

	adminInfo := make(map[string]interface{})
	adminInfo["use_channel"] = c.GetStringSlice("use_channel")
	isMultiKey := common.GetContextKeyBool(c, constant.ContextKeyChannelIsMultiKey)
	if isMultiKey {
		adminInfo["is_multi_key"] = true
		adminInfo["multi_key_index"] = common.GetContextKeyInt(c, constant.ContextKeyChannelMultiKeyIndex)
	}
	service.AppendChannelAffinityAdminInfo(c, adminInfo)
	service.AppendUserAgentAdminInfo(c, adminInfo)
	// 后备渠道信息
	if fallbackFromId := common.GetContextKeyInt(c, constant.ContextKeyFallbackFromChannelId); fallbackFromId > 0 {
		adminInfo["fallback_from_channel_id"] = fallbackFromId
		adminInfo["fallback_to_channel_id"] = common.GetContextKeyInt(c, constant.ContextKeyFallbackToChannelId)
	}
	other["admin_info"] = adminInfo

	startTime := common.GetContextKeyTime(c, constant.ContextKeyRequestStartTime)
	if startTime.IsZero() {
		startTime = time.Now()
	}
	useTimeSeconds := int(time.Since(startTime).Seconds())
	model.RecordErrorLog(c, userId, channelId, modelName, tokenName, err.MaskSensitiveErrorWithStatusCode(), tokenId, useTimeSeconds, common.GetContextKeyBool(c, constant.ContextKeyIsStream), userGroup, other)
}

func RelayMidjourney(c *gin.Context) {
	relayInfo, err := relaycommon.GenRelayInfo(c, types.RelayFormatMjProxy, nil, nil)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"description": fmt.Sprintf("failed to generate relay info: %s", err.Error()),
			"type":        "upstream_error",
			"code":        4,
		})
		return
	}

	var mjErr *dto.MidjourneyResponse
	switch relayInfo.RelayMode {
	case relayconstant.RelayModeMidjourneyNotify:
		mjErr = relay.RelayMidjourneyNotify(c)
	case relayconstant.RelayModeMidjourneyTaskFetch, relayconstant.RelayModeMidjourneyTaskFetchByCondition:
		mjErr = relay.RelayMidjourneyTask(c, relayInfo.RelayMode)
	case relayconstant.RelayModeMidjourneyTaskImageSeed:
		mjErr = relay.RelayMidjourneyTaskImageSeed(c)
	case relayconstant.RelayModeSwapFace:
		mjErr = relay.RelaySwapFace(c, relayInfo)
	default:
		mjErr = relay.RelayMidjourneySubmit(c, relayInfo)
	}
	//err = relayMidjourneySubmit(c, relayMode)
	log.Println(mjErr)
	if mjErr != nil {
		statusCode := http.StatusBadRequest
		if mjErr.Code == 30 {
			mjErr.Result = "当前分组负载已饱和，请稍后再试，或升级账户以提升服务质量。"
			statusCode = http.StatusTooManyRequests
		}
		c.JSON(statusCode, gin.H{
			"description": fmt.Sprintf("%s %s", mjErr.Description, mjErr.Result),
			"type":        "upstream_error",
			"code":        mjErr.Code,
		})
		channelId := c.GetInt("channel_id")
		logger.LogError(c, fmt.Sprintf("relay error (channel #%d, status code %d): %s", channelId, statusCode, fmt.Sprintf("%s %s", mjErr.Description, mjErr.Result)))
	}
}

func RelayNotImplemented(c *gin.Context) {
	err := types.OpenAIError{
		Message: "API not implemented",
		Type:    "new_api_error",
		Param:   "",
		Code:    "api_not_implemented",
	}
	c.JSON(http.StatusNotImplemented, gin.H{
		"error": err,
	})
}

func RelayNotFound(c *gin.Context) {
	err := types.OpenAIError{
		Message: fmt.Sprintf("Invalid URL (%s %s)", c.Request.Method, c.Request.URL.Path),
		Type:    "invalid_request_error",
		Param:   "",
		Code:    "",
	}
	c.JSON(http.StatusNotFound, gin.H{
		"error": err,
	})
}

func RelayTaskFetch(c *gin.Context) {
	relayInfo, err := relaycommon.GenRelayInfo(c, types.RelayFormatTask, nil, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, &dto.TaskError{
			Code:       "gen_relay_info_failed",
			Message:    err.Error(),
			StatusCode: http.StatusInternalServerError,
		})
		return
	}
	if taskErr := relay.RelayTaskFetch(c, relayInfo.RelayMode); taskErr != nil {
		respondTaskError(c, taskErr)
	}
}

func RelayTask(c *gin.Context) {
	perfmetrics.ActiveTracker.OnRequestStart()
	defer perfmetrics.ActiveTracker.OnRequestEnd()

	relayInfo, err := relaycommon.GenRelayInfo(c, types.RelayFormatTask, nil, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, &dto.TaskError{
			Code:       "gen_relay_info_failed",
			Message:    err.Error(),
			StatusCode: http.StatusInternalServerError,
		})
		return
	}

	if taskErr := relay.ResolveOriginTask(c, relayInfo); taskErr != nil {
		respondTaskError(c, taskErr)
		return
	}

	var result *relay.TaskSubmitResult
	var taskErr *dto.TaskError
	defer func() {
		if taskErr != nil && relayInfo.Billing != nil {
			relayInfo.Billing.Refund(c)
		}
	}()

	retryParam := &service.RetryParam{
		Ctx:        c,
		TokenGroup: relayInfo.TokenGroup,
		ModelName:  relayInfo.OriginModelName,
		Retry:      common.GetPointer(0),
	}
	perChannelAttempts := service.CalcPerChannelAttempts(common.RetryTimes)
	maxAttempts := common.RetryTimes + 1

	totalAttempts := 0

	// LockedChannel 短路：distributor 锁定渠道，只尝试一次，不走轮次循环
	if lockedCh, ok := relayInfo.LockedChannel.(*model.Channel); ok && lockedCh != nil {
		if setupErr := middleware.SetupContextForSelectedChannel(c, lockedCh, relayInfo.OriginModelName); setupErr != nil {
			taskErr = service.TaskErrorWrapperLocal(setupErr.Err, "setup_locked_channel_failed", http.StatusInternalServerError)
			respondTaskError(c, taskErr)
			return
		}
		addUsedChannel(c, lockedCh.Id)
		bodyStorage, bodyErr := common.GetBodyStorage(c)
		if bodyErr != nil {
			if common.IsRequestBodyTooLargeError(bodyErr) || errors.Is(bodyErr, common.ErrRequestBodyTooLarge) {
				taskErr = service.TaskErrorWrapperLocal(bodyErr, "read_request_body_failed", http.StatusRequestEntityTooLarge)
			} else {
				taskErr = service.TaskErrorWrapperLocal(bodyErr, "read_request_body_failed", http.StatusBadRequest)
			}
			respondTaskError(c, taskErr)
			return
		}
		c.Request.Body = io.NopCloser(bodyStorage)
		relayInfo.RetryIndex = 1
		result, taskErr = relay.RelayTaskSubmit(c, relayInfo)
		if taskErr != nil && !taskErr.LocalError {
			processChannelError(c, makeChannelError(lockedCh, c),
				types.NewOpenAIError(taskErr.Error, types.ErrorCodeBadResponseStatusCode, taskErr.StatusCode))
		}
		// 跳转到成功/失败处理
		goto taskResult
	}

	// 外层：轮次循环
	for round := 0; ; round++ {
		attemptsBeforeRound := totalAttempts
		if round > 0 {
			retryParam.ResetChannelSequence()
		}
		channelList := service.GetSortedChannelList(retryParam)
		if len(channelList) == 0 {
			taskErr = service.TaskErrorWrapperLocal(fmt.Errorf("no available channel"), "get_channel_failed", http.StatusInternalServerError)
			break
		}

		// 中层：遍历主渠道
		for _, channel := range channelList {
			if channel.Status != common.ChannelStatusEnabled {
				continue
			}

			// 内层：同渠道重试
			for attempt := 0; attempt < perChannelAttempts; attempt++ {
				if totalAttempts >= maxAttempts {
					break
				}
				totalAttempts++
				relayInfo.RetryIndex = totalAttempts

				if setupErr := middleware.SetupContextForSelectedChannel(c, channel, relayInfo.OriginModelName); setupErr != nil {
					taskErr = service.TaskErrorWrapperLocal(setupErr.Err, "setup_channel_failed", http.StatusInternalServerError)
					break
				}

				addUsedChannel(c, channel.Id)
				bodyStorage, bodyErr := common.GetBodyStorage(c)
				if bodyErr != nil {
					if common.IsRequestBodyTooLargeError(bodyErr) || errors.Is(bodyErr, common.ErrRequestBodyTooLarge) {
						taskErr = service.TaskErrorWrapperLocal(bodyErr, "read_request_body_failed", http.StatusRequestEntityTooLarge)
					} else {
						taskErr = service.TaskErrorWrapperLocal(bodyErr, "read_request_body_failed", http.StatusBadRequest)
					}
					break
				}
				c.Request.Body = io.NopCloser(bodyStorage)

				result, taskErr = relay.RelayTaskSubmit(c, relayInfo)
				if taskErr == nil {
					break
				}

				if !taskErr.LocalError {
					processChannelError(c, makeChannelError(channel, c),
						types.NewOpenAIError(taskErr.Error, types.ErrorCodeBadResponseStatusCode, taskErr.StatusCode))
				}

				if !shouldRetryTaskRelay(c, channel.Id, taskErr, maxAttempts-totalAttempts) {
					break
				}
				if common.RetryIntervalMs > 0 {
					retryKeepAliveSleep(c, time.Duration(common.RetryIntervalMs)*time.Millisecond)
				}
			}

			if taskErr == nil {
				break
			}
			if totalAttempts >= maxAttempts {
				break
			}
		}

		if taskErr == nil {
			break
		}
		if totalAttempts >= maxAttempts {
			break
		}
		if totalAttempts == attemptsBeforeRound {
			break
		}
	}

taskResult:
	useChannel := c.GetStringSlice("use_channel")
	if len(useChannel) > 1 {
		retryLogStr := fmt.Sprintf("重试：%s", strings.Trim(strings.Join(strings.Fields(fmt.Sprint(useChannel)), "->"), "[]"))
		logger.LogInfo(c, retryLogStr)
	}

	// ── 成功：结算 + 日志 + 插入任务 ──
	if taskErr == nil {
		if settleErr := service.SettleBilling(c, relayInfo, result.Quota); settleErr != nil {
			common.SysError("settle task billing error: " + settleErr.Error())
		}
		service.LogTaskConsumption(c, relayInfo)

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
		if insertErr := task.Insert(); insertErr != nil {
			common.SysError("insert task error: " + insertErr.Error())
		}
	}

	if taskErr != nil {
		respondTaskError(c, taskErr)
	}
}

// respondTaskError 统一输出 Task 错误响应（含 429 限流提示改写）
func respondTaskError(c *gin.Context, taskErr *dto.TaskError) {
	if taskErr.StatusCode == http.StatusTooManyRequests {
		taskErr.Message = "当前分组上游负载已饱和，请稍后再试"
	}
	c.JSON(taskErr.StatusCode, taskErr)
}

func shouldRetryTaskRelay(c *gin.Context, channelId int, taskErr *dto.TaskError, retryTimes int) bool {
	if taskErr == nil {
		return false
	}
	if service.ShouldSkipRetryAfterChannelAffinityFailure(c) {
		return false
	}
	if retryTimes <= 0 {
		return false
	}
	if _, ok := c.Get("specific_channel_id"); ok {
		return false
	}
	if taskErr.StatusCode == http.StatusTooManyRequests {
		return true
	}
	if taskErr.StatusCode == 307 {
		return true
	}
	if taskErr.StatusCode/100 == 5 {
		// 超时不重试
		if operation_setting.IsAlwaysSkipRetryStatusCode(taskErr.StatusCode) {
			return false
		}
		return true
	}
	if taskErr.StatusCode == http.StatusBadRequest {
		return false
	}
	if taskErr.StatusCode == 408 {
		// azure处理超时不重试
		return false
	}
	if taskErr.LocalError {
		return false
	}
	if taskErr.StatusCode/100 == 2 {
		return false
	}
	return true
}

// retryKeepAliveSleep 在重试等待期间定期发送 SSE 保活信息。
// 仅在流式请求（SSE 响应头已设置）时发送保活，非流式请求退化为普通 Sleep。
// 保活间隔复用全局 PingIntervalSeconds 配置，若未配置则默认 5 秒。
func retryKeepAliveSleep(c *gin.Context, totalWait time.Duration) {
	// 非流式请求直接 Sleep
	if _, set := c.Get("event_stream_headers_set"); !set {
		time.Sleep(totalWait)
		return
	}

	// 确定保活间隔
	generalSettings := operation_setting.GetGeneralSetting()
	pingInterval := time.Duration(generalSettings.PingIntervalSeconds) * time.Second
	if pingInterval <= 0 {
		pingInterval = 5 * time.Second
	}
	if pingInterval > totalWait {
		pingInterval = totalWait
	}

	deadline := time.Now().Add(totalWait)
	ticker := time.NewTicker(pingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if time.Now().After(deadline) {
				return
			}
			if err := helper.RetryKeepAlive(c); err != nil {
				logger.LogDebug(c, "retry keepalive send failed: %s", err.Error())
				return
			}
		case <-c.Request.Context().Done():
			return
		}
	}
}
