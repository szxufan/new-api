package service

import (
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// seedAbility 向 abilities 表写入一条记录
func seedAbility(t *testing.T, group, modelName string, enabled bool) {
	t.Helper()
	require.NoError(t, model.DB.Create(&model.Ability{
		Group:     group,
		Model:     modelName,
		ChannelId: 1,
		Enabled:   enabled,
	}).Error)
}

// init 提前初始化跨数据库兼容的列名（TestMain 注入 DB 后查询 abilities 需要）
func init() {
	model.InitCommonColumnNames()
}

func TestFindGroupForModel(t *testing.T) {
	// 当前分组为 default 时可用分组集合 = UserUsableGroups(default/vip) + default = {default, vip}
	t.Run("model in current group", func(t *testing.T) {
		seedAbility(t, "default", "model-a", true)
		g, ok := FindGroupForModel("default", "model-a")
		assert.True(t, ok)
		assert.Equal(t, "default", g)
	})

	t.Run("model only in another usable group", func(t *testing.T) {
		seedAbility(t, "vip", "model-b", true)
		g, ok := FindGroupForModel("default", "model-b")
		assert.True(t, ok)
		assert.Equal(t, "vip", g)
	})

	t.Run("multiple groups hit prefers current group", func(t *testing.T) {
		seedAbility(t, "default", "model-c", true)
		seedAbility(t, "vip", "model-c", true)
		g, ok := FindGroupForModel("default", "model-c")
		assert.True(t, ok)
		assert.Equal(t, "default", g)
	})

	t.Run("multiple other groups hit returns lexicographically smallest", func(t *testing.T) {
		// 临时扩充可用分组集合，验证按字典序确定性返回
		original := setting.UserUsableGroups2JSONString()
		require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(`{"default":"","bbb":"","aaa":""}`))
		defer func() { _ = setting.UpdateUserUsableGroupsByJSONString(original) }()

		seedAbility(t, "bbb", "model-d", true)
		seedAbility(t, "aaa", "model-d", true)
		g, ok := FindGroupForModel("default", "model-d")
		assert.True(t, ok)
		assert.Equal(t, "aaa", g)
	})

	t.Run("model only in unusable group", func(t *testing.T) {
		seedAbility(t, "private-group", "model-e", true)
		_, ok := FindGroupForModel("default", "model-e")
		assert.False(t, ok)
	})

	t.Run("disabled ability not matched", func(t *testing.T) {
		seedAbility(t, "default", "model-f", false)
		_, ok := FindGroupForModel("default", "model-f")
		assert.False(t, ok)
	})

	t.Run("nonexistent model", func(t *testing.T) {
		_, ok := FindGroupForModel("default", "model-not-exist")
		assert.False(t, ok)
	})

	t.Run("empty model name", func(t *testing.T) {
		_, ok := FindGroupForModel("default", "")
		assert.False(t, ok)
	})

	t.Run("normalized match via gizmo prefix", func(t *testing.T) {
		// FormatMatchingModelName 将 gpt-4-gizmo-xxx 归一化为 gpt-4-gizmo-*，
		// 渠道 abilities 中只需配置通配名
		seedAbility(t, "default", "gpt-4-gizmo-*", true)
		g, ok := FindGroupForModel("default", "gpt-4-gizmo-abc123")
		assert.True(t, ok)
		assert.Equal(t, "default", g)
	})

	t.Run("user group not in usable set still included", func(t *testing.T) {
		// GetUserUsableGroups 保证用户自身分组一定在集合内
		seedAbility(t, "my-own-group", "model-g", true)
		g, ok := FindGroupForModel("my-own-group", "model-g")
		assert.True(t, ok)
		assert.Equal(t, "my-own-group", g)
	})
}
