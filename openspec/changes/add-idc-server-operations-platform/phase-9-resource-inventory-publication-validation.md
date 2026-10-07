# 第九阶段资源库存原子发布验证记录

> **`FB-52`范围纠正：**本文“发布”只表示服务器插件内部供应商容量快照原子完成，不是通用商品库存或商品发布。资源容量同步可以作为服务器异步履约的内部证据，但不得生成商品草稿、修改商品简单库存、改变销售状态或进入报价与结账。下文中把完整快照交给任务`4.14`生成商品草稿的结论已经失效，只可用于清理旧接缝和迁移对账。

## 完成范围

本轮完成任务`9.3.b`。`idc-resource`现在拥有 Provider 库存同步的唯一暂存和发布边界：开始同步时固化渠道、库存类型和页大小；每次只接受与已落库下一游标完全一致的页面；终页成功前继续保留最后完整快照，终页在同一事务内批量写入页面、完成链式指纹、发布新快照并把旧完整快照转为历史。

智简魔方双来源容量观测和资源拥有方内部快照闭环仍可作为服务器插件能力保留；区域、线路、规格、镜像和容量分页投影及未验证财务能力的显式`unsupported`语义属于服务器供应商契约。完整快照不得再交给商品拥有方生成待审核商品草稿。

## 数据库与持久化边界

- 幂等迁移`003-resource-inventory-publication.sql`为库存快照增加固定`page_size`、精确`next_cursor`和`page_count`，并用部分唯一索引保证同一渠道和库存类型最多只有一个`collecting`快照。
- 同步上限固定为 200 页、100,000 条；单页继续服从`resourcecap.MaximumInventoryPageSize`的 500 条上限。上限在写入前检查，禁止无界暂存或排序。
- 页面条目按外部编号稳定排序并规范化 JSON；链式 SHA-256 指纹覆盖前页指纹、渠道、库存类型、页大小、页号、请求游标、下一游标、终页标志和全部白名单投影。
- 单页最多执行一次有界批量`INSERT`，不存在随条目数增长的数据库往返。跨页重复外部编号由唯一约束拒绝，并使本页事务整体回滚。
- 只有终页事务会把同渠道、同库存类型的旧`complete`快照改为`superseded`。错误游标、子集筛选、畸形页、重复页、超限页或显式失败都不改变最后完整快照。

## 资源观察租约反馈闭环

重复真实库测试同时发现并完成`FB-4`：资源观察领取返回的 PostgreSQL`ClaimedAt`曾与 ORM 自动维护的`updated_at`存在微小偏差，导致领取后立即返回且时间等于`ClaimedAt`的有效快照被误判过旧。

幂等迁移`004-resource-observation-lease.sql`新增独立业务字段`observation_claimed_at`。领取事务用同一个 PostgreSQL 时间写入该字段并原样返回；回报只以该字段作为新鲜度下界，不再读取`updated_at`。真实库测试同时证明早于领取时间的快照仍被拒绝、等于领取时间的快照被接受，且落库值与返回值完全一致。

## 验证证据

| 命令或检查 | 结果 |
|---|---|
| `GOWORK=off go test ./backend/... -count=20 -shuffle=on` | 通过；覆盖库存指纹和全部资源插件后端回归 |
| `GOWORK=off go test ./backend -run '^TestResourceFoundationMigrationPostgreSQL$' -count=20 -shuffle=on` | 通过；真实 PostgreSQL 隔离 schema、全迁移两次重放、分页续传、错误游标、部分同步隔离、终页原子切换、跨页重复回滚、观察领取时间一致性均通过 |
| `.\make.cmd lint.go dir=apps/lina-plugins/idc-resource plugins=1` | 通过；`golangci-lint`与`staticcheck U1000`均为 0 问题 |
| `.\make.cmd plugins.check` | 通过；扫描 11 个插件清单和 521 个文件，0 项治理问题 |
| `openspec validate add-idc-server-operations-platform --strict` | 通过 |
| 本轮实现、迁移、测试和文档文件的尾随空白静态扫描 | 通过；未发现新增空白错误 |

## 影响分析

- 架构与依赖注入：新增的`inventorysnapshot.Service`只依赖构造函数显式注入的 resource owner 数据库；本任务没有注册生产 worker，也没有新增宿主或跨插件运行期依赖。实例由后续托管作业装配创建，当前不伪装为已上线调度能力。
- 数据权限：这是固定`tenant 0`的平台内部作业边界，只按已启用渠道和服务生成的不可预测快照号读写；没有新增 HTTP API、用户输入编号入口或绕过调用方数据范围的路径。
- 性能：每页一次批量插入，状态读取和更新均使用快照唯一键或现有渠道/类型/状态索引；固定页数、页大小和总条数上限，不存在`N+1`和无约束全表扫描。
- SQL：两份增量文件仅含幂等 DDL 和注释，无 Seed/Mock、无软删除、无手写自增主键。DAO/DO/Entity均由项目代码生成入口重新生成；`updated_at`继续由 GoFrame 自动维护，观察领取时间使用独立业务字段。
- 缓存：没有新增缓存、缓存键、失效路径或进程内权威状态；分页续传和发布状态只以 PostgreSQL 为准。
- API 与前端：没有新增或修改 HTTP 路由、DTO、权限标签、页面、菜单或用户可观察交互，因此不触发 E2E 和 Chrome 页面验收。
- `i18n`：没有新增运行时文案、错误响应、API 文档元数据或语言键；只同步维护中英文技术说明，运行时`i18n`无影响。
- 开发工具与跨平台：没有修改脚本、构建工具或 CI；Windows 通过项目`make.cmd`入口完成生成和检查，Linux/macOS继续使用同一插件 Go/SQL 资产。

## Lina 审查报告

**变更：** `add-idc-server-operations-platform`
**范围：** 任务`9.3.b`与测试过程中发现的反馈`FB-4`
**审查文件数：** 21
**范围来源：** `git status --short`、`git ls-files --others --exclude-standard`、任务记录、生成文件清单与本轮实现上下文
**已读取规则文件：** `AGENTS.md`、`.agents/rules/openspec.md`、`.agents/rules/documentation.md`、`.agents/instructions/markdown-format.instructions.md`、`.agents/rules/architecture.md`、`.agents/rules/data-permission.md`、`.agents/rules/plugin.md`、`.agents/rules/backend-go.md`、`.agents/rules/database.md`、`.agents/rules/testing.md`、`.agents/rules/i18n.md`

### 发现的问题

未发现阻挡问题。严重问题 0 个，警告 0 个。

### 规则域结论

- OpenSpec 与文档：通过。实现边界、剩余缺口、反馈根因、验证命令和未命中规则域均已如实记录；父任务没有提前完成。
- 架构与插件：通过。库存快照由`resource`拥有方独占持久化，服务只接受显式数据库依赖；`idc-resource`插件根目录不存在本地`AGENTS.md`，适用项目顶级规则。
- 数据权限：通过。内部作业固定使用平台租户、已启用渠道和不可预测快照号，没有新增用户编号入口、存在性泄露或跨租户路径。
- 后端 Go：通过。事务回滚、固定容量上限、每页单次批量插入、自动时间字段边界和显式错误处理均满足门禁；生成的 DAO、DO、Entity已纳入审查。
- 数据库：通过。`003`与`004`是幂等增量 DDL，无 Seed、Mock、软删除或手写自增主键；唯一约束、检查约束和查询索引覆盖发布与领取路径。
- 测试：通过。单元测试、20 轮真实 PostgreSQL 隔离测试、全后端高频回归和 lint 均覆盖当前工作区实现；本轮没有用户可观察页面或 HTTP 行为，不触发 E2E。
- 缓存、API、前端与开发工具：无影响。本轮没有缓存、路由、DTO、页面、脚本、构建或 CI 变更。
- `i18n`：无运行时影响。没有新增错误响应、API 元数据或语言键；中英文插件说明已同步。

### 剩余风险

库存服务目前是内部可测试组件，尚未装配生产托管作业；该装配缺口由资源执行与作业任务承载。完整快照到独立商品规格投影和待审核商品草稿的链路尚未实现，并继续由商品任务`4.14`承载，不影响任务`9.3`的适配器与发布边界验收。
