# 前端数据获取策略（TanStack Query）

> 代码位置：[web/default/src/lib/query-client.ts](../web/default/src/lib/query-client.ts)，
> 在 [main.tsx](../web/default/src/main.tsx) 中通过 `createQueryClient(router)` 应用到全应用。

## 核心原则

**框架不得在任何未被明确请求的时机自动拉取远端数据。**
远端数据与本地编辑状态（表单）必须解耦，远端值永远不允许在用户无感知的情况下覆盖界面。

## 全局配置

```ts
queries: {
  refetchOnWindowFocus: false  // 切换浏览器标签页不触发请求
  refetchOnReconnect: false    // 网络恢复不触发请求
  refetchOnMount: false        // 组件重新挂载只读缓存，不触发请求
  // 不设置 staleTime：三个触发器全部关闭后，数据过期本身不会引发请求
}
```

## 数据加载的合法时机（仅此三种）

| 时机 | 触发方式 | 典型场景 |
|---|---|---|
| 缓存缺失 | 首次挂载某个 query key | 进入页面且本地无数据 |
| 显式失效 | `queryClient.invalidateQueries({ queryKey })` | 增/删/改操作成功后同步列表 |
| 手动刷新 | `refetch()` / `refetchQueries()` | 用户点击刷新按钮 |

## 违反本策略的写法（禁止）

- `useQuery` 里传 `refetchOnWindowFocus: true` / `refetchInterval` 覆盖全局策略
  （例外：仪表盘、日志查看器等**明确以轮询为目的**的页面可显式声明
  `refetchInterval`，已有：`performance-overview`、`view-logs-dialog`、`performance-health-panel`）。
- 表单组件监听 `props.defaultValues` 变化去 `form.reset()`。远端数据刷新后
  props 引用必然变化，任何形式的远端覆盖（含"内容比对后才 reset"）都可能在
  保存中途把用户正在编辑的表单改写掉。表单初始化只在挂载时进行一次。
- 保存前用「与服务器值 diff 为空则拦截提交」的逻辑。保存必须无条件提交，
  是否有变化由服务器判断，前端不做拦截（历史教训：编辑被 refetch 清空后
  点保存被「没有需要保存的更改」拦截）。

## react-hook-form 点号字段名陷阱（历史教训）

RHF 会把 `Controller`/`register` 的 `name` 按 `.` 解析为**嵌套路径**。
若字段名含点号（如后端 option key `mcp_setting.group_image_models` 直接用作
`name`），`field.onChange` 会把值写进 `values.mcp_setting.group_image_models`
嵌套分支，而 `handleSubmit`/`zodResolver` 按扁平 key 读取——结果是
**UI 正常、onChange 正常，但提交时用户编辑被静默丢弃，发出初始值**
（曾导致 MCP 设置页保存丢失全部修改）。

规则：表单字段名一律使用**不含点号的短名**，初始化/提交时与真实存储 key
做显式映射（参见 [mcp-settings-card.tsx](../web/default/src/features/system-settings/models/mcp-settings-card.tsx)
的 `OPTION_TO_FIELD` / `FIELD_TO_OPTION`）。

## 远端变更同步的既定方向

当前采用「显式失效」模型：变更操作成功后由调用方 invalidate 对应 query key。
若未来需要更强的多标签页/多用户一致性，应基于**版本号 + 增量同步**实现，
而不是恢复任何形式的自动覆盖式 refetch。

## 回归测试

[src/lib/query-client.test.ts](../web/default/src/lib/query-client.test.ts)
固定了上述策略：任何把自动 refetch 触发器改回 `true` 的改动都会导致测试失败。
