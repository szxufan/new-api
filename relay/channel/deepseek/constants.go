package deepseek

var ModelList = []string{
	"deepseek-chat", "deepseek-reasoner",
	"deepseek-v4-flash", "deepseek-v4-flash-none", "deepseek-v4-flash-max",
	"deepseek-v4-pro", "deepseek-v4-pro-none", "deepseek-v4-pro-max",
	// 网页搜索/抓取工具端点（/v1/tools/search、/v1/tools/fetch）对应的虚拟模型，
	// 供 MCP web_search / web_fetch 按分组路由与按次计费
	"deepseek-search", "deepseek-fetch",
}

var ChannelName = "deepseek"
