# 第四阶段 Provider 草稿来源投影验证记录

> **`FB-52`失效声明：**把 Provider 计划快照投影为商品草稿来源的方向已经废止。本记录仅用于删除旧接缝、旧测试和开发数据迁移对账，不能作为当前阶段完成证据。服务器插件可以保留供应商容量的内部安全投影，但该投影不得进入通用商品、库存、报价、结账或商品发布。

## 完成范围

本轮完成任务`4.14.a`。`idc-resource`库存快照服务新增内部只读`ReadPlanCatalogSource`，把完整计划快照转换为后续商品草稿命令可消费的安全来源投影。该服务不创建商品、不自动发布，也不向其他插件暴露资源插件的 DAO、Entity 或原始 Provider 投影。

## 安全与性能不变量

- 只接受格式正确且状态为`complete`或`superseded`的`plan`快照；`collecting`、`failed`、空快照、项目数不一致、无完成时间或错误指纹均失败关闭。
- 内部来源保留快照号、完整指纹、观察时间和完成时间。上游`external_ref`只与快照指纹共同生成固定长度摘要草稿键，不出现在返回对象。
- 安全投影只保留有长度上限的来源范围、区域名称、节点名称、节点类型、资源名称和资源类型。`area_id`、`node_id`、`store_id`、状态、余量、成本、凭据和原始 JSON 均不返回。
- 每次读取固定执行一次快照头查询和一次按上游引用稳定排序的项目集合查询；项目上限为 20,000，单项原始投影上限为 16 KiB，不在项目循环中访问数据库。
- 对同一不可变来源快照重复读取返回相同顺序、相同摘要草稿键和相同来源指纹，供后续草稿导入实现幂等重放。

## 验证证据

| 命令或检查 | 结果 |
|---|---|
| `TestResourceFoundationMigrationPostgreSQL -count=20 -shuffle=on` | 通过；覆盖完整计划快照、安全字段投影、来源指纹、摘要键确定性、敏感字段剥离和 collecting 拒绝 |
| `GOWORK=off go test ./backend/... -count=1 -shuffle=on` | 通过；资源插件全部后端包回归通过 |
| `make.cmd lint.go dir=apps/lina-plugins/idc-resource plugins=1` | 通过；`golangci-lint`与`staticcheck U1000`均为 0 问题 |
| `make.cmd plugins.check` | 通过；扫描 566 个文件，0 项插件治理问题 |

## 影响分析

- 架构与插件：安全投影仍是资源拥有方内部服务契约，没有把`resourcecap`或动态插件协议提前扩张，也没有跨插件导入内部实现。下一步必须通过 owner-aware 公开契约接入`core`，不得让`core`直接访问资源表。
- 数据权限：本轮没有 API、用户输入编号入口或后台动作；调用者只可能由后续内部草稿编排显式注入。跨插件命令接入时仍需独立权限、目标范围、审计和来源归属校验。
- 数据库与性能：没有 SQL 或生成模型变更；只复用既有索引化快照头和项目表，一次读取的数据库访问次数不随项目数增长。
- 缓存与 DI：没有缓存、失效、订阅、定时任务或新运行期依赖；服务继续复用构造期注入的同一个`gdb.DB`。
- API、前端、E2E 与`i18n`：没有 DTO、路由、页面、用户可见行为、字典、错误资源或文案变化，因此`i18n`无影响，也不触发 E2E 质量审查和截图验证。
- 开发工具与跨平台：没有修改脚本、构建、CI 或工具配置；验证分别使用现有 Windows 治理入口和 Go 工具链。

## 后续边界

任务`4.14`仍未完成。下一步由`idc-core`新增草稿命令和事务持久化，把安全来源映射为待审核商品草稿；必须实现来源重放幂等、人工补齐检查、权限与审计，并通过受治理的 owner-aware 契约完成跨插件装配。任何输入都不得绕过价格、税务、履约和风险缺失时的发布失败门禁。

## Lina 审查报告

**变更：** `add-idc-server-operations-platform`
**范围：** 任务级，任务`4.14.a`
**审查文件数：** 7
**范围来源：** `git status --short`、`git ls-files --others --exclude-standard`、任务上下文和 OpenSpec 状态
**已读取规则文件：** `AGENTS.md`、`.agents/rules/openspec.md`、`.agents/rules/documentation.md`、`.agents/rules/architecture.md`、`.agents/rules/data-permission.md`、`.agents/rules/plugin.md`、`.agents/rules/backend-go.md`、`.agents/rules/testing.md`、`.agents/rules/i18n.md`

### 发现的问题

未发现阻塞问题。

审查过程中发现并已修复三项任务内问题：`Service`接口方法缺少逐方法边界注释；来源范围使用裸枚举字符串；持久化投影读取没有再次调用共享凭据扫描器。修复后新增数据库篡改负例，并重新运行 20 轮真实 PostgreSQL 测试、全量 Go 回归和 lint。

### 规则域结论

- OpenSpec 与文档：通过。子任务只声称安全来源投影完成，父任务与核心草稿事务保持未完成；严格校验通过。
- 架构与插件：通过。实现仍封装在`idc-resource/internal/service`，没有形成未治理的公共能力或跨插件内部调用；插件根目录不存在本地`AGENTS.md`。
- 后端 Go：通过。依赖仍是构造期注入并复用的同一`gdb.DB`，没有新`New()`、运行时服务图、缓存或启动装配；DO 表达查询条件，单列排序使用`OrderAsc`。
- 性能：通过。固定两次数据库查询，循环只做内存投影；头查询只选 7 列，项目查询只选 3 列并有 20,001 行硬上限，不存在`N+1`。
- 数据权限：通过当前内部边界评估。本轮没有 API、用户动作或跨插件调用，只读取固定平台租户；4.14.b 接线前必须新增明确权限、来源归属和审计，不能直接暴露该内部服务。
- 测试：通过。真实 PostgreSQL 独立 schema 覆盖成功、确定性、敏感字段剥离、`collecting`拒绝和凭据篡改拒绝；测试结束删除隔离 schema，不依赖其他测试方法。
- API、前端与 E2E：未命中。没有 DTO、路由、页面或用户可观察工作流，因此未触发 E2E 质量审查与截图验证。
- `i18n`：无影响。没有用户文案、字典、错误资源、API 文档或语言包变化，因此本子任务不要求运行仓库`i18n`门禁。
- 缓存、SQL 和开发工具：无影响。没有缓存、迁移、生成模型、脚本或工具配置变化。

### 验证证据

- `TestResourceFoundationMigrationPostgreSQL -count=20 -shuffle=on`：通过。
- `GOWORK=off go test ./backend/... -count=1 -shuffle=on`：通过。
- `make.cmd lint.go dir=apps/lina-plugins/idc-resource plugins=1`：通过，0 问题。
- `make.cmd plugins.check`：通过，扫描 566 个文件，0 问题。
- `openspec validate add-idc-server-operations-platform --strict`：通过。

### 摘要

- 严重：0。
- 警告：0。
- 剩余风险：尚未建立跨插件公开契约、核心草稿事务、权限审计和人工补齐门禁；这些均由未完成的`4.14.b`承接，不影响本内部安全投影子任务完成。
