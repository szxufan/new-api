package helper

import (
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
)

// StreamConsumedAmountEnabled token 是否开启"响应返回消耗金额"
func StreamConsumedAmountEnabled(c *gin.Context) bool {
	return common.GetContextKeyBool(c, constant.ContextKeyTokenReturnConsumedQuota)
}

// CalculateStreamConsumedAmount 计算流式请求的消耗金额字符串（token 未开启或无 usage 时返回空串）。
// 与结算共用同一计算函数，保证扩展字段数值与日志消耗一致。
func CalculateStreamConsumedAmount(c *gin.Context, info *relaycommon.RelayInfo, usage *dto.Usage) string {
	if usage == nil || !StreamConsumedAmountEnabled(c) {
		return ""
	}
	return service.CalculateTextConsumedAmount(c, info, usage)
}

// SendFinalUsageResponse 生成最终 usage chunk 并发送给客户端。
// token 开启"响应返回消耗金额"时在 chunk 上附加 x_new_api_consumed_amount 扩展字段。
// 客户端最终 usage 一律由网关用归一化后的权威数值合成，上游 usage 块不透传。
func SendFinalUsageResponse(c *gin.Context, info *relaycommon.RelayInfo, id string, createAt int64, model string, usage *dto.Usage) error {
	if usage == nil {
		return nil
	}
	response := GenerateFinalUsageResponse(id, createAt, model, *usage)
	response.XNewApiConsumedAmount = CalculateStreamConsumedAmount(c, info, usage)
	return ObjectData(c, response)
}
