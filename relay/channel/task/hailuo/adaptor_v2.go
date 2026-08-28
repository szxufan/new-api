package hailuo

import (
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	taskcommon "github.com/QuantumNous/new-api/relay/channel/task/taskcommon"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
)

// ============================================================================
// 视频生成 v2（MiniMax-H3）适配逻辑
//
// v1 与 v2 共用 hailuo.TaskAdaptor（渠道类型 35 只能注册一个 TaskAdaptor），
// 提交期按 info.UpstreamModelName 分流，轮询期按 task.Action 分流 —— 详见
// docs/minimax-video-h3-v2.md。
// ============================================================================

// buildV2Request 由 EstimateBilling 与 BuildRequestBody 共同调用，
// 保证「参与计费的参数」与「发给上游的参数」完全一致。
func (a *TaskAdaptor) buildV2Request(req relaycommon.TaskSubmitReq, info *relaycommon.RelayInfo) (*V2VideoRequest, error) {
	content, err := buildV2Content(req)
	if err != nil {
		return nil, err
	}

	v2Req := &V2VideoRequest{
		Model:      info.UpstreamModelName,
		Content:    content,
		Resolution: normalizeV2Resolution(req.Size, req.Metadata),
		Duration:   clampV2Duration(req.Duration, req.Metadata),
		Ratio:      resolveV2Ratio(content, req.Size, req.Metadata),
	}
	if callback, ok := metadataString(req.Metadata, "callback_url"); ok {
		v2Req.CallbackURL = callback
	}
	if watermark, ok := metadataBool(req.Metadata, "aigc_watermark"); ok {
		v2Req.AigcWatermark = &watermark
	}

	// metadata 覆盖：允许客户端透传上游新增字段。
	// taskcommon 版本会剔除 metadata.model，避免改写模型名绕过计费。
	if err := taskcommon.UnmarshalMetadata(req.Metadata, v2Req); err != nil {
		return nil, err
	}

	// 覆盖后复收敛：客户端 metadata 不得把非法值送到上游
	v2Req.Content = sanitizeV2Content(v2Req.Content)
	if !hasV2Text(v2Req.Content) {
		return nil, fmt.Errorf("content must include a non-empty text item (prompt is required)")
	}
	v2Req.Resolution = normalizeV2Resolution(v2Req.Resolution, nil)
	v2Req.Duration = clampV2Duration(v2Req.Duration, nil)
	v2Req.Ratio = resolveV2Ratio(v2Req.Content, v2Req.Ratio, nil)
	if strings.TrimSpace(v2Req.Model) == "" {
		v2Req.Model = info.UpstreamModelName
	}

	return v2Req, nil
}

// buildV2Content 组装 v2 多模态 content 数组。
// 优先使用 metadata.content（原样透传的逃生通道），否则按图片数量推导角色。
func buildV2Content(req relaycommon.TaskSubmitReq) ([]V2ContentItem, error) {
	if raw, ok := req.Metadata["content"]; ok {
		if items, err := unmarshalV2Content(raw); err == nil && len(items) > 0 {
			items = sanitizeV2Content(items)
			if !hasV2Text(items) {
				return nil, fmt.Errorf("content must include a non-empty text item (prompt is required)")
			}
			return items, nil
		}
		// metadata.content 不可用时回退自动映射，避免直接拒绝整个请求
	}

	prompt := truncateV2Prompt(req.Prompt)
	if strings.TrimSpace(prompt) == "" {
		return nil, fmt.Errorf("content must include a non-empty text item (prompt is required)")
	}

	items := []V2ContentItem{{Type: V2TypeText, Text: prompt}}

	firstFrame, _ := metadataString(req.Metadata, "first_frame_image")
	lastFrame, _ := metadataString(req.Metadata, "last_frame_image")
	referenceImages, _ := metadataStrings(req.Metadata, "reference_images")
	referenceVideos, _ := metadataStrings(req.Metadata, "reference_videos")
	referenceAudios, _ := metadataStrings(req.Metadata, "reference_audios")

	if firstFrame == "" && lastFrame == "" {
		// 未显式指定首/尾帧时，按图片数量映射：1 张→首帧，2 张→首尾帧，≥3 张→参考图
		switch {
		case len(req.Images) == 1:
			firstFrame = req.Images[0]
		case len(req.Images) == 2:
			firstFrame, lastFrame = req.Images[0], req.Images[1]
		case len(req.Images) > 2:
			referenceImages = append(referenceImages, req.Images...)
		case strings.TrimSpace(req.InputReference) != "":
			firstFrame = req.InputReference
		}
	} else {
		// 显式指定了首/尾帧，images 作为参考素材并入
		referenceImages = append(referenceImages, req.Images...)
	}

	if strings.TrimSpace(firstFrame) != "" {
		items = append(items, v2MediaItem(V2TypeImageURL, V2RoleFirstFrame, firstFrame))
	}
	if strings.TrimSpace(lastFrame) != "" {
		items = append(items, v2MediaItem(V2TypeImageURL, V2RoleLastFrame, lastFrame))
	}
	for _, url := range referenceImages {
		items = append(items, v2MediaItem(V2TypeImageURL, V2RoleReferenceImage, url))
	}
	for _, url := range referenceVideos {
		items = append(items, v2MediaItem(V2TypeVideoURL, V2RoleReferenceVideo, url))
	}
	for _, url := range referenceAudios {
		items = append(items, v2MediaItem(V2TypeAudioURL, V2RoleReferenceAudio, url))
	}

	return sanitizeV2Content(items), nil
}

// sanitizeV2Content 收敛 content 数组：剔除空项与非法类型、解决首尾帧与参考素材的互斥、
// 并按上游数量上限截断（按顺序保留靠前的素材）。
func sanitizeV2Content(items []V2ContentItem) []V2ContentItem {
	filtered := make([]V2ContentItem, 0, len(items))
	for _, item := range items {
		switch item.Type {
		case V2TypeText:
			if strings.TrimSpace(item.Text) == "" {
				continue
			}
			item.Text = truncateV2Prompt(item.Text)
			item.ImageURL, item.VideoURL, item.AudioURL = nil, nil, nil
			item.Role = ""
		case V2TypeImageURL, V2TypeVideoURL, V2TypeAudioURL:
			if strings.TrimSpace(v2ItemURL(item)) == "" {
				continue
			}
			item.Text = ""
			if !isV2KnownRole(item.Role) {
				item.Role = defaultV2Role(item.Type)
			}
		default:
			continue
		}
		filtered = append(filtered, item)
	}

	// 图生视频与多模态参考互斥：存在参考素材时，把首/尾帧降级为参考图（保留素材而非丢弃）
	if hasV2Reference(filtered) {
		for i := range filtered {
			if filtered[i].Role == V2RoleFirstFrame || filtered[i].Role == V2RoleLastFrame {
				filtered[i].Type = V2TypeImageURL
				filtered[i].Role = V2RoleReferenceImage
			}
		}
	}

	counts := make(map[string]int, 5)
	mediaTotal := 0
	result := make([]V2ContentItem, 0, len(filtered))
	for _, item := range filtered {
		if item.Type == V2TypeText {
			result = append(result, item)
			continue
		}
		if mediaTotal >= V2MaxMixedAssets {
			continue
		}
		if counts[item.Role] >= v2RoleLimit(item.Role) {
			continue
		}
		counts[item.Role]++
		mediaTotal++
		result = append(result, item)
	}

	return result
}

// normalizeV2Resolution 把任意分辨率表述收敛到 v2 仅支持的 768P / 2K。
// 1080P 及以上向上取到 2K，避免降低画质；无法识别时取默认值。
func normalizeV2Resolution(size string, metadata map[string]any) string {
	if v, ok := metadataString(metadata, "resolution"); ok {
		if r := matchV2Resolution(v); r != "" {
			return r
		}
	}
	// 兼容 /image-debug 视频调试台的嵌套写法 metadata.parameters.resolution
	if params, ok := metadata[metadataParametersKey].(map[string]any); ok {
		if v, ok := metadataString(params, "resolution"); ok {
			if r := matchV2Resolution(v); r != "" {
				return r
			}
		}
	}
	if r := matchV2Resolution(size); r != "" {
		return r
	}
	return V2DefaultResolution
}

func matchV2Resolution(value string) string {
	s := strings.ToUpper(strings.TrimSpace(value))
	if s == "" {
		return ""
	}
	switch {
	case strings.Contains(s, "2K"),
		strings.Contains(s, "1440"),
		strings.Contains(s, "2048"),
		strings.Contains(s, "2560"),
		strings.Contains(s, "1080"):
		return V2Resolution2K
	case strings.Contains(s, "768"),
		strings.Contains(s, "720"),
		strings.Contains(s, "512"),
		strings.Contains(s, "480"):
		return V2Resolution768P
	}
	return ""
}

// clampV2Duration 把时长收敛到 [4,15] 秒；非正值（含调试台的 -1 智能时长）取默认值。
func clampV2Duration(duration int, metadata map[string]any) int {
	if v, ok := metadataInt(metadata, "duration"); ok {
		duration = v
	}
	if duration <= 0 {
		return V2DefaultDuration
	}
	if duration < V2DurationMin {
		return V2DurationMin
	}
	if duration > V2DurationMax {
		return V2DurationMax
	}
	return duration
}

// resolveV2Ratio 按生成场景收敛宽高比：
// i2va 恒为 adaptive；t2va 必须为具体比例（缺省 16:9）；r2va 可选，缺省 adaptive。
func resolveV2Ratio(content []V2ContentItem, size string, metadata map[string]any) string {
	candidate := V2RatioAdaptive
	if v, ok := metadataString(metadata, "ratio"); ok {
		candidate = v
	} else if strings.TrimSpace(size) != "" {
		candidate = strings.TrimSpace(size)
	}

	hasFrame := false
	hasReference := false
	for _, item := range content {
		switch item.Role {
		case V2RoleFirstFrame, V2RoleLastFrame:
			hasFrame = true
		case V2RoleReferenceImage, V2RoleReferenceVideo, V2RoleReferenceAudio:
			hasReference = true
		}
	}

	switch {
	case hasFrame:
		return V2RatioAdaptive
	case hasReference:
		if isV2ValidRatio(candidate) {
			return candidate
		}
		return V2RatioAdaptive
	default:
		if isV2ValidRatio(candidate) {
			return candidate
		}
		return V2DefaultT2VRatio
	}
}

func isV2ValidRatio(ratio string) bool {
	for _, r := range V2ValidRatios {
		if r == ratio {
			return true
		}
	}
	return false
}

// v2ResolutionRatio 分辨率倍率（OtherRatios 的 resolution-* 键）。
// 官方按量价格可调整，管理员亦可在「系统设置 → 计费 → 模型定价」用基础价覆盖。
func v2ResolutionRatio(resolution string) float64 {
	if resolution == V2Resolution768P {
		return 1.0
	}
	return 2.0
}

// EstimateBilling 覆写 taskcommon.BaseBilling 的同名方法：
// v2 按「时长 × 分辨率」计费，并在此标记协议版本（此处是提交期最早能拿到
// UpstreamModelName 且必然被调用的钩子），改写后的 info.Action 随任务落库，
// 供后台轮询选择 v2 查询端点。v1 保持原有纯按次计费。
func (a *TaskAdaptor) EstimateBilling(c *gin.Context, info *relaycommon.RelayInfo) map[string]float64 {
	if !IsVideoV2Model(info.UpstreamModelName) {
		return nil
	}
	info.Action = constant.TaskActionVideoV2Generate

	taskReq, err := relaycommon.GetTaskRequest(c)
	if err != nil {
		return nil
	}
	v2Req, err := a.buildV2Request(taskReq, info)
	if err != nil {
		return nil
	}

	return map[string]float64{
		"seconds": float64(v2Req.Duration),
		fmt.Sprintf("resolution-%s", v2Req.Resolution): v2ResolutionRatio(v2Req.Resolution),
	}
}

// parseV2TaskResult 解析 v2 查询响应。
// 第二个返回值 ok=false 表示该响应不是 v2 结构，调用方应回退 v1 解析。
func (a *TaskAdaptor) parseV2TaskResult(respBody []byte) (taskResult *relaycommon.TaskInfo, ok bool, err error) {
	var resp V2QueryResponse
	if err := common.Unmarshal(respBody, &resp); err != nil {
		return nil, false, nil
	}
	if strings.TrimSpace(resp.Task.Status) == "" {
		return nil, false, nil
	}

	taskResult = &relaycommon.TaskInfo{Code: 0, TaskID: resp.Task.ID}
	switch resp.Task.Status {
	case V2StatusQueued:
		taskResult.Status = model.TaskStatusQueued
		taskResult.Progress = "20%"
	case V2StatusRunning:
		taskResult.Status = model.TaskStatusInProgress
		taskResult.Progress = "50%"
	case V2StatusSucceeded:
		taskResult.Progress = "100%"
		// v2 直接返回可下载的 content.url，无需像 v1 再用 file_id 换取下载地址
		if strings.TrimSpace(resp.Task.Content.URL) == "" {
			taskResult.Status = model.TaskStatusFailure
			taskResult.Reason = "empty content.url in v2 response"
		} else {
			taskResult.Status = model.TaskStatusSuccess
			taskResult.Url = resp.Task.Content.URL
		}
	case V2StatusFailed, V2StatusCancelled:
		taskResult.Status = model.TaskStatusFailure
		taskResult.Progress = "100%"
		taskResult.Reason = v2FailureReason(resp.Task)
	default:
		taskResult.Status = model.TaskStatusInProgress
		taskResult.Progress = "30%"
	}

	return taskResult, true, nil
}

// v2FailureReason 提取失败原因。task.error 的结构未在文档中固定，
// 优先取 message，取不到时回退原始 JSON 文本。
func v2FailureReason(task V2Task) string {
	if len(task.Error) > 0 {
		var errObj struct {
			Message string `json:"message"`
		}
		if err := common.Unmarshal(task.Error, &errObj); err == nil && strings.TrimSpace(errObj.Message) != "" {
			return errObj.Message
		}
		return string(task.Error)
	}
	if task.Status == V2StatusCancelled {
		return "task cancelled"
	}
	return "task failed"
}

// doV2Response 处理 v2 创建任务响应（200 体为 {"task_id": "..."}，无 base_resp 信封）
func (a *TaskAdaptor) doV2Response(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (taskID string, taskData []byte, taskErr *dto.TaskError) {
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", nil, service.TaskErrorWrapper(err, "read_response_body_failed", http.StatusInternalServerError)
	}
	_ = resp.Body.Close()

	var v2Resp V2CreateResponse
	if err := common.Unmarshal(responseBody, &v2Resp); err != nil {
		return "", nil, service.TaskErrorWrapper(errors.Wrapf(err, "body: %s", responseBody), "unmarshal_response_body_failed", http.StatusInternalServerError)
	}

	if strings.TrimSpace(v2Resp.TaskID) == "" {
		var envelope V2ErrorEnvelope
		if err := common.Unmarshal(responseBody, &envelope); err == nil && strings.TrimSpace(envelope.Error.Message) != "" {
			code := envelope.Error.Type
			if code == "" {
				code = "minimax_v2_error"
			}
			return "", nil, service.TaskErrorWrapper(fmt.Errorf("minimax v2 api error: %s", envelope.Error.Message), code, resp.StatusCode)
		}
		return "", nil, service.TaskErrorWrapper(fmt.Errorf("minimax v2 api error: %s", string(responseBody)), "minimax_v2_error", resp.StatusCode)
	}

	ov := dto.NewOpenAIVideo()
	ov.ID = info.PublicTaskID
	ov.TaskID = info.PublicTaskID
	ov.CreatedAt = time.Now().Unix()
	ov.Model = info.OriginModelName

	c.JSON(http.StatusOK, ov)
	return v2Resp.TaskID, responseBody, nil
}

// ============================================================================
// content 与 metadata 辅助函数
// ============================================================================

func v2MediaItem(itemType, role, url string) V2ContentItem {
	url = strings.TrimSpace(url)
	part := &V2URLPart{URL: url}
	item := V2ContentItem{Type: itemType, Role: role}
	switch itemType {
	case V2TypeImageURL:
		item.ImageURL = part
	case V2TypeVideoURL:
		item.VideoURL = part
	case V2TypeAudioURL:
		item.AudioURL = part
	}
	return item
}

func v2ItemURL(item V2ContentItem) string {
	switch item.Type {
	case V2TypeImageURL:
		if item.ImageURL != nil {
			return item.ImageURL.URL
		}
	case V2TypeVideoURL:
		if item.VideoURL != nil {
			return item.VideoURL.URL
		}
	case V2TypeAudioURL:
		if item.AudioURL != nil {
			return item.AudioURL.URL
		}
	}
	return ""
}

func defaultV2Role(itemType string) string {
	switch itemType {
	case V2TypeVideoURL:
		return V2RoleReferenceVideo
	case V2TypeAudioURL:
		return V2RoleReferenceAudio
	default:
		return V2RoleFirstFrame
	}
}

func isV2KnownRole(role string) bool {
	switch role {
	case V2RoleFirstFrame, V2RoleLastFrame, V2RoleReferenceImage, V2RoleReferenceVideo, V2RoleReferenceAudio:
		return true
	}
	return false
}

func v2RoleLimit(role string) int {
	switch role {
	case V2RoleFirstFrame, V2RoleLastFrame:
		return 1
	case V2RoleReferenceImage:
		return V2MaxReferenceImages
	case V2RoleReferenceVideo:
		return V2MaxReferenceVideos
	case V2RoleReferenceAudio:
		return V2MaxReferenceAudios
	}
	return 1
}

func hasV2Text(items []V2ContentItem) bool {
	for _, item := range items {
		if item.Type == V2TypeText && strings.TrimSpace(item.Text) != "" {
			return true
		}
	}
	return false
}

func hasV2Reference(items []V2ContentItem) bool {
	for _, item := range items {
		if item.Role == V2RoleReferenceImage || item.Role == V2RoleReferenceVideo || item.Role == V2RoleReferenceAudio {
			return true
		}
	}
	return false
}

func truncateV2Prompt(prompt string) string {
	// 按字符而非字节截断，避免切断多字节 UTF-8 序列
	runes := []rune(prompt)
	if len(runes) <= V2PromptMaxLen {
		return prompt
	}
	return string(runes[:V2PromptMaxLen])
}

func unmarshalV2Content(raw any) ([]V2ContentItem, error) {
	data, err := common.Marshal(raw)
	if err != nil {
		return nil, err
	}
	var items []V2ContentItem
	if err := common.Unmarshal(data, &items); err != nil {
		return nil, err
	}
	return items, nil
}

const metadataParametersKey = "parameters"

func metadataString(metadata map[string]any, key string) (string, bool) {
	if metadata == nil {
		return "", false
	}
	value, ok := metadata[key].(string)
	if !ok || strings.TrimSpace(value) == "" {
		return "", false
	}
	return strings.TrimSpace(value), true
}

func metadataStrings(metadata map[string]any, key string) ([]string, bool) {
	if metadata == nil {
		return nil, false
	}
	raw, ok := metadata[key]
	if !ok {
		return nil, false
	}
	if list, ok := raw.([]string); ok {
		return nonEmptyStrings(list), true
	}
	list, ok := raw.([]any)
	if !ok {
		return nil, false
	}
	result := make([]string, 0, len(list))
	for _, item := range list {
		if s, ok := item.(string); ok && strings.TrimSpace(s) != "" {
			result = append(result, strings.TrimSpace(s))
		}
	}
	if len(result) == 0 {
		return nil, false
	}
	return result, true
}

func metadataInt(metadata map[string]any, key string) (int, bool) {
	if metadata == nil {
		return 0, false
	}
	switch value := metadata[key].(type) {
	case int:
		return value, true
	case float64:
		return int(value), true
	case string:
		parsed, err := strconv.Atoi(strings.TrimSpace(value))
		if err != nil {
			return 0, false
		}
		return parsed, true
	}
	return 0, false
}

func metadataBool(metadata map[string]any, key string) (bool, bool) {
	if metadata == nil {
		return false, false
	}
	switch value := metadata[key].(type) {
	case bool:
		return value, true
	case string:
		parsed, err := strconv.ParseBool(strings.TrimSpace(value))
		if err != nil {
			return false, false
		}
		return parsed, true
	}
	return false, false
}

func nonEmptyStrings(list []string) []string {
	result := make([]string, 0, len(list))
	for _, item := range list {
		if strings.TrimSpace(item) != "" {
			result = append(result, strings.TrimSpace(item))
		}
	}
	return result
}
