package hailuo

import "strings"

const (
	ChannelName = "hailuo-video"
)

var ModelList = []string{
	"MiniMax-H3",
	"MiniMax-Hailuo-2.3",
	"MiniMax-Hailuo-2.3-Fast",
	"MiniMax-Hailuo-02",
	"T2V-01-Director",
	"T2V-01",
	"I2V-01-Director",
	"I2V-01-live",
	"I2V-01",
	"S2V-01",
}

const (
	TextToVideoEndpoint = "/v1/video_generation"
	QueryTaskEndpoint   = "/v1/query/video_generation"
)

const (
	StatusSuccess    = 0
	StatusRateLimit  = 1002
	StatusAuthFailed = 1004
	StatusNoBalance  = 1008
	StatusSensitive  = 1026
	StatusParamError = 2013
	StatusInvalidKey = 2049
)

const (
	TaskStatusPreparing  = "Preparing"
	TaskStatusQueueing   = "Queueing"
	TaskStatusProcessing = "Processing"
	TaskStatusSuccess    = "Success"
	TaskStatusFailed     = "Fail"
)

const (
	Resolution512P  = "512P"
	Resolution720P  = "720P"
	Resolution768P  = "768P"
	Resolution1080P = "1080P"
)

const (
	DefaultDuration   = 6
	DefaultResolution = Resolution720P
)

// ============================================================================
// 视频生成 v2（MiniMax-H3）协议常量
// 文档：https://platform.minimaxi.com/docs/api-reference/video-generation-v2-create
// ============================================================================

const (
	// V2CreateEndpoint 创建视频生成任务（POST）
	V2CreateEndpoint = "/v2/video_generation"
	// V2QueryEndpoint 查询任务（GET），task_id 为路径参数，需直接拼接在其后
	V2QueryEndpoint = "/v2/query/video_generation/"
)

// V2ModelPrefix v2 协议模型名前缀（小写比较），兼容后续 MiniMax-H3-xxx 变体
const V2ModelPrefix = "minimax-h3"

// v2 任务状态：queued / running / succeeded / failed / cancelled
const (
	V2StatusQueued    = "queued"
	V2StatusRunning   = "running"
	V2StatusSucceeded = "succeeded"
	V2StatusFailed    = "failed"
	V2StatusCancelled = "cancelled"
)

// v2 输出规格：分辨率仅 768P / 2K，时长为 4-15 的整数秒
const (
	V2Resolution768P    = "768P"
	V2Resolution2K      = "2K"
	V2DurationMin       = 4
	V2DurationMax       = 15
	V2DefaultDuration   = 5
	V2DefaultResolution = V2Resolution2K
	V2PromptMaxLen      = 7000
)

// v2 宽高比：adaptive 表示由输入自适应；文生视频不允许 adaptive
const (
	V2RatioAdaptive   = "adaptive"
	V2DefaultT2VRatio = "16:9"
)

var V2ValidRatios = []string{"21:9", "16:9", "4:3", "1:1", "3:4", "9:16"}

// v2 content 元素类型
const (
	V2TypeText     = "text"
	V2TypeImageURL = "image_url"
	V2TypeVideoURL = "video_url"
	V2TypeAudioURL = "audio_url"
)

// v2 content 元素角色
const (
	V2RoleFirstFrame     = "first_frame"
	V2RoleLastFrame      = "last_frame"
	V2RoleReferenceImage = "reference_image"
	V2RoleReferenceVideo = "reference_video"
	V2RoleReferenceAudio = "reference_audio"
)

// v2 素材数量上限：首/尾帧各 1 张，参考图 9 张、参考视频 3 个、参考音频 3 个，混合素材共 12 个
const (
	V2MaxFrameImages     = 2
	V2MaxReferenceImages = 9
	V2MaxReferenceVideos = 3
	V2MaxReferenceAudios = 3
	V2MaxMixedAssets     = 12
)

// IsVideoV2Model 判断上游模型名是否走 v2（H3）协议。
// 前缀匹配以兼容后续 MiniMax-H3-xxx 变体；大小写不敏感。
func IsVideoV2Model(model string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(model)), V2ModelPrefix)
}
