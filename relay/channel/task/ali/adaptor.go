package ali

import (
	"bytes"
	"fmt"
	"io"
	"math"
	"net/http"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay/channel"
	"github.com/QuantumNous/new-api/relay/channel/task/taskcommon"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/samber/lo"

	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
)

// ============================
// Request / Response structures
// ============================

// AliVideoRequest 阿里通义万相视频生成请求
type AliVideoRequest struct {
	Model      string              `json:"model"`
	Input      AliVideoInput       `json:"input"`
	Parameters *AliVideoParameters `json:"parameters,omitempty"`
}

// AliVideoInput 视频输入参数
type AliVideoInput struct {
	Prompt         string `json:"prompt,omitempty"`          // 文本提示词
	ImgURL         string `json:"img_url,omitempty"`         // 首帧图像URL或Base64（图生视频）
	FirstFrameURL  string `json:"first_frame_url,omitempty"` // 首帧图片URL（首尾帧生视频）
	LastFrameURL   string `json:"last_frame_url,omitempty"`  // 尾帧图片URL（首尾帧生视频）
	AudioURL       string `json:"audio_url,omitempty"`       // 音频URL（wan2.5支持）
	NegativePrompt string `json:"negative_prompt,omitempty"` // 反向提示词
	Template       string `json:"template,omitempty"`        // 视频特效模板
	Media          []AliMedia `json:"media,omitempty"`          // 媒体素材列表（HappyHorse / 万相3.0全能参考）
	VideoURL       string `json:"video_url,omitempty"`       // HappyHorse 视频编辑输入视频URL
}

// AliMedia 媒体素材（HappyHorse / 万相3.0）
type AliMedia struct {
	Type string `json:"type"` // first_frame / last_frame / reference_image / reference_video / reference_audio / file / link
	URL  string `json:"url"`  // 图片URL或Base64
}

// AliVideoParameters 视频参数
type AliVideoParameters struct {
	Resolution   string `json:"resolution,omitempty"`    // 分辨率: 480P/720P/1080P（图生视频、首尾帧生视频）
	Size         string `json:"size,omitempty"`          // 尺寸: 如 "832*480"（文生视频）
	Ratio        string `json:"ratio,omitempty"`         // 宽高比: 16:9/4:3/1:1/3:4/9:16/adaptive（HappyHorse、万相3.0）
	Duration     int    `json:"duration,omitempty"`      // 时长: 3-10秒（wan2.x），2-30秒或-1智能时长（万相3.0）
	PromptExtend bool   `json:"prompt_extend,omitempty"` // 是否开启prompt智能改写
	Watermark    bool   `json:"watermark,omitempty"`     // 是否添加水印
	Audio        *bool  `json:"audio,omitempty"`         // 是否添加音频（wan2.5）
	Seed         int    `json:"seed,omitempty"`          // 随机数种子
}

// AliVideoResponse 阿里通义万相响应
type AliVideoResponse struct {
	Output    AliVideoOutput `json:"output"`
	RequestID string         `json:"request_id"`
	Code      string         `json:"code,omitempty"`
	Message   string         `json:"message,omitempty"`
	Usage     *AliUsage      `json:"usage,omitempty"`
}

// AliVideoOutput 输出信息
type AliVideoOutput struct {
	TaskID        string `json:"task_id"`
	TaskStatus    string `json:"task_status"`
	SubmitTime    string `json:"submit_time,omitempty"`
	ScheduledTime string `json:"scheduled_time,omitempty"`
	EndTime       string `json:"end_time,omitempty"`
	OrigPrompt    string `json:"orig_prompt,omitempty"`
	ActualPrompt  string `json:"actual_prompt,omitempty"`
	VideoURL      string `json:"video_url,omitempty"`
	Code          string `json:"code,omitempty"`
	Message       string `json:"message,omitempty"`
}

// AliUsage 使用统计
type AliUsage struct {
	Duration   dto.IntValue `json:"duration,omitempty"`
	VideoCount dto.IntValue `json:"video_count,omitempty"`
	SR         dto.IntValue `json:"SR,omitempty"`
}

type AliMetadata struct {
	// Input 相关
	AudioURL       string `json:"audio_url,omitempty"`       // 音频URL
	ImgURL         string `json:"img_url,omitempty"`         // 图片URL（图生视频）
	FirstFrameURL  string `json:"first_frame_url,omitempty"` // 首帧图片URL（首尾帧生视频）
	LastFrameURL   string `json:"last_frame_url,omitempty"`  // 尾帧图片URL（首尾帧生视频）
	NegativePrompt string `json:"negative_prompt,omitempty"` // 反向提示词
	Template       string `json:"template,omitempty"`        // 视频特效模板
	VideoURL       string `json:"video_url,omitempty"`       // HappyHorse 视频编辑输入视频URL

	// Parameters 相关
	Resolution   *string `json:"resolution,omitempty"`    // 分辨率: 480P/720P/1080P
	Size         *string `json:"size,omitempty"`          // 尺寸: 如 "832*480"
	Ratio        *string `json:"ratio,omitempty"`         // 宽高比: 如 "16:9"
	Duration     *int    `json:"duration,omitempty"`      // 时长
	PromptExtend *bool   `json:"prompt_extend,omitempty"` // 是否开启prompt智能改写
	Watermark    *bool   `json:"watermark,omitempty"`     // 是否添加水印
	Audio        *bool   `json:"audio,omitempty"`         // 是否添加音频
	Seed         *int    `json:"seed,omitempty"`          // 随机数种子
}

// ============================
// Adaptor implementation
// ============================

type TaskAdaptor struct {
	taskcommon.BaseBilling
	ChannelType int
	apiKey      string
	baseURL     string
}

func (a *TaskAdaptor) Init(info *relaycommon.RelayInfo) {
	a.ChannelType = info.ChannelType
	a.baseURL = info.ChannelBaseUrl
	a.apiKey = info.ApiKey
}

func (a *TaskAdaptor) ValidateRequestAndSetAction(c *gin.Context, info *relaycommon.RelayInfo) (taskErr *dto.TaskError) {
	// ValidateMultipartDirect 负责解析并将原始 TaskSubmitReq 存入 context
	return relaycommon.ValidateMultipartDirect(c, info)
}

func (a *TaskAdaptor) BuildRequestURL(info *relaycommon.RelayInfo) (string, error) {
	return fmt.Sprintf("%s/api/v1/services/aigc/video-generation/video-synthesis", a.baseURL), nil
}

// BuildRequestHeader sets required headers for Ali API
func (a *TaskAdaptor) BuildRequestHeader(c *gin.Context, req *http.Request, info *relaycommon.RelayInfo) error {
	req.Header.Set("Authorization", "Bearer "+a.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-DashScope-Async", "enable") // 阿里异步任务必须设置
	return nil
}

func (a *TaskAdaptor) BuildRequestBody(c *gin.Context, info *relaycommon.RelayInfo) (io.Reader, error) {
	taskReq, err := relaycommon.GetTaskRequest(c)
	if err != nil {
		return nil, errors.Wrap(err, "get_task_request_failed")
	}

	aliReq, err := a.convertToAliRequest(info, taskReq)
	if err != nil {
		return nil, errors.Wrap(err, "convert_to_ali_request_failed")
	}
	logger.LogJson(c, "ali video request body", aliReq)

	bodyBytes, err := common.Marshal(aliReq)
	if err != nil {
		return nil, errors.Wrap(err, "marshal_ali_request_failed")
	}
	return bytes.NewReader(bodyBytes), nil
}

var (
	size480p = []string{
		"832*480",
		"480*832",
		"624*624",
	}
	size720p = []string{
		"1280*720",
		"720*1280",
		"960*960",
		"1088*832",
		"832*1088",
	}
	size1080p = []string{
		"1920*1080",
		"1080*1920",
		"1440*1440",
		"1632*1248",
		"1248*1632",
	}
)

func sizeToResolution(size string) (string, error) {
	if lo.Contains(size480p, size) {
		return "480P", nil
	} else if lo.Contains(size720p, size) {
		return "720P", nil
	} else if lo.Contains(size1080p, size) {
		return "1080P", nil
	}
	return "", fmt.Errorf("invalid size: %s", size)
}

func ProcessAliOtherRatios(aliReq *AliVideoRequest) (map[string]float64, error) {
	otherRatios := make(map[string]float64)
	aliRatios := map[string]map[string]float64{
		"wan3.0-video": {
			"480P":  1,
			"720P":  2,
			"1080P": 4,
		},
		"wan3.0-video-prime": {
			"480P":  1,
			"720P":  2,
			"1080P": 4,
		},
		"wan2.6-i2v": {
			"720P":  1,
			"1080P": 1 / 0.6,
		},
		"wan2.5-t2v-preview": {
			"480P":  1,
			"720P":  2,
			"1080P": 1 / 0.3,
		},
		"wan2.2-t2v-plus": {
			"480P":  1,
			"1080P": 0.7 / 0.14,
		},
		"wan2.5-i2v-preview": {
			"480P":  1,
			"720P":  2,
			"1080P": 1 / 0.3,
		},
		"wan2.2-i2v-plus": {
			"480P":  1,
			"1080P": 0.7 / 0.14,
		},
		"wan2.2-kf2v-flash": {
			"480P":  1,
			"720P":  2,
			"1080P": 4.8,
		},
		"wan2.2-i2v-flash": {
			"480P": 1,
			"720P": 2,
		},
		"wan2.2-s2v": {
			"480P": 1,
			"720P": 0.9 / 0.5,
		},
		"happyhorse-1.0-t2v": {
			"720P":  1,
			"1080P": 1.6 / 0.9,
		},
		"happyhorse-1.0-i2v": {
			"720P":  1,
			"1080P": 1.6 / 0.9,
		},
		"happyhorse-1.0-r2v": {
			"720P":  1,
			"1080P": 1.6 / 0.9,
		},
		"happyhorse-1.0-video-edit": {
			"720P":  1,
			"1080P": 1.6 / 0.9,
		},
	}
	var resolution string

	// size match
	if aliReq.Parameters.Size != "" {
		toResolution, err := sizeToResolution(aliReq.Parameters.Size)
		if err != nil {
			return nil, err
		}
		resolution = toResolution
	} else {
		resolution = strings.ToUpper(aliReq.Parameters.Resolution)
		if !strings.HasSuffix(resolution, "P") {
			resolution = resolution + "P"
		}
	}
	if otherRatio, ok := aliRatios[aliReq.Model]; ok {
		if ratio, ok := otherRatio[resolution]; ok {
			otherRatios[fmt.Sprintf("resolution-%s", resolution)] = ratio
		}
	}
	return otherRatios, nil
}

func isHappyHorseModel(model string) bool {
	return strings.Contains(model, "happyhorse")
}

func (a *TaskAdaptor) convertToAliRequest(info *relaycommon.RelayInfo, req relaycommon.TaskSubmitReq) (*AliVideoRequest, error) {
	upstreamModel := req.Model
	if info.IsModelMapped {
		upstreamModel = info.UpstreamModelName
	}

	aliReq := &AliVideoRequest{
		Model:      upstreamModel,
		Input:      AliVideoInput{},
		Parameters: &AliVideoParameters{},
	}

	if isHappyHorseModel(upstreamModel) {
		a.convertHappyHorseRequest(aliReq, req)
	} else if isWan3Model(upstreamModel) {
		a.convertWan3Request(aliReq, req)
	} else {
		a.convertWanRequest(aliReq, req)
	}

	// 从 metadata 中提取额外参数
	if req.Metadata != nil {
		if metadataBytes, err := common.Marshal(req.Metadata); err == nil {
			err = common.Unmarshal(metadataBytes, aliReq)
			if err != nil {
				return nil, errors.Wrap(err, "unmarshal metadata failed")
			}
		} else {
			return nil, errors.Wrap(err, "marshal metadata failed")
		}
	}

	if aliReq.Model != upstreamModel {
		return nil, errors.New("can't change model with metadata")
	}

	return aliReq, nil
}

func (a *TaskAdaptor) convertHappyHorseRequest(aliReq *AliVideoRequest, req relaycommon.TaskSubmitReq) {
	aliReq.Input.Prompt = req.Prompt
	aliReq.Parameters.Duration = 5

	switch {
	case strings.Contains(aliReq.Model, "t2v"):
		// HappyHorse 文生视频
		aliReq.Parameters.Resolution = "1080P"
		aliReq.Parameters.Ratio = "16:9"
		aliReq.Parameters.Watermark = false
	case strings.Contains(aliReq.Model, "i2v"):
		// HappyHorse 图生视频（首帧）
		aliReq.Parameters.Resolution = "1080P"
		aliReq.Parameters.Watermark = false
		if req.InputReference != "" {
			aliReq.Input.Media = []AliMedia{
				{Type: "first_frame", URL: req.InputReference},
			}
		} else if len(req.Images) > 0 {
			aliReq.Input.Media = []AliMedia{
				{Type: "first_frame", URL: req.Images[0]},
			}
		}
	case strings.Contains(aliReq.Model, "r2v"):
		// HappyHorse 参考生视频
		aliReq.Parameters.Resolution = "1080P"
		aliReq.Parameters.Ratio = "16:9"
		aliReq.Parameters.Watermark = false
		if len(req.Images) > 0 {
			media := make([]AliMedia, len(req.Images))
			for i, img := range req.Images {
				media[i] = AliMedia{Type: "reference_image", URL: img}
			}
			aliReq.Input.Media = media
		} else if req.InputReference != "" {
			aliReq.Input.Media = []AliMedia{
				{Type: "reference_image", URL: req.InputReference},
			}
		}
	case strings.Contains(aliReq.Model, "video-edit"):
		// HappyHorse 视频编辑
		aliReq.Parameters.Resolution = "1080P"
		aliReq.Parameters.Watermark = false
		if req.InputReference != "" {
			aliReq.Input.VideoURL = req.InputReference
		}
		if len(req.Images) > 0 {
			media := make([]AliMedia, len(req.Images))
			for i, img := range req.Images {
				media[i] = AliMedia{Type: "reference_image", URL: img}
			}
			aliReq.Input.Media = media
		}
	}

	// 处理 size 参数：HappyHorse 使用 ratio 或 resolution
	if req.Size != "" {
		if strings.Contains(req.Size, ":") {
			aliReq.Parameters.Ratio = req.Size
		} else if strings.Contains(req.Size, "*") {
			resolution, err := sizeToResolution(req.Size)
			if err == nil {
				aliReq.Parameters.Resolution = resolution
			}
		} else {
			resolution := strings.ToUpper(req.Size)
			if !strings.HasSuffix(resolution, "P") {
				resolution = resolution + "P"
			}
			aliReq.Parameters.Resolution = resolution
		}
	}

	// 处理时长
	if req.Duration > 0 {
		aliReq.Parameters.Duration = req.Duration
	} else if req.Seconds != "" {
		seconds, err := strconv.Atoi(req.Seconds)
		if err == nil {
			aliReq.Parameters.Duration = seconds
		}
	}
}

// isWan3Model 判断是否为万相3.0模型（wan3.0-video / wan3.0-video-prime）
func isWan3Model(model string) bool {
	return strings.HasPrefix(model, "wan3.0")
}

// wan3Ratios 万相3.0 支持的官方宽高比
var wan3Ratios = []string{"16:9", "4:3", "1:1", "3:4", "9:16"}

// parseSizeDimensions 解析 "WxH" / "W*H" 像素尺寸
func parseSizeDimensions(size string) (int, int, bool) {
	dims := strings.FieldsFunc(size, func(r rune) bool {
		return r == 'x' || r == 'X' || r == '*'
	})
	if len(dims) != 2 {
		return 0, 0, false
	}
	w, errW := strconv.Atoi(dims[0])
	h, errH := strconv.Atoi(dims[1])
	if errW != nil || errH != nil || w <= 0 || h <= 0 {
		return 0, 0, false
	}
	return w, h, true
}

// sizeToWan3Ratio 将像素尺寸换算为最接近的万相3.0官方宽高比；无法解析时返回空串
func sizeToWan3Ratio(size string) string {
	w, h, ok := parseSizeDimensions(size)
	if !ok {
		return ""
	}
	ratio := float64(w) / float64(h)
	best := "16:9"
	bestDiff := math.MaxFloat64
	for _, r := range wan3Ratios {
		parts := strings.Split(r, ":")
		tw, _ := strconv.Atoi(parts[0])
		th, _ := strconv.Atoi(parts[1])
		diff := math.Abs(ratio - float64(tw)/float64(th))
		if diff < bestDiff {
			bestDiff = diff
			best = r
		}
	}
	return best
}

// sizeToWan3Resolution 将尺寸换算为万相3.0分辨率档位；无法解析时返回空串
func sizeToWan3Resolution(size string) string {
	if size == "" {
		return ""
	}
	upper := strings.ToUpper(size)
	if strings.HasSuffix(upper, "P") {
		return upper
	}
	w, h, ok := parseSizeDimensions(size)
	if !ok {
		return ""
	}
	longest := w
	if h > longest {
		longest = h
	}
	switch {
	case longest >= 1920:
		return "1080P"
	case longest >= 1280:
		return "720P"
	default:
		return "480P"
	}
}

// normalizeWan3Duration 万相3.0 时长归一化：有效范围为 2-30 秒，-1 表示智能时长，
// 未指定时默认 5 秒，超范围值收敛到边界。
func normalizeWan3Duration(req relaycommon.TaskSubmitReq) int {
	duration := 0
	if req.Duration != 0 {
		duration = req.Duration
	} else if req.Seconds != "" {
		if seconds, err := strconv.Atoi(req.Seconds); err == nil {
			duration = seconds
		}
	}
	switch {
	case duration == -1: // 智能时长模式，由模型根据输入自动推荐
		return -1
	case duration <= 0: // 未指定，使用文档默认 5 秒
		return 5
	case duration < 2:
		return 2
	case duration > 30:
		return 30
	default:
		return duration
	}
}

// convertWan3Request 万相3.0（wan3.0-video / wan3.0-video-prime）请求转换。
// 万相3.0 为全能参考模型：统一使用 input.media 数组（first_frame / reference_image 等），
// 宽高比使用 parameters.ratio（支持 adaptive），时长支持 2-30 秒与 -1 智能时长，
// 不使用 wan2.x 的 img_url / first_frame_url / last_frame_url 与 size 字段。
func (a *TaskAdaptor) convertWan3Request(aliReq *AliVideoRequest, req relaycommon.TaskSubmitReq) {
	aliReq.Input.Prompt = req.Prompt
	aliReq.Parameters.PromptExtend = true
	aliReq.Parameters.Watermark = false

	// 媒体输入：单图 → 首帧（图生视频）；多图 → 参考图（参考生视频）
	if len(req.Images) > 0 {
		media := make([]AliMedia, len(req.Images))
		mediaType := "reference_image"
		if len(req.Images) == 1 {
			mediaType = "first_frame"
		}
		for i, img := range req.Images {
			media[i] = AliMedia{Type: mediaType, URL: img}
		}
		aliReq.Input.Media = media
	}

	// 宽高比与分辨率：
	// - "16:9" 等官方宽高比或 "adaptive" 直接透传
	// - "1920*1080" 等像素尺寸换算为最接近的 ratio / resolution
	// - "480P" 等分辨率档位直接透传
	// - 未指定时保持万相3.0 默认（1080P + adaptive）
	if req.Size != "" {
		switch {
		case strings.Contains(req.Size, ":"), strings.EqualFold(req.Size, "adaptive"):
			aliReq.Parameters.Ratio = req.Size
		default:
			if ratio := sizeToWan3Ratio(req.Size); ratio != "" {
				aliReq.Parameters.Ratio = ratio
			}
		}
		if resolution := sizeToWan3Resolution(req.Size); resolution != "" {
			aliReq.Parameters.Resolution = resolution
		}
	}

	aliReq.Parameters.Duration = normalizeWan3Duration(req)
}

func (a *TaskAdaptor) convertWanRequest(aliReq *AliVideoRequest, req relaycommon.TaskSubmitReq) {
	aliReq.Input.Prompt = req.Prompt
	aliReq.Input.ImgURL = req.InputReference
	aliReq.Parameters.PromptExtend = true
	aliReq.Parameters.Watermark = false

	// 处理分辨率映射
	if req.Size != "" {
		if strings.Contains(req.Model, "t2v") && !strings.Contains(req.Size, "*") {
			// wan t2v 不使用此路径，但保留校验
		}
		if strings.Contains(req.Size, "*") {
			aliReq.Parameters.Size = req.Size
		} else {
			resolution := strings.ToUpper(req.Size)
			if !strings.HasSuffix(resolution, "P") {
				resolution = resolution + "P"
			}
			aliReq.Parameters.Resolution = resolution
		}
	} else {
		if strings.Contains(req.Model, "t2v") {
			if strings.HasPrefix(req.Model, "wan2.5") {
				aliReq.Parameters.Size = "1920*1080"
			} else if strings.HasPrefix(req.Model, "wan2.2") {
				aliReq.Parameters.Size = "1920*1080"
			} else {
				aliReq.Parameters.Size = "1280*720"
			}
		} else {
			if strings.HasPrefix(req.Model, "wan2.6") {
				aliReq.Parameters.Resolution = "1080P"
			} else if strings.HasPrefix(req.Model, "wan2.5") {
				aliReq.Parameters.Resolution = "1080P"
			} else if strings.HasPrefix(req.Model, "wan2.2-i2v-flash") {
				aliReq.Parameters.Resolution = "720P"
			} else if strings.HasPrefix(req.Model, "wan2.2-i2v-plus") {
				aliReq.Parameters.Resolution = "1080P"
			} else {
				aliReq.Parameters.Resolution = "720P"
			}
		}
	}

	// 处理时长
	if req.Duration > 0 {
		aliReq.Parameters.Duration = req.Duration
	} else if req.Seconds != "" {
		seconds, err := strconv.Atoi(req.Seconds)
		if err != nil {
			aliReq.Parameters.Duration = seconds
		}
	} else {
		aliReq.Parameters.Duration = 5
	}
}

// EstimateBilling 根据用户请求参数计算 OtherRatios（时长、分辨率等）。
// 在 ValidateRequestAndSetAction 之后、价格计算之前调用。
func (a *TaskAdaptor) EstimateBilling(c *gin.Context, info *relaycommon.RelayInfo) map[string]float64 {
	taskReq, err := relaycommon.GetTaskRequest(c)
	if err != nil {
		return nil
	}

	aliReq, err := a.convertToAliRequest(info, taskReq)
	if err != nil {
		return nil
	}

	otherRatios := map[string]float64{
		"seconds": float64(aliReq.Parameters.Duration),
	}
	ratios, err := ProcessAliOtherRatios(aliReq)
	if err != nil {
		return otherRatios
	}
	for k, v := range ratios {
		otherRatios[k] = v
	}
	return otherRatios
}

// DoRequest delegates to common helper
func (a *TaskAdaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (*http.Response, error) {
	return channel.DoTaskApiRequest(a, c, info, requestBody)
}

// DoResponse handles upstream response
func (a *TaskAdaptor) DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (taskID string, taskData []byte, taskErr *dto.TaskError) {
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		taskErr = service.TaskErrorWrapper(err, "read_response_body_failed", http.StatusInternalServerError)
		return
	}
	_ = resp.Body.Close()

	// 解析阿里响应
	var aliResp AliVideoResponse
	if err := common.Unmarshal(responseBody, &aliResp); err != nil {
		taskErr = service.TaskErrorWrapper(errors.Wrapf(err, "body: %s", responseBody), "unmarshal_response_body_failed", http.StatusInternalServerError)
		return
	}

	// 检查错误
	if aliResp.Code != "" {
		taskErr = service.TaskErrorWrapper(fmt.Errorf("%s: %s", aliResp.Code, aliResp.Message), "ali_api_error", resp.StatusCode)
		return
	}

	if aliResp.Output.TaskID == "" {
		taskErr = service.TaskErrorWrapper(fmt.Errorf("task_id is empty"), "invalid_response", http.StatusInternalServerError)
		return
	}

	// 转换为 OpenAI 格式响应
	openAIResp := dto.NewOpenAIVideo()
	openAIResp.ID = info.PublicTaskID
	openAIResp.TaskID = info.PublicTaskID
	openAIResp.Model = c.GetString("model")
	if openAIResp.Model == "" && info != nil {
		openAIResp.Model = info.OriginModelName
	}
	openAIResp.Status = convertAliStatus(aliResp.Output.TaskStatus)
	openAIResp.CreatedAt = common.GetTimestamp()

	// 返回 OpenAI 格式
	c.JSON(http.StatusOK, openAIResp)

	return aliResp.Output.TaskID, responseBody, nil
}

// FetchTask 查询任务状态
func (a *TaskAdaptor) FetchTask(baseUrl, key string, body map[string]any, proxy string) (*http.Response, error) {
	taskID, ok := body["task_id"].(string)
	if !ok {
		return nil, fmt.Errorf("invalid task_id")
	}

	uri := fmt.Sprintf("%s/api/v1/tasks/%s", baseUrl, taskID)

	req, err := http.NewRequest(http.MethodGet, uri, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+key)

	client, err := service.GetHttpClientWithProxy(proxy)
	if err != nil {
		return nil, fmt.Errorf("new proxy http client failed: %w", err)
	}
	return client.Do(req)
}

func (a *TaskAdaptor) GetModelList() []string {
	return ModelList
}

func (a *TaskAdaptor) GetChannelName() string {
	return ChannelName
}

// ParseTaskResult 解析任务结果
func (a *TaskAdaptor) ParseTaskResult(respBody []byte) (*relaycommon.TaskInfo, error) {
	var aliResp AliVideoResponse
	if err := common.Unmarshal(respBody, &aliResp); err != nil {
		return nil, errors.Wrap(err, "unmarshal task result failed")
	}

	taskResult := relaycommon.TaskInfo{
		Code: 0,
	}

	// 状态映射
	switch aliResp.Output.TaskStatus {
	case "PENDING":
		taskResult.Status = model.TaskStatusQueued
	case "RUNNING":
		taskResult.Status = model.TaskStatusInProgress
	case "SUCCEEDED":
		taskResult.Status = model.TaskStatusSuccess
		// 阿里直接返回视频URL，不需要额外的代理端点
		taskResult.Url = aliResp.Output.VideoURL
	case "FAILED", "CANCELED", "UNKNOWN":
		taskResult.Status = model.TaskStatusFailure
		if aliResp.Message != "" {
			taskResult.Reason = aliResp.Message
		} else if aliResp.Output.Message != "" {
			taskResult.Reason = fmt.Sprintf("task failed, code: %s , message: %s", aliResp.Output.Code, aliResp.Output.Message)
		} else {
			taskResult.Reason = "task failed"
		}
	default:
		taskResult.Status = model.TaskStatusQueued
	}

	return &taskResult, nil
}

func (a *TaskAdaptor) ConvertToOpenAIVideo(task *model.Task) ([]byte, error) {
	openAIResp := dto.NewOpenAIVideo()
	openAIResp.ID = task.TaskID
	// 状态取数据库实时字段（后台轮询器会持续更新），
	// 不能依赖 task.Data 中保存的创建响应（仅含 PENDING 且无 video_url）
	openAIResp.Status = task.Status.ToVideoStatus()
	openAIResp.Model = task.Properties.OriginModelName
	openAIResp.SetProgressStr(task.Progress)
	openAIResp.CreatedAt = task.CreatedAt
	openAIResp.CompletedAt = task.UpdatedAt

	// 视频 URL 由后台轮询成功后写入 PrivateData.ResultURL
	openAIResp.SetMetadata("url", task.GetResultURL())

	// 失败信息：优先使用轮询写入的 FailReason，其次解析任务数据中的错误
	if task.Status == model.TaskStatusFailure {
		if task.FailReason != "" {
			openAIResp.Error = &dto.OpenAIVideoError{
				Code:    "task_failed",
				Message: task.FailReason,
			}
			return common.Marshal(openAIResp)
		}
		var aliResp AliVideoResponse
		if err := common.Unmarshal(task.Data, &aliResp); err == nil {
			code, message := aliResp.Code, aliResp.Message
			if code == "" {
				code, message = aliResp.Output.Code, aliResp.Output.Message
			}
			if message != "" {
				openAIResp.Error = &dto.OpenAIVideoError{
					Code:    code,
					Message: message,
				}
			}
		}
	}

	return common.Marshal(openAIResp)
}

func convertAliStatus(aliStatus string) string {
	switch aliStatus {
	case "PENDING":
		return dto.VideoStatusQueued
	case "RUNNING":
		return dto.VideoStatusInProgress
	case "SUCCEEDED":
		return dto.VideoStatusCompleted
	case "FAILED", "CANCELED", "UNKNOWN":
		return dto.VideoStatusFailed
	default:
		return dto.VideoStatusUnknown
	}
}
