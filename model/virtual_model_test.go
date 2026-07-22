package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupVirtualModelTestDB(t *testing.T) {
	t.Helper()
	common.UsingSQLite = true
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	DB = db
	require.NoError(t, DB.AutoMigrate(&Channel{}, &Ability{}, &VirtualModel{}))
	// 清空缓存，避免测试间互相污染
	virtualModelCacheLock.Lock()
	virtualModelCache = make(map[string]*VirtualModel)
	virtualModelCacheLock.Unlock()
}

func newSpeedVirtualModel(t *testing.T, name string) *VirtualModel {
	t.Helper()
	vm := &VirtualModel{
		Name:   name,
		Mode:   VirtualModelModeSpeed,
		Status: common.ChannelStatusEnabled,
	}
	require.NoError(t, vm.SetTargets([]VirtualModelTarget{
		{Model: "gpt-4o"},
		{Model: "gpt-4o-mini"},
	}))
	return vm
}

func TestVirtualModelValidate(t *testing.T) {
	setupVirtualModelTestDB(t)

	// 名称为空
	vm := newSpeedVirtualModel(t, "")
	assert.Error(t, vm.Validate())

	// 模式非法
	vm = newSpeedVirtualModel(t, "vm-bad-mode")
	vm.Mode = "unknown"
	assert.Error(t, vm.Validate())

	// 速度模式子模型不足 2 个
	vm = &VirtualModel{Name: "vm-one-target", Mode: VirtualModelModeSpeed, Status: 1}
	require.NoError(t, vm.SetTargets([]VirtualModelTarget{{Model: "gpt-4o"}}))
	assert.Error(t, vm.Validate())

	// 质量模式缺少聚合模型
	vm = &VirtualModel{Name: "vm-quality-no-agg", Mode: VirtualModelModeQuality, Status: 1}
	require.NoError(t, vm.SetTargets([]VirtualModelTarget{{Model: "gpt-4o"}}))
	assert.Error(t, vm.Validate())

	// 质量模式配置完整
	vm = &VirtualModel{Name: "vm-quality-ok", Mode: VirtualModelModeQuality, Status: 1}
	require.NoError(t, vm.SetTargets([]VirtualModelTarget{{Model: "gpt-4o"}}))
	require.NoError(t, vm.SetAggregator(&VirtualModelAggregator{Model: "gpt-4o"}))
	assert.NoError(t, vm.Validate())

	// 指定不存在的渠道
	vm = &VirtualModel{Name: "vm-bad-channel", Mode: VirtualModelModeSpeed, Status: 1}
	require.NoError(t, vm.SetTargets([]VirtualModelTarget{
		{Model: "gpt-4o", ChannelId: 9999},
		{Model: "gpt-4o-mini"},
	}))
	assert.Error(t, vm.Validate())

	// 抢跑时间为负数
	vm = newSpeedVirtualModel(t, "vm-negative-headstart")
	vm.HeadStartStreamMs = -1
	assert.Error(t, vm.Validate())
	vm = newSpeedVirtualModel(t, "vm-negative-headstart2")
	vm.HeadStartNonStreamMs = -1
	assert.Error(t, vm.Validate())

	// 抢跑时间合法（0=关闭）
	vm = newSpeedVirtualModel(t, "vm-headstart-ok")
	vm.HeadStartStreamMs = 500
	vm.HeadStartNonStreamMs = 2000
	assert.NoError(t, vm.Validate())
}

func TestVirtualModelCRUDAndCache(t *testing.T) {
	setupVirtualModelTestDB(t)

	vm := newSpeedVirtualModel(t, "vm-speed-1")
	require.NoError(t, vm.Insert())
	assert.NotZero(t, vm.Id)

	// 缓存命中
	cached := GetVirtualModel("vm-speed-1")
	require.NotNil(t, cached)
	assert.Equal(t, vm.Id, cached.Id)
	assert.Equal(t, VirtualModelModeSpeed, cached.Mode)

	// 名称列表
	names := GetEnabledVirtualModelNames()
	assert.Contains(t, names, "vm-speed-1")

	// 禁用后缓存不再命中
	vm.Status = common.ChannelStatusManuallyDisabled
	require.NoError(t, vm.Update())
	assert.Nil(t, GetVirtualModel("vm-speed-1"))
	assert.NotContains(t, GetEnabledVirtualModelNames(), "vm-speed-1")

	// 删除
	require.NoError(t, DeleteVirtualModel(vm.Id))
	var count int64
	require.NoError(t, DB.Model(&VirtualModel{}).Count(&count).Error)
	assert.Zero(t, count)
}

func TestVirtualModelNameConflict(t *testing.T) {
	setupVirtualModelTestDB(t)

	// abilities 表中已有真实模型 gpt-real
	require.NoError(t, DB.Create(&Ability{Group: "default", Model: "gpt-real", ChannelId: 1, Enabled: true}).Error)

	vm := newSpeedVirtualModel(t, "gpt-real")
	assert.Error(t, vm.Insert(), "与真实模型重名应拒绝插入")

	vm = newSpeedVirtualModel(t, "gpt-virtual")
	assert.NoError(t, vm.Insert())
}

func TestVirtualModelBranchFactor(t *testing.T) {
	setupVirtualModelTestDB(t)

	speed := newSpeedVirtualModel(t, "vm-factor-speed")
	assert.Equal(t, 2, speed.BranchFactor())

	quality := &VirtualModel{Name: "vm-factor-quality", Mode: VirtualModelModeQuality, Status: 1}
	require.NoError(t, quality.SetTargets([]VirtualModelTarget{
		{Model: "a"}, {Model: "b"}, {Model: "c"},
	}))
	require.NoError(t, quality.SetAggregator(&VirtualModelAggregator{Model: "agg"}))
	assert.Equal(t, 4, quality.BranchFactor())

	empty := &VirtualModel{Name: "vm-factor-empty", Mode: VirtualModelModeSpeed}
	assert.Equal(t, 1, empty.BranchFactor())
}
