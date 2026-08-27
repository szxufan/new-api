package dto

import (
	"encoding/json"
)

type TaskError struct {
	Code       string `json:"code"`
	Message    string `json:"message"`
	Data       any    `json:"data"`
	StatusCode int    `json:"-"`
	LocalError bool   `json:"-"`
	Error      error  `json:"-"`
}

type TaskData interface {
	SunoDataResponse | []SunoDataResponse | string | any
}

const TaskSuccessCode = "success"

type TaskResponse[T TaskData] struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Data    T      `json:"data"`
}

func (t *TaskResponse[T]) IsSuccess() bool {
	return t.Code == TaskSuccessCode
}

type TaskDto struct {
	ID         int64           `json:"id"`
	CreatedAt  int64           `json:"created_at"`
	UpdatedAt  int64           `json:"updated_at"`
	TaskID     string          `json:"task_id"`
	Platform   string          `json:"platform"`
	UserId     int             `json:"user_id"`
	Group      string          `json:"group"`
	ChannelId  int             `json:"channel_id"`
	Quota      int             `json:"quota"`
	Action     string          `json:"action"`
	Status     string          `json:"status"`
	FailReason string          `json:"fail_reason"`
	ResultURL  string          `json:"result_url,omitempty"` // 任务结果 URL（视频地址等）
	SubmitTime int64           `json:"submit_time"`
	StartTime  int64           `json:"start_time"`
	FinishTime int64           `json:"finish_time"`
	Progress   string          `json:"progress"`
	Properties any             `json:"properties"`
	Username   string          `json:"username,omitempty"`
	Data       json.RawMessage `json:"data"`
	// PollRecord 最后一次后端轮询上游的请求与响应，仅管理员任务列表接口填充
	PollRecord *TaskPollRecord `json:"poll_record,omitempty"`
}

// TaskPollRecord 最后一次后端轮询上游的请求与响应记录（与 model.TaskPollRecord 同形）
type TaskPollRecord struct {
	Time       int64           `json:"time"`                  // 轮询时间（unix 秒）
	Method     string          `json:"method,omitempty"`      // HTTP 方法（GET/POST）
	URL        string          `json:"url,omitempty"`         // 实际请求的上游 URL
	StatusCode int             `json:"status_code,omitempty"` // 上游 HTTP 状态码
	Request    json.RawMessage `json:"request,omitempty"`     // 轮询请求体
	Response   json.RawMessage `json:"response,omitempty"`    // 上游响应体（已脱敏/截断）
}

type FetchReq struct {
	IDs []string `json:"ids"`
}
