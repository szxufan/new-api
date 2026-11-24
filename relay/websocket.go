package relay

import (
	"bytes"
	"fmt"
	"net/http"

	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

func WssHelper(c *gin.Context, info *relaycommon.RelayInfo) (newAPIError *types.NewAPIError) {
	info.InitChannelMeta(c)
	if firstMessage, ok := c.Get("first_wss_request"); ok {
		if message, valid := firstMessage.([]byte); valid {
			info.FirstWsMessage = message
			info.FirstWsMessageType = c.GetInt("first_wss_request_type")
		}
	}

	adaptor := GetAdaptor(info.ApiType)
	if adaptor == nil {
		return types.NewError(fmt.Errorf("invalid api type: %d", info.ApiType), types.ErrorCodeInvalidApiType, types.ErrOptionWithSkipRetry())
	}
	adaptor.Init(info)

	statusCodeMappingStr := c.GetString("status_code_mapping")
	resp, err := adaptor.DoRequest(c, info, bytes.NewReader(nil))
	if err != nil {
		return types.NewError(err, types.ErrorCodeDoRequestFailed)
	}

	if resp != nil {
		targetWs, ok := resp.(*websocket.Conn)
		if !ok || targetWs == nil {
			// 非 WebSocket 渠道可能进入普通 HTTP 分支，不能让类型断言 panic；
			// 返回可重试错误，让控制器切换到支持该协议的其他渠道。
			if httpResp, isHTTP := resp.(*http.Response); isHTTP {
				service.CloseResponseBodyGracefully(httpResp)
			}
			return types.NewError(fmt.Errorf("adaptor returned %T for websocket relay", resp), types.ErrorCodeInvalidApiType)
		}
		info.TargetWs = targetWs
		defer info.TargetWs.Close()
	}

	usage, newAPIError := adaptor.DoResponse(c, nil, info)
	if newAPIError != nil {
		// reset status code 重置状态码
		service.ResetStatusCode(newAPIError, statusCodeMappingStr)
		return newAPIError
	}
	service.PostWssConsumeQuota(c, info, info.UpstreamModelName, usage.(*dto.RealtimeUsage), "")
	return nil
}
