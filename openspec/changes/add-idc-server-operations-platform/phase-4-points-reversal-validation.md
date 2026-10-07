# 第四阶段积分冲回验证记录

## 范围

本记录覆盖任务`4.9.b`：积分奖励批次、退款冲回、多个部分退款和欠积分后续偿还。通用积分兑换的冻结/释放/核销编排将在结算与积分兑换入口实现；充值赠送/返现和营销触达仍由`4.9.c`、`4.9.d`承接。

## 实现证据

- `backend/internal/service/marketing/marketing_points.go`提供插件内部`PointsService`，只接收调用方已授权的精确客户和业务来源，不发布匿名 HTTP 入口。
- 奖励先锁定客户积分账户，以`customer + source_type + source_reference`复用完全相同的请求并拒绝变化重放；新批次和账户投影、`earn`/`debt_repay`追加流水在同一事务提交。
- 退款同样先锁账户再锁原奖励批次，固定先冲该批次可用积分、再冲冻结积分，剩余量只有落在尚未记欠的已消费积分内才形成欠积分。过期、已冲回或已经形成欠口的数量不能重复冲回。
- 多个部分退款使用各自幂等来源并允许共同引用原奖励流水；原`earn`流水不修改。新奖励优先偿还账户欠积分，被偿还部分进入新批次`reversed`桶并追加`debt_repay`流水。
- `020-marketing-foundation.sql`增加`reversal_shortfall_points`及“不超过已消费积分”的数据库约束，保留`available + frozen + consumed + expired + reversed = original`守恒；反向流水改为允许多个幂等退款引用同一原流水并新增查找索引。

## 验证结果

| 门禁 | 结果 |
|---|---|
| `go test ./backend/internal/service/marketing -count=20 -shuffle=on`，显式设置隔离 PostgreSQL 测试 DSN | 通过；覆盖奖励/退款重放、变化重放冲突、可用和冻结冲回、已消费欠口、两次部分退款、新奖励还债，以及两个并发退款争抢剩余可冲回量时仅一个成功 |
| `go test ./backend -run '^TestMarketingFoundationMigrationPostgreSQL$' -count=1` | 通过；18 张营销表、批次守恒和新增欠口上限约束在随机隔离 schema 生效 |
| DAO 生成 | 通过；只重建名称精确为`linapro_idc_codegen`的本地专用数据库营销表，项目标准生成入口已同步新字段 |
| `go test ./backend/... -count=20 -shuffle=on` | 通过 |
| `make.cmd lint.go plugins=1 dir=apps/lina-plugins/idc-core/backend` | 通过，`0 issues` |
| `make.cmd plugins.check` | 通过，扫描`799`个插件文件且`0 findings` |
| `openspec validate add-idc-server-operations-platform --strict` | 通过 |
| `make.cmd i18n.check plugins=1` | 插件运行时扫描和消息覆盖通过；仓库级结果仍只有宿主通知预览既有的 6 个前端缺失键，本切片未修改宿主且未新增缺失键 |
| `make.cmd build plugins=1 platforms=windows/amd64,linux/amd64 verbose=1` | 通过；Windows、Linux、Portal 和 11 个插件完整构建，核心插件版本为`v0.1.25` |

## 规则域影响

- 架构、后端与性能：积分仍是`core`营销子域，不进入余额或佣金；每次操作使用固定查询数和单客户事务锁，没有外部调用或按返回行数查询。
- 数据权限：服务不推断身份或暴露目标存在性，未来调用方必须先证明精确客户范围。
- 数据库：迁移只含幂等 DDL，无 Mock、手写自增主键或金额浮点；批次和欠口由数据库约束与事务双重保护。
- API、前端、缓存：无影响，本任务没有新增 HTTP、页面或缓存。
- `i18n`：新增 3 个稳定错误码均提供中英文消息；无新增用户页面文案。

## Lina 审查结论

- 已按本轮实际影响重新读取开发流程、文档、架构、数据权限、插件、后端、数据库、测试和`i18n`规则；API、前端、缓存和开发工具没有实现影响。
- 事务锁顺序固定为账户后批次；数据库守恒、来源幂等和并发争抢均有真实 PostgreSQL 证据，未发现本切片阻断项或警告项。
- 通用兑换的冻结/释放/核销以及结算入口接线不属于`4.9.b`，没有用测试夹具或营销表冒充尚未实现的结算能力。
