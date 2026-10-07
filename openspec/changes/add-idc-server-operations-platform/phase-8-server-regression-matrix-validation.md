# 第 8 阶段服务器回归矩阵验证记录

> **`FB-52`范围纠正：**本文验证的服务器开关机、重装、换`IP`、变配、资源回收和裸金属降级仍是服务器插件专属能力，不应删除；但依赖旧通用`service_instance`、服务器履约单元或旧续费资源接缝的证据只能用于迁移对账，不能证明通用商品、长期权益或`FB-52`已经完成。服务器字段和运行投影迁入服务器插件契约后，必须以`service_entitlement`归属和冻结业务拥有方证据重新执行本矩阵。

## 验证范围

本记录对应任务 8.11 的服务器高风险回归矩阵。付费变配失败由[服务器变配与所有权转移验证记录](phase-8-server-resize-transfer-validation.md)中的独立真实 PostgreSQL 组合回归证明，本文引用该权威结果而不重复实现；本文直接覆盖三维状态非法跃迁、服务操作互斥、续费与终止竞态、退款后资源残留保护、短期票据、换 IP 并发、部分回收失败和裸金属能力降级。

测试没有连接真实供应商，没有运行 Playwright，也没有修改 `web-antd`。真实数据库测试只在本机 PostgreSQL 内创建带固定前缀的随机 schema，结束后只删除自己的 schema。

## 回归矩阵

| 风险场景 | 直接证据 | 结果 |
| --- | --- | --- |
| 三维状态非法跃迁 | `internal/service/serveroperation/serveroperation_command_test.go::TestAllowedActionUsesFrozenEntitlementAndRuntimeMatrix`；`internal/service/serveroperation/serveroperation_regression_matrix_test.go::TestThreeDimensionalStateAndBareMetalCapabilityProjectionFailClosed` | 通过。权益终止、运行态未知、供应商任务执行中或已有业务操作时均不暴露新操作。 |
| 服务操作互斥 | `server_operation_postgres_test.go::TestServerOperationCreateReplayAndConvergePostgreSQL` | 真实 PostgreSQL 通过。相同请求只重放同一任务；同一服务已有进行中操作时拒绝第二个不同操作，收敛后才解除互斥。 |
| 续费与终止竞态 | `internal/service/renewal/renewal_postgres_integration_test.go::TestRenewalPaymentPostgreSQLSerializesExpiryTermination` | 真实 PostgreSQL 通过。续费和到期终止共用服务锁，两个先后顺序都不会产生双重有效结果。 |
| 付费变配失败 | `TestResizeProviderFailureUsesUnifiedRefundPostgreSQL`及原合同退款与变配退款组合回归 | 真实 PostgreSQL 通过。Provider 明确失败只产生一笔统一退款；资金来源、佣金冲回和状态收敛保持一致，结果不明不重复退款。 |
| 退款后资源残留保护 | `internal/service/commerce/commerce_online_refund_postgres_test.go::TestOnlineRefundPostgreSQLKeepsServiceLockedForHighPriorityRecovery` | 真实 PostgreSQL 通过。退款进入高优先级恢复后，服务继续锁定，不会把可能仍存在的资源当成可正常使用。 |
| 短期票据 | `internal/service/renewalsource/serveroperationsource_postgres_test.go::TestServerOperationSourcePostgreSQLFreezesNormalAction` | 真实 PostgreSQL 通过。票据第一次读取后不能再次读取；到期票据按数据库时间拒绝，并保持未读取状态，不访问秘密存储。 |
| 换 IP 并发 | `internal/service/renewalsource/ip_change_concurrency_postgres_test.go::TestIPChangeReservationPostgreSQLSerializesConcurrentLastAddress` | 真实 PostgreSQL 通过。同一资源池只剩一个 IP 时，两次同时申请只有一次成功，数据库也只有一条有效保留记录。 |
| 部分回收失败 | `cap/resourcecap/residual_resource_test.go::TestResidualResourcesAreBoundedTerminateOnlyAndSecretFree`；`cap/lifecycleresourcecap/residual_resource_test.go::TestLifecycleResidualResourceProjectionFailsClosed`；`internal/service/renewalsource/lifecyclesource_postgres_test.go::TestResourceLifecycleSourcePostgreSQLFreezesTaskIntent`；`internal/service/renewal/renewal_lifecycle_postgres_test.go::TestServiceLifecyclePartialReclaimFreezesBoundedResidualResourcesPostgreSQL`；`internal/controller/admin/provider_task_residual_mapping_test.go::TestProviderTaskDetailMapsOnlySafeResidualResourceFields` | 通过。资源插件把未回收清单传给核心；核心保持 `terminating` 并进入人工处理；后台只显示资源类型、不透明资源号、安全状态和原因码。 |
| 裸金属能力降级 | `internal/service/serveroperation/serveroperation_regression_matrix_test.go::TestThreeDimensionalStateAndBareMetalCapabilityProjectionFailClosed` | 通过。裸金属只展示供应商明确支持且当前状态允许的操作，不会因为产品类型自行补出重装等能力。 |

## 部分回收失败闭环

资源插件的版本化操作结果增加 `residualResources`。每条只允许四个字段：资源类型、不透明资源号、固定状态和稳定原因码；最多 16 条，重复项拒绝，只有终止任务的失败或未知结果可以携带。原始供应商响应、账号、密码、令牌和连接内容不得进入该投影；原因码还会经过凭据内容扫描。

资源任务将这组字段写入本地不可变证据，再通过 `lifecycleresourcecap` v2 投影给核心。核心只在终止任务进入 `manual_review` 时保存这组 JSON；服务运行态继续保持 `terminating`，不会把部分成功误判为资源已经完全释放。资源插件自己的任务详情接口和插件页面展示同一份最多 16 条的安全清单，便于人工继续处理。

数据库迁移 `068-server-lifecycle-residual-resource.sql` 可重复执行。字段为非空 JSON 数组，数据库约束限制最多 16 条，并限制非空清单只能出现在终止任务的人工处理状态。该字段属于单任务详情，不新增列表逐行查询；资源侧详情只增加一次有上限的证据查询，不形成 N+1 查询。

## 发现并修复的真实问题

换 IP 保留逻辑原来存在三处同类生产问题：

1. 当前时间、资源池编号和已保留数量是数据库单值，却使用了只适合结构行的 `GetScan`，正常请求因此会被判成库存不可用。
2. PostgreSQL JSON 键存在运算符 `?` 被 GoFrame 当成参数占位符，库存 SQL 会变成错误语句。
3. 预留消费和释放事务读取数据库时间时仍有一处单值 `GetScan`，会让 Provider 成功后的库存消费或明确失败后的库存释放误报不可用。

`internal/service/serveroperationsource/serveroperationsource_ip_inventory.go` 已统一用 `GetValue` 读取单值，并用 `jsonb_exists` 查询 JSON 键。真实 PostgreSQL 并发测试确认修复后只会占用最后一个 IP 一次，并继续验证成功消费和明确释放都能正确完成。

## 执行记录

- 最终交付版本将核心插件提升到`v0.1.60`、资源插件提升到`v0.1.10`，并把资源插件最低核心依赖收紧为`>=0.1.60 <1.0.0`。标准启动日志确认核心先升级成功、资源后升级成功；数据库确认两插件均为`enabled`，`068`的`residual_resources JSONB NOT NULL DEFAULT []`字段和约束真实存在，避免已安装`v0.1.59`环境跳过新增迁移。
- 资源插件 `GOWORK=off go test ./...`：通过。
- 核心插件 `GOWORK=off go test ./cap/lifecycleresourcecap ./internal/service/lifecycle ./internal/service/renewal ./internal/service/serveroperation`：通过。
- 设置本机隔离 `LINAPRO_IDC_RESOURCE_TEST_DSN` 后运行资源侧 PostgreSQL 回归：通过；换 IP 最后库存并发用例追加验证消费和释放路径，单独复验用时 0.314 秒。
- 设置本机隔离 `LINAPRO_IDC_CORE_TEST_DSN` 后运行部分回收和续费/终止竞态：通过，用时 2.160 秒。
- 设置本机隔离 `LINAPRO_IDC_CORE_TEST_DSN` 后运行退款后资源保护：通过，用时 2.294 秒。
- 付费变配失败及原合同退款组合 PostgreSQL 回归：通过，两项合计 9.997 秒；详细资金和状态证据见独立变配验证记录。
- 067 迁移接线修复后，设置本机隔离 `LINAPRO_IDC_CORE_TEST_DSN` 运行服务操作互斥和收敛：通过，用时 1.662 秒。修复前的迁移缺列失败没有被当成业务通过。
- 设置本机隔离 `LINAPRO_IDC_CORE_TEST_DSN` 重放服务、操作和 068 未回收字段迁移：通过，用时 1.311 秒。
- 插件新增前端文件和中英文 JSON 执行 Prettier 检查：通过。
- `@lina/web-antd` 只作为宿主执行只读 TypeScript 类型检查和双语键检查：通过；没有修改宿主文件。双语检查覆盖 2 种语言的 4 个宿主语言文件，插件自身新增键也完成 JSON 解析与中英文对齐检查。

## 规则影响

- 数据权限：未新增列表、导出或跨租户读取；任务详情继续使用原有精确任务权限和租户条件。
- 多语言：资源插件中英文同时增加未回收清单和四种安全状态文案。
- 缓存：无缓存读写或失效策略变化。
- 开发工具：未修改脚本、构建命令或 CI。
- 前端边界：只修改 `idc-resource/frontend`，未向 `web-antd` 写入业务源码或依赖。
