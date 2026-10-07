# 第四阶段商品草稿发布作业生命周期验证记录

> **`FB-52`失效声明：**本作业只服务已经废止的 Provider 商品草稿路径，不能作为当前插件作业或商品发布完成证据。旧租约和状态仅用于清理与开发数据迁移对账，不保留兼容 worker。

## 实现范围

本轮完成`4.14.c.3.c`的实现。`idc-resource/backend/plugin.go`通过阻塞式`pluginhost.Jobs`声明唯一的商品草稿投递任务，不自行启动根 worker 或无托管 goroutine。任务每分钟触发一次，每轮最多连续执行 8 次`catalogpublication.Service.RunOnce`，每次仍只领取一条请求，队列为空立即停止；单条数据库租约固定为 2 分钟。

生产处理器在首次真实执行时一次性懒加载依赖：冻结`idc-core`声明期`catalogcap`来源工厂、构造走同一 owner-aware invoker 的源码客户端，并注入资源库存快照服务。仅投影已安装任务清单或查看停用任务时不会读取数据库、冻结依赖或创建执行器；同一已注册处理器后续执行复用该实例。

任务声明带稳定英文源文案，并为宿主生成的处理器引用补充`en-US`与`zh-CN`运行时翻译。宿主统一任务管理继续负责展示、关键词匹配、调度、单例并发和插件启停，不在资源插件内复制调度器。

## 启停与恢复语义

- 宿主只为已启用插件发布可执行处理器；停用时移除该插件的处理器并同步内置任务投影，因此停用后不会开始新的领取。
- 处理器同步执行且每次只处理一条，不派生 goroutine。停用发生在调用途中时，当前调用可在活动租约内完成；若进程中止、调用超时或完成时租约已过期，完整租约 fencing 会拒绝迟到提交。
- 待处理、重试等待、活动租约和全部追加式事件均保留在资源数据库。重新启用会重建处理器；未过期状态继续按原计划执行，过期租约先追加恢复事件，再以同一不可变来源重领。
- 核心草稿的来源唯一键和逐项安全字段核对继续收敛“前次成功但响应丢失”的精确重放，恢复路径不会创建第二个来源草稿或改写历史。

## 验证证据

| 命令或检查 | 结果 |
|---|---|
| `GOWORK=off go test ./backend/internal/composition -run CatalogPublication -count=20 -shuffle=on` | 通过；覆盖主节点唯一注册、任务清单投影零构造、首次执行懒加载、成功构造复用、临时构造失败后恢复、空队列停止、单轮 8 条硬上限、次节点不注册和错误透传 |
| `GOWORK=off go test ./backend/... -count=20 -shuffle=on` | 串行重跑通过；资源插件全部后端包高重复回归通过 |
| `GOWORK=off go test ./internal/service/plugin/internal/integration ./internal/service/jobhandler ./internal/service/cron -count=1` | 通过；宿主任务声明投影、插件启停处理器注册/移除和统一调度回归通过 |
| `LINAPRO_IDC_RESOURCE_TEST_DSN=... GOWORK=off go test ./backend -run TestResourceFoundationMigrationPostgreSQL -count=20 -shuffle=on` | 通过；覆盖迟到完成拒绝、过期租约恢复、同源重放、并发单领、死信和历史保留 |
| `make.cmd lint.go dir=apps/lina-plugins/idc-resource plugins=1` | 通过；`golangci-lint`和`staticcheck U1000`均为 0 问题 |
| `make.cmd i18n.check` | 资源任务双语键的运行时扫描与消息覆盖通过；仓库前端键检查仍仅被宿主通知预览页 6 个既有缺失键阻断 |

并行执行宿主测试与资源插件全包测试时，宿主测试短暂创建并删除`apps/lina-plugins/plugin-dev-dynamic-ref-revive`，与资源治理遍历产生一次测试夹具竞争。改为串行后资源全包 20 次全部通过；该现象不涉及生产代码或业务并发。

## 影响分析

- 架构与插件：作业闭环全部位于`idc-resource`，只消费`idc-core/backend/cap/catalogcap`公开契约和声明期`SPI`。未修改宿主、其他插件内部包或框架目录。
- 依赖注入与失败关闭：任务投影无副作用；处理器首次执行才构造依赖。核心声明、数据库或 owner 客户端不可用时处理器返回错误，不领取请求，不降级到内部实现或服务定位器。
- 性能：每分钟最多连续处理 8 条，单次领取仍为 1 条；单例调度与数据库`SKIP LOCKED`共同限制并发。空队列只做一次有界领取，来源读取固定两次集合查询，最大 20,000 个候选，没有按候选项查询；相较每秒空轮询显著减少任务日志和数据库压力。
- 数据权限与审计：没有用户入口、对象参数或跨租户查询。宿主启用状态控制处理器可用性；草稿写入仍经过固定来源身份、精确能力授权和核心同事务业务审计。
- SQL 与历史：本轮没有新增或修改 SQL。停用不会删除或重写请求、租约和事件，历史保留继续由已验证的数据库约束保证。
- 缓存：无新增缓存。任务可用性由宿主启停状态和处理器注册表控制，业务续跑以资源数据库为唯一事实来源。
- `i18n`：新增任务名称和描述的中英文运行时键，英文源文案与宿主`AddWithMetadata`契约一致。资源范围消息覆盖通过；完整仓库门禁仍被禁止在本插件变更中修改的宿主页既有 6 个缺键阻断。
- API、前端与 E2E：没有新增 HTTP API、DTO、页面或用户交互流程；任务元数据由既有宿主管理页显示，因此本轮不需要新增 Chrome 或 Playwright 验证。
- 开发工具与跨平台：没有修改脚本、构建入口或开发工具。代码不使用平台路径或系统命令，验证在 Windows PowerShell 完成。

## 当前任务状态

`4.14.c.3.c`的代码、测试和双语资源已经实现，未发现业务或安全阻断。按照仓库门禁一致性要求，在`make.cmd i18n.check`仍因禁止修改的宿主通知预览页 6 个既有缺失键失败期间，任务清单暂不勾选；这不授权越界修改`apps/lina-vben`。

## Lina 审查报告

**变更：**`add-idc-server-operations-platform`
**范围：**任务级，`4.14.c.3.c`
**审查文件数：**11
**范围来源：**`git status --short`、插件子仓状态展开、未跟踪文件展开和任务上下文
**已读取规则：** `AGENTS.md`、`.agents/rules/openspec.md`、`.agents/rules/documentation.md`、`.agents/rules/architecture.md`、`.agents/rules/data-permission.md`、`.agents/rules/plugin.md`、`.agents/rules/backend-go.md`、`.agents/rules/database.md`、`.agents/rules/testing.md`、`.agents/rules/i18n.md`

### 发现的问题

- **严重，外部门禁：** `apps/lina-vben/apps/web-antd/src/views/system/message/notice-preview-modal.vue:16`、`:42`、`:45`引用的 3 个键在宿主`en-US`与`zh-CN`各缺一份，共 6 项。依据`.agents/rules/i18n.md`与`.agents/rules/openspec.md`，完整`i18n`门禁未通过会阻止任务勾选。当前仓库没有`.contributing`，本插件任务禁止修改宿主目录，因此只记录阻断，不越界修复。
- **警告：** 0。

### 规则域结论

- 架构、插件与后端 Go：通过。唯一生产入口为`pluginhost.Jobs`，无根 worker、无 goroutine、无跨插件内部包导入；依赖成功后缓存、失败后可重试。
- 性能：通过。每轮最多 8 次单条领取，空队列立即停止，来源保持固定两次集合查询，没有`N+1`。
- 数据权限：通过。没有用户入口或对象参数；宿主启停状态控制处理器，业务写仍经过 owner-aware 授权和审计。
- 测试：通过。任务级、插件级、真实 PostgreSQL 和宿主作业生命周期均有当前工作区证据。
- `i18n`：资源插件新增中英文键及消息覆盖通过；完整仓库被上述宿主既有问题阻断。
- SQL、缓存、API、前端、E2E 与开发工具：本任务无变更；理由已在影响分析记录。

### 摘要

- 本任务实现问题：严重 0，警告 0。
- 仓库外部阻断：严重 1。
- 建议操作：在具备宿主修改权限的独立变更中补齐 6 个通知预览键，重跑`make.cmd i18n.check`后再勾选`4.14.c.2`与`4.14.c.3.c`。
