# 第六阶段账单生命周期验证记录

## 实现范围

本轮完成任务`6.3`的内部账单生命周期服务，代码闭环在`idc-core/backend/internal/service/billing`，插件版本由`v0.1.42`提升到`v0.1.43`。服务不新增`HTTP`路由、控制器或页面，只向后续支付、退款、人工财务和到期任务所有者提供固定语义方法；调用方不能提交目标状态、已付金额、可退金额或已退金额。

`027-billing-invoice-lifecycle.sql`只收紧既有账单生命周期检查约束，并允许部分支付账单在到期后保持真实已付金额进入逾期。迁移不新增字段、索引、种子数据或软删除语义，重复执行使用同名约束先删除后重建，真实 PostgreSQL 已验证幂等。标准 DAO 生成入口已重新执行；本轮没有模型形状变化，也没有手工修改生成文件。

## 状态与金额不变量

- 九种状态统一为`draft`、`pending_payment`、`partially_paid`、`paid`、`overdue`、`cancelled`、`partially_refunded`、`refunded`和`bad_debt`。开立、到期、取消、支付、退款、冲回和坏账识别均使用语义方法，禁止任意状态设置器。
- 每次写操作先锁定唯一账单，再读取有界聚合并验证头部金额等于追加式支付/退款分配净额；最终使用账单版本条件更新头部投影。相同来源与相同内容返回首次分配，不同金额、账本交易、账单明细或原分配复用同一来源时失败关闭。
- 支付必须引用同客户、同币种、正确交易类型且未冲销的不可变账本交易，金额不能超过账单、可选账单明细或账本交易的剩余未分配金额。超额款不自动记入账单，留给后续待归属资金流程处理。
- 退款必须绑定明确账单明细、原支付分配和已完成退款账本交易，并同时满足账单当前可退额、明细可退上限、原支付有效余额与账本交易未分配余额。部分退款保持`partially_refunded`，只有可退上限全部退完才进入`refunded`。
- 支付和退款纠错只追加引用原分配的新冲回记录，不修改或删除原历史。`paid_amount`和`refunded_amount`始终等于有效分配净额；已结清状态满足`refundable_amount = min(paid_amount, 明细可退上限合计) - refunded_amount`。
- 坏账只允许已到期且存在欠款的账单，必须引用同币种、未冲销且精确等于欠款的唯一写销交易，并证明应收贷记和坏账费用借记金额一致。坏账不接受直接支付恢复；未来回款必须先由专用流程冲回写销。

## 测试与门禁证据

1. `GOWORK=off go test ./backend/internal/service/billing -count=20 -shuffle=on`通过，约`19.6s`；每个实库用例使用随机隔离 schema 并自动清理。
2. 实库生命周期覆盖草稿开立与重放、无资金取消、部分支付、数据库时间逾期、首次结清、超额拒绝、部分退款、全额退款、逐笔退款冲回、支付冲回、精确坏账和坏账后直接回款拒绝。
3. 八个并发调用者重放同一支付分配时只产生一条分配和一次“首次结清”结果；头部金额、支付净额、退款净额和追加记录数在完整冲回后仍一致。
4. `GOWORK=off go test ./backend -run '^TestBillingLedgerFoundationMigrationPostgreSQL$' -count=1 -v`通过，并连续执行`023`与`027`迁移证明幂等；专用数据库安全门禁及账单 codegen 准备测试通过。
5. `make.cmd dao dir=apps/lina-plugins/idc-core/backend`通过；`027`没有模型形状变化，DAO、DO 与 Entity 继续由标准生成器维护。
6. `GOWORK=off go test ./backend/... -count=1 -shuffle=on`通过，包含账单真实 PostgreSQL 和既有 commerce、customer、audit、outbox 等回归。
7. `make.cmd lint.go plugins=1 dir=apps/lina-plugins/idc-core`通过，`golangci-lint`和`staticcheck U1000`均为`0`问题。审查中已把错误码集中到`billing_code.go`、替换生产代码直接`fmt.Errorf`并拆分高复杂度退款校验。
8. `make.cmd i18n.check plugins=1`确认运行时硬编码扫描与宿主/插件消息覆盖通过，新增五类账单错误均有中英文资源；完整前端键门禁仍只被宿主通知预览页中英文各三个既有缺键阻断。
9. `make.cmd plugins.check format=json`通过，扫描`1049`个文件、`11`份配置和`11`份清单，治理发现为`0`。
10. `openspec validate add-idc-server-operations-platform --strict`通过。
11. `make.cmd build plugins=1 platforms=windows/amd64,linux/amd64 verbose=1`通过；管理端、独立 Portal、`11`个插件和双平台后端正式产物均成功生成，仅保留宿主已有非阻断`TS4058`和大分块告警。

## 独立复审与规则影响

- 数据权限与`API`：本轮没有新增`HTTP API`、`DTO`、路由、菜单或终端用户数据面。内部投影有固定字段且不含渠道凭据；未来客户和后台入口必须分别执行所有权或直接员工、精确权限、默认商家和可表达数据范围校验，不能直接暴露本服务。
- 性能：每次操作只处理一张账单，按业务唯一键和既有索引执行固定数量查询，没有随账单行数增长的应用层循环或`N+1`。聚合使用数据库标量集合查询；账单、分配和账本交易锁顺序稳定。
- 缓存：财务真相只读取 PostgreSQL，没有引入缓存或跨节点陈旧窗口。
- 前端与 E2E：没有用户可观察页面、路由或交互变化，本轮不触发浏览器 E2E；完整客户付款、后台财务与退款 E2E 仍由后续`6.7`和`6.9`任务拥有。
- 开发工具：没有修改构建、CI、代码生成脚本或跨平台入口，只执行已有 Windows 标准入口；Linux/macOS 继续使用等价`make dao`与 lint 目标。
- 数据库：`027`为 DDL，表名保持完整插件前缀；没有 DML、DAO 字段、时间字段、软删除或索引变化。真实 PostgreSQL 重复迁移与约束行为均已验证。
- 复审结果：任务范围内未发现仍未处理的严重或高优先级问题。仓库根不存在`.contributing`，宿主通知预览页既有六个缺键不属于本业务插件授权范围，因此`6.3`父项和`6.3.c`暂不勾选。

**已读取规则：**`AGENTS.md`、`.agents/rules/openspec.md`、`.agents/rules/documentation.md`、`.agents/rules/architecture.md`、`.agents/rules/data-permission.md`、`.agents/rules/plugin.md`、`.agents/rules/backend-go.md`、`.agents/rules/database.md`、`.agents/rules/testing.md`、`.agents/rules/i18n.md`。
