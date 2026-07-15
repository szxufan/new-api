package xunfei_maas

// ModelList 讯飞MaaS平台支持的模型列表
// 讯飞MaaS平台通过 /v2/models 接口动态获取可用模型，此处仅提供常见模型作为参考
var ModelList = []string{
	"spark-4.0-ultra",
	"spark-4.0-ultra-128k",
	"spark-max",
	"spark-max-128k",
	"spark-max-32k",
	"spark-lite",
	"spark-lite-128k",
	"spark-3.5-max",
	"spark-3.5-max-128k",
	"generalv3.5",
	"generalv3",
	"4.0Ultra",
	"max-32k",
	"max-128k",
}

// ChannelName 渠道名称
var ChannelName = "xunfei_maas"
