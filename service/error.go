package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/types"
)

func MidjourneyErrorWrapper(code int, desc string) *dto.MidjourneyResponse {
	return &dto.MidjourneyResponse{
		Code:        code,
		Description: desc,
	}
}

func MidjourneyErrorWithStatusCodeWrapper(code int, desc string, statusCode int) *dto.MidjourneyResponseWithStatusCode {
	return &dto.MidjourneyResponseWithStatusCode{
		StatusCode: statusCode,
		Response:   *MidjourneyErrorWrapper(code, desc),
	}
}

//// OpenAIErrorWrapper wraps an error into an OpenAIErrorWithStatusCode
//func OpenAIErrorWrapper(err error, code string, statusCode int) *dto.OpenAIErrorWithStatusCode {
//	text := err.Error()
//	lowerText := strings.ToLower(text)
//	if !strings.HasPrefix(lowerText, "get file base64 from url") && !strings.HasPrefix(lowerText, "mime type is not supported") {
//		if strings.Contains(lowerText, "post") || strings.Contains(lowerText, "dial") || strings.Contains(lowerText, "http") {
//			common.SysLog(fmt.Sprintf("error: %s", text))
//			text = "请求上游地址失败"
//		}
//	}
//	openAIError := dto.OpenAIError{
//		Message: text,
//		Type:    "new_api_error",
//		Code:    code,
//	}
//	return &dto.OpenAIErrorWithStatusCode{
//		Error:      openAIError,
//		StatusCode: statusCode,
//	}
//}
//
//func OpenAIErrorWrapperLocal(err error, code string, statusCode int) *dto.OpenAIErrorWithStatusCode {
//	openaiErr := OpenAIErrorWrapper(err, code, statusCode)
//	openaiErr.LocalError = true
//	return openaiErr
//}

func ClaudeErrorWrapper(err error, code string, statusCode int) *dto.ClaudeErrorWithStatusCode {
	text := err.Error()
	lowerText := strings.ToLower(text)
	if !strings.HasPrefix(lowerText, "get file base64 from url") {
		if strings.Contains(lowerText, "post") || strings.Contains(lowerText, "dial") || strings.Contains(lowerText, "http") {
			common.SysLog(fmt.Sprintf("error: %s", text))
			text = "请求上游地址失败"
		}
	}
	claudeError := types.ClaudeError{
		Message: text,
		Type:    "new_api_error",
	}
	return &dto.ClaudeErrorWithStatusCode{
		Error:      claudeError,
		StatusCode: statusCode,
	}
}

func ClaudeErrorWrapperLocal(err error, code string, statusCode int) *dto.ClaudeErrorWithStatusCode {
	claudeErr := ClaudeErrorWrapper(err, code, statusCode)
	claudeErr.LocalError = true
	return claudeErr
}

func RelayErrorHandler(ctx context.Context, resp *http.Response, showBodyWhenFail bool) (newApiErr *types.NewAPIError) {
	newApiErr = types.InitOpenAIError(types.ErrorCodeBadResponseStatusCode, resp.StatusCode)

	// #region debug-point D:relay-error-handler-entry
	(func() {
		envBytes, _ := readDebugEnv()
		if envBytes != nil {
			go reportDebugEvent(envBytes, "D", "service/error.go:88", fmt.Sprintf("[DEBUG] RelayErrorHandler entry: resp.StatusCode=%d, newApiErr.StatusCode=%d", resp.StatusCode, newApiErr.StatusCode), map[string]any{"resp_status": resp.StatusCode, "err_status": newApiErr.StatusCode})
		}
	})()
	// #endregion

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return
	}
	CloseResponseBodyGracefully(resp)
	var errResponse dto.GeneralErrorResponse
	buildErrWithBody := func(message string) error {
		if message == "" {
			return fmt.Errorf("bad response status code %d, body: %s", resp.StatusCode, string(responseBody))
		}
		return fmt.Errorf("bad response status code %d, message: %s, body: %s", resp.StatusCode, message, string(responseBody))
	}

	err = common.Unmarshal(responseBody, &errResponse)
	if err != nil {
		if showBodyWhenFail {
			newApiErr.Err = buildErrWithBody("")
		} else {
			logger.LogError(ctx, fmt.Sprintf("bad response status code %d, body: %s", resp.StatusCode, string(responseBody)))
			newApiErr.Err = fmt.Errorf("bad response status code %d", resp.StatusCode)
		}
		return
	}

	if common.GetJsonType(errResponse.Error) == "object" {
		// General format error (OpenAI, Anthropic, Gemini, etc.)
		oaiError := errResponse.TryToOpenAIError()
		if oaiError != nil {
			newApiErr = types.WithOpenAIError(*oaiError, resp.StatusCode)
			if showBodyWhenFail {
				newApiErr.Err = buildErrWithBody(newApiErr.Error())
			}
			// #region debug-point D:relay-error-handler-return-withopenai
			(func() {
				envBytes, _ := readDebugEnv()
				if envBytes != nil {
					go reportDebugEvent(envBytes, "D", "service/error.go:133", fmt.Sprintf("[DEBUG] RelayErrorHandler return(WithOpenAIError): resp.StatusCode=%d, newApiErr.StatusCode=%d", resp.StatusCode, newApiErr.StatusCode), map[string]any{"resp_status": resp.StatusCode, "err_status": newApiErr.StatusCode})
				}
			})()
			// #endregion
			return
		}
	}
	newApiErr = types.NewOpenAIError(errors.New(errResponse.ToMessage()), types.ErrorCodeBadResponseStatusCode, resp.StatusCode)
	if showBodyWhenFail {
		newApiErr.Err = buildErrWithBody(newApiErr.Error())
	}
	// #region debug-point D:relay-error-handler-return-newopenai
	(func() {
		envBytes, _ := readDebugEnv()
		if envBytes != nil {
			go reportDebugEvent(envBytes, "D", "service/error.go:142", fmt.Sprintf("[DEBUG] RelayErrorHandler return(NewOpenAIError): resp.StatusCode=%d, newApiErr.StatusCode=%d", resp.StatusCode, newApiErr.StatusCode), map[string]any{"resp_status": resp.StatusCode, "err_status": newApiErr.StatusCode})
		}
	})()
	// #endregion
	return
}

func ResetStatusCode(newApiErr *types.NewAPIError, statusCodeMappingStr string) {
	if newApiErr == nil {
		return
	}
	// #region debug-point A:reset-status-code-entry
	(func() {
		envBytes, _ := readDebugEnv()
		if envBytes != nil {
			go reportDebugEvent(envBytes, "A", "service/error.go:144", fmt.Sprintf("[DEBUG] ResetStatusCode entry: original StatusCode=%d, mapping=%s", newApiErr.StatusCode, statusCodeMappingStr), map[string]any{"original_status": newApiErr.StatusCode, "mapping": statusCodeMappingStr})
		}
	})()
	// #endregion
	if statusCodeMappingStr == "" || statusCodeMappingStr == "{}" {
		return
	}
	statusCodeMapping := make(map[string]any)
	err := common.Unmarshal([]byte(statusCodeMappingStr), &statusCodeMapping)
	if err != nil {
		return
	}
	if newApiErr.StatusCode == http.StatusOK {
		return
	}
	codeStr := strconv.Itoa(newApiErr.StatusCode)
	if value, ok := statusCodeMapping[codeStr]; ok {
		intCode, ok := parseStatusCodeMappingValue(value)
		if !ok {
			return
		}
		// #region debug-point A:reset-status-code-change
		(func() {
			envBytes, _ := readDebugEnv()
			if envBytes != nil {
				go reportDebugEvent(envBytes, "A", "service/error.go:163", fmt.Sprintf("[DEBUG] ResetStatusCode CHANGED: %d -> %d", newApiErr.StatusCode, intCode), map[string]any{"from": newApiErr.StatusCode, "to": intCode})
			}
		})()
		// #endregion
		newApiErr.StatusCode = intCode
	}
}

func parseStatusCodeMappingValue(value any) (int, bool) {
	switch v := value.(type) {
	case string:
		if v == "" {
			return 0, false
		}
		statusCode, err := strconv.Atoi(v)
		if err != nil {
			return 0, false
		}
		return statusCode, true
	case float64:
		if v != math.Trunc(v) {
			return 0, false
		}
		return int(v), true
	case int:
		return v, true
	case json.Number:
		statusCode, err := strconv.Atoi(v.String())
		if err != nil {
			return 0, false
		}
		return statusCode, true
	default:
		return 0, false
	}
}

func TaskErrorWrapperLocal(err error, code string, statusCode int) *dto.TaskError {
	openaiErr := TaskErrorWrapper(err, code, statusCode)
	openaiErr.LocalError = true
	return openaiErr
}

func TaskErrorWrapper(err error, code string, statusCode int) *dto.TaskError {
	text := err.Error()
	lowerText := strings.ToLower(text)
	if strings.Contains(lowerText, "post") || strings.Contains(lowerText, "dial") || strings.Contains(lowerText, "http") {
		common.SysLog(fmt.Sprintf("error: %s", text))
		//text = "请求上游地址失败"
		text = common.MaskSensitiveInfo(text)
	}
	//避免暴露内部错误
	taskError := &dto.TaskError{
		Code:       code,
		Message:    text,
		StatusCode: statusCode,
		Error:      err,
	}

	return taskError
}

// TaskErrorFromAPIError 将 PreConsumeBilling 返回的 NewAPIError 转换为 TaskError。
func TaskErrorFromAPIError(apiErr *types.NewAPIError) *dto.TaskError {
	if apiErr == nil {
		return nil
	}
	return &dto.TaskError{
		Code:       string(apiErr.GetErrorCode()),
		Message:    apiErr.Err.Error(),
		StatusCode: apiErr.StatusCode,
		Error:      apiErr.Err,
	}
}

// readDebugEnv reads the debug env file and returns its content as a struct.
// #region debug-point debug-helpers
func ReadDebugEnv() (map[string]string, error) {
	return readDebugEnv()
}

func ReportDebugEvent(env map[string]string, hypothesisId, location, msg string, dataMap map[string]any) {
	reportDebugEvent(env, hypothesisId, location, msg, dataMap)
}

func readDebugEnv() (map[string]string, error) {
	data, err := os.ReadFile("/home/xufan/trea_project/new-api/.dbg/429-fallback.env")
	if err != nil {
		return nil, err
	}
	m := make(map[string]string)
	for _, line := range strings.Split(string(data), "\n") {
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			m[parts[0]] = parts[1]
		}
	}
	return m, nil
}

func reportDebugEvent(env map[string]string, hypothesisId, location, msg string, dataMap map[string]any) {
	url := env["DEBUG_SERVER_URL"]
	session := env["DEBUG_SESSION_ID"]
	if url == "" || session == "" {
		return
	}
	payload := map[string]any{
		"sessionId":     session,
		"runId":          "pre-fix",
		"hypothesisId":   hypothesisId,
		"location":       location,
		"msg":            msg,
		"data":           dataMap,
		"ts":             jsonTimeNowMs(),
		"traceId":        "",
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return
	}
	req, err := http.NewRequest("POST", url, strings.NewReader(string(body)))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return
	}
	resp.Body.Close()
}

func jsonTimeNowMs() int64 {
	return time.Now().UnixNano() / 1e6
}

// #endregion
