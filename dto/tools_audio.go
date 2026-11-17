package dto

import (
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/types"
)

// DeepSeek 原生语音工具 API（POST /v1/tools/tts、POST /v1/tools/asr）的请求结构，
// 供网关原生 /v1/tools/* 端点中继使用（请求体原样透传上游）。
// 文档：https://api-docs.deepseek.com/zh-cn/api/tts 、/zh-cn/api/asr

// ToolsTTSRequest 原生语音合成请求。
// 可选标量使用指针 + omitempty（规则 6）：显式零值仍需发送到上游。
type ToolsTTSRequest struct {
	BaseRequest
	// Model 虚拟模型名（deepseek-tts），仅用于路由与计费，不发送到上游
	Model string `json:"-"`
	// Text 要朗读的文本，不能为空
	Text string `json:"text"`
	// VoiceID 音色，上游默认 stella（女声），另支持 tide（男声）
	VoiceID string `json:"voice_id,omitempty"`
	// Format 音频格式，wav（默认）或 pcm
	Format string `json:"format,omitempty"`
	// FilterMarkdown 朗读前是否去除 Markdown 格式，上游默认 true
	FilterMarkdown *bool `json:"filter_markdown,omitempty"`
}

func (r *ToolsTTSRequest) SetModelName(modelName string) {
	r.Model = modelName
}

// GetTokenCountMeta 预扣费按文本字符数估算（与 OpenAI 兼容路径 dto.AudioRequest 一致）
func (r *ToolsTTSRequest) GetTokenCountMeta() *types.TokenCountMeta {
	return &types.TokenCountMeta{
		CombineText: r.Text,
		TokenType:   types.TokenTypeTextNumber,
	}
}

// Validate 校验合成请求参数（与上游限制一致）
func (r *ToolsTTSRequest) Validate() error {
	if strings.TrimSpace(r.Text) == "" {
		return fmt.Errorf("text is required")
	}
	return nil
}

// ToolsASRRequest 原生语音识别请求。
// 上游仅接受 multipart 的 file 字段，音频文件由请求体携带，DTO 只承载路由/计费用的虚拟模型名。
type ToolsASRRequest struct {
	BaseRequest
	// Model 虚拟模型名（deepseek-asr），仅用于路由与计费，不发送到上游
	Model string `json:"-"`
}

func (r *ToolsASRRequest) SetModelName(modelName string) {
	r.Model = modelName
}
