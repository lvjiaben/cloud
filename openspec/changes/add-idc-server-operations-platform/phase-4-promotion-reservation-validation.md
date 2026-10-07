# 第四阶段优惠预占服务验证记录

## 范围

本记录覆盖任务`4.9.a`：本地优惠资格计算、确定性互斥与叠加、活动/客户/券三层名额预占，以及核销、取消和过期释放。积分冲回、赠送/返现权益和营销触达同意仍分别由`4.9.b`至`4.9.d`承接。

## 实现证据

- `backend/internal/service/marketing`提供插件内部服务边界；调用方负责先证明客户数据权限，服务本身不发布匿名或后台 HTTP 接口。
- 读取路径使用固定数量、有上限的本地 PostgreSQL 查询装配活动、权益、商品范围、目标快照和请求指定的客户券，不调用 Provider、支付或其他外部平台。
- 选择顺序固定为活动优先级、活动编号和权益编号；互斥、同组互斥和可叠加都有稳定结果及排除原因。固定金额、客户价和比例金额都使用整数分，比例按半入舍入且优惠不超过剩余应付金额。
- 预占记录同时冻结规范化请求摘要和权益版本摘要。事务先锁定已选活动与权益，再重新计算；活动总名额、逐客户名额、券使用次数和两类保留记录必须全部成功，否则整体回滚。
- 核销把`reserved`转为`consumed`，取消与过期只释放`reserved`。相同请求重放复用原记录，改变请求重放返回冲突；终态重复操作不重复改变计数。
- `020-marketing-foundation.sql`为通用活动保留记录增加请求摘要约束，DAO/DO/Entity已通过项目标准生成入口同步。

## 验证结果

| 门禁 | 结果 |
|---|---|
| `go test ./backend/internal/service/marketing -count=20 -shuffle=on`，显式设置隔离 PostgreSQL 测试 DSN | 通过；每次创建并只删除随机`linapro_promotion_test_*`schema，覆盖最后一个总名额并发争抢、请求重放冲突、释放后复用、核销幂等、逐客户名额与券次数守恒 |
| `go test ./backend -run '^TestMarketingFoundationMigrationPostgreSQL$' -count=1` | 通过；`001`、`003`与`020`在隔离 schema 应用，18 张营销表及新增请求摘要约束有效 |
| `go test ./backend/... -count=20 -shuffle=on` | 通过；全量插件后端单元测试和编译门禁通过 |
| DAO 生成 | 通过；仅重建名称精确为`linapro_idc_codegen`的本地专用数据库营销表，再执行项目标准`make.cmd dao`入口 |
| `make.cmd lint.go plugins=1 dir=apps/lina-plugins/idc-core/backend` | 通过；`golangci-lint`与`staticcheck U1000`均为 0 问题 |
| `make.cmd plugins.check`与`openspec validate add-idc-server-operations-platform --strict` | 通过；11 个插件、798 个文件均无治理问题，OpenSpec 严格校验通过 |
| `make.cmd build plugins=1 platforms=windows/amd64,linux/amd64 verbose=1` | 通过；`v0.1.24`、迁移、生成模型、双语资源、Portal 与 Windows/Linux 后端均完成正式构建 |
| Go 格式与差异检查 | 通过；`gofmt`及`git diff --check`无异常 |

## 规则域结论

- 架构与性能：通过。同步路径仅访问本地数据库，批量读取有硬上限；写路径按拥有行执行受限事务更新，没有外部网络调用。
- 数据权限：通过。新增服务是插件内部能力，不自行推断操作者或客户；未来 HTTP/结算调用方必须注入已授权的精确客户上下文。
- 数据库：通过。迁移为幂等 DDL，无 Mock 或手写自增主键；金额使用`BIGINT`，计数约束和事务共同阻止超发。
- API、前端与缓存：无影响。本任务没有新增 HTTP 契约、页面或缓存，运行时接入留给结算任务。
- `i18n`：通过插件范围检查。5 个新增稳定错误码均提供中英文消息，运行时扫描和消息覆盖为 0 问题；仓库总检查仍只报告禁止在本变更修改的宿主通知预览页 3 个键中英文各缺失一次，共 6 个既有问题，本任务没有新增缺口。

## Lina 审查结论

审查范围为任务`4.9.a`新增或修改的营销迁移、生成 DAO/DO/Entity、`backend/internal/service/marketing`、插件清单与双语错误资源，以及对应 OpenSpec 和偏好记录。实际读取并应用了项目与插件`AGENTS.md`检查结果，以及`.agents/rules/openspec.md`、`documentation.md`、`architecture.md`、`data-permission.md`、`plugin.md`、`backend-go.md`、`database.md`、`testing.md`、`i18n.md`、`api-contract.md`、`cache-consistency.md`和`dev-tooling.md`。

未发现任务内阻断或警告。数据权限由未来结算调用方提供精确客户上下文，当前内部服务没有可被匿名或后台直接调用的 HTTP 入口；API、前端和缓存均无影响。剩余测试缺口是尚未接入第五阶段不可变报价与订单事务，以及尚未实现后台活动配置界面，这两项不属于本子任务，分别由结算和后续营销管理任务承接。
