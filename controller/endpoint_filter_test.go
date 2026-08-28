package controller

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type userModelsResponse struct {
	Success bool     `json:"success"`
	Data    []string `json:"data"`
}

type userGroupsResponse struct {
	Success bool                      `json:"success"`
	Data    map[string]map[string]any `json:"data"`
}

func setupEndpointFilterTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	gin.SetMode(gin.TestMode)

	origIsMasterNode := common.IsMasterNode
	origSQLitePath := common.SQLitePath
	origUsingSQLite, origUsingMySQL := common.UsingSQLite, common.UsingMySQL
	origUsingPostgreSQL, origRedisEnabled := common.UsingPostgreSQL, common.RedisEnabled
	origSQLDSN, hadSQLDSN := os.LookupEnv("SQL_DSN")
	t.Cleanup(func() {
		common.IsMasterNode = origIsMasterNode
		common.SQLitePath = origSQLitePath
		common.UsingSQLite = origUsingSQLite
		common.UsingMySQL = origUsingMySQL
		common.UsingPostgreSQL = origUsingPostgreSQL
		common.RedisEnabled = origRedisEnabled
		if hadSQLDSN {
			_ = os.Setenv("SQL_DSN", origSQLDSN)
		} else {
			_ = os.Unsetenv("SQL_DSN")
		}
		model.InvalidatePricingCache()
	})

	common.IsMasterNode = false
	common.SQLitePath = fmt.Sprintf("file:%s_init?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	common.UsingSQLite = false
	common.UsingMySQL = false
	common.UsingPostgreSQL = false
	common.RedisEnabled = false
	require.NoError(t, os.Setenv("SQL_DSN", "local"))
	// 触发 model.initCol() 初始化 commonGroupCol/commonKeyCol 等列名
	require.NoError(t, model.InitDB())
	if model.DB != nil {
		sqlDB, err := model.DB.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	}
	model.DB = nil
	model.LOG_DB = nil

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	model.DB = db
	model.LOG_DB = db

	require.NoError(t, db.AutoMigrate(&model.User{}, &model.Channel{}, &model.Ability{}, &model.Model{}, &model.Vendor{}))
	return db
}

// insertTestChannel 插入启用渠道并同步写入 enabled abilities，模拟正常渠道配置。
func insertTestChannel(t *testing.T, db *gorm.DB, name, models, group string, channelType int) *model.Channel {
	t.Helper()
	channel := &model.Channel{
		Name:   name,
		Type:   channelType,
		Key:    "sk-test",
		Status: common.ChannelStatusEnabled,
		Models: models,
		Group:  group,
	}
	require.NoError(t, db.Create(channel).Error)
	for _, m := range strings.Split(models, ",") {
		require.NoError(t, db.Create(&model.Ability{
			Group:     group,
			Model:     m,
			ChannelId: channel.Id,
			Enabled:   true,
		}).Error)
	}
	return channel
}

func doUserModelsRequest(t *testing.T, query string, userId int) userModelsResponse {
	t.Helper()
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/user/models"+query, nil)
	c.Set("id", userId)
	GetUserModels(c)

	var resp userModelsResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	return resp
}

func doUserGroupsRequest(t *testing.T, query string, userId int) userGroupsResponse {
	t.Helper()
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/user/self/groups"+query, nil)
	c.Set("id", userId)
	GetUserGroups(c)

	var resp userGroupsResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	return resp
}

// TestGetUserModelsEndpointFilter 验证模型列表的 endpoint 过滤：
// 图像模型（命中 IsImageGenerationModel）获得 image-generation 端点，聊天模型被过滤。
func TestGetUserModelsEndpointFilter(t *testing.T) {
	db := setupEndpointFilterTestDB(t)

	insertTestChannel(t, db, "openai-img", "gpt-image-1,dall-e-3,gpt-4o", "default", constant.ChannelTypeOpenAI)
	model.RefreshPricing()

	user := &model.User{
		Username: "endpoint-user",
		Password: "password123",
		Group:    "default",
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
	}
	require.NoError(t, db.Create(user).Error)

	// 无参数：返回全部模型（兼容性回归）
	resp := doUserModelsRequest(t, "", user.Id)
	require.True(t, resp.Success)
	require.ElementsMatch(t, []string{"gpt-image-1", "dall-e-3", "gpt-4o"}, resp.Data)

	// ?endpoint=image-generation：仅保留支持图像生成端点的模型
	resp = doUserModelsRequest(t, "?endpoint=image-generation", user.Id)
	require.True(t, resp.Success)
	require.ElementsMatch(t, []string{"gpt-image-1", "dall-e-3"}, resp.Data)

	// ?endpoint=image-edit：无模型元数据自定义端点，返回空
	resp = doUserModelsRequest(t, "?endpoint=image-edit", user.Id)
	require.True(t, resp.Success)
	require.Empty(t, resp.Data)
}

// TestGetUserModelsEndpointFilterCustomMeta 验证模型元数据自定义端点参与过滤：
// 配置 image-edit 端点的模型应被 ?endpoint=image-edit 命中。
func TestGetUserModelsEndpointFilterCustomMeta(t *testing.T) {
	db := setupEndpointFilterTestDB(t)

	insertTestChannel(t, db, "meta-img", "custom-img", "default", constant.ChannelTypeOpenAI)

	require.NoError(t, db.Create(&model.Model{
		ModelName: "custom-img",
		NameRule:  model.NameRuleExact,
		Status:    1,
		Endpoints: `{"image-edit":{"path":"/v1/images/edit","method":"POST"}}`,
	}).Error)
	model.RefreshPricing()

	user := &model.User{
		Username: "endpoint-meta-user",
		Password: "password123",
		Group:    "default",
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
	}
	require.NoError(t, db.Create(user).Error)

	// 自定义端点整体替换默认端点：支持 image-edit，不支持 image-generation
	resp := doUserModelsRequest(t, "?endpoint=image-edit", user.Id)
	require.True(t, resp.Success)
	require.ElementsMatch(t, []string{"custom-img"}, resp.Data)

	resp = doUserModelsRequest(t, "?endpoint=image-generation", user.Id)
	require.True(t, resp.Success)
	require.Empty(t, resp.Data)
}

// TestGetUserGroupsEndpointFilter 验证分组列表的 endpoint 过滤：
// 仅返回包含支持对应端点模型的分组；无参数时返回全部可用分组。
func TestGetUserGroupsEndpointFilter(t *testing.T) {
	db := setupEndpointFilterTestDB(t)

	insertTestChannel(t, db, "default-img", "gpt-image-1", "default", constant.ChannelTypeOpenAI)
	model.RefreshPricing()

	user := &model.User{
		Username: "endpoint-group-user",
		Password: "password123",
		Group:    "default",
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
	}
	require.NoError(t, db.Create(user).Error)

	// 无参数：返回全部可用分组（default/vip 来自默认可用分组与分组倍率设置）
	resp := doUserGroupsRequest(t, "", user.Id)
	require.True(t, resp.Success)
	require.Contains(t, resp.Data, "default")
	require.Contains(t, resp.Data, "vip")

	// ?endpoint=image-generation：vip 分组无启用模型被过滤
	resp = doUserGroupsRequest(t, "?endpoint=image-generation", user.Id)
	require.True(t, resp.Success)
	require.Contains(t, resp.Data, "default")
	require.NotContains(t, resp.Data, "vip")
}

// TestGetUserModelsPromptOptimizeFilter 验证 ?prompt_optimize=true 过滤：
// 仅返回被管理员标记 prompt_optimize=1 的模型；可与 ?endpoint= 组合取交集；未传时行为不变。
func TestGetUserModelsPromptOptimizeFilter(t *testing.T) {
	db := setupEndpointFilterTestDB(t)

	insertTestChannel(t, db, "openai-chat", "gpt-4o,gpt-4o-mini,claude-3-5-sonnet", "default", constant.ChannelTypeOpenAI)

	// 仅标记 gpt-4o；gpt-4o-mini 显式为 0；claude-3-5-sonnet 无元数据记录
	require.NoError(t, db.Create(&model.Model{
		ModelName:      "gpt-4o",
		NameRule:       model.NameRuleExact,
		Status:         1,
		PromptOptimize: 1,
	}).Error)
	require.NoError(t, db.Create(&model.Model{
		ModelName:      "gpt-4o-mini",
		NameRule:       model.NameRuleExact,
		Status:         1,
		PromptOptimize: 0,
	}).Error)
	model.RefreshPricing()

	user := &model.User{
		Username: "prompt-optimize-user",
		Password: "password123",
		Group:    "default",
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
	}
	require.NoError(t, db.Create(user).Error)

	// 无参数：返回全部模型（兼容性回归）
	resp := doUserModelsRequest(t, "", user.Id)
	require.True(t, resp.Success)
	require.ElementsMatch(t, []string{"gpt-4o", "gpt-4o-mini", "claude-3-5-sonnet"}, resp.Data)

	// ?prompt_optimize=true：仅返回被标记的模型
	resp = doUserModelsRequest(t, "?prompt_optimize=true", user.Id)
	require.True(t, resp.Success)
	require.ElementsMatch(t, []string{"gpt-4o"}, resp.Data)

	// 与 ?endpoint= 组合取交集
	resp = doUserModelsRequest(t, "?prompt_optimize=true&endpoint=openai", user.Id)
	require.True(t, resp.Success)
	require.ElementsMatch(t, []string{"gpt-4o"}, resp.Data)

	// 与不匹配的端点组合：交集为空
	resp = doUserModelsRequest(t, "?prompt_optimize=true&endpoint=image-generation", user.Id)
	require.True(t, resp.Success)
	require.Empty(t, resp.Data)
}

// TestGetUserModelsPromptOptimizeFilterNameRule 验证标记记录按名称规则（前缀/后缀/包含）匹配实际模型名：
// 元数据以「qwen3.8」前缀规则标记时，渠道中所有 qwen3.8-* 模型均应命中。
func TestGetUserModelsPromptOptimizeFilterNameRule(t *testing.T) {
	db := setupEndpointFilterTestDB(t)

	insertTestChannel(t, db, "qwen-chat", "qwen3.8-plus,qwen3.8-flash,qwen-max", "default", constant.ChannelTypeOpenAI)

	// 前缀规则标记 qwen3.8
	require.NoError(t, db.Create(&model.Model{
		ModelName:      "qwen3.8",
		NameRule:       model.NameRulePrefix,
		Status:         1,
		PromptOptimize: 1,
	}).Error)
	model.RefreshPricing()

	user := &model.User{
		Username: "prompt-optimize-rule-user",
		Password: "password123",
		Group:    "default",
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
	}
	require.NoError(t, db.Create(user).Error)

	// 前缀规则：qwen3.8-plus / qwen3.8-flash 命中，qwen-max 不命中
	resp := doUserModelsRequest(t, "?prompt_optimize=true", user.Id)
	require.True(t, resp.Success)
	require.ElementsMatch(t, []string{"qwen3.8-plus", "qwen3.8-flash"}, resp.Data)

	// 改为后缀规则：仅 *-flash 命中
	require.NoError(t, db.Model(&model.Model{}).Where("model_name = ?", "qwen3.8").
		Updates(map[string]any{"model_name": "-flash", "name_rule": model.NameRuleSuffix}).Error)
	resp = doUserModelsRequest(t, "?prompt_optimize=true", user.Id)
	require.True(t, resp.Success)
	require.ElementsMatch(t, []string{"qwen3.8-flash"}, resp.Data)

	// 改为包含规则：所有含 "3.8" 的模型命中
	require.NoError(t, db.Model(&model.Model{}).Where("model_name = ?", "-flash").
		Updates(map[string]any{"model_name": "3.8", "name_rule": model.NameRuleContains}).Error)
	resp = doUserModelsRequest(t, "?prompt_optimize=true", user.Id)
	require.True(t, resp.Success)
	require.ElementsMatch(t, []string{"qwen3.8-plus", "qwen3.8-flash"}, resp.Data)
}
