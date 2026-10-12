package controller

import (
	"context"
	"fmt"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay"

	"github.com/gin-gonic/gin"
	"github.com/samber/lo"
)

// virtualModelTestQuestions 简单问题池：每次测试随机抽取一题
var virtualModelTestQuestions = []string{
	"What is the capital of France?",
	"What is 7 multiplied by 8?",
	"Which planet is the largest in the solar system?",
	"How many days are there in a week?",
	"What color do you get when you mix red and blue?",
	"At what temperature does water boil in degrees Celsius?",
	"How many continents are there in the world?",
	"What is the chemical symbol for gold?",
	"Name a mammal that can fly.",
	"How many sides does a triangle have?",
}

const (
	// virtualModelTestMaxTokens 测试请求的 max_tokens，兼顾可读输出与测试成本
	virtualModelTestMaxTokens = 128
	// virtualModelTestTimeout 单次测试的整体超时（含质量模式聚合分支）
	virtualModelTestTimeout = 180 * time.Second
)

// TestVirtualModel 虚拟模型测试：速度模式返回每个子模型的响应时间，
// 质量模式用随机简单问题完整走一遍聚合流程并返回每个子模型与聚合模型的输出
func TestVirtualModel(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	var vm model.VirtualModel
	if err := model.DB.First(&vm, id).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	targets, err := vm.GetTargets()
	if err != nil {
		common.ApiError(c, fmt.Errorf("子模型配置解析失败: %w", err))
		return
	}
	if len(targets) == 0 {
		common.ApiErrorMsg(c, "虚拟模型没有配置子模型")
		return
	}

	testUserID, err := resolveChannelTestUserID(c)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	timeoutCtx, cancel := context.WithTimeout(context.Background(), virtualModelTestTimeout)
	defer cancel()
	testCtx, err := buildVirtualModelTestContext(timeoutCtx, testUserID)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	isStream := c.Query("stream") == "true"
	testReq := &dto.GeneralOpenAIRequest{
		Model:  vm.Name,
		Stream: lo.ToPtr(isStream),
		Messages: []dto.Message{
			{
				Role:    "user",
				Content: buildVirtualModelTestQuestion(),
			},
		},
		MaxTokens: lo.ToPtr(uint(virtualModelTestMaxTokens)),
	}

	result := relay.RunVirtualModelTest(testCtx, &vm, testReq)
	common.ApiSuccess(c, result)
}

// buildVirtualModelTestQuestion 随机抽取一个简单问题并追加短随机标记：
// 防止上游 prompt 命中缓存导致测速失真；同一次测试内所有分支使用同一问题保证横向可比
func buildVirtualModelTestQuestion() string {
	question := virtualModelTestQuestions[rand.Intn(len(virtualModelTestQuestions))]
	return fmt.Sprintf("%s (test %s)", question, common.GetRandomString(6))
}

// buildVirtualModelTestContext 构造合成的中继测试上下文（不经过分发与计费中间件），
// 分支内部自带 ContextKeyVirtualBranch 标记，PostTextConsumeQuota 会跳过结算
func buildVirtualModelTestContext(timeoutCtx context.Context, testUserID int) (*gin.Context, error) {
	w := httptest.NewRecorder()
	testCtx, _ := gin.CreateTestContext(w)
	testCtx.Request = (&http.Request{
		Method: "POST",
		URL:    &url.URL{Path: "/v1/chat/completions"},
		Header: make(http.Header),
	}).WithContext(timeoutCtx)
	testCtx.Request.Header.Set("Content-Type", "application/json")

	cache, err := model.GetUserCache(testUserID)
	if err != nil {
		return nil, err
	}
	cache.WriteContext(testCtx)
	testCtx.Set("id", testUserID)
	return testCtx, nil
}
