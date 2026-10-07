# 第四阶段商品草稿能力运行时验证记录

> **`FB-52`失效声明：**Provider 容量快照跨插件创建通用商品草稿的能力与运行时已经废止。本记录只用于删除旧 capability、作业、租约、测试和开发数据时迁移对账，不能作为当前插件能力完成证据，也不得保留兼容入口。

## 当前结论

最初验证完成`4.14.c.2`的核心运行时闭环但未勾选该子任务；后续`4.14.c.3.b/c`现已补齐资源侧持久化执行器和生产作业生命周期。`idc-core`已在声明期注册`catalogcap v1` descriptor 和来源工厂；descriptor 持有的 invoker 初始未绑定并拒绝全部调用。启动装配只有在宿主权限依赖、固定来源系统身份、业务审计、商品草稿事务服务和管理路由全部构造成功后，才把同一个服务实例单次绑定给 invoker。

动态调用和源码来源调用现在共用同一 invoker，因此共同执行精确 caller、owner、service、version、method、空资源逃逸参数、请求结构和`32 MiB`载荷校验。源码来源在序列化前先校验最多`20,000`项的结构，序列化后再执行字节上限检查。绑定失败、重复绑定、错误来源或超限请求全部失败关闭。

核心事务服务必须注入`idccap.BusinessAuditWriter`。首次创建和精确重放都会在同一数据库事务追加固定系统身份`plugin:idc-resource`的业务审计；审计失败时草稿与候选整体回滚。调用方不能通过请求载荷伪造租户、用户或系统身份。

## 后续闭环

`idc-resource`现已从完整库存快照生成安全`ProviderDraftSource`，在终态`plan`快照事务内登记本地持久化发布请求，并通过带 PostgreSQL 租约栅栏的`pluginhost.Jobs`处理器在事务外调用同一`catalogcap`来源客户端。模块停用后宿主移除处理器并停止新领取，在途调用同步收口或由租约过期恢复，重新启用后按持久化状态续跑。证据见[第四阶段商品草稿持久化发布执行器验证记录](phase-4-catalog-publication-executor-validation.md)和[第四阶段商品草稿发布作业生命周期验证记录](phase-4-catalog-publication-job-lifecycle-validation.md)。

运行时仍保持以下门禁：跨插件调用只发生在资源事务外；超时按结果未知处理，只能用同一个不可变来源重试；核心来源唯一键和逐项安全字段重放校验负责收敛，禁止用改写后的内容复用同一快照键。由于仓库级`i18n`门禁仍被禁止修改的宿主通知预览页 6 个既有缺失键阻断，任务清单暂不勾选`4.14.c.2`。

## 验证证据

| 命令或检查 | 结果 |
|---|---|
| `go test ./backend/cap/catalogcap/... ./backend/internal/service/catalogdraft ./backend -count=20 -shuffle=on` | 通过；覆盖未绑定拒绝、单次绑定、精确身份、源码与动态同路径、无效来源调用前拒绝和响应校验 |
| `LINAPRO_IDC_CORE_TEST_DSN=... go test ./backend/internal/service/catalogdraft -count=20 -shuffle=on` | 通过；真实 PostgreSQL 覆盖并发来源重放、审计写入和审计失败时草稿/候选回滚 |
| `GOWORK=off go test ./backend/... -count=1` | 通过；核心插件全部后端包回归通过 |
| `make.cmd lint.go dir=apps/lina-plugins/idc-core plugins=1` | 通过；`golangci-lint`与`staticcheck U1000`均为 0 问题 |
| `make.cmd plugins.check` | 通过；扫描 585 个文件，0 项插件治理问题 |
| `openspec validate add-idc-server-operations-platform --strict` | 通过 |

## 影响分析

- 架构与插件：能力 owner 仍为`idc-core`；来源工厂只写后冻结，运行期没有全局`Resolve/Get`。资源插件只在库存事务内写本地发布请求，尚未执行跨插件自动发布，避免把远程边界放入库存事务。
- DI 与启动失败：invoker在声明期可注册但保持不可用；审计、权限或路由任一装配失败都不会绑定。服务只在启动阶段构造，没有请求内`New()`。
- 数据权限与审计：只接受`idc-resource`调用精确写方法，不接受表或资源参数；业务审计使用固定系统来源并与草稿事务原子提交。当前无用户 HTTP 入口。
- 性能：来源最多`20,000`项、动态载荷最多`32 MiB`；校验和映射均为单次线性遍历，事务使用有界集合查询和批量写入，没有随候选数增长的查询次数。
- SQL 与缓存：本轮运行时接线没有新增 SQL、缓存、订阅或失效逻辑；使用`4.14.b`已验证的草稿表和事务。
- API、前端、E2E 与`i18n`：没有新增 HTTP API、页面、用户可见行为、字典或错误资源，技术错误仅存在于插件内部能力边界，因此无 E2E 和`i18n`影响。

## Lina 审查报告

**变更：**`add-idc-server-operations-platform`
**范围：**任务级，`4.14.c.2`部分实现
**已读取规则：** `AGENTS.md`、`.agents/rules/openspec.md`、`.agents/rules/documentation.md`、`.agents/rules/architecture.md`、`.agents/rules/data-permission.md`、`.agents/rules/plugin.md`、`.agents/rules/backend-go.md`、`.agents/rules/testing.md`、`.agents/rules/i18n.md`

### 审查结论

- 严重：0。
- 警告：0。
- 已修复：源码来源此前依赖 invoker 二次校验，可能先序列化无效大对象；现在改为序列化前结构校验、序列化后字节校验，并新增 owner 未被调用的失败测试。
- 剩余工作：worker 租约、结果状态转换和模块停用生命周期尚未实现，因此相关任务保持未完成，不形成自动业务入口。
- 无影响域：没有 API、前端、E2E、缓存、SQL 和`i18n`变化；没有修改宿主、工具链或插件根目录外生产代码。
