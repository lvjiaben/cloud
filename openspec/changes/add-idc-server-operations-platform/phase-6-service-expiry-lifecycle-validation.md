# 阶段 6.6 服务到期生命周期自动化验证

> **`FB-52`部分失效声明：**提醒、到期栅栏和持久任务框架可作为迁移对账参考；把暂停、恢复、终止和 Provider 任务当作所有服务的通用到期行为已经失效，不能作为当前长期权益生命周期完成证据。现行模型只对冻结契约声明持续管理的`service_entitlement`调度，并通过精确业务拥有方接缝执行其声明的动作；服务器暂停、恢复和资源终止只属于服务器插件。

## 交付结果

本切片已经实现按服务冻结的到期策略、提醒事实、宽限与到期收敛，以及暂停、续费后恢复和终止的 Provider 任务闭环。调度器只创建不可变任务，不在扫描事务修改服务状态或调用上游；处理器按服务顺序消费任务，并通过版本化 core-to-resource 契约让 resource owner 独占 Provider 任务、资源映射和能力证据。

仓库级功能实现、真实 PostgreSQL、静态检查、插件治理、OpenSpec 严格校验和 Windows/Linux amd64 构建均已通过。仓库级多语言门禁仍只被禁止在本业务插件任务中修改的宿主通知预览页 6 个既有缺键阻断，因此`6.6.a`至`6.6.c`可以勾选，`6.6.d`和顶级`6.6`继续保留未勾选。

## 数据与调度边界

- `032-service-expiry-scheduling.sql`新增按服务不可变生命周期策略、最多 16 条规范化提醒规则、生命周期任务和追加式任务事件，并给服务保存当前策略指针。
- 主节点每 30 秒执行一次调度和处理作业，生产批量固定为 32，服务层硬上限为 64。候选使用 PostgreSQL 权威时间、稳定排序和集合查询，一次批量插入任务与初始事件，不产生按返回行数增长的数据库查询。
- 调度任务冻结服务版本、原到期日、策略版本、动作、提醒序号和幂等/内容摘要。续费推进到期日后，旧任务因到期与策略栅栏失效；调度事务不修改权益、运行或 Provider 任务状态。
- 本地提醒在同一事务写入`idc.service.lifecycle.reminder.requested` Outbox 事实；宽限与到期使用服务版本栅栏收敛。通知模板与实际渠道投递由后续通知模块任务拥有，本切片不伪造“已送达”。
- 暂停、恢复和终止创建统一`business_operation`并由`lifecycleresourcecap/v1`调用 resource owner。resource 的`owner_metadata`只保存内部不可变意图，`sanitized_parameters`才会进入 Provider；暂停只发送`{"reason":"due"}`，恢复和终止发送空对象。

## 依赖注入追踪

- 生命周期任务来源由`idc-resource/backend/plugin.go`在声明期通过`lifecycleresourcecap/spi.Declare`注册，工厂只用 resource 自有 DAO 数据库构造`lifecyclesource.Service`；core 在作业注册期调用`Freeze()`取得不可变快照并显式构造来源，不通过运行期服务定位器访问 resource 内部实现。
- 生命周期调度器、处理器与提醒 Outbox 都由`idc-core/backend/plugin.go`在同一个作业装配入口构造。Outbox 的 owner 是 core，`outbox.NewEnqueuer(g.DB(), outbox.Config{})`与处理器共享同一 core 数据库实例，并作为强制构造参数传入`lifecycle.NewProcessor`，不存在隐藏全局实例或测试专用生产依赖。
- `lifecycleresourcecap`公开边界对动作和任务状态使用具名枚举类型；core 只在内部持久化边界进行显式转换，resource 的不可变任务元数据、能力选择和状态投影保持强类型，非法字符串在跨插件契约校验处失败关闭。

## 危险操作与失败收敛

- 自动终止只允许`running/stopped/suspended`三个稳定运行态。开通中、恢复中、变更中、终止中、已终止和未知状态不会进入自动回收。
- 终止领取先检查仍在执行或待人工确认的续费操作，再把原运行态冻结到任务并预留`terminating`。Provider 明确失败时恢复原运行态；明确能力不可用且尚未创建上游写任务时同样恢复并记录确定失败；写入是否发生未知或进入人工接管时保持`terminating`。
- 自动恢复要求服务当前`last_business_operation_no`仍精确关联一次权威成功的生命周期暂停。商家后续人工、安全或滥用暂停会使旧恢复候选和已创建恢复任务同时失去资格。
- resource owner 把合法 Provider 机器错误码规范化为 core 可持久化的小写稳定码，只返回脱敏状态、摘要和时间，不返回原始错误文本、凭据或上游资源号。
- 同一服务较早的未终结生命周期任务会阻挡较晚任务；任务、业务操作和服务状态均使用版本栅栏，任务事件数与聚合版本持续一致。

## 反馈闭环

- `FB-5`修复自动续费并发时的 PostgreSQL 锁等待快照问题。owner 行锁与最新尝试读取拆成两个语句，等待锁后的事务能看到前一事务刚提交的尝试，不再重复分配序号 1。
- `FB-6`把自动终止从排除式判断收紧为稳定运行态白名单，并在候选查询、插入重检、执行资格和终止预留四层保持一致。
- `FB-7`为自动恢复补充当前生命周期暂停来源证明，避免历史自动暂停覆盖后续人工或安全暂停。
- `FB-8`区分明确未写入的能力不可用与结果未知，前者安全恢复，后者保持终止中并转人工。
- `FB-9`统一 resource Provider 失败码与 core 生命周期失败码格式，明确失败不再被误判为投影冲突。
- 独立`lina-review`补充收紧了跨插件动作/任务状态具名类型、core 内部生命周期枚举和数据库错误保留；所有`RowsAffected`、事件写入和提醒载荷编码错误都保留原始因果链，resource 重放比较不再忽略摘要解码失败。

## 已执行验证

| 验证项 | 结果 |
| --- | --- |
| core 生命周期真实 PostgreSQL 随机顺序测试 20 轮 | 通过；覆盖 8 路并发调度、批量上限、任务重放、提醒 Outbox、宽限/到期、暂停/恢复/终止顺序、续费栅栏、失败与能力不可用恢复、人工暂停和不稳定运行态保护 |
| 同一服务并发自动续费真实 PostgreSQL 测试 20 轮 | 通过；两个执行者复用同一尝试事实，无重复扣款、重复尝试或唯一键冲突 |
| resource 生命周期任务意图真实 PostgreSQL 随机顺序测试 20 轮 | 通过；覆盖任务创建/重放/冲突、能力快照、可选到期原因、内部元数据隔离、精确 Provider 参数和失败码规范化 |
| core 与 resource 生命周期单元测试随机顺序 20 轮 | 通过；覆盖 SPI 冻结、稳定终止白名单、恢复来源证明和失败状态投影 |
| core 全后端真实 PostgreSQL 回归：`go test ./backend/... -count=1 -shuffle=on -p 1` | 通过 |
| resource 全后端真实 PostgreSQL 回归：`go test ./backend/... -count=1 -shuffle=on -p 1` | 通过 |
| 智简魔方适配器后端回归 | 通过 |
| core、resource 与 zjmfcloud：`make.cmd lint.go plugins=1 dir=...` | 通过；`golangci-lint`和`staticcheck U1000`均为 0 问题 |
| 插件治理扫描 | 通过；11 个清单、1153 个文件、1131 个 Go 文件、0 项发现 |
| `openspec validate add-idc-server-operations-platform --strict` | 通过 |
| core、resource、zjmfcloud 的 Windows/Linux amd64 构建 | 通过 |
| 运行时硬编码、宿主/插件消息覆盖和新增双语字典 | 通过 |
| 仓库级前端多语言键覆盖 | 仍仅被禁止修改的宿主通知预览页中英文各 3 个既有缺键阻断，共 6 项 |

## 影响审查

- 数据权限：本切片没有新增 HTTP、列表、详情、导出、下载或后台写接口。调度与处理入口只由插件主节点生产作业构造，固定使用单商家默认租户并从权威服务事实派生客户与资源归属，不形成身份或数据存在性绕过。
- 缓存：无影响。策略、服务、任务、业务操作、Outbox、能力快照和 Provider 任务全部以 PostgreSQL 为权威源，没有新增缓存或陈旧窗口。
- API、前端与 E2E：无变更。本切片不提前宣称`6.7`管理/Portal 页面和`6.9`浏览器整链路完成。
- i18n：新增生命周期动作与状态字典已提供中英文镜像；没有新增用户可见硬编码、API 文档源文本或前端键。仓库既有 6 个宿主缺键不由本切片引入。
- 开发工具与跨平台：没有修改根`Makefile`、`make.cmd`、根`hack`、CI 或工具脚本；正式构建已在 Windows 和 Linux amd64 目标完成。
- 性能：候选扫描和任务创建采用有界集合 SQL，处理阶段每个外部动作单独收敛但总批量不超过 64；同步客户请求不调用 Provider，不存在按服务列表行数展开的`N+1`路径。

## 仓库级既有阻断

`make.cmd i18n.check plugins=1`确认运行时硬编码扫描与宿主/插件消息覆盖通过，唯一失败仍是`apps/lina-vben`通知预览页引用的 3 个键在中英文各缺一份。仓库不存在`.contributing`，当前业务插件变更禁止修改宿主目录，因此不越界修复；`6.6.d`和顶级`6.6`保持未勾选。

**已读取规则：**`AGENTS.md`、`.agents/rules/openspec.md`、`.agents/rules/documentation.md`、`.agents/instructions/markdown-format.instructions.md`、`.agents/rules/architecture.md`、`.agents/rules/data-permission.md`、`.agents/rules/plugin.md`、`.agents/rules/backend-go.md`、`.agents/rules/database.md`、`.agents/rules/testing.md`、`.agents/rules/i18n.md`。
