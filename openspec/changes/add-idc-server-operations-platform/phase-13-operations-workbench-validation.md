# 第13阶段运营工作台实现记录

## 本批已实现

- `apps/lina-plugins/idc-core/backend/internal/service/operations/` 新增有界运营快照读取：订单、任务、风险、欠费账单四类队列使用一次 `UNION ALL` 聚合查询；账单金额按原币种分组，未配置汇率时不会相加。
- 后台接口 `/api/v1/admin/operations/workbench` 由精确权限 `idc:admin:operations:workbench:view` 保护，返回生成时间、队列数量和原币种金额。
- 插件隔离页面 `/idc/operations/workbench` 支持首次加载、手动刷新、失败重试提示和中英文显示；不修改宿主前端。
- 风险判断器只接受商家明确传入的规则阈值，不在代码里偷偷写默认金额或次数；命中后返回规则编号、动作和可审计证据代码。

## 验证

```text
$env:GOWORK='off'
go test ./backend/internal/service/operations ./backend/internal/service/saleschannel ./backend/internal/controller/admin ./backend/api/admin/v1 -count=1
```

通过。

```text
pnpm test:typecheck
pnpm build
.\make.cmd lint.go dir=apps/lina-plugins/idc-core plugins=1
.\make.cmd i18n.check plugins=1
.\make.cmd plugins.check
openspec validate add-idc-server-operations-platform --strict
git diff --check
```

以上门禁通过。`TC016-operations-workbench.ts` 已创建；四个业务端口启动后，真实浏览器已打开运营工作台并确认队列、报表、告警和任务中心正常显示。

## 最终完成状态

13.1 至 13.10 已收口。运营快照仅在业务水位或内容变化时递增修订，定时刷新使用内部可信入口；HTTP 读取要求宿主业务上下文和平台级数据范围，匿名、窄范围和模拟身份均失败关闭。

## 本批增量

- 工作台快照新增 `watermark`/`changed`，并注册单主节点每 30 秒刷新任务；同时提供原币种报表、异步任务列表和失败/结果未知/长时间运行告警接口。
- 新增后台运营页面的报表和告警区块，刷新时并行读取快照、报表和告警，金额仍按币种分开显示。
- 新增商品暂停影响只读预览 `GET /api/v1/admin/catalog/products/{productNo}/pause-impact-preview`：统计购物车、未付款单、已付款未完成单和已售服务，明确暂停只阻止新销售，不改变已有服务。
- 聚合队列扩展到待开通、暂停商品、到期服务、工单、退款、支付异常、换 IP、个人/企业实名、合同异常、待处理合同、佣金结算、通知失败、资源告警和进行中操作，共 21 类。
- 经营报表使用一次集合化 CTE，逐币种返回收入、应收、实收、现金流、余额、退款、成本、毛利、客户、订单、商品、销售、AFF 和供应商质量。
- `TC008`已被内容管理阶段占用，因此使用未冲突的`TC016-operations-workbench.ts`承载阶段 13 场景，未覆盖既有用例。
- 后台真实页面`http://127.0.0.1:15666/admin/idc/operations/workbench`已在浏览器确认 21 类队列、原币种报表、任务告警和异步任务中心均能实际加载。
- 当前工作区已重新通过 Go 全包测试、真实 PostgreSQL 测试、插件 lint、前端类型检查与构建、E2E 治理、插件治理、i18n、严格 OpenSpec 校验和`git diff --check`。
