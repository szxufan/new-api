package ali

var ModelList = []string{
	"wan3.0-video",              // 万相3.0 标准版（全能参考，最长30秒）推荐
	"wan3.0-video-prime",        // 万相3.0 高速版（能力对齐标准版，端到端速度显著提升）
	"wan2.5-i2v-preview",        // 万相2.5 preview（有声视频）推荐
	"wan2.2-i2v-flash",          // 万相2.2极速版（无声视频）
	"wan2.2-i2v-plus",           // 万相2.2专业版（无声视频）
	"wanx2.1-i2v-plus",          // 万相2.1专业版（无声视频）
	"wanx2.1-i2v-turbo",         // 万相2.1极速版（无声视频）
	"happyhorse-1.0-t2v",        // HappyHorse 文生视频
	"happyhorse-1.0-i2v",        // HappyHorse 图生视频（首帧）
	"happyhorse-1.0-r2v",        // HappyHorse 参考生视频
	"happyhorse-1.0-video-edit", // HappyHorse 视频编辑
	// 百炼第三方托管模型：仅华北2（北京）地域，渠道 base URL 必须为
	// https://{WorkspaceId}.cn-beijing.maas.aliyuncs.com，详见 docs/ali-bailian-minimax-video.md
	"MiniMax/MiniMax-H3", // MiniMax H3 视频生成（DashScope 异步视频协议）
}

var ChannelName = "ali"
