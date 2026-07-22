package controller

import (
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay/helper"

	"github.com/gin-gonic/gin"
)

// GetAllVirtualModels 获取全部虚拟模型
func GetAllVirtualModels(c *gin.Context) {
	vms, err := model.GetAllVirtualModels()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, vms)
}

// GetVirtualModel 按 ID 获取单个虚拟模型
func GetVirtualModel(c *gin.Context) {
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
	common.ApiSuccess(c, &vm)
}

// CreateVirtualModel 新建虚拟模型
func CreateVirtualModel(c *gin.Context) {
	var vm model.VirtualModel
	if err := c.ShouldBindJSON(&vm); err != nil {
		common.ApiError(c, err)
		return
	}
	vm.Id = 0
	if err := vm.Insert(); err != nil {
		common.ApiError(c, err)
		return
	}
	warnVirtualModelPricing(c, &vm)
	common.ApiSuccess(c, &vm)
}

// UpdateVirtualModel 更新虚拟模型
func UpdateVirtualModel(c *gin.Context) {
	var vm model.VirtualModel
	if err := c.ShouldBindJSON(&vm); err != nil {
		common.ApiError(c, err)
		return
	}
	if vm.Id == 0 {
		common.ApiErrorMsg(c, "缺少虚拟模型 ID")
		return
	}
	var existing model.VirtualModel
	if err := model.DB.First(&existing, vm.Id).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	vm.CreatedTime = existing.CreatedTime
	if err := vm.Update(); err != nil {
		common.ApiError(c, err)
		return
	}
	warnVirtualModelPricing(c, &vm)
	common.ApiSuccess(c, &vm)
}

// DeleteVirtualModel 删除虚拟模型
func DeleteVirtualModel(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if err := model.DeleteVirtualModel(id); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}

// warnVirtualModelPricing 虚拟模型未配置价格时打印告警
// （未配置价格的虚拟模型在 ListModels 中对普通用户不可见，且预扣/结算按默认回退倍率可能失真）
func warnVirtualModelPricing(c *gin.Context, vm *model.VirtualModel) {
	if !helper.HasModelBillingConfig(vm.Name) {
		common.SysLog("virtual model " + vm.Name + " has no billing config (model ratio), please configure its price in 运营-模型价格")
	}
}
