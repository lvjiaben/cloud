# 第六阶段信用退款与财务回归验证记录

## 完成范围

本记录覆盖任务`6.13`至`6.13.c`。信用购买退款已经接入普通订单退款流程，并在信用账户的同一事务中按“未出账本金、已出账未还本金、已实际偿还本金”依次分配。前两部分冲减应收并恢复可用额度；已还部分按原还款分配退回，不能再次增加额度。

用户确认整张信用账单因退款减为零时，客户和后台都显示独立终态“已退款”。该状态不冒充“已付款”“已结清”或“已取消”，原账单本金、还款来源、负数退款明细和退款历史继续保留。

## 实现与资金边界

- 信用退款计划冻结每段敞口、原还款分配、退款去向和证据摘要；重放内容相同才返回原结果，计划、金额或证据变化均失败关闭。
- 未出账和已出账未还部分只形成信用释放，不伪造现金支付或现金退款分配；只有已实际还款部分才按原在线支付、客户余额或经审计同意的同币种余额去向退款。
- 原渠道明确不可退时，只有客户已作出可审计同意，才允许把对应部分退到同币种余额；未知去向、缺少原资金证据或无法识别的来源不会默认按信用释放处理。
- 普通在线退款现可识别`credit_settled`账单，保存精确信用敞口和还款分配，不创建假的支付分配。最终入账前再次核对冻结计划，变化后停止执行。
- 合并或拆分账单的退款继续绑定原订单项和派生账单项；多次部分退款按剩余可退金额计算，不会把部分退款误标为整张全退。

## 数据、权限与性能检查

- 迁移`053-credit-refund-lifecycle.sql`补齐信用退款生命周期、分配和“已退款”状态；`054-credit-online-refund-integration.sql`把信用来源接入在线退款，并以数据库约束保证一条来源只能是支付分配或精确信用绑定之一。
- 两份迁移只有 DDL 和必要字典数据，没有 Mock 数据；在独立 PostgreSQL 中连续执行两次通过，证明可重复安装。
- 客户退款仍由当前登录客户身份和订单归属过滤；内部信用入账服务只消费已经通过所有权检查、带完整摘要的精确敞口，不新增后台越权入口。
- 单次退款选择继续受最多`100`项限制，信用资金行最多`1000`条；读取和入账使用有界集合查询及数据库索引，没有按返回行逐条访问外部服务的`N+1`问题。
- 本切片不使用缓存，资金、敞口、账单状态和退款结果都直接以本地 PostgreSQL 权威事实为准。

## 自动化验证

| 验证 | 结果 |
| --- | --- |
| `TestCreditRefundPostgreSQLCoversUnbilledBilledAndRepaidStates` | 通过；覆盖未出账、已出账未还、部分还款和全额还款四种状态 |
| `TestCreditRefundPostgreSQLSupportsMultiplePartialRefunds` | 通过；按`300/200/500`完成多次部分退款并验证最终“已退款” |
| `TestCreditRefundPostgreSQLRejectsChangedPlanAndMissingFundingEvidence` | 通过；计划变化和资金证据缺失都不写入 |
| `TestOnlineRefundPostgreSQLCompletesCreditSettledOrder` | 通过；信用结清订单经普通退款入口完成，且无虚假现金分配 |
| `TestInvoiceTransformPostgreSQLRefundsDerivedMergeAndSplitItems` | 通过；覆盖合并、拆分和多段退款分配 |
| `TestCreditRefundPostedSourceTypeFailsClosed` | 通过；未知来源不会降级成信用释放 |
| `go test ./backend/internal/service/billing -count=1 -v` | 通过；包含并发占用、预还款、逾期、滞纳金、坏账、对账、账单、支付和退款全回归 |
| `go test ./backend/internal/service/commerce -run '^TestOnlineRefundPostgreSQL' -count=1 -v` | 通过，`7`项在线退款真实 PostgreSQL 场景全部通过 |
| `go test ./backend/... -count=1 -p 1` | 除内容中心按其数据库保护规则拒绝错误库名外，其余包通过；随后使用内容中心规定的独立数据库单独重跑并通过，因此插件全部后端包均完成各自要求的数据库验证 |
| `go test ./backend/internal/controller/... ./backend -run '^$' -count=1` | 通过；控制器和插件启动装配可编译 |
| `make.cmd lint.go dir=apps/lina-plugins/idc-core plugins=1` | 通过；`golangci-lint`零问题，`staticcheck`通过 |
| `make.cmd i18n.check` | 通过；运行时文案与前端词条覆盖均为零违规 |
| `make.cmd plugins.check` | 通过；插件治理零问题 |
| `openspec validate add-idc-server-operations-platform --strict` | 通过 |

## 代码生成与审查记录

本轮按项目流程执行了`make dao`和`make ctrl`。DAO 生成按当前插件配置完成；在线退款表暂不在 DAO 配置中，生产代码使用有界 SQL，不手改生成的 DAO、DO 或 Entity。

`make ctrl`发现当前插件的手写控制器与生成器所有权尚未完全收敛：生成器为已有实现追加了重复构造代码，并生成`178`个未实现占位方法。重复产物已全部撤掉，原手写控制器保留，重复扫描为`0`，控制器和整插件重新编译通过。本记录不把该过程写成“控制器生成干净通过”；完整生成治理仍由任务`18.8`负责。

本切片已执行任务级`lina-review`：阻断问题`0`项；上述代码生成所有权问题为`1`项非阻断警告。退款服务依赖在插件启动时只创建一次，并同时注入退款、财务结算和信用命令，未在请求路径临时创建另一套状态服务。没有新增前端页面、路由或交互；“已退款”的真实浏览器验证由紧接的任务`6.14`完成，不能用本次后端通过代替该任务。

## 影响判断

- API：现有客户与后台信用状态字典补充`refunded`，接口时间和金额格式未改变。
- 数据权限：没有新增管理入口；客户订单归属、当前主体和退款对象存在性继续在查询前校验。
- 国际化：中英文“已退款”字典以及中文接口文档已同步，完整双语门禁通过。
- 前端：本任务未修改`web-antd`、`web-customer`或插件前端；下一项只按`6.14`补端到端验证。
- 开发工具：未修改生成器、构建脚本或跨平台入口，只执行现有命令并记录生成器的真实结果。
