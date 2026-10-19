package dto

// DeepSeek 网页搜索 / 抓取工具 API（POST /v1/tools/search、POST /v1/tools/fetch）
// 的请求与响应结构，供 MCP web_search / web_fetch 工具中继使用。
// 文档：https://api-docs.deepseek.com/zh-cn/api/search 、/zh-cn/api/fetch

// WebSearchRequest 网页搜索请求。
// 可选标量使用指针 + omitempty（规则 6）：显式零值仍需发送到上游。
type WebSearchRequest struct {
	BaseRequest
	// Model 虚拟模型名（deepseek-search），仅用于路由与计费，不发送到上游
	Model string `json:"-"`
	// Query 搜索词，1~128 个 Unicode 字符，不能全为空白
	Query string `json:"query"`
	// Count 返回条数，1~50，上游默认 10
	Count *int `json:"count,omitempty"`
	// OriQuestion 触发搜索的用户原始问题，有助于上游理解搜索意图
	OriQuestion *string `json:"ori_question,omitempty"`
	// NeedContent 是否返回网页正文（默认 false，此时 content 为空字符串）
	NeedContent *bool `json:"need_content,omitempty"`
}

func (r *WebSearchRequest) SetModelName(modelName string) {
	r.Model = modelName
}

// WebSearchResult 单条搜索结果
type WebSearchResult struct {
	// RerankScore 排序分数 0~3，>=1.5 为优质结果
	RerankScore float64 `json:"rerank_score"`
	Title       string  `json:"title"`
	Url         string  `json:"url"`
	// Snippet 短摘要（约 100~200 字符）
	Snippet string `json:"snippet"`
	// Summary 长摘要（约 600~800 字符，适合供模型阅读）
	Summary string `json:"summary"`
	// Content 网页正文；need_content 为 false 或缺失时为空字符串
	Content  string  `json:"content"`
	SiteName *string `json:"site_name"`
	SiteIcon *string `json:"site_icon"`
	// Date 发布时间（UTC RFC 3339），可为 null
	Date *string `json:"date"`
	// Authority 权威性 0~4（-1 表示未知）
	Authority int `json:"authority"`
}

// WebSearchResponse 网页搜索响应
type WebSearchResponse struct {
	// Results 无匹配结果时为空数组
	Results []WebSearchResult `json:"results"`
}

// WebFetchRequest 网页抓取请求
type WebFetchRequest struct {
	BaseRequest
	// Model 虚拟模型名（deepseek-fetch），仅用于路由与计费，不发送到上游
	Model string `json:"-"`
	// Url 需为 HTTP/HTTPS 且含 host 的 URL，不能带用户名或密码，长度 <= 8192
	Url string `json:"url"`
}

func (r *WebFetchRequest) SetModelName(modelName string) {
	r.Model = modelName
}

// WebFetchResponse 网页抓取响应，Content 为抓取后的 Markdown 正文
type WebFetchResponse struct {
	Title   string `json:"title"`
	Content string `json:"content"`
}
