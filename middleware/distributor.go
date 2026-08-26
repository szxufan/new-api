package middleware

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

type ModelRequest struct {
	Model string `json:"model"`
	Group string `json:"group,omitempty"`
}

func Distribute() func(c *gin.Context) {
	return func(c *gin.Context) {
		var channel *model.Channel
		channelId, ok := common.GetContextKey(c, constant.ContextKeyTokenSpecificChannelId)
		modelRequest, shouldSelectChannel, err := getModelRequest(c)
		if err != nil {
			abortWithOpenAiMessage(c, http.StatusBadRequest, i18n.T(c, i18n.MsgDistributorInvalidRequest, map[string]any{"Error": err.Error()}))
			return
		}

		if modelRequest.Model != "" && shouldSelectChannel {
			if fallbackModel := model.GetModelFallbackModel(modelRequest.Model); fallbackModel != "" {
				applied, fallbackErr := applyMultimodalFallback(c, fallbackModel)
				if fallbackErr == nil && applied {
					originalModel := modelRequest.Model
					modelRequest.Model = fallbackModel
					common.SetContextKey(c, constant.ContextKeyOriginalModelBeforeFallback, originalModel)
					common.SetContextKey(c, constant.ContextKeyFallbackModel, fallbackModel)
					logger.LogInfo(c, fmt.Sprintf("model %s downgraded to %s due to multimodal content",
						originalModel, fallbackModel))
				}
			}
		}

		if ok {
			id, err := strconv.Atoi(channelId.(string))
			if err != nil {
				abortWithOpenAiMessage(c, http.StatusBadRequest, i18n.T(c, i18n.MsgDistributorInvalidChannelId))
				return
			}
			channel, err = model.GetChannelById(id, true)
			if err != nil {
				abortWithOpenAiMessage(c, http.StatusBadRequest, i18n.T(c, i18n.MsgDistributorInvalidChannelId))
				return
			}
			if channel.Status != common.ChannelStatusEnabled {
				abortWithOpenAiMessage(c, http.StatusForbidden, i18n.T(c, i18n.MsgDistributorChannelDisabled))
				return
			}
			// 渠道分组黑名单：命中用户自身分组时禁止使用（含 token 指定渠道）
			userGroup := common.GetContextKeyString(c, constant.ContextKeyUserGroup)
			if channel.IsUserGroupBlacklisted(userGroup) {
				abortWithOpenAiMessage(c, http.StatusForbidden, i18n.T(c, i18n.MsgDistributorGroupBlacklisted))
				return
			}
		} else {
			// Select a channel for the user
			// check token model mapping
			modelLimitEnable := common.GetContextKeyBool(c, constant.ContextKeyTokenModelLimitEnabled)
			if modelLimitEnable {
				s, ok := common.GetContextKey(c, constant.ContextKeyTokenModelLimit)
				if !ok {
					// token model limit is empty, all models are not allowed
					abortWithOpenAiMessage(c, http.StatusForbidden, i18n.T(c, i18n.MsgDistributorTokenNoModelAccess))
					return
				}
				var tokenModelLimit map[string]bool
				tokenModelLimit, ok = s.(map[string]bool)
				if !ok {
					tokenModelLimit = map[string]bool{}
				}
				matchName := ratio_setting.FormatMatchingModelName(modelRequest.Model) // match gpts & thinking-*
				if _, ok := tokenModelLimit[matchName]; !ok {
					abortWithOpenAiMessage(c, http.StatusForbidden, i18n.T(c, i18n.MsgDistributorTokenModelForbidden, map[string]any{"Model": modelRequest.Model}))
					return
				}
			}

			// 虚拟模型：命中则跳过单渠道选择，由执行层并发调度多个真实模型
			if shouldSelectChannel && modelRequest.Model != "" {
				if vm := model.GetVirtualModel(modelRequest.Model); vm != nil {
					common.SetContextKey(c, constant.ContextKeyVirtualModelId, vm.Id)
					common.SetContextKey(c, constant.ContextKeyOriginalModel, modelRequest.Model)
					common.SetContextKey(c, constant.ContextKeyRequestStartTime, time.Now())
					c.Next()
					return
				}
			}

			if shouldSelectChannel {
				if modelRequest.Model == "" {
					abortWithOpenAiMessage(c, http.StatusBadRequest, i18n.T(c, i18n.MsgDistributorModelNameRequired))
					return
				}
				var selectGroup string
				usingGroup := common.GetContextKeyString(c, constant.ContextKeyUsingGroup)
				// check path is /pg/chat/completions
				if strings.HasPrefix(c.Request.URL.Path, "/pg/chat/completions") {
					playgroundRequest := &dto.PlayGroundRequest{}
					err = common.UnmarshalBodyReusable(c, playgroundRequest)
					if err != nil {
						abortWithOpenAiMessage(c, http.StatusBadRequest, i18n.T(c, i18n.MsgDistributorInvalidPlayground, map[string]any{"Error": err.Error()}))
						return
					}
					if playgroundRequest.Group != "" {
						if !service.GroupInUserUsableGroups(usingGroup, playgroundRequest.Group) && playgroundRequest.Group != usingGroup {
							abortWithOpenAiMessage(c, http.StatusForbidden, i18n.T(c, i18n.MsgDistributorGroupAccessDenied))
							return
						}
						usingGroup = playgroundRequest.Group
						common.SetContextKey(c, constant.ContextKeyUsingGroup, usingGroup)
					}
				}

				userGroup := common.GetContextKeyString(c, constant.ContextKeyUserGroup)
				if preferredChannelID, found := service.GetPreferredChannelByAffinity(c, modelRequest.Model, usingGroup); found {
					preferred, err := model.CacheGetChannel(preferredChannelID)
					// 渠道分组黑名单：亲和性命中的渠道若拉黑用户自身分组，视为未命中，走正常选择
					if err == nil && preferred != nil && !preferred.IsUserGroupBlacklisted(userGroup) {
						if preferred.Status != common.ChannelStatusEnabled {
							// 亲和性渠道不可用，区分原因处理
							if preferred.Status == common.ChannelStatusRateLimited429 || preferred.Status == common.ChannelStatusAutoDisabled {
								// 429 限流或自动禁用：尝试后备渠道
								fallbackChannel, fallbackErr := model.CacheGetFallbackChannel(preferred.GetFallbackChannelIDs(), modelRequest.Model)
								// 渠道分组黑名单：命中用户自身分组的后备渠道不采用，走原有逻辑
								if fallbackErr == nil && fallbackChannel != nil && !fallbackChannel.IsUserGroupBlacklisted(userGroup) {
									// 使用后备渠道，但亲和性仍记录为原渠道
									channel = fallbackChannel
									selectGroup = usingGroup
									service.MarkChannelAffinityUsed(c, usingGroup, preferred.Id)
									common.SetContextKey(c, constant.ContextKeyFallbackFromChannelId, preferred.Id)
									common.SetContextKey(c, constant.ContextKeyFallbackToChannelId, fallbackChannel.Id)
									common.SetContextKey(c, constant.ContextKeyOriginalChannelId, preferred.Id)
								} else {
									// 后备渠道也不可用，走原有逻辑
									if service.ShouldSkipRetryAfterChannelAffinityFailure(c) {
										abortWithOpenAiMessage(c, http.StatusForbidden, i18n.T(c, i18n.MsgDistributorAffinityChannelDisabled))
										return
									}
								}
							} else {
								// 其他非启用状态（手动禁用等），走原有逻辑
								if service.ShouldSkipRetryAfterChannelAffinityFailure(c) {
									abortWithOpenAiMessage(c, http.StatusForbidden, i18n.T(c, i18n.MsgDistributorAffinityChannelDisabled))
									return
								}
							}
						} else if usingGroup == "auto" {
							autoGroups := service.GetUserAutoGroup(userGroup)
							for _, g := range autoGroups {
								if model.IsChannelEnabledForGroupModel(g, modelRequest.Model, preferred.Id) {
									selectGroup = g
									common.SetContextKey(c, constant.ContextKeyAutoGroup, g)
									channel = preferred
									service.MarkChannelAffinityUsed(c, g, preferred.Id)
									break
								}
							}
						} else if model.IsChannelEnabledForGroupModel(usingGroup, modelRequest.Model, preferred.Id) {
							channel = preferred
							selectGroup = usingGroup
							service.MarkChannelAffinityUsed(c, usingGroup, preferred.Id)
						}
					}
				}

				if channel == nil {
					channel, selectGroup, err = service.CacheGetRandomSatisfiedChannel(&service.RetryParam{
						Ctx:        c,
						ModelName:  modelRequest.Model,
						TokenGroup: usingGroup,
						Retry:      common.GetPointer(0),
					})
					if err != nil {
						showGroup := usingGroup
						if usingGroup == "auto" {
							showGroup = fmt.Sprintf("auto(%s)", selectGroup)
						}
						message := i18n.T(c, i18n.MsgDistributorGetChannelFailed, map[string]any{"Group": showGroup, "Model": modelRequest.Model, "Error": err.Error()})
						abortWithOpenAiMessage(c, http.StatusServiceUnavailable, message, types.ErrorCodeModelNotFound)
						return
					}
					if channel == nil {
						// 常规渠道选择找不到可用渠道，尝试从被限流/禁用的渠道中寻找后备渠道
						if usingGroup == "auto" {
							// auto 模式下，遍历所有 auto 分组
							autoGroups := service.GetUserAutoGroup(userGroup)
							for _, g := range autoGroups {
								fallbackCh, originalId, fallbackErr := model.CacheGetDisabledChannelsWithFallback(g, modelRequest.Model)
								// 渠道分组黑名单：命中用户自身分组的后备渠道跳过，继续尝试下一分组
								if fallbackErr == nil && fallbackCh != nil && !fallbackCh.IsUserGroupBlacklisted(userGroup) {
									channel = fallbackCh
									selectGroup = g
									common.SetContextKey(c, constant.ContextKeyAutoGroup, g)
									common.SetContextKey(c, constant.ContextKeyFallbackFromChannelId, originalId)
									common.SetContextKey(c, constant.ContextKeyFallbackToChannelId, fallbackCh.Id)
									common.SetContextKey(c, constant.ContextKeyOriginalChannelId, originalId)
									break
								}
							}
						} else {
							fallbackCh, originalId, fallbackErr := model.CacheGetDisabledChannelsWithFallback(usingGroup, modelRequest.Model)
							// 渠道分组黑名单：命中用户自身分组的后备渠道不采用
							if fallbackErr == nil && fallbackCh != nil && !fallbackCh.IsUserGroupBlacklisted(userGroup) {
								channel = fallbackCh
								selectGroup = usingGroup
								common.SetContextKey(c, constant.ContextKeyFallbackFromChannelId, originalId)
								common.SetContextKey(c, constant.ContextKeyFallbackToChannelId, fallbackCh.Id)
								common.SetContextKey(c, constant.ContextKeyOriginalChannelId, originalId)
							}
						}
						if channel == nil {
							logger.LogWarn(c.Request.Context(), fmt.Sprintf("[no_channel] group=%s model=%s cache_enabled=%t path=%s",
								usingGroup, modelRequest.Model, common.MemoryCacheEnabled, c.Request.URL.Path))
							abortWithOpenAiMessage(c, http.StatusServiceUnavailable, i18n.T(c, i18n.MsgDistributorNoAvailableChannel, map[string]any{"Group": usingGroup, "Model": modelRequest.Model}), types.ErrorCodeModelNotFound)
							return
						}
					}
				}
			}
		}
		common.SetContextKey(c, constant.ContextKeyRequestStartTime, time.Now())
		SetupContextForSelectedChannel(c, channel, modelRequest.Model)
		// 记录首次选中的渠道 ID（供重试路径使用）
		if channel != nil {
			if originalId := common.GetContextKeyInt(c, constant.ContextKeyOriginalChannelId); originalId <= 0 {
				common.SetContextKey(c, constant.ContextKeyOriginalChannelId, channel.Id)
			}
		}
		c.Next()
		if channel != nil && c.Writer != nil && c.Writer.Status() < http.StatusBadRequest {
			// 如果使用了后备渠道，亲和性记录为原渠道
			if fallbackFromId := common.GetContextKeyInt(c, constant.ContextKeyFallbackFromChannelId); fallbackFromId > 0 {
				service.RecordChannelAffinity(c, fallbackFromId)
			} else {
				service.RecordChannelAffinity(c, channel.Id)
			}
		}
	}
}

// getModelFromRequest 从请求中读取模型信息
// 根据 Content-Type 自动处理：
// - application/json
// - application/x-www-form-urlencoded
// - multipart/form-data
func getModelFromRequest(c *gin.Context) (*ModelRequest, error) {
	if strings.HasPrefix(c.Request.Header.Get("Content-Type"), "application/json") {
		modelRequest, err := getModelFromJSONBody(c)
		if err != nil {
			return nil, errors.New(i18n.T(c, i18n.MsgDistributorInvalidRequest, map[string]any{"Error": err.Error()}))
		}
		return modelRequest, nil
	}

	var modelRequest ModelRequest
	err := common.UnmarshalBodyReusable(c, &modelRequest)
	if err != nil {
		return nil, errors.New(i18n.T(c, i18n.MsgDistributorInvalidRequest, map[string]any{"Error": err.Error()}))
	}
	return &modelRequest, nil
}

func getModelFromJSONBody(c *gin.Context) (*ModelRequest, error) {
	storage, err := common.GetBodyStorage(c)
	if err != nil {
		return nil, err
	}
	requestBody, err := storage.Bytes()
	if err != nil {
		return nil, err
	}
	if !gjson.ValidBytes(requestBody) {
		return nil, errors.New("invalid JSON request body")
	}

	values := gjson.GetManyBytes(requestBody, "model", "group")
	model, err := getJSONStringValue(values[0], "model")
	if err != nil {
		return nil, err
	}
	group, err := getJSONStringValue(values[1], "group")
	if err != nil {
		return nil, err
	}

	if _, seekErr := storage.Seek(0, io.SeekStart); seekErr != nil {
		return nil, seekErr
	}
	c.Request.Body = io.NopCloser(storage)

	return &ModelRequest{
		Model: model,
		Group: group,
	}, nil
}

func getJSONStringValue(result gjson.Result, field string) (string, error) {
	if !result.Exists() || result.Type == gjson.Null {
		return "", nil
	}
	if result.Type != gjson.String {
		return "", fmt.Errorf("field %s must be a string", field)
	}
	return result.String(), nil
}

// resolveVideoRelayMode 解析视频任务相关路径对应的 relay mode：
// - POST → RelayModeVideoSubmit（创建异步任务）
// - GET → RelayModeVideoFetchByID（查询任务）
// remix（/v1/videos/{id}/remix）也属于提交动作 → RelayModeVideoSubmit。
// 非视频任务路径返回 RelayModeUnknown。
func resolveVideoRelayMode(method, path string) int {
	if strings.Contains(path, "/v1/videos/") && strings.HasSuffix(path, "/remix") {
		return relayconstant.RelayModeVideoSubmit
	}
	if strings.Contains(path, "/v1/videos") || strings.Contains(path, "/pg/videos") || strings.Contains(path, "/v1/video/generations") {
		if method == http.MethodPost {
			return relayconstant.RelayModeVideoSubmit
		}
		if method == http.MethodGet {
			return relayconstant.RelayModeVideoFetchByID
		}
	}
	return relayconstant.RelayModeUnknown
}

func getModelRequest(c *gin.Context) (*ModelRequest, bool, error) {
	var modelRequest ModelRequest
	shouldSelectChannel := true
	var err error
	if strings.Contains(c.Request.URL.Path, "/mj/") {
		relayMode := relayconstant.Path2RelayModeMidjourney(c.Request.URL.Path)
		if relayMode == relayconstant.RelayModeMidjourneyTaskFetch ||
			relayMode == relayconstant.RelayModeMidjourneyTaskFetchByCondition ||
			relayMode == relayconstant.RelayModeMidjourneyNotify ||
			relayMode == relayconstant.RelayModeMidjourneyTaskImageSeed {
			shouldSelectChannel = false
		} else {
			midjourneyRequest := dto.MidjourneyRequest{}
			err = common.UnmarshalBodyReusable(c, &midjourneyRequest)
			if err != nil {
				return nil, false, errors.New(i18n.T(c, i18n.MsgDistributorInvalidMidjourney, map[string]any{"Error": err.Error()}))
			}
			midjourneyModel, mjErr, success := service.GetMjRequestModel(relayMode, &midjourneyRequest)
			if mjErr != nil {
				return nil, false, fmt.Errorf("%s", mjErr.Description)
			}
			if midjourneyModel == "" {
				if !success {
					return nil, false, fmt.Errorf("%s", i18n.T(c, i18n.MsgDistributorInvalidParseModel))
				} else {
					// task fetch, task fetch by condition, notify
					shouldSelectChannel = false
				}
			}
			modelRequest.Model = midjourneyModel
		}
		c.Set("relay_mode", relayMode)
	} else if strings.Contains(c.Request.URL.Path, "/suno/") {
		relayMode := relayconstant.Path2RelaySuno(c.Request.Method, c.Request.URL.Path)
		if relayMode == relayconstant.RelayModeSunoFetch ||
			relayMode == relayconstant.RelayModeSunoFetchByID {
			shouldSelectChannel = false
		} else {
			modelName := service.CoverTaskActionToModelName(constant.TaskPlatformSuno, c.Param("action"))
			modelRequest.Model = modelName
		}
		c.Set("platform", string(constant.TaskPlatformSuno))
		c.Set("relay_mode", relayMode)
	} else if relayMode := resolveVideoRelayMode(c.Request.Method, c.Request.URL.Path); relayMode != relayconstant.RelayModeUnknown {
		// 视频任务（/v1/videos、/pg/videos、/v1/video/generations）：
		// - POST → RelayModeVideoSubmit（创建异步任务）
		// - GET → RelayModeVideoFetchByID（查询任务，从原任务推导模型并锁定渠道）
		if relayMode == relayconstant.RelayModeVideoSubmit && !strings.Contains(c.Request.URL.Path, "/remix") {
			// curl https://api.openai.com/v1/videos \
			//   -H "Authorization: Bearer $OPENAI_API_KEY" \
			//   -F "model=sora-2" \
			//   -F "prompt=A calico cat playing a piano on stage"
			//   -F input_reference="@image.jpg"
			req, err := getModelFromRequest(c)
			if err != nil {
				return nil, false, err
			}
			if req != nil {
				modelRequest.Model = req.Model
			}
		} else {
			// remix（锁定原任务渠道）或任务查询：不需要选择渠道
			shouldSelectChannel = false
			if relayMode == relayconstant.RelayModeVideoFetchByID {
				modelRequest.Model = getTaskOriginModelName(c)
			}
		}
		c.Set("relay_mode", relayMode)
	} else if strings.HasPrefix(c.Request.URL.Path, "/v1beta/models/") || strings.HasPrefix(c.Request.URL.Path, "/v1/models/") {
		// Gemini API 路径处理: /v1beta/models/gemini-2.0-flash:generateContent
		relayMode := relayconstant.RelayModeGemini
		modelName := extractModelNameFromGeminiPath(c.Request.URL.Path)
		if modelName != "" {
			modelRequest.Model = modelName
		}
		c.Set("relay_mode", relayMode)
	} else if !strings.HasPrefix(c.Request.URL.Path, "/v1/audio/transcriptions") && !strings.Contains(c.Request.Header.Get("Content-Type"), "multipart/form-data") {
		req, err := getModelFromRequest(c)
		if err != nil {
			return nil, false, err
		}
		modelRequest.Model = req.Model
	}
	if strings.HasPrefix(c.Request.URL.Path, "/v1/realtime") {
		//wss://api.openai.com/v1/realtime?model=gpt-4o-realtime-preview-2024-10-01
		modelRequest.Model = c.Query("model")
	}
	if strings.HasPrefix(c.Request.URL.Path, "/v1/moderations") {
		if modelRequest.Model == "" {
			modelRequest.Model = "text-moderation-stable"
		}
	}
	if strings.HasSuffix(c.Request.URL.Path, "embeddings") {
		if modelRequest.Model == "" {
			modelRequest.Model = c.Param("model")
		}
	}
	if strings.HasPrefix(c.Request.URL.Path, "/v1/images/generations") || strings.HasPrefix(c.Request.URL.Path, "/pg/images/generations") {
		modelRequest.Model = common.GetStringIfEmpty(modelRequest.Model, "dall-e")
	} else if strings.HasPrefix(c.Request.URL.Path, "/v1/images/edits") || strings.HasPrefix(c.Request.URL.Path, "/pg/images/edits") {
		//modelRequest.Model = common.GetStringIfEmpty(c.PostForm("model"), "gpt-image-1")
		contentType := c.ContentType()
		if slices.Contains([]string{gin.MIMEPOSTForm, gin.MIMEMultipartPOSTForm}, contentType) {
			req, err := getModelFromRequest(c)
			if err == nil && req.Model != "" {
				modelRequest.Model = req.Model
			}
		}
	}
	if strings.HasPrefix(c.Request.URL.Path, "/v1/audio") {
		relayMode := relayconstant.RelayModeAudioSpeech
		if strings.HasPrefix(c.Request.URL.Path, "/v1/audio/speech") {

			modelRequest.Model = common.GetStringIfEmpty(modelRequest.Model, "tts-1")
		} else if strings.HasPrefix(c.Request.URL.Path, "/v1/audio/translations") {
			// 先尝试从请求读取
			if req, err := getModelFromRequest(c); err == nil && req.Model != "" {
				modelRequest.Model = req.Model
			}
			modelRequest.Model = common.GetStringIfEmpty(modelRequest.Model, "whisper-1")
			relayMode = relayconstant.RelayModeAudioTranslation
		} else if strings.HasPrefix(c.Request.URL.Path, "/v1/audio/transcriptions") {
			// 先尝试从请求读取
			if req, err := getModelFromRequest(c); err == nil && req.Model != "" {
				modelRequest.Model = req.Model
			}
			modelRequest.Model = common.GetStringIfEmpty(modelRequest.Model, "whisper-1")
			relayMode = relayconstant.RelayModeAudioTranscription
		}
		c.Set("relay_mode", relayMode)
	}
	if strings.HasPrefix(c.Request.URL.Path, "/pg/chat/completions") {
		// playground chat completions
		req, err := getModelFromRequest(c)
		if err != nil {
			return nil, false, err
		}
		modelRequest.Model = req.Model
		modelRequest.Group = req.Group
		common.SetContextKey(c, constant.ContextKeyTokenGroup, modelRequest.Group)
	}

	if strings.HasPrefix(c.Request.URL.Path, "/v1/responses/compact") && modelRequest.Model != "" {
		modelRequest.Model = ratio_setting.WithCompactModelSuffix(modelRequest.Model)
	}
	return &modelRequest, shouldSelectChannel, nil
}

// 修复 #4834: GET /v1/video/generations/:task_id && /v1/video/:task_id 此前不解析 model，
// 当 token 启用「可用模型限制」时，下游 modelLimitEnable 校验会因
// modelRequest.Model 为空而误报 "This token has no access to model"。
// 从已存储的任务记录中回填 OriginModelName 即可让校验走在正确的模型上。
func getTaskOriginModelName(c *gin.Context) string {
	if !common.GetContextKeyBool(c, constant.ContextKeyTokenModelLimitEnabled) {
		return ""
	}

	taskId := c.Param("task_id")
	if taskId == "" {
		// jimeng adapter
		taskId = c.GetString("task_id")
	}
	if taskId == "" {
		return ""
	}

	userId := c.GetInt("id")
	if task, exist, err := model.GetByTaskId(userId, taskId); err == nil && exist && task != nil {
		return task.Properties.OriginModelName
	}
	return ""
}

func SetupContextForSelectedChannel(c *gin.Context, channel *model.Channel, modelName string) *types.NewAPIError {
	c.Set("original_model", modelName) // for retry
	if channel == nil {
		return types.NewError(errors.New("channel is nil"), types.ErrorCodeGetChannelFailed, types.ErrOptionWithSkipRetry())
	}
	common.SetContextKey(c, constant.ContextKeyChannelId, channel.Id)
	common.SetContextKey(c, constant.ContextKeyChannelName, channel.Name)
	common.SetContextKey(c, constant.ContextKeyChannelType, channel.Type)
	common.SetContextKey(c, constant.ContextKeyChannelCreateTime, channel.CreatedTime)
	common.SetContextKey(c, constant.ContextKeyChannelSetting, channel.GetSetting())
	common.SetContextKey(c, constant.ContextKeyChannelOtherSetting, channel.GetOtherSettings())
	paramOverride := channel.GetParamOverride()
	headerOverride := channel.GetHeaderOverride()
	if mergedParam, applied := service.ApplyChannelAffinityOverrideTemplate(c, paramOverride); applied {
		paramOverride = mergedParam
	}
	common.SetContextKey(c, constant.ContextKeyChannelParamOverride, paramOverride)
	common.SetContextKey(c, constant.ContextKeyChannelHeaderOverride, headerOverride)
	if nil != channel.OpenAIOrganization && *channel.OpenAIOrganization != "" {
		common.SetContextKey(c, constant.ContextKeyChannelOrganization, *channel.OpenAIOrganization)
	}
	common.SetContextKey(c, constant.ContextKeyChannelAutoBan, channel.GetAutoBan())
	common.SetContextKey(c, constant.ContextKeyChannelDisable429Ban, channel.GetDisable429Ban())
	common.SetContextKey(c, constant.ContextKeyChannelModelMapping, channel.GetModelMapping())
	common.SetContextKey(c, constant.ContextKeyChannelStatusCodeMapping, channel.GetStatusCodeMapping())

	key, index, newAPIError := channel.GetNextEnabledKey()
	if newAPIError != nil {
		return newAPIError
	}
	if channel.ChannelInfo.IsMultiKey {
		common.SetContextKey(c, constant.ContextKeyChannelIsMultiKey, true)
		common.SetContextKey(c, constant.ContextKeyChannelMultiKeyIndex, index)
	} else {
		// 必须设置为 false，否则在重试到单个 key 的时候会导致日志显示错误
		common.SetContextKey(c, constant.ContextKeyChannelIsMultiKey, false)
	}
	// c.Request.Header.Set("Authorization", fmt.Sprintf("Bearer %s", key))
	common.SetContextKey(c, constant.ContextKeyChannelKey, key)
	common.SetContextKey(c, constant.ContextKeyChannelBaseUrl, channel.GetBaseURL())

	common.SetContextKey(c, constant.ContextKeySystemPromptOverride, false)

	// TODO: api_version统一
	switch channel.Type {
	case constant.ChannelTypeAzure:
		c.Set("api_version", channel.Other)
	case constant.ChannelTypeVertexAi:
		c.Set("region", channel.Other)
	case constant.ChannelTypeXunfei:
		c.Set("api_version", channel.Other)
	case constant.ChannelTypeGemini:
		c.Set("api_version", channel.Other)
	case constant.ChannelTypeAli:
		c.Set("plugin", channel.Other)
	case constant.ChannelCloudflare:
		c.Set("api_version", channel.Other)
	case constant.ChannelTypeMokaAI:
		c.Set("api_version", channel.Other)
	case constant.ChannelTypeCoze:
		c.Set("bot_id", channel.Other)
	}
	return nil
}

// extractModelNameFromGeminiPath 从 Gemini API URL 路径中提取模型名
// 输入格式: /v1beta/models/gemini-2.0-flash:generateContent
// 输出: gemini-2.0-flash
func extractModelNameFromGeminiPath(path string) string {
	// 查找 "/models/" 的位置
	modelsPrefix := "/models/"
	modelsIndex := strings.Index(path, modelsPrefix)
	if modelsIndex == -1 {
		return ""
	}

	// 从 "/models/" 之后开始提取
	startIndex := modelsIndex + len(modelsPrefix)
	if startIndex >= len(path) {
		return ""
	}

	// 查找 ":" 的位置，模型名在 ":" 之前
	colonIndex := strings.Index(path[startIndex:], ":")
	if colonIndex == -1 {
		// 如果没有找到 ":"，返回从 "/models/" 到路径结尾的部分
		return path[startIndex:]
	}

	// 返回模型名部分
	return path[startIndex : startIndex+colonIndex]
}

func hasMultimodalContentInBytes(body []byte) bool {
	messages := gjson.GetBytes(body, "messages")
	if messages.IsArray() {
		for _, msg := range messages.Array() {
			content := msg.Get("content")
			if content.IsArray() {
				for _, item := range content.Array() {
					contentType := item.Get("type").String()
					switch contentType {
					case "image_url", "video_url", "input_audio", "file", "image":
						return true
					}
				}
			}
		}
	}

	input := gjson.GetBytes(body, "input")
	if input.IsArray() {
		for _, item := range input.Array() {
			itemType := item.Get("type").String()
			if itemType == "input_image" || itemType == "input_file" || itemType == "input_video" {
				return true
			}
			content := item.Get("content")
			if content.IsArray() {
				for _, sub := range content.Array() {
					subType := sub.Get("type").String()
					if subType == "input_image" || subType == "input_file" || subType == "input_video" {
						return true
					}
				}
			}
		}
	}

	return false
}

func applyMultimodalFallback(c *gin.Context, fallbackModel string) (bool, error) {
	storage, err := common.GetBodyStorage(c)
	if err != nil {
		return false, err
	}
	body, err := storage.Bytes()
	if err != nil {
		return false, err
	}

	if !hasMultimodalContentInBytes(body) {
		storage.Seek(0, io.SeekStart)
		c.Request.Body = io.NopCloser(storage)
		return false, nil
	}

	modified, err := sjson.SetBytes(body, "model", fallbackModel)
	if err != nil {
		storage.Seek(0, io.SeekStart)
		c.Request.Body = io.NopCloser(storage)
		return false, err
	}

	newStorage, err := common.CreateBodyStorage(modified)
	if err != nil {
		storage.Seek(0, io.SeekStart)
		c.Request.Body = io.NopCloser(storage)
		return false, err
	}

	storage.Close()
	c.Set(common.KeyBodyStorage, newStorage)
	c.Request.Body = io.NopCloser(newStorage)
	return true, nil
}
