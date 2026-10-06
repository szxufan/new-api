package service

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/system_setting"
)

// dingTalkMessage 钉钉群机器人 markdown 消息体
type dingTalkMessage struct {
	MsgType  string          `json:"msgtype"`
	Markdown dingTalkMarkdown `json:"markdown"`
}

type dingTalkMarkdown struct {
	Title string `json:"title"`
	Text  string `json:"text"`
}

type dingTalkResponse struct {
	ErrCode int    `json:"errcode"`
	ErrMsg  string `json:"errmsg"`
}

// SendDingTalkNotify 发送钉钉群机器人消息（markdown 类型）。
// secret 非空时启用钉钉"加签"安全设置：对 timestamp+"\n"+secret 计算 HMAC-SHA256 并 base64 编码后拼入 URL。
func SendDingTalkNotify(webhookURL string, secret string, title string, text string) error {
	if webhookURL == "" {
		return fmt.Errorf("dingtalk webhook url is empty")
	}

	payloadBytes, err := common.Marshal(dingTalkMessage{
		MsgType:  "markdown",
		Markdown: dingTalkMarkdown{Title: title, Text: text},
	})
	if err != nil {
		return fmt.Errorf("failed to marshal dingtalk payload: %v", err)
	}

	if secret != "" {
		timestamp := time.Now().UnixMilli()
		stringToSign := fmt.Sprintf("%d\n%s", timestamp, secret)
		mac := hmac.New(sha256.New, []byte(secret))
		mac.Write([]byte(stringToSign))
		sign := base64.StdEncoding.EncodeToString(mac.Sum(nil))
		sep := "&"
		if !strings.Contains(webhookURL, "?") {
			sep = "?"
		}
		webhookURL = fmt.Sprintf("%s%stimestamp=%d&sign=%s", webhookURL, sep, timestamp, url.QueryEscape(sign))
	}

	var req *http.Request
	var resp *http.Response

	if system_setting.EnableWorker() {
		workerReq := &WorkerRequest{
			URL:    webhookURL,
			Key:    system_setting.WorkerValidKey,
			Method: http.MethodPost,
			Headers: map[string]string{
				"Content-Type": "application/json; charset=utf-8",
			},
			Body: payloadBytes,
		}

		resp, err = DoWorkerRequest(workerReq)
		if err != nil {
			return fmt.Errorf("failed to send dingtalk request through worker: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return fmt.Errorf("dingtalk request failed with status code: %d", resp.StatusCode)
		}
	} else {
		// SSRF防护：验证钉钉 webhook URL（非Worker模式）
		fetchSetting := system_setting.GetFetchSetting()
		if err := common.ValidateURLWithFetchSetting(webhookURL, fetchSetting.EnableSSRFProtection, fetchSetting.AllowPrivateIp, fetchSetting.DomainFilterMode, fetchSetting.IpFilterMode, fetchSetting.DomainList, fetchSetting.IpList, fetchSetting.AllowedPorts, fetchSetting.ApplyIPFilterForDomain); err != nil {
			return fmt.Errorf("request reject: %v", err)
		}

		req, err = http.NewRequest(http.MethodPost, webhookURL, bytes.NewBuffer(payloadBytes))
		if err != nil {
			return fmt.Errorf("failed to create dingtalk request: %v", err)
		}
		req.Header.Set("Content-Type", "application/json; charset=utf-8")

		client := GetHttpClient()
		resp, err = client.Do(req)
		if err != nil {
			return fmt.Errorf("failed to send dingtalk request: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return fmt.Errorf("dingtalk request failed with status code: %d", resp.StatusCode)
		}
	}

	// 钉钉无论成败都返回 200，需解析 errcode
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if err != nil {
		return fmt.Errorf("failed to read dingtalk response: %v", err)
	}
	var dr dingTalkResponse
	if err := common.Unmarshal(body, &dr); err != nil {
		return fmt.Errorf("failed to parse dingtalk response: %v", err)
	}
	if dr.ErrCode != 0 {
		return fmt.Errorf("dingtalk notify failed: errcode=%d errmsg=%s", dr.ErrCode, dr.ErrMsg)
	}
	return nil
}
