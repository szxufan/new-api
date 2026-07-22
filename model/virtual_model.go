package model

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/QuantumNous/new-api/common"
)

const (
	// VirtualModelModeSpeed 速度模式：并发请求所有子模型，取第一个有效回复
	VirtualModelModeSpeed = "speed"
	// VirtualModelModeQuality 质量模式：等待所有子模型回复后由聚合模型给出最终答案
	VirtualModelModeQuality = "quality"
)

// VirtualModelTarget 虚拟模型的子模型配置
type VirtualModelTarget struct {
	Model     string `json:"model"`                // 真实模型名（必填）
	ChannelId int    `json:"channel_id,omitempty"` // 指定渠道；0=按分组自动选择
	Group     string `json:"group,omitempty"`      // 自动选择时使用的分组；空=跟随请求者分组
}

// VirtualModelAggregator 质量模式的聚合模型配置
type VirtualModelAggregator struct {
	Model          string `json:"model"` // 聚合模型（质量模式必填）
	ChannelId      int    `json:"channel_id,omitempty"`
	Group          string `json:"group,omitempty"`
	PromptTemplate string `json:"prompt_template,omitempty"` // 留空使用内置默认模板
}

// VirtualModel 虚拟模型：一个虚拟模型名映射到多个真实模型+渠道
type VirtualModel struct {
	Id                   int    `json:"id" gorm:"primaryKey"`
	Name                 string `json:"name" gorm:"uniqueIndex;size:128;not null"` // 用户请求时使用的虚拟模型名
	Mode                 string `json:"mode" gorm:"size:16;not null"`              // speed | quality
	Targets              string `json:"targets" gorm:"type:text"`                  // JSON: []VirtualModelTarget
	Aggregator           string `json:"aggregator" gorm:"type:text"`               // JSON: VirtualModelAggregator
	HeadStartStreamMs    int    `json:"head_start_stream_ms" gorm:"default:0"`     // 速度模式-流式请求抢跑时间（毫秒），0=关闭
	HeadStartNonStreamMs int    `json:"head_start_non_stream_ms" gorm:"default:0"` // 速度模式-非流式请求抢跑时间（毫秒），0=关闭
	QualityTriggerCount  int    `json:"quality_trigger_count" gorm:"default:1"`    // 质量模式：收到第 N 个子模型回复后启动等待计时
	QualityWaitMs        int    `json:"quality_wait_ms" gorm:"default:0"`          // 质量模式：触发后等待其余模型的最长时间（毫秒），0=关闭
	Status               int    `json:"status" gorm:"default:1"`                   // 1=启用 2=禁用
	CreatedTime          int64  `json:"created_time" gorm:"bigint"`
	UpdatedTime          int64  `json:"updated_time" gorm:"bigint"`
}

func (vm *VirtualModel) GetTargets() ([]VirtualModelTarget, error) {
	var targets []VirtualModelTarget
	if vm.Targets == "" {
		return targets, nil
	}
	err := json.Unmarshal([]byte(vm.Targets), &targets)
	return targets, err
}

func (vm *VirtualModel) SetTargets(targets []VirtualModelTarget) error {
	data, err := json.Marshal(targets)
	if err != nil {
		return err
	}
	vm.Targets = string(data)
	return nil
}

func (vm *VirtualModel) GetAggregator() (*VirtualModelAggregator, error) {
	if vm.Aggregator == "" {
		return nil, nil
	}
	var agg VirtualModelAggregator
	err := json.Unmarshal([]byte(vm.Aggregator), &agg)
	if err != nil {
		return nil, err
	}
	return &agg, nil
}

func (vm *VirtualModel) SetAggregator(agg *VirtualModelAggregator) error {
	if agg == nil {
		vm.Aggregator = ""
		return nil
	}
	data, err := json.Marshal(agg)
	if err != nil {
		return err
	}
	vm.Aggregator = string(data)
	return nil
}

// Validate 校验虚拟模型配置合法性
func (vm *VirtualModel) Validate() error {
	vm.Name = strings.TrimSpace(vm.Name)
	if vm.Name == "" {
		return errors.New("虚拟模型名称不能为空")
	}
	if vm.Mode != VirtualModelModeSpeed && vm.Mode != VirtualModelModeQuality {
		return fmt.Errorf("不支持的模式: %s", vm.Mode)
	}
	targets, err := vm.GetTargets()
	if err != nil {
		return fmt.Errorf("子模型配置解析失败: %w", err)
	}
	if vm.Mode == VirtualModelModeSpeed && len(targets) < 2 {
		return errors.New("速度模式至少需要 2 个子模型")
	}
	if len(targets) < 1 {
		return errors.New("至少需要 1 个子模型")
	}
	for i, t := range targets {
		if strings.TrimSpace(t.Model) == "" {
			return fmt.Errorf("第 %d 个子模型的模型名不能为空", i+1)
		}
		if t.ChannelId > 0 {
			if _, err := GetChannelById(t.ChannelId, false); err != nil {
				return fmt.Errorf("第 %d 个子模型指定的渠道 #%d 不存在", i+1, t.ChannelId)
			}
		}
	}
	if vm.Mode == VirtualModelModeQuality {
		agg, err := vm.GetAggregator()
		if err != nil {
			return fmt.Errorf("聚合配置解析失败: %w", err)
		}
		if agg == nil || strings.TrimSpace(agg.Model) == "" {
			return errors.New("质量模式必须配置聚合模型")
		}
		if agg.ChannelId > 0 {
			if _, err := GetChannelById(agg.ChannelId, false); err != nil {
				return fmt.Errorf("聚合模型指定的渠道 #%d 不存在", agg.ChannelId)
			}
		}
	}
	if vm.HeadStartStreamMs < 0 || vm.HeadStartNonStreamMs < 0 {
		return errors.New("抢跑时间不能为负数")
	}
	if vm.QualityWaitMs < 0 {
		return errors.New("质量模式等待时间不能为负数")
	}
	if vm.QualityTriggerCount < 0 {
		return errors.New("质量模式触发数量不能为负数")
	}
	return nil
}

// IsVirtualModelNameConflict 检查名称是否与 abilities 表中已有真实模型冲突
func IsVirtualModelNameConflict(name string) (bool, error) {
	var count int64
	err := DB.Model(&Ability{}).Where("model = ?", name).Count(&count).Error
	return count > 0, err
}

// BranchFactor 返回最坏情况下的并发上游请求数（用于预扣费放大）
func (vm *VirtualModel) BranchFactor() int {
	targets, err := vm.GetTargets()
	if err != nil || len(targets) == 0 {
		return 1
	}
	factor := len(targets)
	if vm.Mode == VirtualModelModeQuality {
		factor++ // 聚合模型额外一次请求
	}
	return factor
}

// ---------------- 内存缓存 ----------------

var virtualModelCache = make(map[string]*VirtualModel) // name -> enabled VirtualModel
var virtualModelCacheLock sync.RWMutex

// InitVirtualModelCache 从数据库加载全部启用的虚拟模型到内存缓存
func InitVirtualModelCache() {
	var vms []*VirtualModel
	if err := DB.Where("status = ?", common.ChannelStatusEnabled).Find(&vms).Error; err != nil {
		common.SysLog("failed to load virtual models: " + err.Error())
		return
	}
	newCache := make(map[string]*VirtualModel, len(vms))
	for _, vm := range vms {
		newCache[vm.Name] = vm
	}
	virtualModelCacheLock.Lock()
	virtualModelCache = newCache
	virtualModelCacheLock.Unlock()
}

// GetVirtualModel 按名称获取启用的虚拟模型，未命中返回 nil
func GetVirtualModel(name string) *VirtualModel {
	virtualModelCacheLock.RLock()
	defer virtualModelCacheLock.RUnlock()
	return virtualModelCache[name]
}

// GetVirtualModelById 按 ID 获取虚拟模型（含禁用），供执行层按 context 中的 ID 加载
func GetVirtualModelById(id int) (*VirtualModel, error) {
	virtualModelCacheLock.RLock()
	for _, vm := range virtualModelCache {
		if vm.Id == id {
			virtualModelCacheLock.RUnlock()
			return vm, nil
		}
	}
	virtualModelCacheLock.RUnlock()
	var vm VirtualModel
	if err := DB.First(&vm, id).Error; err != nil {
		return nil, err
	}
	if vm.Status != common.ChannelStatusEnabled {
		return nil, errors.New("虚拟模型已禁用")
	}
	return &vm, nil
}

// GetEnabledVirtualModelNames 返回全部启用的虚拟模型名
func GetEnabledVirtualModelNames() []string {
	virtualModelCacheLock.RLock()
	defer virtualModelCacheLock.RUnlock()
	names := make([]string, 0, len(virtualModelCache))
	for name := range virtualModelCache {
		names = append(names, name)
	}
	return names
}

// ---------------- CRUD ----------------

func GetAllVirtualModels() ([]*VirtualModel, error) {
	var vms []*VirtualModel
	err := DB.Order("id desc").Find(&vms).Error
	return vms, err
}

func (vm *VirtualModel) Insert() error {
	if err := vm.Validate(); err != nil {
		return err
	}
	conflict, err := IsVirtualModelNameConflict(vm.Name)
	if err != nil {
		return err
	}
	if conflict {
		return fmt.Errorf("虚拟模型名称 %q 与已有真实模型冲突", vm.Name)
	}
	now := common.GetTimestamp()
	vm.CreatedTime = now
	vm.UpdatedTime = now
	if vm.Status == 0 {
		vm.Status = common.ChannelStatusEnabled
	}
	if err := DB.Create(vm).Error; err != nil {
		return err
	}
	InitVirtualModelCache()
	return nil
}

func (vm *VirtualModel) Update() error {
	if err := vm.Validate(); err != nil {
		return err
	}
	conflict, err := IsVirtualModelNameConflict(vm.Name)
	if err != nil {
		return err
	}
	if conflict {
		return fmt.Errorf("虚拟模型名称 %q 与已有真实模型冲突", vm.Name)
	}
	vm.UpdatedTime = common.GetTimestamp()
	if err := DB.Model(&VirtualModel{}).Where("id = ?", vm.Id).Updates(map[string]interface{}{
		"name":                     vm.Name,
		"mode":                     vm.Mode,
		"targets":                  vm.Targets,
		"aggregator":               vm.Aggregator,
		"head_start_stream_ms":     vm.HeadStartStreamMs,
		"head_start_non_stream_ms": vm.HeadStartNonStreamMs,
		"quality_trigger_count":    vm.QualityTriggerCount,
		"quality_wait_ms":          vm.QualityWaitMs,
		"status":                   vm.Status,
		"updated_time":             vm.UpdatedTime,
	}).Error; err != nil {
		return err
	}
	InitVirtualModelCache()
	return nil
}

func DeleteVirtualModel(id int) error {
	if err := DB.Delete(&VirtualModel{}, id).Error; err != nil {
		return err
	}
	InitVirtualModelCache()
	return nil
}
