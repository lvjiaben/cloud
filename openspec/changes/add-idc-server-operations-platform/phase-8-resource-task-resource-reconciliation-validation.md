# 资源任务无任务号对账验证记录

## 完成范围

本轮完成任务 8.5.c.2.a。资源拥有方现在可以集合化解析当前活跃资源映射，并对没有上游任务号的`result_unknown`任务执行只读`QueryResource`对账。8.5.c.2、8.5.c 和 8.5 顶级任务继续保持未完成；开通映射、跨 owner 回报、参数型能力专用对账、秘密句柄孤儿补偿和生产作业装配仍在 8.5.c.2.b。

- 一次最多解析或领取 32 个任务；资源映射按本地资源号集合一次投影，并同时核对渠道、资源池和`active`状态，不逐任务查询。
- 开通任务不要求已有映射；其他写能力缺少精确活跃映射时不会调用 Provider。
- 没有上游任务绑定的未知任务只进入独立只读观察租约；领取使用 PostgreSQL 时间、`FOR UPDATE SKIP LOCKED`、随机令牌、版本和有效期完整栅栏。
- `QueryResource`返回的资源编号必须与活跃映射精确一致，观察时间不得早于本次租约。
- 只有启动、停止、暂停、恢复和终止能由精确目标状态证明幂等结果。重启、重装、变配、换`IP`和重置密码不能由通用运行状态推断成功。
- 每次成功收敛至少写入一条拥有方资源状态证据；Provider 返回的有限脱敏证据逐条持久化。证据与任务成功状态在同一事务提交，任一证据写入失败都会回滚结案。
- 证据表是不可变历史，只保存随机证据编号、任务编号、稳定来源与类型、有限引用、脱敏摘要和观察时间；不保存原始响应、秘密句柄或凭据。

## 数据库与代码生成

- 新增幂等迁移`002-resource-task-reconciliation.sql`和`plugin_idc_resource_task_evidence`表。
- 迁移测试按文件名顺序执行全部资源迁移，并在同一隔离 schema 中重复执行两次。
- 表使用数据库生成自增主键，业务代码不写入主键和`created_at`；`tenant_id`固定为 0，任务外键使用`ON DELETE RESTRICT`。
- 查询索引固定为`tenant_id + task_no + observed_at DESC + id DESC`，支撑任务时间线读取，不引入随任务数增长的装配查询。
- 已通过项目`linactl dao`入口重新生成 DAO/DO/Entity；生成数据库仅使用名称固定为`linapro_idc_codegen`的专用数据库。

## 验证结果

- `GOWORK=off go test ./backend/... -count=20 -shuffle=on`：通过。
- `GOWORK=off go test ./backend -run TestResourceFoundationMigrationPostgreSQL -count=20`：真实 PostgreSQL 隔离 schema、迁移重复执行、映射解析、旧快照拒绝、资源身份拒绝、任务与两类证据原子收敛全部通过。
- `make.cmd lint.go dir=apps/lina-plugins/idc-resource plugins=1`：`golangci-lint`与`staticcheck`通过，0 项问题。
- `make.cmd plugins.check`：通过，扫描 11 个插件清单和 518 个文件，0 项治理问题。
- `openspec validate add-idc-server-operations-platform --strict`：通过。
- `git diff --check`：通过；仅报告工作树已有根`Makefile`和`make.cmd`换行提示，无空白错误。

## 影响判断

- `API`：没有新增或修改 HTTP API、路由、DTO、权限标签或公开响应。
- 数据权限：仅新增 resource owner 内部后台作业路径，固定处理 tenant 0 中本插件持有且具有精确活跃映射的任务；没有新增接受用户任务号、资源号或渠道号的查询/执行入口。
- SQL：新增一张不可变对账证据表及一个任务时间线索引；迁移幂等、无 Seed/Mock、无软删除、无应用写入自增主键，并完成真实 PostgreSQL 验证及 DAO 生成。
- 缓存：没有新增缓存、快照缓存、失效路径或进程内权威状态，租约与对账仍以 PostgreSQL 为准。
- `i18n`：没有用户可见页面、运行时文案、API 文档、错误响应、菜单、字典或语言包变化。
- 开发工具与跨平台：仅在插件代码生成配置中增加新表名，未修改生成器、脚本、CI 或跨平台入口；Windows 使用项目`linactl`入口完成生成，Linux/macOS 继续由同一插件 Makefile 调用该入口。
- 测试与 E2E：没有页面或用户可观察前端变化，不触发 E2E 与 Chrome 验收；内部状态与持久化行为由单元测试和真实 PostgreSQL 集成测试覆盖。
- 性能：数据库领取与映射解析均为有界集合查询；同一批次每个渠道只构造一次 Provider。成功收敛最多持久化 1 条拥有方状态证据和 Provider 契约允许的 32 条证据，写入次数具有固定硬上限。

## 剩余门禁

- 开通同步成功和带任务号终态成功仍需把权威资源映射、任务结案和跨 owner 回报原子收敛。
- 重启、重装、变配、换`IP`和重置密码需要参数或结果专用观察器，不能复用通用运行状态判断。
- Provider 输出秘密已经由安全 sink 写入、但本地任务事务失败时，需要可审计的孤儿句柄补偿与撤销作业。
- 在上述能力、启动依赖和故障隔离完成并审查前，不注册`pluginhost.Jobs`生产 worker。

## 完成后审查

已按`lina-review`审查本任务手写文件和本轮代码生成器触及的全部 DAO/DO/Entity。审查发现并修复两项问题：领取租约最初为了建立快照时效下界而显式写入`updated_at`，违反 GoFrame 自动时间维护规则，现已删除并由 ORM 自动维护；新增测试文件及若干内部符号最初缺少用途注释，现已补齐。修复后重新执行全后端 20 轮测试、真实 PostgreSQL 20 轮测试、定向 Go lint、插件治理、OpenSpec 严格校验和空白检查，全部通过。

最终审查未发现阻塞问题，严重 0 项、警告 0 项。当前剩余风险属于已明确留在 8.5.c.2.b 的未实现边界：参数型能力专用对账、开通映射与跨 owner 原子回报、秘密孤儿补偿和生产作业装配；这些能力未被当前完成项伪装为可用。E2E 质量审查未触发，因为本轮没有页面、HTTP API、用户可观察交互或端到端前台流程变化。

## `FB-4`领取时间反馈补充

后续真实 PostgreSQL 重复回归发现，资源观察领取返回的数据库`ClaimedAt`与 GoFrame 自动维护的`updated_at`可能相差数十微秒；原实现以`updated_at`作为快照新鲜度下界，会误拒绝领取后立即查询且时间等于`ClaimedAt`的结果。现已通过独立业务字段`observation_claimed_at`保存并原样返回同一个 PostgreSQL 时间，回报不再复用自动时间字段。早于领取时间的快照仍拒绝，等于领取时间的快照正常进入对账；20 轮真实库隔离测试通过。该修复不改变 HTTP API、缓存、数据权限或`i18n`边界。
