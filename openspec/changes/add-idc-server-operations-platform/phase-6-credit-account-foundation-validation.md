# 第六阶段逐币种信用账户数据底座验证记录

## 结论

任务`6.10`完成。`idc-core`新增逐币种信用政策、申请与审批、账户与条款、额度敞口、信用账单与明细、还款与分配、逾期案件、坏账和完整信用流水共 13 张表。金额统一使用`BIGINT`最小货币单位，业务历史不使用软删除；信用事实与余额、赠送和佣金账户继续分离。

## 数据库不变量

- 同一客户每个币种最多一个信用账户，账户引用同客户、同币种且类型固定为`customer_credit`的分层账本账户。总额度始终等于可用额、未出账本金、已出账本金和已核销本金之和，逾期本金不得大于已出账本金。
- 逐币种政策版本冻结最高额度、双人复核阈值、账单日、还款期、宽限期和逾期费。申请额度不得超过政策上限；达到阈值必须两名不同员工依次批准，员工申请不得自审。风险资料只保存私有引用和摘要。
- 账户每个版本必须存在一条不可修改的完整前后快照流水。提交时核对连续版本、上一条结束快照、本次动作金额语义和账户最新投影；未写流水直接修改额度、断裂版本或篡改历史都会失败。
- 敞口本金在未出账、已出账、已偿还、已释放和已核销桶之间守恒。信用账单头必须等于不可变明细汇总，已还金额必须等于还款分配净额，核销金额必须等于双人复核坏账净额及正式账本交易。
- 还款只能来自在线支付、客户余额、人工收款或迁移；佣金不在允许来源内。账单还款、未出账预还款和坏账收回使用不同分配类型，分配与冲正绑定同账户、同客户和同币种。
- 逾期案件独立记录宽限、冻结、服务处置待办、核销和解决状态。冻结及坏账状态必须与信用账户和信用账单当前投影一致；服务运行状态仍由后续独立服务任务处理。

## SQL 分类与生成

- `034-credit-account-foundation.sql`只包含幂等 DDL、内建字典 Seed DML、约束函数、触发器和表注释，不包含 Mock 数据或手写自增主键。
- 13 张表均使用完整`plugin_idc_core_`前缀和固定`tenant_id = 0`，无`deleted_at`；所有`*_minor`列均为`BIGINT`。
- 迁移重复执行通过；代码生成数据库只允许名称精确为`linapro_idc_codegen`，清理范围显式限定为本插件账务及信用表。标准 DAO 入口生成 13 组 DAO、DO 和 Entity。
- 8 组信用字典在`en-US`和`zh-CN`中逐值对齐；插件版本提升为`v0.1.47`。

## 验证结果

| 验证 | 结果 |
| --- | --- |
| `TestBillingLedgerFoundationMigrationPostgreSQL -count=5 -shuffle=on` | 通过；隔离 schema 重放迁移、13 张信用表形状、金额类型、不可变触发器及双语字典一致 |
| `TestCreditFoundationPostgreSQLPreservesApprovalAndMoneyInvariants -count=20 -shuffle=on` | 通过；真实 PostgreSQL 覆盖少审批拒绝、双审批开户、占用、出账、在线还款、预还款、逾期、冻结、坏账、重复敞口出账拒绝、超额预还款拒绝、非私有风险引用拒绝、跨币种拒绝、无流水更新拒绝和历史篡改拒绝 |
| `TestBillingLedgerFoundationCodegenPostgreSQL` | 通过；仅重建专用`linapro_idc_codegen`数据库的明确账务与信用表，迁移重复执行成功 |
| `make.cmd dao dir=apps/lina-plugins/idc-core/backend` | 通过；新增 13 组类型安全生成模型 |
| `LINAPRO_IDC_CORE_TEST_DSN=... GOWORK=off go test ./backend/... -count=1 -shuffle=on` | 通过；包含真实 PostgreSQL 的完整插件后端回归通过 |
| `make.cmd lint.go plugins=1 dir=apps/lina-plugins/idc-core/backend` | 通过；`golangci-lint`和`staticcheck U1000`均为 0 问题 |
| Windows/Linux AMD64 `go build ./backend/...` | 通过 |
| `make.cmd plugins.check format=json` | 通过；扫描 11 份清单、1248 个文件，0 项治理发现 |
| `make.cmd i18n.check plugins=1` | 插件运行时文案与消息覆盖通过；仓库级前端键检查仍仅被禁止修改的宿主通知预览页 6 个既有缺键阻断，本任务未新增缺键 |

## 影响判断

- 接口与前端：本任务只交付 SQL 数据底座和生成模型，不新增 HTTP API、菜单或页面；不存在新的数据权限入口。信用审批、对账、服务处置接口和页面属于后续`6.11—6.14`。
- 缓存：无缓存读写或失效路径。
- 外部系统：迁移和测试均不访问支付渠道、Provider、智简魔方或线上业务数据；真实 PostgreSQL 测试只使用随机隔离 schema，DAO 生成只使用专用本地数据库。
- 性能：账户、申请、账单、还款、逾期和流水均为客户/币种、状态/到期时间及倒序键集读取建立索引；没有逐行装配或`N+1`查询实现。
