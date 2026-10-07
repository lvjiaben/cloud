# 第四阶段商品草稿能力契约验证记录

## 完成范围

本轮完成任务`4.14.c.1`。`idc-core/backend/cap/catalogcap`新增版本化`v1`公共契约，唯一写方法为`provider-draft.create`；descriptor把该方法声明为写风险、无表和资源逃逸参数，并绑定`host:idc:catalog.provider-draft.create`方法级能力。`bridge`子包提供显式 transport 注入和 WASI guest client。核心内部事务服务实现公共契约并提供严格 caller invoker；`idc-resource`把已验证的完整计划快照映射为同一公共来源类型。

本记录冻结的是`4.14.c.1`完成时的契约状态；当时没有在`backend/plugin.go`注册 descriptor，也没有源码插件运行期调用、计划任务、路由或后台按钮。当前工作区后续已推进`4.14.c.2`的部分运行时装配，最新状态和验证证据见[第四阶段商品草稿能力运行时验证记录](phase-4-catalog-capability-runtime-validation.md)。本记录的任务级结论不等同于运行时子任务已经完成。

## 契约不变量

- 能力身份固定为`owner=idc-core`、`service=catalog`、`version=v1`、`method=provider-draft.create`，依赖声明必须精确匹配且不能重复方法。
- 公共请求只包含内部快照号、指纹、观察与完成时间，以及摘要键和安全标签；项目数上限`20,000`，动态命令总载荷上限`32 MiB`。
- bridge 固定传递空`resourceRef`和空`table`；owner invoker再次拒绝非空资源、表、错误 owner/service/version/method，以及任何不是`idc-resource`的调用插件。
- bridge 在调用前验证请求、调用后验证响应；`published`不属于允许返回状态。核心实现仍只调用既有`pending_review`事务，不增加接受、物化或发布路径。
- `resource`只导入`core/backend/cap/catalogcap`，没有导入其他插件的`internal`、DAO、DO 或 Entity；安全来源仍由资源拥有方原有两次有界查询产生。

## 验证证据

| 命令或检查 | 结果 |
|---|---|
| `go test ./backend/cap/catalogcap/... ./backend/internal/service/catalogdraft -count=20 -shuffle=on` | 通过；覆盖 owner/version/method、写风险、动态 envelope、请求/响应失败关闭和非资源插件拒绝 |
| `TestResourceFoundationMigrationPostgreSQL -count=20 -shuffle=on` | 通过；覆盖真实完整快照到公共契约的快照号、指纹、摘要键和安全字段一致性 |
| 两个插件分别执行`GOWORK=off go test ./backend/... -count=1` | 通过；核心与资源全部后端包回归通过 |
| 两个插件分别执行`make.cmd lint.go dir=... plugins=1` | 通过；`golangci-lint`与`staticcheck U1000`均为 0 问题 |
| `make.cmd plugins.check` | 通过；扫描 584 个文件，0 项插件治理问题 |
| `openspec validate add-idc-server-operations-platform --strict` | 通过 |

## 影响分析

- 架构与插件：能力属于`idc-core`拥有的商品域写契约，源码调用依赖同一宽`catalogcap.Service`，动态调用依赖 owner-aware descriptor 和 bridge SDK。本记录只评价契约阶段；后续运行时实现不得改变这里冻结的 owner、方法、风险和载荷不变量。
- DI：bridge transport、公共 Service 和 owner invoker均通过构造函数显式注入。后续启动装配状态单独记录，仍禁止请求内临时`New()`或全局服务定位。
- 数据权限：descriptor声明精确方法能力，invoker只允许资源插件身份；生产注册、系统审计和来源工厂的后续状态见运行时验证记录，资源自动发布和停用生命周期仍不得从本契约完成状态推断。
- 性能：转换为纯内存单次线性映射；数据库读取仍固定两次，核心重放仍为有界集合查询。动态请求与 owner invoker均有`32 MiB`硬上限。
- 缓存：无影响，没有缓存、订阅、失效或本地派生状态。
- API、前端、E2E 与`i18n`：没有 HTTP DTO、路由、页面、用户可见文案、字典或错误资源变化；技术契约错误只在插件调用边界返回，因此未触发 E2E 和仓库`i18n`门禁。
- 模块依赖：`catalogcap/bridge`复用公开`pluginbridge`传输，`go mod tidy`仅补入其现有 Protobuf 编码依赖，没有修改宿主或开发工具。

## Lina 审查报告

**变更：** `add-idc-server-operations-platform`
**范围：** 任务级，任务`4.14.c.1`
**审查文件数：** 16
**范围来源：** 工作区状态、未跟踪插件目录展开、任务上下文和 OpenSpec 状态
**已读取规则文件：** `AGENTS.md`、`.agents/rules/openspec.md`、`.agents/rules/documentation.md`、`.agents/rules/architecture.md`、`.agents/rules/data-permission.md`、`.agents/rules/plugin.md`、`.agents/rules/backend-go.md`、`.agents/rules/testing.md`、`.agents/rules/i18n.md`

### 发现的问题

未发现阻塞问题。

审查过程中发现并已修复一项容量问题：最初只有项目数量与单字段上限，没有限制 20,000 项合并后的动态调用载荷。当前 bridge 请求和响应、owner invoker输入同时使用`32 MiB`硬上限。

### 规则域结论

- OpenSpec 与文档：通过。只标记契约子任务完成，运行期接线父任务保持未完成；严格校验通过。
- 架构与插件：通过。契约 owner 明确，source 与 dynamic 两种消费形态齐备，资源插件只依赖公开`backend/cap`；两个插件根目录均不存在本地`AGENTS.md`。
- 后端 Go 与 DI：通过。构造依赖显式，未在请求路径创建服务，公共契约不泄漏内部模型；两个插件 lint 和全后端测试通过。
- 数据权限：通过当前未注册边界评估。动态 invoker精确限制调用插件和方法，但生产权限、审计与停用门禁仍由未完成子任务承接。
- 测试与性能：通过。契约测试覆盖成功和失败路径；真实 PostgreSQL覆盖资源映射；循环只做内存转换，没有`N+1`。
- API、前端、E2E、缓存和`i18n`：无影响，原因见影响分析。
- 开发工具：无影响；模块依赖补全不是工具链变更。

### 摘要

- 严重：0。
- 警告：0。
- 剩余风险：本记录不评价后续运行时装配是否完整；`4.14.c.2`与`4.14.c.3`必须以独立验证记录和任务状态为准。
