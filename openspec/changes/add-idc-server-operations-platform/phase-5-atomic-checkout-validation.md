# 第五阶段原子结账验证记录

> **`FB-52`部分失效声明：**本记录中的本地原子事务、幂等、金额和 Outbox 思路仅可作为迁移对账参考；逐台服务器履约单元、资源池库存锁和 Provider 快照不是通用结账模型，不能作为当前阶段完成证据。现行结账按订单项冻结商品版本简单库存和一份通用交付记录；财务结清后才按冻结插件契约的`per_order_item`或`per_quantity_unit`粒度创建履约意图，不在结账中预建服务器单元。

## 完成范围

原任务`5.4`及`5.4.a`至`5.4.d`只代表旧模型曾经实现。订单、订单项、原始待付账单、账单明细、商品简单库存预留、营销名额核销、购物车关闭和订单 Outbox 仍应保持本地原子提交；逐台履约单元和资源池库存必须按`FB-52`重写。

当前切片没有新增 HTTP 路由或页面，也没有创建支付尝试、支付/退款分配、账本交易、分录、余额快照、服务或 Provider 任务。首次付款实名、支付有效期后台配置、过期取消与释放、支付渠道和门户结算页面分别由`5.5`至`5.7`及后续支付任务负责。

## 核心不变量

- 所有权只从当前活动客户主体取得。报价必须属于该客户、来源为客户新购、仍为活动购物车的最新报价，购物车与报价均未过期，客户主体版本和状态没有变化。
- 原始结账幂等键只计算`SHA-256`后保存。事务先锁“客户 + 幂等摘要”作用域；相同键和相同请求返回首个订单，相同键换报价或同报价换键拒绝。
- 锁顺序固定为幂等作用域、报价、购物车、客户、排序后的资源池绑定、库存快照、营销预留。全部结账读写显式使用事务回调传入的`gdb.TX`，不依赖数据库上下文的隐式事务路由。
- 资源池绑定是库存互斥与容量汇总边界。报价中的快照必须仍是该绑定按`observed_at DESC, id DESC`确定的最新快照；活动`held/committed`预留按整个绑定集合汇总，不能通过生成新快照绕过旧预留。
- 自动库存项目创建一份`held`预留，持有截止时间与订单及账单付款截止时间一致；同项目的所有逐台履约单元引用这份预留。`manual_confirmation`项目不伪造快照或预留，并使订单进入人工审核。
- 单次结账最多处理`100`个报价项目和`1000`个逐台履约单元。报价项、订单项、账单行、预留和履约单元均集合读取或批量写入，没有随项目逐项查询的`N+1`路径。
- 原始账单以订单唯一。账单头金额复制订单，账单行金额复制不可变订单项，并由`023-billing-ledger-foundation.sql`的复合外键和延迟提交校验复核客户、订单、币种及五类金额一致性。
- 只有带`reservation_reference`的报价调整才要求结账核销。生产默认营销适配器实现核销端口；如果报价含预留但未装配核销器，结账失败关闭，不能静默遗留营销名额。
- `commerce.order.created`通过绑定调用方显式数据库的 Outbox `Enqueuer`写入活动事务。同步路径不调用支付、Provider、通知、合同、税务发票或`AI`，也不创建任何伪支付或资金账本记录。

## 测试与门禁

1. `commerce_checkout_test.go`验证原始幂等键摘要、请求摘要绑定、非法输入、人工库存分流和`1000`个履约单元上限。
2. `TestCommerceCheckoutAtomicPostgreSQL`使用真实 PostgreSQL 同时发起`8`个相同结账请求，所有成功响应收敛到同一订单和账单；直接核对订单、订单项、账单、账单行、预留、两个逐台单元、一个 Outbox、已核销营销预留、已关闭购物车及零账本交易。
3. `TestCommerceCheckoutOutboxFailureRollsBackPostgreSQL`在营销核销之后注入 Outbox 失败，证明订单、账单、预留、履约和事件全部为零，购物车恢复活动，营销预留仍为`active`。
4. `TestCommerceCheckoutInventoryContentionPostgreSQL`让两个客户并发争抢同一绑定最后一个可售单位，结果严格为一单成功、一单报价失效，活动预留合计不超过一。
5. `TestCommerceCheckoutRejectsSupersededInventoryPostgreSQL`在报价后追加更新的权威库存快照，旧报价结账被拒绝且不产生订单。
6. 隔离真实 PostgreSQL 运行`go test ./internal/service/commerce -run '^TestCommerceCheckout' -count=20 -shuffle=on`通过；并发重放、故障回滚、容量争抢和旧快照拒绝持续稳定。
7. `go test ./internal/service/catalogdraft -run '^TestCatalogProductCopyPostgreSQLIntegration$' -count=20 -shuffle=on`通过。全插件门禁曾发现该测试在完整审核夹具已经创建`tax-provider-cny`后再次插入同一税策；已删除重复测试写入，不改生产逻辑。
8. 配置真实 PostgreSQL 后运行`GOWORK=off go test ./... -count=1 -shuffle=on`通过。
9. `make.cmd lint.go plugins=1 dir=apps/lina-plugins/idc-core/backend`通过，`golangci-lint`和`staticcheck U1000`均为零问题。
10. `GOWORK=off go test ./... -count=20 -shuffle=on`通过，覆盖插件全后端持续重放；Windows 环境未启用 CGO，因而无法执行 Go race detector，本轮不把未实际运行的竞态门禁记为通过。
11. `make.cmd plugins.check`扫描`11`份插件清单、`965`个文件，治理问题为零；`openspec validate add-idc-server-operations-platform --strict`通过。
12. `make.cmd build plugins=1 platforms=windows/amd64,linux/amd64 verbose=1`通过，宿主前端、门户插件以及 Windows/Linux 后端产物均成功生成。
13. `make.cmd i18n.check plugins=1`确认本插件运行时硬编码和消息覆盖均为零问题；完整仓库仍被禁止修改的宿主通知预览页缺少中英文各`3`个既有前端键阻断，共`6`项，与本阶段改动无关。任务父项和子项继续不勾选，直至宿主基线修复。

## 复审结论

本轮按`lina-review`检查了客户所有权、对象存在性、幂等冲突、锁序、库存快照新旧、跨快照容量、营销预留、金额复制、事务边界、Outbox 数据库绑定、原始键泄露、外部调用隔离和查询上限。审查期间先发现测试使用错误库存快照号导致容量测试无效，随后改为追加不可变的限量快照；又把库存从“精确快照汇总”收紧为“绑定互斥、最新快照重检、绑定级活动预留汇总”。当前没有遗留的严重或高优先级问题。

## 规则影响记录

- 架构与插件：全部生产逻辑闭环在`idc-core`，没有修改`lina-core`、`lina-vben`、根构建脚本或其他插件；Outbox 只增加显式数据库的窄`Enqueuer`构造，不开放跨插件内部实现。
- 数据权限：服务入口只从当前主体解析客户编号，不接受调用方传入客户编号；当前没有新增 HTTP 数据面，后续`5.7`接口仍需使用客户所有权路由与存在性保护。
- 数据库：复用`022`和`023`表及约束，没有新增或修改迁移、索引、DAO、软删除或时间字段。所有截止时间取数据库时间；账单金额继续由`023`延迟约束保护。
- API 与前端：没有新增 HTTP API、DTO、页面、菜单或用户可见文案，故本轮无接口生成、前端和 E2E 影响；这些入口留在`5.7`与`5.11`。
- `i18n`：没有新增业务错误码、运行时消息、API 文档或界面文案。插件自身无新增缺键；完整仓库门禁仍受禁止修改的宿主通知预览页中英文各`3`个既有缺键阻断，因此任务父项和子项暂不勾选。
- 缓存与外部系统：结账不使用缓存，不调用任何外部网络；PostgreSQL 是唯一同步事实源。

**已读取规则：**`AGENTS.md`、`.agents/rules/openspec.md`、`.agents/rules/documentation.md`、`.agents/rules/architecture.md`、`.agents/rules/data-permission.md`、`.agents/rules/plugin.md`、`.agents/rules/backend-go.md`、`.agents/rules/database.md`、`.agents/rules/testing.md`、`.agents/rules/i18n.md`、`.agents/rules/dev-tooling.md`
