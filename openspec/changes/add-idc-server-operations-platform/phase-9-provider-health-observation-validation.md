# Provider 健康采集验证记录

## 验证范围

本记录覆盖任务`9.5.c`：把真实 Provider 写入、精确任务查询、精确资源查询、库存分页与认证分类接入`idc-resource`不可变健康快照。插件根目录不存在独立`AGENTS.md`，适用项目顶层规范。

本轮没有访问线上 Provider；真实数据库验证只使用自动生成并在测试结束后删除的隔离 PostgreSQL schema。

## 实现结论

- 写入、任务查询、资源查询和库存分页都在 Provider 调用点使用 Go 单调时钟计算实际耗时；Provider 构造、本地资源映射或保护门禁失败不产生伪上游样本。
- 本地任务转换、查询重调度/收敛、库存重试/失败/发布先持久化，随后追加健康快照。健康失败会令当前作业报告错误，但不会把已保存业务真相回滚或标成未发生。
- 任务查询校验返回任务号及可选资源号必须与查询目标一致；资源查询校验资源号和观察新鲜度。调用错误按规范错误码记录，畸形或错对象响应记录`invalid_response + result_unknown`，不保存原始错误或响应。
- 合法资源详情即使尚未达到写操作目标，也表示查询链路健康，因此记查询成功并继续只读退避，不把“业务尚未收敛”误算为 Provider 查询故障。
- 库存同步在调用失败、空响应、Stage 校验失败或成功发布后记录`inventory`维度；已经失去租约的陈旧 worker 不追加误导性健康样本。
- 任一规范错误码为`authentication`的写入、查询或库存调用，会在同一数据库事务内追加原操作与`authentication`两条快照。双维度要么一起提交、要么一起回滚；只有`write`操作推进写熔断窗口。
- 写熔断开启时，成功查询或库存快照仍按自身结果记`healthy`，不会因为渠道正受写保护而被错误标成`unavailable`。

## 规则域结论

- 架构、插件与 DI：健康服务由启动期唯一构造并显式注入任务执行器和库存 worker；实现闭环在资源插件，没有宿主目录变化。
- 数据库与性能：复用`013`中当前状态和不可变快照；每次调用固定追加一条，认证分类固定追加两条，不随历史数量增长，不新增查询扇出或`N+1`。
- 数据权限：仅记录 tenant 0 内部渠道的规范机器指标，不新增 HTTP、用户输入对象编号、列表或导出入口。
- 缓存：健康权威源仍为 PostgreSQL，无缓存、失效或跨节点陈旧窗口。
- 安全：不保存原始错误、响应、秘密引用、凭据或业务正文；稳定摘要码继续由严格机器值校验保护。
- 文档和`i18n`：中英文`README`同步维护；没有运行时页面、菜单、按钮、API 元数据或翻译资源变化，运行时`i18n`无影响。
- API、前端、E2E 与开发工具：均无变更；后端行为由单元和真实 PostgreSQL 测试覆盖。

## 阶段边界

任务`9.5.c`完成，父任务`9.5`保持未完成。商家可编辑阈值、管理员通知、健康后台页面和经精确权限、数据范围、最近重新认证及审计控制的人工接管尚未实现。

## 验证证据

| 命令或检查 | 结果 |
|---|---|
| `go test ./backend/internal/service/providerhealth ./backend/internal/service/resourcetask ./backend/internal/service/inventorysnapshot -count=20 -shuffle=on` | 通过；覆盖输入安全、查询成功/失败采集、库存边界及双维度约束 |
| `TestResourceFoundationMigrationPostgreSQL -count=3 -shuffle=on` | 通过；写熔断、查询/库存快照及认证双维度在隔离 schema 中持久化 |
| `go test ./backend/... -count=20 -shuffle=on`（`GOWORK=off`） | 通过；资源插件全部后端包重复乱序回归 |
| `.\make.cmd lint.go dir=apps/lina-plugins/idc-resource plugins=1` | 通过；`golangci-lint`和`staticcheck U1000`均为零问题 |
| `openspec validate add-idc-server-operations-platform --strict`与`git diff --check` | 通过；OpenSpec 严格校验和差异空白检查无错误 |

## Lina 审查摘要

本次任务级审查从工作区与插件未跟踪文件清单、OpenSpec 状态和任务上下文开始，重新读取`AGENTS.md`、`openspec.md`、`documentation.md`、`architecture.md`、`data-permission.md`、`plugin.md`、`backend-go.md`、`database.md`、`testing.md`、`i18n.md`与`cache-consistency.md`；资源插件根目录不存在局部`AGENTS.md`。

审查发现任务执行器和库存 worker 曾各自声明重复窄健康接口，且健康组件导出了具体实现。现已收敛为健康 owner 自有`Service`宽接口、私有`serviceImpl`和启动期唯一实例，调用方通过构造函数显式复用。审查还确认任务查询校验任务/资源编号、合法未收敛资源读取不误报、库存失租不追加样本、认证双快照同事务、写保护不污染只读健康分类。本轮没有 HTTP、前端、E2E、运行时`i18n`、缓存或开发工具变化，对应规则域均为无影响。

修复后重新执行全后端 20 轮乱序测试、真实 PostgreSQL 三轮、官方 lint、OpenSpec 严格校验和差异检查。最终严重问题 0 个、警告 0 个，未发现阻塞`9.5.c`完成的问题。父任务仍因商家策略、通知、后台健康页和人工接管未实现而保持未完成。
