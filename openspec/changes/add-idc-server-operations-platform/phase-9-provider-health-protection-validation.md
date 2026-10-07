# Provider 写健康保护验证记录

## 验证范围

本记录覆盖任务`9.5.b`：`idc-resource`资源拥有方持久化写健康窗口、不可变观察、跨作业/跨节点保护、任务领取门禁及执行批次二次门禁。插件根目录不存在独立`AGENTS.md`，因此适用项目顶层规范。

本轮没有调用任何线上 Provider，没有读取或修改生产用户、实例、订单或凭据。真实数据库验证只在自动生成的隔离 PostgreSQL schema 中执行，结束后仅删除该 schema。

## 实现结论

- 新增`plugin_idc_resource_channel_health_state`作为每渠道当前健康窗口，使用`tenant_id + channel_no`唯一约束、渠道外键、计数约束和仅覆盖已打开保护行的索引。该表属于可变运行状态，不使用软删除。
- 既有`health_snapshot`增加受检查约束的`operation_kind`，每个观察以 63 字符随机编号追加不可变快照；只记录规范操作类别、健康分类、毫秒耗时、连续失败数、熔断状态、稳定机器摘要码和 PostgreSQL 观察时间。
- 写窗口最多 20 个样本；首个安全默认策略要求至少 5 个样本，并在失败总数达到 3、未知结果达到 2 或连续失败达到 3 时开启 5 分钟保护。成功会清零连续失败，但不会伪造或抹除窗口内历史失败。
- 所有保护时间使用 PostgreSQL 权威时钟。保护到期后，整批渠道在同一事务内恢复计数与状态；写观察与当前状态在渠道行锁内串行推进，不依赖应用节点本地缓存。
- 任务领取查询直接排除保护未到期的渠道，因此任务保持`pending/retry_wait`且尝试次数不变。执行器对本批最多 32 个不同渠道再执行一次集合门禁，覆盖任务领取后已经提交的保护变化；该竞态路径通过精确任务、尝试号、租约令牌、状态和数据库租约栅栏无损释放任务并回退领取时增加的尝试次数，不会按任务或渠道逐条查询。
- 只有真正越过 Provider `Execute`边界的调用才记录写健康和实际单调耗时。保护门禁、资源映射失败及未构造 Provider 的调用不伪造上游样本。写结果先进入权威任务状态，再追加健康观察；健康持久化失败不会把未保存的任务结果伪装为成功。
- 写保护不接入任务和资源观察路径，精确`QueryTask`与`QueryResource`仍可收敛未知或运行中结果。

## 数据库与性能

迁移`013-resource-channel-health-protection.sql`使用`ADD COLUMN IF NOT EXISTS`、存在性保护的约束重建、`CREATE TABLE IF NOT EXISTS`和`CREATE INDEX IF NOT EXISTS`，连续执行结果一致。健康状态和快照均属于平台 tenant 0 的内部资源拥有方数据，没有新增外部对象编号或 HTTP 数据入口。

高频领取利用`channel_health_state(tenant_id, channel_no, write_protected_until)`部分索引核对当前渠道；执行器把最多 32 个渠道去重后用一次`WHERE IN`集合查询。窗口更新只锁定一个渠道状态行并单次追加快照，不随历史快照数量扫描，未引入`N+1`或无界装配。

## 阶段边界

任务`9.5.b`完成，但父任务`9.5`保持未完成。商家可编辑健康阈值、保护通知、健康后台页面和具有精确权限、目标数据范围、最近重新认证及审计证据的人工接管入口尚未实现。认证、普通查询和库存同步的健康观察虽已有数据库操作类别契约，本子项只接入写执行路径，不宣称完成全部健康采集。

## 规则域结论

- 架构、插件和依赖注入：全部实现闭环在`idc-resource`；启动装配显式构造共享健康服务并注入任务执行器，没有修改`lina-core`或`lina-vben`。
- 数据权限：仅处理 tenant 0 内部渠道与任务，没有新增用户输入编号、列表、导出或管理入口。未来人工接管仍必须使用独立权限与数据范围校验。
- 数据库：迁移幂等，DAO/DO/Entity 由官方`make dao`生成；健康状态为显式生命周期，快照为不可变证据，均不采用软删除。
- 缓存一致性：保护权威源为 PostgreSQL，不使用缓存，因此不存在缓存失效、节点本地陈旧值或启动期预热要求。
- 后端和测试：集合门禁、窗口阈值、失败关闭、跨服务实例可见性、保护期不领取及到期恢复均有单元或真实 PostgreSQL 测试。
- 文档和`i18n`：中英文`README`同步维护；没有新增运行时页面、菜单、按钮、接口元数据、错误翻译键或语言资源，运行时`i18n`无影响。
- 开发工具：仅扩展插件现有 DAO 表清单，没有新增或修改脚本及命令实现。Windows 使用项目`make.cmd dao`入口；Linux/macOS 使用同一 Go 工具链的`make dao`入口。本轮不改变任一平台的默认开发路径。
- API、前端与`E2E`：没有新增 HTTP 路由、DTO、页面或用户可观察交互，因此没有对应影响；后台策略和人工接管 UI 留在父任务。

## 验证证据

| 命令或检查 | 结果 |
|---|---|
| `go test ./backend/internal/service/providerhealth ./backend/internal/service/resourcetask -count=20 -shuffle=on`（`GOWORK=off`） | 通过；覆盖有界阈值、连续失败恢复、输入失败关闭、批量去重/上限、一次批量门禁及保护时零 Provider 调用 |
| `TestResourceFoundationMigrationPostgreSQL -count=3` | 通过；隔离 schema 内连续执行全部迁移，两个服务实例共享保护，保护任务不领取且尝试次数为 0，到期后状态恢复 |
| `make dao dir=apps/lina-plugins/idc-resource/backend` | 通过；使用专用`linapro_idc_codegen`数据库和 PostgreSQL 管理角色生成 DAO/DO/Entity |
| `go test ./backend/... -count=20 -shuffle=on`（`GOWORK=off`） | 通过；资源插件全部后端包重复乱序回归 |
| `.\make.cmd lint.go dir=apps/lina-plugins/idc-resource plugins=1` | 通过；`golangci-lint`和`staticcheck U1000`均为零问题 |
| `openspec validate add-idc-server-operations-platform --strict`与`git diff --check` | 通过；OpenSpec 严格校验和差异空白检查无错误 |

官方`db.init`仍被仓库既有宿主 Controller 重复声明阻断，且宿主目录在无`.contributing`时禁止修改。本轮使用已有受限测试入口在随机 schema 内重复运行插件全部迁移，未触碰应用表或生产数据。

## Lina 审查摘要

本次任务级审查从`git status --short`、`git ls-files --others --exclude-standard`、插件未跟踪文件展开和 OpenSpec 状态开始，并重新读取`AGENTS.md`、`openspec.md`、`documentation.md`、`architecture.md`、`data-permission.md`、`plugin.md`、`backend-go.md`、`database.md`、`testing.md`、`i18n.md`、`cache-consistency.md`与`dev-tooling.md`；资源插件根目录不存在局部`AGENTS.md`。

审查先发现一处领取后保护竞态：执行批次二次门禁虽然阻止 Provider 调用，但原实现会把该次领取计入尝试预算。现已新增租约栅栏下的无损延期，精确回退本次尝试并清空租约，单元测试和真实 PostgreSQL 场景均覆盖。审查同时把新健康服务的事务访问收敛到生成 DAO 的`Transaction/Ctx`边界，保留固定列置空作为生成 nullable DO 无法表达`NULL`的窄例外。修复后重新执行全部测试、真实 PostgreSQL、官方 lint、OpenSpec 严格校验和差异检查，最终严重问题 0 个、警告 0 个，未发现阻塞`9.5.b`完成的问题。

剩余风险均属于父任务已声明边界：商家可编辑阈值、通知、后台健康页、认证/普通查询/库存采集观察和人工接管尚未实现；没有因此勾选父任务`9.5`。本轮没有公开 API、前端、E2E、运行时翻译、缓存或宿主目录变化，对应规则域均为无影响。
