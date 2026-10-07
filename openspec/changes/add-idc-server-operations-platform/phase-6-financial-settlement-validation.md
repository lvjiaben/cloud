# 统一财务结清权威事实验证记录

> **`FB-52`部分失效声明：**不可变结清头、来源证据和同事务 Outbox 仍是当前正确边界；把后续履约写成逐台服务器履约单元，或把试用绑定服务器身份的结论已经失效。结清只发布通用财务事实，消费者再按冻结插件契约创建履约意图；本记录不能单独证明通用履约已经完成。

## 范围与结论

本记录覆盖任务`6.12.a`：在`idc-core`内建立订单唯一的不可变财务结清头、最多`1000`条来源证据和同事务去重 Outbox 事件。结清只把订单财务状态推进为`paid`，不会修改库存预留、履约单元、服务权益、资源运行状态或 Provider 任务。

任务内实现、真实 PostgreSQL 集成测试、20 轮随机并发回归、定向 Go lint、插件结构治理和 OpenSpec 严格校验已经通过。仓库级国际化门禁仍被禁止在本任务修改的宿主通知预览页面 6 个既有缺键阻断，因此`6.12.a`保持未勾选，不虚报全仓通过。

## 权威边界

| 边界 | 实现结论 |
|---|---|
| 结清 owner | `billing.FinancialSettlementService`是唯一可持久化`financially_settled`事实的内部权威 |
| 资金来源 | 普通正金额只接受渠道付款、余额和已入账人工收款的有效支付分配净额 |
| 独立资格 | 信用已由`6.12.b`原子接线；试用留给`6.12.c`，当前仍失败关闭 |
| 履约 owner | 结清只发布`commerce.order.financially_settled`；`8.2.b`消费者后续按结清号幂等推进业务操作 |
| 消息语义 | 业务事实和 Outbox 同事务；传输允许至少一次，消费者不得假设网络恰好一次 |
| 数据权限 | 当前入口是无 HTTP 暴露、无列表枚举的系统内部精确订单动作；它固定默认单商家租户并重读订单、账单、客户与实名归属，后续公开调用方必须在自身边界完成权限和客户归属门禁 |

## 来源与拒绝矩阵

| 结清依据 | 当前状态 | 强制证据 |
|---|---|---|
| 渠道付款 | 核心 owner 入口已由`6.12.b`接线，支付插件实际调用待阶段`7` | 同订单账单的不可变渠道支付分配、未冲销账本交易、同一来源引用和金额守恒 |
| 余额 | 支持 | 客户余额借记、商家收入贷记、账单分配和金额守恒 |
| 人工收款 | 支持 | 商家现金借记、应收贷记、已入账申请、证明与到账账户私有引用、必要的第二人批准 |
| 合法零金额 | 支持 | 小计、优惠、净额和税额均为零，且没有支付分配 |
| 百分百优惠 | 支持 | 正数小计被优惠精确抵消，净额和税额为零，且没有支付分配 |
| 有效信用 | 支持 | 同结清事务创建并绑定同订单、账单、客户、币种和金额的有效信用敞口及占用流水 |
| 试用 | 失败关闭 | 必须按客户、商品和订单项购买数量原子占用权威资格，后续履约粒度由冻结插件契约决定 |
| 账单减免、迁移、冲销或不足额来源 | 拒绝 | 不得产生新履约资格 |

## 数据库与性能

- 新增迁移`035-financial-settlement.sql`，只包含 DDL，没有 Seed 或 Mock 数据；表名使用完整插件 ID 前缀。
- 结清头和来源表没有`updated_at`、`deleted_at`，更新与删除由不可变触发器拒绝；自增`id`由 PostgreSQL 身份列生成，业务写入不显式指定。
- 每个订单号和原始订单账单号最多一个结清头；结清来源通过外键精确绑定支付分配或信用敞口，提交时由延迟头约束与语句级来源约束核对数量、类型和金额守恒。
- 迁移使用存在性保护、可替换函数和先删后建触发器，真实数据库重复执行已通过。
- 正金额证据使用一个有界 CTE 查询集合化装配支付分配、账本交易、冲销、分录、账户和人工复核，最多读取`2001`行用于拒绝超限；应用层不会逐来源查询数据库或外部支付插件。
- 新增原交易反查索引，结清头按订单、账单唯一，来源按结清号和身份建立索引；数据库访问次数不随来源行数线性增加。

## 依赖注入与模块影响

- 新构造函数逐项接收显式`gdb.DB`和`outbox.Enqueuer`，不使用`Deps/Options`聚合，也不在构造函数或请求路径临时创建关键服务。
- 数据库 owner 是调用结清协调器的`core`启动装配；Outbox owner 是既有`outbox.NewEnqueuer`，必须绑定同一个显式数据库，并通过 GoFrame 事务上下文加入调用方事务。余额、人工收款、信用和渠道资金核心入口已由`6.12.b`完成生产装配，支付插件实际调用留在阶段`7`；当前没有隐藏全局服务图。后续证据见[财务结清 Owner 接线验证记录](phase-6-financial-settlement-owner-validation.md)。
- 没有新增缓存、缓存失效或跨节点派生状态，缓存一致性无影响。
- 没有新增 HTTP API、Controller、路由、前端页面或用户可观察交互，不触发 API DTO、前端或 E2E 资产变更。
- 新增 4 个稳定`bizerr`错误码及中英文资源；插件自身运行时消息覆盖通过。
- 生成的 DAO、DO 和 Entity 均由`make dao`产生，没有手工编辑生成文件。

## 自动化验证

| 验证 | 结果 |
|---|---|
| `go test ./internal/service/billing -run '^(TestFinancialSettlement|TestNewFinancialSettlement)' -count=20 -shuffle=on` | 通过 |
| `go test ./internal/service/commerce -run '^TestFinancialSettlementPostgreSQL' -count=20 -shuffle=on` | 通过，覆盖 8 路并发、余额、人工收款、零金额、全额优惠、减免拒绝、事务回滚、不可变约束和迁移幂等 |
| `make.cmd lint.go plugins=1 dir=apps/lina-plugins/idc-core/backend` | 通过，`golangci-lint`零问题，`staticcheck U1000`通过 |
| `make.cmd plugins.check format=json` | 通过，扫描`1309`个文件，零发现 |
| `openspec validate add-idc-server-operations-platform --strict` | 通过 |
| `make.cmd i18n.check plugins=1` | 本插件扫描与消息覆盖通过；全仓前端键覆盖被宿主既有 6 个缺键阻断 |

## 已知外部阻断

1. `make.cmd i18n.check plugins=1`报告宿主文件`apps/lina-vben/apps/web-antd/src/views/system/message/notice-preview-modal.vue`在`en-US`和`zh-CN`各缺少`plugin.content-notice.preview.title`、`plugin.content-notice.fields.type`、`plugin.content-notice.fields.createdBy`。根目录缺少`.contributing`，本任务不得修改宿主前端。

## Lina 审查结论

**变更：** `add-idc-server-operations-platform`

**范围：** 任务`6.12.a`，共 24 个实现、生成、测试、资源和记录文件。

**范围来源：** `git status --short`、`git ls-files --others --exclude-standard`、插件目录展开结果与任务上下文。插件根目录不存在额外`AGENTS.md`。

**已读取规则：** `AGENTS.md`、`.agents/rules/openspec.md`、`.agents/rules/documentation.md`、`.agents/rules/architecture.md`、`.agents/rules/data-permission.md`、`.agents/rules/plugin.md`、`.agents/rules/backend-go.md`、`.agents/rules/database.md`、`.agents/rules/testing.md`、`.agents/rules/i18n.md`和`.agents/instructions/markdown-format.instructions.md`。

审查中发现并修复了显式写`updated_at`、来源枚举裸字符串、测试文件缺用途注释、资金证据关联缺租户条件以及关联子查询随来源增长的问题。修复后未发现本任务实现范围内的阻挡性代码问题。数据权限采用内部精确订单动作例外且失败关闭；缓存、HTTP API、前端、E2E 和开发工具跨平台行为无影响。仓库级国际化强制门禁仍受上述宿主文件阻断，因此任务状态继续保持未完成。
