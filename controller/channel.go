package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	relaychannel "github.com/QuantumNous/new-api/relay/channel"
	"github.com/QuantumNous/new-api/relay/channel/gemini"
	"github.com/QuantumNous/new-api/relay/channel/ollama"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type OpenAIModel struct {
	ID         string         `json:"id"`
	Object     string         `json:"object"`
	Created    int64          `json:"created"`
	OwnedBy    string         `json:"owned_by"`
	Metadata   map[string]any `json:"metadata,omitempty"`
	Permission []struct {
		ID                 string `json:"id"`
		Object             string `json:"object"`
		Created            int64  `json:"created"`
		AllowCreateEngine  bool   `json:"allow_create_engine"`
		AllowSampling      bool   `json:"allow_sampling"`
		AllowLogprobs      bool   `json:"allow_logprobs"`
		AllowSearchIndices bool   `json:"allow_search_indices"`
		AllowView          bool   `json:"allow_view"`
		AllowFineTuning    bool   `json:"allow_fine_tuning"`
		Organization       string `json:"organization"`
		Group              string `json:"group"`
		IsBlocking         bool   `json:"is_blocking"`
	} `json:"permission"`
	Root   string `json:"root"`
	Parent string `json:"parent"`
}

type OpenAIModelsResponse struct {
	Data    []OpenAIModel `json:"data"`
	Success bool          `json:"success"`
}

func parseStatusFilter(statusParam string) int {
	switch strings.ToLower(statusParam) {
	case "enabled", "1":
		return common.ChannelStatusEnabled
	case "disabled", "0":
		return 0
	default:
		return -1
	}
}

func clearChannelInfo(channel *model.Channel) {
	if channel.ChannelInfo.IsMultiKey {
		channel.ChannelInfo.MultiKeyDisabledReason = nil
		channel.ChannelInfo.MultiKeyDisabledTime = nil
	}
}

func applyChannelStatusFilter(query *gorm.DB, statusFilter int) *gorm.DB {
	if statusFilter == common.ChannelStatusEnabled {
		return query.Where("status = ?", common.ChannelStatusEnabled)
	}
	if statusFilter == 0 {
		return query.Where("status != ?", common.ChannelStatusEnabled)
	}
	return query
}

func buildChannelListQuery(group string, statusFilter int, typeFilter int) *gorm.DB {
	query := model.DB.Model(&model.Channel{})
	query = model.ApplyChannelGroupFilter(query, group)
	query = applyChannelStatusFilter(query, statusFilter)
	if typeFilter >= 0 {
		query = query.Where("type = ?", typeFilter)
	}
	return query
}

// sortChannelsByAffinityCount 按亲和性计数（升/降序）稳定排序渠道列表。
// 亲和性计数为运行期计算值，需在 service.FillChannelAffinityCounts 之后调用。
func sortChannelsByAffinityCount(channels []*model.Channel, desc bool) {
	sort.SliceStable(channels, func(i, j int) bool {
		if desc {
			return channels[i].AffinityCount > channels[j].AffinityCount
		}
		return channels[i].AffinityCount < channels[j].AffinityCount
	})
}

// tagChannelRange 记录 Tag 模式下某个标签的渠道在聚合列表中的切片范围
type tagChannelRange struct {
	tag   string
	start int
	end   int
	total int64
}

// applyTagAffinityOrder 对按 Tag 顺序收集的渠道列表，按每个 Tag 组的亲和性计数合计重新排序。
// ranges 需按收集顺序传入；排序稳定，合计相同时保持原有 Tag 顺序。
func applyTagAffinityOrder(channels []*model.Channel, ranges []tagChannelRange, desc bool) {
	if len(ranges) < 2 {
		return
	}
	for i := range ranges {
		var total int64
		for j := ranges[i].start; j < ranges[i].end; j++ {
			total += channels[j].AffinityCount
		}
		ranges[i].total = total
	}
	sort.SliceStable(ranges, func(i, j int) bool {
		if ranges[i].total == ranges[j].total {
			return false
		}
		if desc {
			return ranges[i].total > ranges[j].total
		}
		return ranges[i].total < ranges[j].total
	})
	reordered := make([]*model.Channel, 0, len(channels))
	for _, r := range ranges {
		reordered = append(reordered, channels[r.start:r.end]...)
	}
	copy(channels, reordered)
}

func GetAllChannels(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	channelData := make([]*model.Channel, 0)
	idSort, _ := strconv.ParseBool(c.Query("id_sort"))
	sortOptions := model.NewChannelSortOptions(c.Query("sort_by"), c.Query("sort_order"), idSort)
	enableTagMode, _ := strconv.ParseBool(c.Query("tag_mode"))
	groupFilter := model.NormalizeChannelGroupFilter(c.Query("group"))
	statusParam := c.Query("status")
	// statusFilter: -1 all, 1 enabled, 0 disabled (include auto & manual)
	statusFilter := parseStatusFilter(statusParam)
	// type filter
	typeStr := c.Query("type")
	typeFilter := -1
	if typeStr != "" {
		if t, err := strconv.Atoi(typeStr); err == nil {
			typeFilter = t
		}
	}

	var total int64
	var tagRanges []tagChannelRange

	if enableTagMode {
		tags, err := model.GetPaginatedChannelTags(buildChannelListQuery(groupFilter, statusFilter, typeFilter), pageInfo.GetStartIdx(), pageInfo.GetPageSize())
		if err != nil {
			common.SysError("failed to get paginated tags: " + err.Error())
			c.JSON(http.StatusOK, gin.H{"success": false, "message": "获取标签失败，请稍后重试"})
			return
		}
		total, err = model.CountChannelTags(buildChannelListQuery(groupFilter, statusFilter, typeFilter))
		if err != nil {
			common.SysError("failed to count tags: " + err.Error())
			c.JSON(http.StatusOK, gin.H{"success": false, "message": "获取标签数量失败，请稍后重试"})
			return
		}
		for _, tag := range tags {
			if tag == nil || *tag == "" {
				continue
			}
			var tagChannels []*model.Channel
			err := sortOptions.Apply(buildChannelListQuery(groupFilter, statusFilter, typeFilter).Where("tag = ?", *tag)).
				Omit("key").
				Find(&tagChannels).Error
			if err != nil {
				common.SysError("failed to get channels by tag: " + err.Error())
				c.JSON(http.StatusOK, gin.H{"success": false, "message": "获取标签渠道失败，请稍后重试"})
				return
			}
			tagRanges = append(tagRanges, tagChannelRange{
				tag:   *tag,
				start: len(channelData),
				end:   len(channelData) + len(tagChannels),
			})
			channelData = append(channelData, tagChannels...)
		}
	} else {
		if err := buildChannelListQuery(groupFilter, statusFilter, typeFilter).Count(&total).Error; err != nil {
			common.SysError("failed to count channels: " + err.Error())
			c.JSON(http.StatusOK, gin.H{"success": false, "message": "获取渠道数量失败，请稍后重试"})
			return
		}

		if sortOptions.SortByAffinityCount {
			// 亲和性计数非数据库列：全量查询后在内存中排序，再手动分页
			err := buildChannelListQuery(groupFilter, statusFilter, typeFilter).
				Omit("key").
				Find(&channelData).Error
			if err != nil {
				common.SysError("failed to get channels: " + err.Error())
				c.JSON(http.StatusOK, gin.H{"success": false, "message": "获取渠道列表失败，请稍后重试"})
				return
			}
		} else {
			err := sortOptions.Apply(buildChannelListQuery(groupFilter, statusFilter, typeFilter)).
				Limit(pageInfo.GetPageSize()).
				Offset(pageInfo.GetStartIdx()).
				Omit("key").
				Find(&channelData).Error
			if err != nil {
				common.SysError("failed to get channels: " + err.Error())
				c.JSON(http.StatusOK, gin.H{"success": false, "message": "获取渠道列表失败，请稍后重试"})
				return
			}
		}
	}

	for _, datum := range channelData {
		clearChannelInfo(datum)
	}
	service.FillChannelAffinityCounts(channelData)

	if sortOptions.SortByAffinityCount {
		desc := sortOptions.SortOrder != "asc"
		if enableTagMode {
			// 按每个 Tag 组的亲和性计数合计重排 Tag 顺序
			applyTagAffinityOrder(channelData, tagRanges, desc)
		} else {
			// 内存排序后手动分页
			sortChannelsByAffinityCount(channelData, desc)
			startIdx := pageInfo.GetStartIdx()
			endIdx := startIdx + pageInfo.GetPageSize()
			if endIdx > len(channelData) {
				endIdx = len(channelData)
			}
			if startIdx > endIdx {
				startIdx = endIdx
			}
			channelData = channelData[startIdx:endIdx]
		}
	}

	countQuery := buildChannelListQuery(groupFilter, statusFilter, -1)
	var results []struct {
		Type  int64
		Count int64
	}
	if err := countQuery.Select("type, count(*) as count").Group("type").Find(&results).Error; err != nil {
		common.SysError("failed to count channel types: " + err.Error())
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "获取渠道类型统计失败，请稍后重试"})
		return
	}
	typeCounts := make(map[int64]int64)
	for _, r := range results {
		typeCounts[r.Type] = r.Count
	}
	common.ApiSuccess(c, gin.H{
		"items":       channelData,
		"total":       total,
		"page":        pageInfo.GetPage(),
		"page_size":   pageInfo.GetPageSize(),
		"type_counts": typeCounts,
	})
}

func buildFetchModelsHeaders(channel *model.Channel, key string) (http.Header, error) {
	var headers http.Header
	switch channel.Type {
	case constant.ChannelTypeAnthropic:
		headers = GetClaudeAuthHeader(key)
	default:
		headers = GetAuthHeader(key)
	}

	headerOverride := channel.GetHeaderOverride()
	for k, v := range headerOverride {
		if relaychannel.IsHeaderPassthroughRuleKey(k) {
			continue
		}
		str, ok := v.(string)
		if !ok {
			return nil, fmt.Errorf("invalid header override for key %s", k)
		}
		if strings.Contains(str, "{api_key}") {
			str = strings.ReplaceAll(str, "{api_key}", key)
		}
		headers.Set(k, str)
	}

	return headers, nil
}

func FetchUpstreamModels(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}

	channel, err := model.GetChannelById(id, true)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	ids, err := fetchChannelUpstreamModelIDs(channel)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": fmt.Sprintf("获取模型列表失败: %s", err.Error()),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    ids,
	})
}

func FixChannelsAbilities(c *gin.Context) {
	success, fails, err := model.FixAbility()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"success": success,
			"fails":   fails,
		},
	})
}

func SearchChannels(c *gin.Context) {
	keyword := c.Query("keyword")
	group := c.Query("group")
	modelKeyword := c.Query("model")
	statusParam := c.Query("status")
	statusFilter := parseStatusFilter(statusParam)
	idSort, _ := strconv.ParseBool(c.Query("id_sort"))
	sortOptions := model.NewChannelSortOptions(c.Query("sort_by"), c.Query("sort_order"), idSort)
	enableTagMode, _ := strconv.ParseBool(c.Query("tag_mode"))
	channelData := make([]*model.Channel, 0)
	var tagRanges []tagChannelRange
	if enableTagMode {
		tags, err := model.SearchTags(keyword, group, modelKeyword, idSort)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": err.Error(),
			})
			return
		}
		for _, tag := range tags {
			if tag != nil && *tag != "" {
				var tagChannels []*model.Channel
				err := sortOptions.Apply(buildChannelListQuery(group, -1, -1).Where("tag = ?", *tag)).
					Omit("key").
					Find(&tagChannels).Error
				if err != nil {
					c.JSON(http.StatusOK, gin.H{
						"success": false,
						"message": err.Error(),
					})
					return
				}
				tagRanges = append(tagRanges, tagChannelRange{
					tag:   *tag,
					start: len(channelData),
					end:   len(channelData) + len(tagChannels),
				})
				channelData = append(channelData, tagChannels...)
			}
		}
	} else {
		channels, err := model.SearchChannels(keyword, group, modelKeyword, idSort, sortOptions)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": err.Error(),
			})
			return
		}
		channelData = channels
	}

	if statusFilter == common.ChannelStatusEnabled || statusFilter == 0 {
		filtered := make([]*model.Channel, 0, len(channelData))
		for _, ch := range channelData {
			if statusFilter == common.ChannelStatusEnabled && ch.Status != common.ChannelStatusEnabled {
				continue
			}
			if statusFilter == 0 && ch.Status == common.ChannelStatusEnabled {
				continue
			}
			filtered = append(filtered, ch)
		}
		channelData = filtered
	}

	// calculate type counts for search results
	typeCounts := make(map[int64]int64)
	for _, channel := range channelData {
		typeCounts[int64(channel.Type)]++
	}

	typeParam := c.Query("type")
	typeFilter := -1
	if typeParam != "" {
		if tp, err := strconv.Atoi(typeParam); err == nil {
			typeFilter = tp
		}
	}

	if typeFilter >= 0 {
		filtered := make([]*model.Channel, 0, len(channelData))
		for _, ch := range channelData {
			if ch.Type == typeFilter {
				filtered = append(filtered, ch)
			}
		}
		channelData = filtered
	}

	page, _ := strconv.Atoi(c.DefaultQuery("p", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}

	if sortOptions.SortByAffinityCount {
		// 亲和性计数非数据库列：全量填充计数后在内存中排序
		service.FillChannelAffinityCounts(channelData)
		desc := sortOptions.SortOrder != "asc"
		if enableTagMode {
			// 按每个 Tag 组的亲和性计数合计重排 Tag 顺序
			applyTagAffinityOrder(channelData, tagRanges, desc)
		} else {
			sortChannelsByAffinityCount(channelData, desc)
		}
	}

	total := len(channelData)
	startIdx := (page - 1) * pageSize
	if startIdx > total {
		startIdx = total
	}
	endIdx := startIdx + pageSize
	if endIdx > total {
		endIdx = total
	}

	pagedData := channelData[startIdx:endIdx]

	for _, datum := range pagedData {
		clearChannelInfo(datum)
	}
	if !sortOptions.SortByAffinityCount {
		service.FillChannelAffinityCounts(pagedData)
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"items":       pagedData,
			"total":       total,
			"type_counts": typeCounts,
		},
	})
}

func GetChannel(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	channel, err := model.GetChannelById(id, false)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if channel != nil {
		clearChannelInfo(channel)
	}

	// 附带后备渠道信息
	type FallbackChannelInfo struct {
		Id   int    `json:"id"`
		Name string `json:"name"`
		Type int    `json:"type"`
	}
	fallbackInfos := make([]FallbackChannelInfo, 0)
	for _, fid := range channel.GetFallbackChannelIDs() {
		if fb, err := model.GetChannelById(fid, false); err == nil {
			fallbackInfos = append(fallbackInfos, FallbackChannelInfo{
				Id:   fb.Id,
				Name: fb.Name,
				Type: fb.Type,
			})
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success":           true,
		"message":           "",
		"data":              channel,
		"fallback_channels": fallbackInfos,
	})
}

// GetChannelKey 获取渠道密钥（需要通过安全验证中间件）
// 此函数依赖 SecureVerificationRequired 中间件，确保用户已通过安全验证
func GetChannelKey(c *gin.Context) {
	userId := c.GetInt("id")
	channelId, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiError(c, fmt.Errorf("渠道ID格式错误: %v", err))
		return
	}

	// 获取渠道信息（包含密钥）
	channel, err := model.GetChannelById(channelId, true)
	if err != nil {
		common.ApiError(c, fmt.Errorf("获取渠道信息失败: %v", err))
		return
	}

	if channel == nil {
		common.ApiError(c, fmt.Errorf("渠道不存在"))
		return
	}

	// 记录操作日志
	model.RecordLog(userId, model.LogTypeSystem, fmt.Sprintf("查看渠道密钥信息 (渠道ID: %d)", channelId))

	// 返回渠道密钥
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "获取成功",
		"data": map[string]interface{}{
			"key": channel.Key,
		},
	})
}

// validateTwoFactorAuth 统一的2FA验证函数
func validateTwoFactorAuth(twoFA *model.TwoFA, code string) bool {
	// 尝试验证TOTP
	if cleanCode, err := common.ValidateNumericCode(code); err == nil {
		if isValid, _ := twoFA.ValidateTOTPAndUpdateUsage(cleanCode); isValid {
			return true
		}
	}

	// 尝试验证备用码
	if isValid, err := twoFA.ValidateBackupCodeAndUpdateUsage(code); err == nil && isValid {
		return true
	}

	return false
}

// validateChannel 通用的渠道校验函数
func validateChannel(channel *model.Channel, isAdd bool) error {
	// 校验渠道专属重试次数：0=使用全局，-1=禁止重试，N>0=覆盖
	if channel.RetryTimes != nil && *channel.RetryTimes < -1 {
		return fmt.Errorf("渠道重试次数[retry_times] 不能小于 -1")
	}

	// 校验 channel settings
	if err := channel.ValidateSettings(); err != nil {
		return fmt.Errorf("渠道额外设置[channel setting] 格式错误：%s", err.Error())
	}

	// 如果是添加操作，检查 channel 和 key 是否为空
	if isAdd {
		if channel == nil || channel.Key == "" {
			return fmt.Errorf("channel cannot be empty")
		}

		// 检查模型名称长度是否超过 255
		for _, m := range channel.GetModels() {
			if len(m) > 255 {
				return fmt.Errorf("模型名称过长: %s", m)
			}
		}
	}

	// VertexAI 特殊校验
	if channel.Type == constant.ChannelTypeVertexAi {
		if channel.Other == "" {
			return fmt.Errorf("部署地区不能为空")
		}

		regionMap, err := common.StrToMap(channel.Other)
		if err != nil {
			return fmt.Errorf("部署地区必须是标准的Json格式，例如{\"default\": \"us-central1\", \"region2\": \"us-east1\"}")
		}

		if regionMap["default"] == nil {
			return fmt.Errorf("部署地区必须包含default字段")
		}
	}

	// Codex OAuth key validation (optional, only when JSON object is provided)
	if channel.Type == constant.ChannelTypeCodex {
		trimmedKey := strings.TrimSpace(channel.Key)
		if isAdd || trimmedKey != "" {
			if !strings.HasPrefix(trimmedKey, "{") {
				return fmt.Errorf("Codex key must be a valid JSON object")
			}
			var keyMap map[string]any
			if err := common.Unmarshal([]byte(trimmedKey), &keyMap); err != nil {
				return fmt.Errorf("Codex key must be a valid JSON object")
			}
			if v, ok := keyMap["access_token"]; !ok || v == nil || strings.TrimSpace(fmt.Sprintf("%v", v)) == "" {
				return fmt.Errorf("Codex key JSON must include access_token")
			}
			if v, ok := keyMap["account_id"]; !ok || v == nil || strings.TrimSpace(fmt.Sprintf("%v", v)) == "" {
				return fmt.Errorf("Codex key JSON must include account_id")
			}
		}
	}

	// 校验后备渠道
	if err := validateFallbackChannelIds(channel.Id, channel.GetFallbackChannelIDs()); err != nil {
		return err
	}

	// 规范化分组黑名单：逐项 trim、去空、去重（不校验分组是否存在，与 Group 字段行为一致）
	if channel.GroupBlacklist != nil {
		normalized := normalizeGroupBlacklist(*channel.GroupBlacklist)
		channel.GroupBlacklist = &normalized
	}

	return nil
}

// normalizeGroupBlacklist 规范化逗号分隔的分组黑名单字符串：逐项 trim、去空、去重。
func normalizeGroupBlacklist(blacklist string) string {
	if blacklist == "" {
		return ""
	}
	seen := make(map[string]bool)
	result := make([]string, 0)
	for _, group := range strings.Split(blacklist, ",") {
		group = strings.TrimSpace(group)
		if group == "" || seen[group] {
			continue
		}
		seen[group] = true
		result = append(result, group)
	}
	return strings.Join(result, ",")
}

// validateFallbackChannelIds 校验后备渠道ID列表的有效性
// 检查: 存在性、非自身、无传递性循环依赖
func validateFallbackChannelIds(channelId int, fallbackIds []int) error {
	for _, id := range fallbackIds {
		if id == channelId {
			return fmt.Errorf("渠道不能将自己设为后备渠道")
		}
		fallback, err := model.GetChannelById(id, false)
		if err != nil {
			return fmt.Errorf("后备渠道 #%d 不存在", id)
		}
		// 深度循环依赖检测：使用 DFS 遍历后备链
		visited := map[int]bool{channelId: true}
		if hasCircularFallback(fallback.Id, channelId, visited) {
			return fmt.Errorf("检测到循环依赖：渠道 #%d 的后备渠道链最终指向当前渠道 #%d", id, channelId)
		}
	}
	return nil
}

// hasCircularFallback 深度优先搜索检查是否存在循环依赖
func hasCircularFallback(fromChannelId int, targetId int, visited map[int]bool) bool {
	if fromChannelId == targetId {
		return true
	}
	if visited[fromChannelId] {
		return false
	}
	visited[fromChannelId] = true
	channel, err := model.GetChannelById(fromChannelId, false)
	if err != nil {
		return false // 读取失败视为无循环
	}
	for _, fid := range channel.GetFallbackChannelIDs() {
		if hasCircularFallback(fid, targetId, visited) {
			return true
		}
	}
	return false
}

// cleanFallbackReferences 清理所有渠道中指向被删除渠道的后备引用
func cleanFallbackReferences(deletedIds []int) {
	idSet := make(map[int]bool, len(deletedIds))
	for _, id := range deletedIds {
		idSet[id] = true
	}

	// 遍历所有渠道，清理后备引用
	channels, err := model.GetAllChannels(0, 0, true, false)
	if err != nil {
		common.SysLog(fmt.Sprintf("清理后备渠道引用失败: %v", err))
		return
	}
	for _, ch := range channels {
		fallbackIds := ch.GetFallbackChannelIDs()
		if len(fallbackIds) == 0 {
			continue
		}
		filtered := make([]int, 0, len(fallbackIds))
		changed := false
		for _, fid := range fallbackIds {
			if idSet[fid] {
				changed = true
				continue
			}
			filtered = append(filtered, fid)
		}
		if changed {
			ch.SetFallbackChannelIDs(filtered)
			if err := model.DB.Model(ch).Update("other_info", ch.OtherInfo).Error; err != nil {
				common.SysLog(fmt.Sprintf("清理渠道 #%d 的后备引用失败: %v", ch.Id, err))
			}
		}
	}
}

func RefreshCodexChannelCredential(c *gin.Context) {
	channelId, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiError(c, fmt.Errorf("invalid channel id: %w", err))
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	oauthKey, ch, err := service.RefreshCodexChannelCredential(ctx, channelId, service.CodexCredentialRefreshOptions{ResetCaches: true})
	if err != nil {
		common.SysError("failed to refresh codex channel credential: " + err.Error())
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "刷新凭证失败，请稍后重试"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "refreshed",
		"data": gin.H{
			"expires_at":   oauthKey.Expired,
			"last_refresh": oauthKey.LastRefresh,
			"account_id":   oauthKey.AccountID,
			"email":        oauthKey.Email,
			"channel_id":   ch.Id,
			"channel_type": ch.Type,
			"channel_name": ch.Name,
		},
	})
}

type AddChannelRequest struct {
	Mode                      string                `json:"mode"`
	MultiKeyMode              constant.MultiKeyMode `json:"multi_key_mode"`
	BatchAddSetKeyPrefix2Name bool                  `json:"batch_add_set_key_prefix_2_name"`
	Channel                   *model.Channel        `json:"channel"`
	Disable429Ban             *bool                 `json:"disable_429_ban"` // 是否禁用429自动限流；nil=不设置(默认false)，true/false=显式设置
}

func getVertexArrayKeys(keys string) ([]string, error) {
	if keys == "" {
		return nil, nil
	}
	var keyArray []interface{}
	err := common.Unmarshal([]byte(keys), &keyArray)
	if err != nil {
		return nil, fmt.Errorf("批量添加 Vertex AI 必须使用标准的JsonArray格式，例如[{key1}, {key2}...]，请检查输入: %w", err)
	}
	cleanKeys := make([]string, 0, len(keyArray))
	for _, key := range keyArray {
		var keyStr string
		switch v := key.(type) {
		case string:
			keyStr = strings.TrimSpace(v)
		default:
			bytes, err := json.Marshal(v)
			if err != nil {
				return nil, fmt.Errorf("Vertex AI key JSON 编码失败: %w", err)
			}
			keyStr = string(bytes)
		}
		if keyStr != "" {
			cleanKeys = append(cleanKeys, keyStr)
		}
	}
	if len(cleanKeys) == 0 {
		return nil, fmt.Errorf("批量添加 Vertex AI 的 keys 不能为空")
	}
	return cleanKeys, nil
}

func AddChannel(c *gin.Context) {
	addChannelRequest := AddChannelRequest{}
	err := c.ShouldBindJSON(&addChannelRequest)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	// 使用统一的校验函数
	if err := validateChannel(addChannelRequest.Channel, true); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	addChannelRequest.Channel.CreatedTime = common.GetTimestamp()
	// 应用请求体中显式提供的 disable_429_ban 设置
	if addChannelRequest.Disable429Ban != nil {
		addChannelRequest.Channel.ChannelInfo.Disable429Ban = *addChannelRequest.Disable429Ban
	}
	keys := make([]string, 0)
	switch addChannelRequest.Mode {
	case "multi_to_single":
		addChannelRequest.Channel.ChannelInfo.IsMultiKey = true
		addChannelRequest.Channel.ChannelInfo.MultiKeyMode = addChannelRequest.MultiKeyMode
		if addChannelRequest.Channel.Type == constant.ChannelTypeVertexAi && addChannelRequest.Channel.GetOtherSettings().VertexKeyType != dto.VertexKeyTypeAPIKey {
			array, err := getVertexArrayKeys(addChannelRequest.Channel.Key)
			if err != nil {
				c.JSON(http.StatusOK, gin.H{
					"success": false,
					"message": err.Error(),
				})
				return
			}
			addChannelRequest.Channel.ChannelInfo.MultiKeySize = len(array)
			addChannelRequest.Channel.Key = strings.Join(array, "\n")
		} else {
			cleanKeys := make([]string, 0)
			for _, key := range strings.Split(addChannelRequest.Channel.Key, "\n") {
				if key == "" {
					continue
				}
				key = strings.TrimSpace(key)
				cleanKeys = append(cleanKeys, key)
			}
			addChannelRequest.Channel.ChannelInfo.MultiKeySize = len(cleanKeys)
			addChannelRequest.Channel.Key = strings.Join(cleanKeys, "\n")
		}
		keys = []string{addChannelRequest.Channel.Key}
	case "batch":
		if addChannelRequest.Channel.Type == constant.ChannelTypeVertexAi && addChannelRequest.Channel.GetOtherSettings().VertexKeyType != dto.VertexKeyTypeAPIKey {
			// multi json
			keys, err = getVertexArrayKeys(addChannelRequest.Channel.Key)
			if err != nil {
				c.JSON(http.StatusOK, gin.H{
					"success": false,
					"message": err.Error(),
				})
				return
			}
		} else {
			keys = strings.Split(addChannelRequest.Channel.Key, "\n")
		}
	case "single":
		keys = []string{addChannelRequest.Channel.Key}
	default:
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "不支持的添加模式",
		})
		return
	}

	channels := make([]model.Channel, 0, len(keys))
	for _, key := range keys {
		if key == "" {
			continue
		}
		localChannel := addChannelRequest.Channel
		localChannel.Key = key
		if addChannelRequest.BatchAddSetKeyPrefix2Name && len(keys) > 1 {
			keyPrefix := localChannel.Key
			if len(localChannel.Key) > 8 {
				keyPrefix = localChannel.Key[:8]
			}
			localChannel.Name = fmt.Sprintf("%s %s", localChannel.Name, keyPrefix)
		}
		channels = append(channels, *localChannel)
	}
	err = model.BatchInsertChannels(channels)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	service.ResetProxyClientCache()
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
	})
}

func DeleteChannel(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	channel := model.Channel{Id: id}
	err := channel.Delete()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	// 清理其他渠道中指向已删除渠道的后备引用
	cleanFallbackReferences([]int{id})
	model.InitChannelCache()
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
	})
}

// RateLimitChannelRequest 手动限流请求
type RateLimitChannelRequest struct {
	DurationHours float64 `json:"duration_hours"` // 限流时长（小时）
	Reason        string  `json:"reason"`         // 限流原因
}

// RateLimitChannel 手动设置渠道限流
func RateLimitChannel(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "invalid channel id",
		})
		return
	}

	var req RateLimitChannelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "invalid request body",
		})
		return
	}

	if req.DurationHours <= 0 {
		req.DurationHours = 1.0 // 默认1小时
	}
	if req.Reason == "" {
		req.Reason = "manual rate limit"
	}

	channel, err := model.GetChannelById(id, false)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "channel not found",
		})
		return
	}

	service.ManualRateLimitChannel(id, channel.Name, req.DurationHours, req.Reason)
	model.InitChannelCache()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "channel rate limited",
	})
}

// UnrateLimitChannelRequest 解除限流请求
type UnrateLimitChannelRequest struct {
	KeyIndex *int `json:"key_index"` // 可选，多Key模式下指定Key索引
}

// UnrateLimitChannel 解除渠道限流
func UnrateLimitChannel(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "invalid channel id",
		})
		return
	}

	channel, err := model.GetChannelById(id, false)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "channel not found",
		})
		return
	}

	var usingKey string
	if channel.ChannelInfo.IsMultiKey {
		var req UnrateLimitChannelRequest
		if err := c.ShouldBindJSON(&req); err == nil && req.KeyIndex != nil {
			keys := channel.GetKeys()
			if *req.KeyIndex >= 0 && *req.KeyIndex < len(keys) {
				usingKey = keys[*req.KeyIndex]
			}
		}
	}

	service.UnrateLimitChannel(id, usingKey, channel.Name)
	model.InitChannelCache()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "channel rate limit removed",
	})
}

func DeleteDisabledChannel(c *gin.Context) {
	rows, err := model.DeleteDisabledChannel()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	model.InitChannelCache()
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    rows,
	})
}

type ChannelTag struct {
	Tag            string  `json:"tag"`
	NewTag         *string `json:"new_tag"`
	Priority       *int64  `json:"priority"`
	Weight         *uint   `json:"weight"`
	ModelMapping   *string `json:"model_mapping"`
	Models         *string `json:"models"`
	Groups         *string `json:"groups"`
	ParamOverride  *string `json:"param_override"`
	HeaderOverride *string `json:"header_override"`
}

func DisableTagChannels(c *gin.Context) {
	channelTag := ChannelTag{}
	err := c.ShouldBindJSON(&channelTag)
	if err != nil || channelTag.Tag == "" {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "参数错误",
		})
		return
	}
	err = model.DisableChannelByTag(channelTag.Tag)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	model.InitChannelCache()
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
	})
}

func EnableTagChannels(c *gin.Context) {
	channelTag := ChannelTag{}
	err := c.ShouldBindJSON(&channelTag)
	if err != nil || channelTag.Tag == "" {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "参数错误",
		})
		return
	}
	err = model.EnableChannelByTag(channelTag.Tag)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	model.InitChannelCache()
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
	})
}

func EditTagChannels(c *gin.Context) {
	channelTag := ChannelTag{}
	err := c.ShouldBindJSON(&channelTag)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "参数错误",
		})
		return
	}
	if channelTag.Tag == "" {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "tag不能为空",
		})
		return
	}
	if channelTag.ParamOverride != nil {
		trimmed := strings.TrimSpace(*channelTag.ParamOverride)
		if trimmed != "" && !json.Valid([]byte(trimmed)) {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "参数覆盖必须是合法的 JSON 格式",
			})
			return
		}
		channelTag.ParamOverride = common.GetPointer(trimmed)
	}
	if channelTag.HeaderOverride != nil {
		trimmed := strings.TrimSpace(*channelTag.HeaderOverride)
		if trimmed != "" && !json.Valid([]byte(trimmed)) {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "请求头覆盖必须是合法的 JSON 格式",
			})
			return
		}
		channelTag.HeaderOverride = common.GetPointer(trimmed)
	}
	err = model.EditChannelByTag(channelTag.Tag, channelTag.NewTag, channelTag.ModelMapping, channelTag.Models, channelTag.Groups, channelTag.Priority, channelTag.Weight, channelTag.ParamOverride, channelTag.HeaderOverride)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	model.InitChannelCache()
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
	})
}

type ChannelBatch struct {
	Ids []int   `json:"ids"`
	Tag *string `json:"tag"`
}

func DeleteChannelBatch(c *gin.Context) {
	channelBatch := ChannelBatch{}
	err := c.ShouldBindJSON(&channelBatch)
	if err != nil || len(channelBatch.Ids) == 0 {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "参数错误",
		})
		return
	}
	err = model.BatchDeleteChannels(channelBatch.Ids)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	// 清理其他渠道中指向已删除渠道的后备引用
	cleanFallbackReferences(channelBatch.Ids)
	model.InitChannelCache()
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    len(channelBatch.Ids),
	})
}

type PatchChannel struct {
	model.Channel
	MultiKeyMode  *string `json:"multi_key_mode"`
	KeyMode       *string `json:"key_mode"`        // 多key模式下密钥覆盖或者追加
	Disable429Ban *bool   `json:"disable_429_ban"` // 是否禁用429自动限流；nil=不修改，true/false=显式设置
}

func UpdateChannel(c *gin.Context) {
	channel := PatchChannel{}
	err := c.ShouldBindJSON(&channel)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	// 使用统一的校验函数
	if err := validateChannel(&channel.Channel, false); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	// Preserve existing ChannelInfo to ensure multi-key channels keep correct state even if the client does not send ChannelInfo in the request.
	originChannel, err := model.GetChannelById(channel.Id, true)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	// Always copy the original ChannelInfo so that fields like IsMultiKey and MultiKeySize are retained.
	channel.ChannelInfo = originChannel.ChannelInfo
	// 应用请求体中显式提供的 disable_429_ban 设置（nil 表示不修改，保留原值）
	if channel.Disable429Ban != nil {
		channel.ChannelInfo.Disable429Ban = *channel.Disable429Ban
	}

	// 手动设置非0余额时标记为"曾经有过非0余额"，用于余额为0自动禁用判定
	if channel.Balance > 0 {
		channel.ChannelInfo.BalanceEverNonZero = true
	}

	// If the request explicitly specifies a new MultiKeyMode, apply it on top of the original info.
	if channel.MultiKeyMode != nil && *channel.MultiKeyMode != "" {
		channel.ChannelInfo.MultiKeyMode = constant.MultiKeyMode(*channel.MultiKeyMode)
	}

	// 处理多key模式下的密钥追加/覆盖逻辑
	if channel.KeyMode != nil && channel.ChannelInfo.IsMultiKey {
		switch *channel.KeyMode {
		case "append":
			// 追加模式：将新密钥添加到现有密钥列表
			if originChannel.Key != "" {
				var newKeys []string
				var existingKeys []string

				// 解析现有密钥
				if strings.HasPrefix(strings.TrimSpace(originChannel.Key), "[") {
					// JSON数组格式
					var arr []json.RawMessage
					if err := json.Unmarshal([]byte(strings.TrimSpace(originChannel.Key)), &arr); err == nil {
						existingKeys = make([]string, len(arr))
						for i, v := range arr {
							existingKeys[i] = string(v)
						}
					}
				} else {
					// 换行分隔格式
					existingKeys = strings.Split(strings.Trim(originChannel.Key, "\n"), "\n")
				}

				// 处理 Vertex AI 的特殊情况
				if channel.Type == constant.ChannelTypeVertexAi && channel.GetOtherSettings().VertexKeyType != dto.VertexKeyTypeAPIKey {
					// 尝试解析新密钥为JSON数组
					if strings.HasPrefix(strings.TrimSpace(channel.Key), "[") {
						array, err := getVertexArrayKeys(channel.Key)
						if err != nil {
							c.JSON(http.StatusOK, gin.H{
								"success": false,
								"message": "追加密钥解析失败: " + err.Error(),
							})
							return
						}
						newKeys = array
					} else {
						// 单个JSON密钥
						newKeys = []string{channel.Key}
					}
				} else {
					// 普通渠道的处理
					inputKeys := strings.Split(channel.Key, "\n")
					for _, key := range inputKeys {
						key = strings.TrimSpace(key)
						if key != "" {
							newKeys = append(newKeys, key)
						}
					}
				}

				seen := make(map[string]struct{}, len(existingKeys)+len(newKeys))
				for _, key := range existingKeys {
					normalized := strings.TrimSpace(key)
					if normalized == "" {
						continue
					}
					seen[normalized] = struct{}{}
				}
				dedupedNewKeys := make([]string, 0, len(newKeys))
				for _, key := range newKeys {
					normalized := strings.TrimSpace(key)
					if normalized == "" {
						continue
					}
					if _, ok := seen[normalized]; ok {
						continue
					}
					seen[normalized] = struct{}{}
					dedupedNewKeys = append(dedupedNewKeys, normalized)
				}

				allKeys := append(existingKeys, dedupedNewKeys...)
				channel.Key = strings.Join(allKeys, "\n")
			}
		case "replace":
			// 覆盖模式：直接使用新密钥（默认行为，不需要特殊处理）
		}
	}
	err = channel.Update()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	// 手动设余额为0且曾经有过非0余额时触发禁用（仅当渠道当前为启用状态）
	// channel.Update() 内部已 First() 重新加载，channel 此时是最新值
	if channel.Balance <= 0 && channel.Status == common.ChannelStatusEnabled {
		service.DisableChannelIfBalanceDepleted(&channel.Channel, "余额被手动清零")
	}
	model.InitChannelCache()
	service.ResetProxyClientCache()
	channel.Key = ""
	clearChannelInfo(&channel.Channel)
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    channel,
	})
}

func FetchModels(c *gin.Context) {
	var req struct {
		BaseURL string `json:"base_url"`
		Type    int    `json:"type"`
		Key     string `json:"key"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid request",
		})
		return
	}

	baseURL := req.BaseURL
	if baseURL == "" {
		baseURL = constant.ChannelBaseURLs[req.Type]
	}

	// remove line breaks and extra spaces.
	key := strings.TrimSpace(req.Key)
	key = strings.Split(key, "\n")[0]

	if req.Type == constant.ChannelTypeOllama {
		models, err := ollama.FetchOllamaModels(baseURL, key)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": fmt.Sprintf("获取Ollama模型失败: %s", err.Error()),
			})
			return
		}

		names := make([]string, 0, len(models))
		for _, modelInfo := range models {
			names = append(names, modelInfo.Name)
		}

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data":    names,
		})
		return
	}

	if req.Type == constant.ChannelTypeGemini {
		models, err := gemini.FetchGeminiModels(baseURL, key, "")
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": fmt.Sprintf("获取Gemini模型失败: %s", err.Error()),
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data":    models,
		})
		return
	}

	client := &http.Client{}
	url := fmt.Sprintf("%s/v1/models", baseURL)

	request, err := http.NewRequest("GET", url, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	request.Header.Set("Authorization", "Bearer "+key)

	response, err := client.Do(request)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	//check status code
	if response.StatusCode != http.StatusOK {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to fetch models",
		})
		return
	}
	defer response.Body.Close()

	var result struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}

	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	var models []string
	for _, model := range result.Data {
		models = append(models, model.ID)
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    models,
	})
}

func BatchSetChannelTag(c *gin.Context) {
	channelBatch := ChannelBatch{}
	err := c.ShouldBindJSON(&channelBatch)
	if err != nil || len(channelBatch.Ids) == 0 {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "参数错误",
		})
		return
	}
	err = model.BatchSetChannelTag(channelBatch.Ids, channelBatch.Tag)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	model.InitChannelCache()
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    len(channelBatch.Ids),
	})
}

func GetTagModels(c *gin.Context) {
	tag := c.Query("tag")
	if tag == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "tag不能为空",
		})
		return
	}

	channels, err := model.GetChannelsByTag(tag, false, false) // idSort=false, selectAll=false
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	var longestModels string
	maxLength := 0

	// Find the longest models string among all channels with the given tag
	for _, channel := range channels {
		if channel.Models != "" {
			currentModels := strings.Split(channel.Models, ",")
			if len(currentModels) > maxLength {
				maxLength = len(currentModels)
				longestModels = channel.Models
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    longestModels,
	})
}

// CopyChannel handles cloning an existing channel with its key.
// POST /api/channel/copy/:id
// The cloned channel is always created in the disabled (manually disabled) state.
// Optional query params:
//
//	suffix         - string appended to the original name (default "_复制")
//	reset_balance  - bool, when true will reset balance & used_quota to 0 (default true)
func CopyChannel(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "invalid id"})
		return
	}

	suffix := c.DefaultQuery("suffix", "_复制")
	resetBalance := true
	if rbStr := c.DefaultQuery("reset_balance", "true"); rbStr != "" {
		if v, err := strconv.ParseBool(rbStr); err == nil {
			resetBalance = v
		}
	}

	// fetch original channel with key
	origin, err := model.GetChannelById(id, true)
	if err != nil {
		common.SysError("failed to get channel by id: " + err.Error())
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "获取渠道信息失败，请稍后重试"})
		return
	}

	// clone channel
	clone := *origin // shallow copy is sufficient as we will overwrite primitives
	clone.Id = 0     // let DB auto-generate
	clone.CreatedTime = common.GetTimestamp()
	clone.Name = origin.Name + suffix
	clone.Status = common.ChannelStatusManuallyDisabled // copied channels start disabled
	clone.TestTime = 0
	clone.ResponseTime = 0
	if resetBalance {
		clone.Balance = 0
		clone.UsedQuota = 0
	}

	// insert
	// 注意：不能使用 BatchInsertChannels，因为它内部通过 lo.Chunk 对值类型切片做了 copy，
	// GORM 回填的自增 ID 只会写到副本里，无法写回 clone，导致返回的 id 恒为 0。
	// 这里直接使用事务插入并补建 abilities，保证 clone.Id 被正确回填。
	tx := model.DB.Begin()
	if tx.Error != nil {
		common.SysError("failed to begin tx for clone channel: " + tx.Error.Error())
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "复制渠道失败，请稍后重试"})
		return
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()
	if err := tx.Create(&clone).Error; err != nil {
		tx.Rollback()
		common.SysError("failed to clone channel: " + err.Error())
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "复制渠道失败，请稍后重试"})
		return
	}
	if err := clone.AddAbilities(tx); err != nil {
		tx.Rollback()
		common.SysError("failed to add abilities for cloned channel: " + err.Error())
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "复制渠道失败，请稍后重试"})
		return
	}
	if err := tx.Commit().Error; err != nil {
		common.SysError("failed to commit clone channel: " + err.Error())
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "复制渠道失败，请稍后重试"})
		return
	}
	model.InitChannelCache()
	// success
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": gin.H{"id": clone.Id}})
}

// MultiKeyManageRequest represents the request for multi-key management operations
type MultiKeyManageRequest struct {
	ChannelId int    `json:"channel_id"`
	Action    string `json:"action"`              // "disable_key", "enable_key", "delete_key", "delete_disabled_keys", "get_key_status"
	KeyIndex  *int   `json:"key_index,omitempty"` // for disable_key, enable_key, and delete_key actions
	Page      int    `json:"page,omitempty"`      // for get_key_status pagination
	PageSize  int    `json:"page_size,omitempty"` // for get_key_status pagination
	Status    *int   `json:"status,omitempty"`    // for get_key_status filtering: 1=enabled, 2=manual_disabled, 3=auto_disabled, nil=all
}

// MultiKeyStatusResponse represents the response for key status query
type MultiKeyStatusResponse struct {
	Keys       []KeyStatus `json:"keys"`
	Total      int         `json:"total"`
	Page       int         `json:"page"`
	PageSize   int         `json:"page_size"`
	TotalPages int         `json:"total_pages"`
	// Statistics
	EnabledCount        int `json:"enabled_count"`
	ManualDisabledCount int `json:"manual_disabled_count"`
	AutoDisabledCount   int `json:"auto_disabled_count"`
}

type KeyStatus struct {
	Index        int    `json:"index"`
	Status       int    `json:"status"` // 1: enabled, 2: disabled
	DisabledTime int64  `json:"disabled_time,omitempty"`
	Reason       string `json:"reason,omitempty"`
	KeyPreview   string `json:"key_preview"` // first 10 chars of key for identification
}

// ManageMultiKeys handles multi-key management operations
func ManageMultiKeys(c *gin.Context) {
	request := MultiKeyManageRequest{}
	err := c.ShouldBindJSON(&request)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	channel, err := model.GetChannelById(request.ChannelId, true)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "渠道不存在",
		})
		return
	}

	if !channel.ChannelInfo.IsMultiKey {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "该渠道不是多密钥模式",
		})
		return
	}

	lock := model.GetChannelPollingLock(channel.Id)
	lock.Lock()
	defer lock.Unlock()

	switch request.Action {
	case "get_key_status":
		keys := channel.GetKeys()

		// Default pagination parameters
		page := request.Page
		pageSize := request.PageSize
		if page <= 0 {
			page = 1
		}
		if pageSize <= 0 {
			pageSize = 50 // Default page size
		}

		// Statistics for all keys (unchanged by filtering)
		var enabledCount, manualDisabledCount, autoDisabledCount int

		// Build all key status data first
		var allKeyStatusList []KeyStatus
		for i, key := range keys {
			status := 1 // default enabled
			var disabledTime int64
			var reason string

			if channel.ChannelInfo.MultiKeyStatusList != nil {
				if s, exists := channel.ChannelInfo.MultiKeyStatusList[i]; exists {
					status = s
				}
			}

			// Count for statistics (all keys)
			switch status {
			case 1:
				enabledCount++
			case 2:
				manualDisabledCount++
			case 3:
				autoDisabledCount++
			}

			if status != 1 {
				if channel.ChannelInfo.MultiKeyDisabledTime != nil {
					disabledTime = channel.ChannelInfo.MultiKeyDisabledTime[i]
				}
				if channel.ChannelInfo.MultiKeyDisabledReason != nil {
					reason = channel.ChannelInfo.MultiKeyDisabledReason[i]
				}
			}

			// Create key preview (first 10 chars)
			keyPreview := key
			if len(key) > 10 {
				keyPreview = key[:10] + "..."
			}

			allKeyStatusList = append(allKeyStatusList, KeyStatus{
				Index:        i,
				Status:       status,
				DisabledTime: disabledTime,
				Reason:       reason,
				KeyPreview:   keyPreview,
			})
		}

		// Apply status filter if specified
		var filteredKeyStatusList []KeyStatus
		if request.Status != nil {
			for _, keyStatus := range allKeyStatusList {
				if keyStatus.Status == *request.Status {
					filteredKeyStatusList = append(filteredKeyStatusList, keyStatus)
				}
			}
		} else {
			filteredKeyStatusList = allKeyStatusList
		}

		// Calculate pagination based on filtered results
		filteredTotal := len(filteredKeyStatusList)
		totalPages := (filteredTotal + pageSize - 1) / pageSize
		if totalPages == 0 {
			totalPages = 1
		}
		if page > totalPages {
			page = totalPages
		}

		// Calculate range for current page
		start := (page - 1) * pageSize
		end := start + pageSize
		if end > filteredTotal {
			end = filteredTotal
		}

		// Get the page data
		var pageKeyStatusList []KeyStatus
		if start < filteredTotal {
			pageKeyStatusList = filteredKeyStatusList[start:end]
		}

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "",
			"data": MultiKeyStatusResponse{
				Keys:                pageKeyStatusList,
				Total:               filteredTotal, // Total of filtered results
				Page:                page,
				PageSize:            pageSize,
				TotalPages:          totalPages,
				EnabledCount:        enabledCount,        // Overall statistics
				ManualDisabledCount: manualDisabledCount, // Overall statistics
				AutoDisabledCount:   autoDisabledCount,   // Overall statistics
			},
		})
		return

	case "disable_key":
		if request.KeyIndex == nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "未指定要禁用的密钥索引",
			})
			return
		}

		keyIndex := *request.KeyIndex
		if keyIndex < 0 || keyIndex >= channel.ChannelInfo.MultiKeySize {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "密钥索引超出范围",
			})
			return
		}

		if channel.ChannelInfo.MultiKeyStatusList == nil {
			channel.ChannelInfo.MultiKeyStatusList = make(map[int]int)
		}
		if channel.ChannelInfo.MultiKeyDisabledTime == nil {
			channel.ChannelInfo.MultiKeyDisabledTime = make(map[int]int64)
		}
		if channel.ChannelInfo.MultiKeyDisabledReason == nil {
			channel.ChannelInfo.MultiKeyDisabledReason = make(map[int]string)
		}

		channel.ChannelInfo.MultiKeyStatusList[keyIndex] = 2 // disabled

		err = channel.Update()
		if err != nil {
			common.ApiError(c, err)
			return
		}

		model.InitChannelCache()
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "密钥已禁用",
		})
		return

	case "enable_key":
		if request.KeyIndex == nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "未指定要启用的密钥索引",
			})
			return
		}

		keyIndex := *request.KeyIndex
		if keyIndex < 0 || keyIndex >= channel.ChannelInfo.MultiKeySize {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "密钥索引超出范围",
			})
			return
		}

		// 从状态列表中删除该密钥的记录，使其回到默认启用状态
		if channel.ChannelInfo.MultiKeyStatusList != nil {
			delete(channel.ChannelInfo.MultiKeyStatusList, keyIndex)
		}
		if channel.ChannelInfo.MultiKeyDisabledTime != nil {
			delete(channel.ChannelInfo.MultiKeyDisabledTime, keyIndex)
		}
		if channel.ChannelInfo.MultiKeyDisabledReason != nil {
			delete(channel.ChannelInfo.MultiKeyDisabledReason, keyIndex)
		}

		err = channel.Update()
		if err != nil {
			common.ApiError(c, err)
			return
		}

		model.InitChannelCache()
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "密钥已启用",
		})
		return

	case "enable_all_keys":
		// 清空所有禁用状态，使所有密钥回到默认启用状态
		var enabledCount int
		if channel.ChannelInfo.MultiKeyStatusList != nil {
			enabledCount = len(channel.ChannelInfo.MultiKeyStatusList)
		}

		channel.ChannelInfo.MultiKeyStatusList = make(map[int]int)
		channel.ChannelInfo.MultiKeyDisabledTime = make(map[int]int64)
		channel.ChannelInfo.MultiKeyDisabledReason = make(map[int]string)

		err = channel.Update()
		if err != nil {
			common.ApiError(c, err)
			return
		}

		model.InitChannelCache()
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": fmt.Sprintf("已启用 %d 个密钥", enabledCount),
		})
		return

	case "disable_all_keys":
		// 禁用所有启用的密钥
		if channel.ChannelInfo.MultiKeyStatusList == nil {
			channel.ChannelInfo.MultiKeyStatusList = make(map[int]int)
		}
		if channel.ChannelInfo.MultiKeyDisabledTime == nil {
			channel.ChannelInfo.MultiKeyDisabledTime = make(map[int]int64)
		}
		if channel.ChannelInfo.MultiKeyDisabledReason == nil {
			channel.ChannelInfo.MultiKeyDisabledReason = make(map[int]string)
		}

		var disabledCount int
		for i := 0; i < channel.ChannelInfo.MultiKeySize; i++ {
			status := 1 // default enabled
			if s, exists := channel.ChannelInfo.MultiKeyStatusList[i]; exists {
				status = s
			}

			// 只禁用当前启用的密钥
			if status == 1 {
				channel.ChannelInfo.MultiKeyStatusList[i] = 2 // disabled
				disabledCount++
			}
		}

		if disabledCount == 0 {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "没有可禁用的密钥",
			})
			return
		}

		err = channel.Update()
		if err != nil {
			common.ApiError(c, err)
			return
		}

		model.InitChannelCache()
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": fmt.Sprintf("已禁用 %d 个密钥", disabledCount),
		})
		return

	case "delete_key":
		if request.KeyIndex == nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "未指定要删除的密钥索引",
			})
			return
		}

		keyIndex := *request.KeyIndex
		if keyIndex < 0 || keyIndex >= channel.ChannelInfo.MultiKeySize {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "密钥索引超出范围",
			})
			return
		}

		keys := channel.GetKeys()
		var remainingKeys []string
		var newStatusList = make(map[int]int)
		var newDisabledTime = make(map[int]int64)
		var newDisabledReason = make(map[int]string)

		newIndex := 0
		for i, key := range keys {
			// 跳过要删除的密钥
			if i == keyIndex {
				continue
			}

			remainingKeys = append(remainingKeys, key)

			// 保留其他密钥的状态信息，重新索引
			if channel.ChannelInfo.MultiKeyStatusList != nil {
				if status, exists := channel.ChannelInfo.MultiKeyStatusList[i]; exists && status != 1 {
					newStatusList[newIndex] = status
				}
			}
			if channel.ChannelInfo.MultiKeyDisabledTime != nil {
				if t, exists := channel.ChannelInfo.MultiKeyDisabledTime[i]; exists {
					newDisabledTime[newIndex] = t
				}
			}
			if channel.ChannelInfo.MultiKeyDisabledReason != nil {
				if r, exists := channel.ChannelInfo.MultiKeyDisabledReason[i]; exists {
					newDisabledReason[newIndex] = r
				}
			}
			newIndex++
		}

		if len(remainingKeys) == 0 {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "不能删除最后一个密钥",
			})
			return
		}

		// Update channel with remaining keys
		channel.Key = strings.Join(remainingKeys, "\n")
		channel.ChannelInfo.MultiKeySize = len(remainingKeys)
		channel.ChannelInfo.MultiKeyStatusList = newStatusList
		channel.ChannelInfo.MultiKeyDisabledTime = newDisabledTime
		channel.ChannelInfo.MultiKeyDisabledReason = newDisabledReason

		err = channel.Update()
		if err != nil {
			common.ApiError(c, err)
			return
		}

		model.InitChannelCache()
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "密钥已删除",
		})
		return

	case "delete_disabled_keys":
		keys := channel.GetKeys()
		var remainingKeys []string
		var deletedCount int
		var newStatusList = make(map[int]int)
		var newDisabledTime = make(map[int]int64)
		var newDisabledReason = make(map[int]string)

		newIndex := 0
		for i, key := range keys {
			status := 1 // default enabled
			if channel.ChannelInfo.MultiKeyStatusList != nil {
				if s, exists := channel.ChannelInfo.MultiKeyStatusList[i]; exists {
					status = s
				}
			}

			// 只删除自动禁用（status == 3）的密钥，保留启用（status == 1）和手动禁用（status == 2）的密钥
			if status == 3 {
				deletedCount++
			} else {
				remainingKeys = append(remainingKeys, key)
				// 保留非自动禁用密钥的状态信息，重新索引
				if status != 1 {
					newStatusList[newIndex] = status
					if channel.ChannelInfo.MultiKeyDisabledTime != nil {
						if t, exists := channel.ChannelInfo.MultiKeyDisabledTime[i]; exists {
							newDisabledTime[newIndex] = t
						}
					}
					if channel.ChannelInfo.MultiKeyDisabledReason != nil {
						if r, exists := channel.ChannelInfo.MultiKeyDisabledReason[i]; exists {
							newDisabledReason[newIndex] = r
						}
					}
				}
				newIndex++
			}
		}

		if deletedCount == 0 {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "没有需要删除的自动禁用密钥",
			})
			return
		}

		// Update channel with remaining keys
		channel.Key = strings.Join(remainingKeys, "\n")
		channel.ChannelInfo.MultiKeySize = len(remainingKeys)
		channel.ChannelInfo.MultiKeyStatusList = newStatusList
		channel.ChannelInfo.MultiKeyDisabledTime = newDisabledTime
		channel.ChannelInfo.MultiKeyDisabledReason = newDisabledReason

		err = channel.Update()
		if err != nil {
			common.ApiError(c, err)
			return
		}

		model.InitChannelCache()
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": fmt.Sprintf("已删除 %d 个自动禁用的密钥", deletedCount),
			"data":    deletedCount,
		})
		return

	default:
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "不支持的操作",
		})
		return
	}
}

// OllamaPullModel 拉取 Ollama 模型
func OllamaPullModel(c *gin.Context) {
	var req struct {
		ChannelID int    `json:"channel_id"`
		ModelName string `json:"model_name"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid request parameters",
		})
		return
	}

	if req.ChannelID == 0 || req.ModelName == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Channel ID and model name are required",
		})
		return
	}

	// 获取渠道信息
	channel, err := model.GetChannelById(req.ChannelID, true)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "Channel not found",
		})
		return
	}

	// 检查是否是 Ollama 渠道
	if channel.Type != constant.ChannelTypeOllama {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "This operation is only supported for Ollama channels",
		})
		return
	}

	baseURL := constant.ChannelBaseURLs[channel.Type]
	if channel.GetBaseURL() != "" {
		baseURL = channel.GetBaseURL()
	}

	key := strings.Split(channel.Key, "\n")[0]
	err = ollama.PullOllamaModel(baseURL, key, req.ModelName)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": fmt.Sprintf("Failed to pull model: %s", err.Error()),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": fmt.Sprintf("Model %s pulled successfully", req.ModelName),
	})
}

// OllamaPullModelStream 流式拉取 Ollama 模型
func OllamaPullModelStream(c *gin.Context) {
	var req struct {
		ChannelID int    `json:"channel_id"`
		ModelName string `json:"model_name"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid request parameters",
		})
		return
	}

	if req.ChannelID == 0 || req.ModelName == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Channel ID and model name are required",
		})
		return
	}

	// 获取渠道信息
	channel, err := model.GetChannelById(req.ChannelID, true)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "Channel not found",
		})
		return
	}

	// 检查是否是 Ollama 渠道
	if channel.Type != constant.ChannelTypeOllama {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "This operation is only supported for Ollama channels",
		})
		return
	}

	baseURL := constant.ChannelBaseURLs[channel.Type]
	if channel.GetBaseURL() != "" {
		baseURL = channel.GetBaseURL()
	}

	// 设置 SSE 头部
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Access-Control-Allow-Origin", "*")

	key := strings.Split(channel.Key, "\n")[0]

	// 创建进度回调函数
	progressCallback := func(progress ollama.OllamaPullResponse) {
		data, _ := json.Marshal(progress)
		fmt.Fprintf(c.Writer, "data: %s\n\n", string(data))
		c.Writer.Flush()
	}

	// 执行拉取
	err = ollama.PullOllamaModelStream(baseURL, key, req.ModelName, progressCallback)

	if err != nil {
		errorData, _ := json.Marshal(gin.H{
			"error": err.Error(),
		})
		fmt.Fprintf(c.Writer, "data: %s\n\n", string(errorData))
	} else {
		successData, _ := json.Marshal(gin.H{
			"message": fmt.Sprintf("Model %s pulled successfully", req.ModelName),
		})
		fmt.Fprintf(c.Writer, "data: %s\n\n", string(successData))
	}

	// 发送结束标志
	fmt.Fprintf(c.Writer, "data: [DONE]\n\n")
	c.Writer.Flush()
}

// OllamaDeleteModel 删除 Ollama 模型
func OllamaDeleteModel(c *gin.Context) {
	var req struct {
		ChannelID int    `json:"channel_id"`
		ModelName string `json:"model_name"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid request parameters",
		})
		return
	}

	if req.ChannelID == 0 || req.ModelName == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Channel ID and model name are required",
		})
		return
	}

	// 获取渠道信息
	channel, err := model.GetChannelById(req.ChannelID, true)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "Channel not found",
		})
		return
	}

	// 检查是否是 Ollama 渠道
	if channel.Type != constant.ChannelTypeOllama {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "This operation is only supported for Ollama channels",
		})
		return
	}

	baseURL := constant.ChannelBaseURLs[channel.Type]
	if channel.GetBaseURL() != "" {
		baseURL = channel.GetBaseURL()
	}

	key := strings.Split(channel.Key, "\n")[0]
	err = ollama.DeleteOllamaModel(baseURL, key, req.ModelName)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": fmt.Sprintf("Failed to delete model: %s", err.Error()),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": fmt.Sprintf("Model %s deleted successfully", req.ModelName),
	})
}

// OllamaVersion 获取 Ollama 服务版本信息
func OllamaVersion(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid channel id",
		})
		return
	}

	channel, err := model.GetChannelById(id, true)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "Channel not found",
		})
		return
	}

	if channel.Type != constant.ChannelTypeOllama {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "This operation is only supported for Ollama channels",
		})
		return
	}

	baseURL := constant.ChannelBaseURLs[channel.Type]
	if channel.GetBaseURL() != "" {
		baseURL = channel.GetBaseURL()
	}

	key := strings.Split(channel.Key, "\n")[0]
	version, err := ollama.FetchOllamaVersion(baseURL, key)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": fmt.Sprintf("获取Ollama版本失败: %s", err.Error()),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"version": version,
		},
	})
}
