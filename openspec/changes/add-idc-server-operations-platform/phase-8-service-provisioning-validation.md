# 服务器开通业务收敛验证记录

## 验证范围

本记录覆盖任务`6.12.d`、`8.2.b.1`和`8.5.c.2.b.2.b.1`。范围从统一`financially_settled`事件开始，经过 core 的逐台服务与业务操作物化、resource 的资源任务创建与查询，最终回到 core 的权益、运行状态、订单、履约单元和库存预留收敛，并由 resource 幂等接收终态回执管理初始密码生命周期。

本轮没有调用线上智简魔方公有云写接口，也没有修改任何已有上游实例、用户或任务。

## 业务与拥有方边界

- `financially_settled`仍是新购开通唯一财务入口。消费者按不可变结清号消费事务`Outbox`事件，物化逐台服务实例和`business_operation`；余额、人工收款、零金额、优惠或试用来源都不能直接把履约单元改为可执行。
- core 独占服务权益、运行状态、业务操作、订单、履约与库存预留，只保存 resource 任务的不透明编号、渠道编号和脱敏证据摘要，不读取 resource 表。
- resource 独占 Provider 任务、上游客户映射和初始密码秘密句柄。开通请求必须命中精确客户映射与新鲜能力证据；本地客户编号不得冒充上游客户编号。
- 跨 owner 结果采用可重放回执：core 先提交业务终态，再提交带请求摘要、任务证据摘要和冻结 core 终态投影摘要的回执。回执摘要不依赖后续可变的服务版本；暂时失败只保留`pending`并重试同一回执，不重复创建或查询资源任务，也不回滚已确认的 core 终态。

## 状态与补偿结论

| 场景 | core 结论 | resource 密码结论 | 库存结论 |
| --- | --- | --- | --- |
| Provider 权威成功 | 服务权益`active`、运行态`running`、操作与履约成功 | 收到精确成功回执后才由`pending`变为`revealable` | 对应数量计入`consumed_quantity` |
| Provider 权威失败 | 服务取消、操作与履约失败 | 精确失败回执先幂等吊销秘密，再保存不可变回执 | 对应数量计入`released_quantity` |
| 结果未知 | core 保持业务操作与履约执行中，把运行态投影为未知并继续只读轮询 | 不发送终态回执，继续`pending`且不可领取 | 不提前释放 |
| 授权人工接管 | core 收敛人工状态并失败关闭 | 精确人工回执保留`pending`且不可领取 | 不自动释放 |
| 多台服务器混合成功与失败 | 订单`partially_fulfilled` | 每台按自身回执处理 | 预留`partial`并同时保存消耗与释放数量 |
| 创建秘密后资源任务事务确定回滚 | 不产生执行绑定 | resource 确认任务不存在后按版本精确吊销孤儿秘密 | core 后续安全重试 |
| 资源任务写入结果无法确认 | 不假设未创建 | 不自动吊销，避免误删已被任务绑定的秘密 | 等待权威恢复或人工处理 |

## 实现证据

- core 新增`service_provision_execution`，保存逐台执行、不透明 Provider 任务引用、请求与能力证据、终态和独立回执投递状态；状态、时间和终态约束由 PostgreSQL 检查约束封闭。
- core 开通 worker 使用有界批次和集合查询装配商品配置，严格投影内存、CPU、系统盘、数据盘、带宽、流量、IP、备份、快照和镜像参数；Provider 调用在数据库事务外，返回后以业务操作版本栅栏附着。
- 同一业务操作的并发 worker 即使同时完成外部`CreateTask`，最终也只收敛一条执行记录和一组状态事件；resource 以同一请求摘要复用同一资源任务。
- resource 新增不可变`provision_outcome_receipt`。完全一致的回执可以重放，改变请求、任务、状态或证据的重放冲突；成功、失败、未知和人工状态分别执行固定密码策略。
- resource 的秘密孤儿补偿只在任务创建失败且随后权威查询证明任务不存在时执行。查询本身失败时保持秘密，禁止把未知写结果误判为未写入。
- core 与 resource 的开通处理器都通过源码插件托管作业装配，没有新增根级 worker 或无托管 goroutine。

## 自动化验证

以下命令在`GOWORK=off`下通过：

```text
core: go test ./backend/cap/provisionresourcecap/... ./backend/internal/service/provisioning ./backend/internal/composition -count=20 -shuffle=on
core: go test ./backend/internal/service/commerce -run '^TestServiceProvisioningPostgreSQL' -count=3 -shuffle=on
core: go test ./backend -run '^TestCommerceFoundationMigrationPostgreSQL$' -count=1
core: go test ./backend -run '^$'
resource: go test ./backend/internal/service/provisionsource -count=20 -shuffle=on
resource: go test ./backend/internal/service/provisionsource -run '^TestProvisionSourcePostgreSQL' -count=3 -shuffle=on
resource: go test ./backend -run '^TestResourceFoundationMigrationPostgreSQL$' -count=1
resource: go test ./backend/internal/composition ./backend -run '^$'
```

以下官方静态检查通过：

```text
make.cmd lint.go dir=apps/lina-plugins/idc-core plugins=1
make.cmd lint.go dir=apps/lina-plugins/idc-resource plugins=1
```

真实 PostgreSQL 回归直接断言：统一事件只物化一次；两台服务器一成一败得到一个活动服务、一个取消服务、订单部分履约、库存消耗`1`并释放`1`；两个 worker 并发派单只形成一条 core 执行记录；`result_unknown`不发送回执且后续权威成功可以恢复；core 提交成功而回执暂时失败时，再次运行只补报回执；resource 已接收回执但 core 本地确认失败时，即使服务版本随后变化也会重发同一冻结摘要；任务事务强制回滚时 resource 中任务和凭据均为`0`且秘密只吊销一次。

## 规则影响与剩余范围

- 数据权限：本轮只有内部托管作业与公开跨插件能力，没有新增 HTTP 列表、详情、导出或人工执行入口；没有扩大客户或员工可见数据。
- `i18n`：新增库存预留`partial`字典中英文文案。`make.cmd i18n.check plugins=1`的运行时扫描为`0`违规，宿主和插件消息覆盖通过；仓库级前端键检查仍被禁止修改的宿主`apps/lina-vben/apps/web-antd/src/views/system/message/notice-preview-modal.vue`既有 6 个缺键阻断，分别是中英文的`plugin.content-notice.preview.title`、`plugin.content-notice.fields.type`和`plugin.content-notice.fields.createdBy`。根目录没有`.contributing`，本业务插件不得修改宿主冒充本任务完成。
- 性能：单轮最多选择`64`个工作项，配置一次集合读取，订单与库存按受影响订单收敛，禁止按配置项执行数据库查询。
- 尚未完成：暂停、恢复、重装、变配、重置密码、换 IP 等非新购业务操作的 core 门禁与全部 Saga 组合；其余参数型能力专用对账、任务和秘密补偿；客户安全领取初始密码的短期授权 API 与前端页面。这些内容继续保留在父任务，不因本记录而视为完成。

## Lina 复审结论

本次任务级复审重新读取`AGENTS.md`以及`.agents/rules/openspec.md`、`documentation.md`、`architecture.md`、`data-permission.md`、`plugin.md`、`api-contract.md`、`backend-go.md`、`database.md`、`testing.md`和`i18n.md`。两个插件根目录均不存在局部`AGENTS.md`。

复审修复了校验函数复杂度、内部查询`SELECT *`、生产包测试专用 helper、`result_unknown`被错误终结、可变 core 版本导致回执重放冲突五类问题。修复后未发现授权范围内的阻断问题；缓存一致性、前端 UI、E2E 和开发工具未命中，因为本轮未新增缓存、页面、浏览器工作流或工具脚本。仓库级`i18n`既有宿主阻断按上文保留，不改变本阶段插件实现结论，也不授权修改宿主。
