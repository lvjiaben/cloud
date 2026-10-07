# 第五阶段交易并发与回滚回归验证记录

> **`FB-52`部分失效声明：**本记录只保留通用幂等、事务回滚、金额守恒和并发框架的迁移对账价值；依赖资源池库存、逐台服务器履约单元或服务器身份的用例不能冒充通用商品回归。当前验收必须改用商品版本简单库存和冻结插件契约，并至少覆盖服务器、`SSL`和手机三类非同构商品。

## 实现范围

本轮完成任务 `5.9` 的交易回归补强。该任务不新增生产接口、数据库结构、页面或业务状态，只审计 `5.1`—`5.8` 已实现的购物车、报价、结账、协助下单和订单取消测试矩阵，并为实际缺失的并发与状态组合补真实 PostgreSQL 回归。

对表后确认原测试已覆盖普通结账 8 路同键重放、同报价换键冲突、双客户争抢最后库存、旧库存快照、报价并发单版本成功、结账 `Outbox` 故障、报价持久化故障、取消 8 路重放、取消 `Outbox` 故障和协助报价审计故障。本轮新增旧报价失效、部分履约取消保护和协助下单并发三项直接证据。

## 回归矩阵

| 风险 | 权威回归 | 结果 |
|---|---|---|
| 并发结算与重复提交 | `TestCommerceCheckoutAtomicPostgreSQL` 使用 8 个 goroutine 同时提交同一客户、报价和幂等键 | 只创建 1 个订单、账单、库存预留和 Outbox，所有调用返回同一订单；同报价换键拒绝 |
| 条件库存扣减 | `TestCommerceCheckoutInventoryContentionPostgreSQL` 让两个客户同时争抢最后 1 个可售单元 | 仅 1 个成功，活动预留总量不超过 1 |
| 库存版本变化 | `TestCommerceCheckoutRejectsSupersededInventoryPostgreSQL` 在报价后写入新的权威库存快照 | 旧库存报价拒绝结账，不创建订单，购物车保持活动 |
| 报价变化 | 新增 `TestCommerceCheckoutRejectsSupersededQuotePostgreSQL`，同一购物车生成新报价后提交旧报价 | 旧报价拒绝结账，购物车仍指向新报价且不产生订单 |
| 后台协助下单 | `TestAssistedQuoteApprovalAndCheckoutPostgreSQL` 增强为 8 路同键并发下单 | 只创建并返回 1 个待付款/人工审核订单，顺序重放继续复用同一订单 |
| 部分履约 | 新增 `TestOrderCancellationRejectsPartialFulfillmentPostgreSQL`，2 个履约单元中 1 个已成功 | 整单取消失败；订单、账单和库存保持原状，成功单元与待付款单元均不被覆盖，不创建取消记录 |
| 结账事务回滚 | `TestCommerceCheckoutOutboxFailureRollsBackPostgreSQL` 注入 Outbox 故障 | 订单、订单项、账单、账单项、库存、履约和 Outbox 全部为 0，购物车及营销预留恢复原状态 |
| 报价事务回滚 | `assertCommerceMarketingRollback` 注入报价快照写入故障 | 营销预留数量、购物车版本和最新报价不变 |
| 取消事务回滚 | `TestOrderCancellationOutboxFailureRollsBackPostgreSQL` 注入取消 Outbox 故障 | 订单、账单、库存、营销和履约状态全部回滚 |
| 协助申请事务回滚 | `TestAssistedQuoteAuditFailureRollsBackPostgreSQL` 注入审计故障 | 协助申请及明细不落库 |

## 验证证据

1. 使用隔离测试数据库运行 `go test ./backend/internal/service/commerce -run 'Test(CommerceCheckout|OrderCancellation|AssistedQuote)' -count=1 -v`：全部选定授权与集成场景通过，新旧交易链路在同一真实 PostgreSQL 运行中通过。
2. 对三个新增或增强的高风险场景运行 `go test ./backend/internal/service/commerce -run 'Test(CommerceCheckoutRejectsSupersededQuote|OrderCancellationRejectsPartialFulfillment|AssistedQuoteApprovalAndCheckout)PostgreSQL' -count=10 -shuffle=on`：通过，耗时约 29 秒。
3. `GOWORK=off go test ./backend/... -count=20 -shuffle=on`：通过；插件全部后端包随机顺序 20 轮稳定。
4. `make.cmd lint.go plugins=1 dir=apps/lina-plugins/idc-core/backend`：通过；`golangci-lint`与 `staticcheck U1000`均为 0 问题。
5. 本轮只修改测试与 OpenSpec 证据，没有生产 Go、SQL、API、前端或清单变更；紧邻的 `5.8` 双平台正式构建结果继续适用。
6. `make.cmd plugins.check format=json`：通过；扫描 1037 个插件文件，0 项治理问题。
7. `openspec validate add-idc-server-operations-platform --strict`：通过。

## 复审结论与未完成边界

本轮按 `lina-review`逐项核对任务文字与现有测试，避免重复堆叠同类 happy-path。复审确认并修补三个真实空白：旧报价在新报价产生后的结账拒绝、部分履约后的整单取消保护、后台协助下单的并发幂等。新增测试直接查询订单、账单、库存、履约、取消和购物车权威表，没有用 mock 代替事务结论。高重复与全包随机测试通过后，未发现 `5.9` 范围内仍缺失的严重或高优先级回归。

任务 `5.9` 的实现和任务内验证已完成。仓库级 `i18n` 门禁仍被禁止修改的宿主通知预览页 6 个既有缺键阻断，因此任务保留未勾选。性能指标与固定 benchmark 属于 `5.10`；完整公开目录、实名恢复、重复提交、变价确认和后台协助订单浏览器链路属于 `5.11`。

## 规则影响记录

- 架构、API、数据权限、数据库和缓存：均无生产影响；本轮没有新增或修改运行时入口、DTO、表、索引、权限或缓存。
- 测试：新增回归全部使用隔离随机 schema 和真实 PostgreSQL；并发 goroutine 写各自结果槽，测试不共享可变断言状态。
- 性能：本轮不是 benchmark，不把功能回归时长冒充 `p95/p99`；正式性能结论留给 `5.10`。
- 前端与 `i18n`：无用户可见行为和文案变更，浏览器测试不适用；仓库既有缺键不由本测试任务修复。
- 开发工具与跨平台：未修改任何构建、测试或生成脚本；生产代码未变化，`5.8` 已通过 Windows/Linux 正式构建。

**已读取规则：** `AGENTS.md`、`.agents/rules/openspec.md`、`.agents/rules/documentation.md`、`.agents/rules/architecture.md`、`.agents/rules/data-permission.md`、`.agents/rules/plugin.md`、`.agents/rules/api-contract.md`、`.agents/rules/backend-go.md`、`.agents/rules/database.md`、`.agents/rules/frontend-ui.md`、`.agents/rules/testing.md`、`.agents/rules/dev-tooling.md`、`.agents/rules/i18n.md`
