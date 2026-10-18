package dto

import (
	"encoding/json"
	"strings"

	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
)

// System One 决策模型协议，阿里云百炼（compatible-mode/v1/systemone）与
// Ollama（/v1/systemone）共用同一套请求/响应结构：
// 输入为 state + questions（choice/noul/score 三种题型），非流式，
// 一次返回带概率分布的结构化判定 answers。
// 参考：https://docs.ollama.com/api/systemone 、阿里云百炼 decision-model-api

const (
	SystemOneQuestionTypeChoice = "choice"
	SystemOneQuestionTypeNoul   = "noul"
	SystemOneQuestionTypeScore  = "score"
)

type SystemOneQuestion struct {
	Type         string          `json:"type"`                    // choice / noul / score
	Instructions string          `json:"instructions,omitempty"`  // 问题描述或评判标准
	Criteria     json.RawMessage `json:"criteria,omitempty"`      // choice: 选项名→描述对象；score: 由低到高的等级描述数组；noul: 可选 {"true":...,"false":...}
}

type SystemOneRequest struct {
	Model     string                       `json:"model"`
	State     json.RawMessage              `json:"state"`                // 待决策的业务状态，string / object / array
	Questions map[string]SystemOneQuestion `json:"questions"`            // key 为自定义问题 id
	Images    []string                     `json:"images,omitempty"`     // Ollama: base64 图片，共享给所有问题
	KeepAlive json.RawMessage              `json:"keep_alive,omitempty"` // Ollama: 模型驻留时长，string|number
}

func (r *SystemOneRequest) IsStream(c *gin.Context) bool {
	return false
}

func (r *SystemOneRequest) GetTokenCountMeta() *types.TokenCountMeta {
	var sb strings.Builder
	sb.Write(r.State)
	for name, question := range r.Questions {
		sb.WriteString("\n")
		sb.WriteString(name)
		sb.WriteString("\n")
		sb.WriteString(question.Instructions)
		sb.Write(question.Criteria)
	}
	return &types.TokenCountMeta{
		CombineText: sb.String(),
	}
}

func (r *SystemOneRequest) SetModelName(modelName string) {
	if modelName != "" {
		r.Model = modelName
	}
}

type SystemOneUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens,omitempty"` // Ollama 返回内部评分生成的 token；百炼不返回
}

type SystemOneAnswer struct {
	Type          string             `json:"type"`
	Choice        *string            `json:"choice,omitempty"`        // choice: 概率最高的选项名
	Noul          *float64           `json:"noul,omitempty"`          // noul: P(yes)，0=否 1=是
	Score         *float64           `json:"score,omitempty"`         // score: 概率加权期望，可落在两级之间
	Probabilities map[string]float64 `json:"probabilities,omitempty"` // 各选项/等级概率，和为 1
	Confidence    *float64           `json:"confidence,omitempty"`    // 分布集中度置信度
	Legend        map[string]string  `json:"legend,omitempty"`        // score: 等级索引→等级描述
}

type SystemOneResponse struct {
	Model     string                     `json:"model"`
	RequestID string                     `json:"request_id,omitempty"` // 百炼返回
	Answers   map[string]SystemOneAnswer `json:"answers"`
	Usage     SystemOneUsage             `json:"usage"`
	LatencyMs *float64                   `json:"latency_ms,omitempty"` // 百炼返回
}
