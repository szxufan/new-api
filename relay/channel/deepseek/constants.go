package deepseek

var ModelList = []string{
	// 新命名（deepseek-flash 为当前主力模型，默认开启思考模式；-none/-max 后缀切换思考模式）
	"deepseek-flash", "deepseek-flash-none", "deepseek-flash-max",
	// 旧命名：上游已下线但仍可调用（请求由新模型服务并按新价格计费），保留以兼容存量渠道
	"deepseek-chat", "deepseek-reasoner",
	"deepseek-v4-flash", "deepseek-v4-flash-none", "deepseek-v4-flash-max",
	"deepseek-v4-pro", "deepseek-v4-pro-none", "deepseek-v4-pro-max",
	// 网页搜索/抓取工具端点（/v1/tools/search、/v1/tools/fetch）对应的虚拟模型，
	// 供 MCP web_search / web_fetch 按分组路由与按次计费
	"deepseek-search", "deepseek-fetch",
}

var ChannelName = "deepseek"
