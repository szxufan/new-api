# 模型定价：模型名称下拉候选（Model Pricing Name Suggestions）

## 背景

在「系统设置 → 计费 → 模型定价」（`/system-settings/billing/model-pricing`）页面中，管理员通过「添加模型」为模型配置价格/倍率。此前模型名称字段是一个纯文本输入框，需要手动键入完整的模型标识符，容易拼写出错，也无法直观看到「哪些渠道模型还没有配置定价」。

本功能将模型名称字段升级为「可输入 + 可下拉选择」的组合框（Combobox）：

- 下拉候选来自**启用渠道中已配置的模型**，管理员可以直接点选，无需手输；
- 候选列表会**排除当前定价草稿中已设置过定价的模型**，只提示尚未定价的模型；
- 仍然**保留手动输入任意模型名**的能力（候选只是辅助，不是约束）。

## 候选数据源

- 候选模型通过现有接口 `GET /api/channel/models_enabled` 获取，内容为：
  - `abilities` 表中 `enabled = true` 的去重模型名（即**启用渠道**上配置的模型）；
  - 以及启用的**虚拟模型**。
- 该接口要求管理员权限（`AdminAuth`），与模型定价页面的访问权限一致。
- 禁用渠道上配置的模型**不会**出现在候选中（但仍可手动输入）。

## 过滤规则

候选列表 = 启用渠道模型 − 当前草稿中已定价的模型。

「已定价」以**当前页面草稿**为准：可视化编辑器从 `ModelPrice`、`ModelRatio`、`CacheRatio`、`CreateCacheRatio`、`CompletionRatio`、`ImageRatio`、`AudioRatio`、`AudioCompletionRatio`、`billing_setting.billing_mode`、`billing_setting.billing_expr` 这 10 个 JSON 配置中解析出所有已出现的模型名。因此：

- 新增并保存某模型定价后，该模型会立即从候选中消失；
- 删除某模型的定价后，该模型会重新出现在候选中；
- 判定基于草稿而非后端已保存状态，与表格中展示的模型集合保持一致，避免草稿未保存时重复添加。

## 交互说明

- 「添加模型」时：模型名称字段显示下拉触发器，聚焦或输入时展示候选，支持按输入内容过滤；选中候选即填入表单。
- 输入任意自定义名称后按回车，可直接使用该自定义值（`allowCustomValue`）。
- 「编辑」已有模型定价时：模型名称字段保持禁用（名称作为主键不可修改），行为与之前一致。
- 接口请求失败或无候选时，下拉为空，手动输入不受影响。

## 实现位置（前端）

| 文件 | 说明 |
|---|---|
| `web/default/src/features/system-settings/models/model-name-options.ts` | 纯函数 `buildModelNameOptions`：去重、去空白、排除已定价模型、按字典序排序，生成下拉选项 |
| `web/default/src/features/system-settings/models/model-ratio-visual-editor.tsx` | 通过 `useQuery`（queryKey `channel_models_enabled`，`staleTime` 5 分钟）拉取启用渠道模型，结合当前草稿已定价模型计算候选，并下传给编辑面板/抽屉 |
| `web/default/src/features/system-settings/models/model-pricing-sheet.tsx` | `ModelPricingEditorPanel` / `ModelPricingSheet` 新增 `modelNameOptions` 属性；新增模式下模型名称用 `Combobox`（`allowCustomValue`）渲染，编辑模式保持禁用输入框 |
| `web/default/src/features/system-settings/models/model-name-options.test.ts` | `buildModelNameOptions` 单元测试 |

复用了现有 `@/components/ui/combobox` 的 `options + allowCustomValue` 模式（与渠道编辑抽屉的渠道类型选择一致），未引入新组件。

## 测试

```bash
cd web/default
bun run test -- model-name-options
```
