# 第四阶段商品草稿持久化发布执行器验证记录

> **`FB-52`失效声明：**从 Provider 快照读取候选并调用`CreateProviderDraft`的执行器已经废止。本记录只用于删除旧作业、旧接缝和开发数据迁移对账，不能作为当前商品发布证据。

## 实现范围

本轮完成`4.14.c.3.b`。`idc-resource`新增内部`catalogpublication.Service`，每次`RunOnce`最多领取一条`pending/retry_wait`请求。领取、过期回收、完成、重试和死信均使用 PostgreSQL 时间、`FOR UPDATE SKIP LOCKED`和随机租约令牌；每次转换在同一资源事务追加不可变事件。

领取事务提交后，执行器调用已注入的`inventorysnapshot.Service.ReadProviderDraftSource`。该读取固定执行一次快照头查询和一次有界候选集合查询，最多返回 20,000 个安全候选，不按候选项查询。随后执行器在所有资源事务之外调用已注入的`catalogcap.Service.CreateProviderDraft`，没有跨插件数据库事务或同步站内`HTTP`。

目录调用失败被视为结果未知，资源侧只保存`CATALOG_RESULT_UNKNOWN`机器码并延后同一不可变来源重试。来源读取失败保存`SOURCE_READ_FAILED`；来源结构不合法进入`SOURCE_INVALID`死信；非法目录响应保存`CATALOG_RESPONSE_INVALID`并重试。任何原始错误、响应正文、秘密或自由文本都不会进入请求或事件历史。

成功完成要求请求号、`leased`状态、尝试号、随机令牌和未过期租约全部匹配。事件只保存租约令牌的`SHA-256`摘要。尝试预算耗尽或过期租约达到上限时进入`dead_letter`；未耗尽的过期租约先记录`LEASE_EXPIRED`重试事件，再允许同一来源重新领取。

该子任务交付时没有注册`pluginhost.Jobs`，也没有自行启动 goroutine。后续`4.14.c.3.c`现已完成生产作业装配、停用门禁和恢复续跑实现，当前状态见[第四阶段商品草稿发布作业生命周期验证记录](phase-4-catalog-publication-job-lifecycle-validation.md)。

## 前置缺陷修复

真实连续快照测试发现，`005-resource-catalog-draft-publication.sql`原先把发布请求外键固定为`plan + complete`，会阻止下一份完整计划快照把旧快照改为`superseded`，与旧不可变来源可重放的设计冲突。

迁移现把请求来源约束调整为`plan + complete/superseded`，外键增加`ON UPDATE CASCADE`。请求登记仍只发生在终态`complete`事务；后续替换只同步来源状态，不修改快照项目、指纹或请求身份。迁移重复执行会幂等升级旧约束，连续快照与旧来源精确重放已进入真实 PostgreSQL 回归。

## 验证证据

| 命令或检查 | 结果 |
|---|---|
| `LINAPRO_IDC_RESOURCE_TEST_DSN=... GOWORK=off go test ./backend -run TestResourceFoundationMigrationPostgreSQL -count=20 -shuffle=on` | 通过；覆盖迁移重复执行、连续计划快照、成功投递、超时未知、同源重放、租约过期拒绝完成、过期恢复、并发单领、尝试耗尽死信和事件顺序 |
| `GOWORK=off go test ./backend/... -count=20 -shuffle=on` | 通过；资源插件后端全量高重复回归通过 |
| `make.cmd lint.go dir=apps/lina-plugins/idc-resource plugins=1` | 通过；`golangci-lint`和`staticcheck U1000`均为 0 问题 |
| `make.cmd plugins.check` | 通过；扫描 598 个文件，0 项插件治理问题 |
| `openspec validate add-idc-server-operations-platform --strict` | 通过 |
| `make.cmd i18n.check` | 本次运行时扫描与消息覆盖通过；仓库前端键检查仍仅被宿主通知预览页 6 个既有缺失键阻断 |

## 影响分析

- 架构与依赖注入：新增依赖 owner 为`idc-resource`库存服务和`idc-core catalogcap`。`New`只接收两个显式接口；后续`4.14.c.3.c`已在唯一`pluginhost.Jobs`回调中懒加载启动期声明的`catalogcap.Service`并注册生产处理器。
- SQL：只升级发布请求到快照的复合外键和来源检查约束；没有新增表、字段或用户数据回填。迁移重复执行两次通过，旧请求状态随父快照安全级联。
- 性能：每次最多领取一条。一次空领取最多回收一条过期租约并查询一条候选；一次执行的来源读取固定两次查询，事件和状态转换为常数次数据库操作，不随候选数产生`N+1`。
- 数据权限：没有`HTTP API`、用户输入对象编号、列表或下载入口。执行器只处理`tenant_id=0`平台内部请求，来源身份由本地外键和固定请求号证明；生产模块启停授权留到作业装配层。
- 缓存：无影响。请求、租约、事件和目录草稿结果均以数据库与核心幂等契约为权威，没有新增缓存或失效路径。
- `i18n`：无新增用户可见文案、接口文档源文本、插件清单、菜单或语言资源。运行时扫描和消息覆盖通过；仓库失败仅来自禁止在本变更修改的宿主页面既有缺失键。
- API、前端与 E2E：无影响。没有路由、DTO、页面或用户可观察交互，因此不需要 Chrome 或 Playwright 验证。
- 开发工具与跨平台：无脚本、构建入口或开发工具变更；验证命令在 Windows PowerShell 执行，生产代码不增加平台相关路径或系统调用。

## Lina 审查报告

**变更：**`add-idc-server-operations-platform`
**范围：**任务级，`4.14.c.3.b`
**已读取规则：** `AGENTS.md`、`.agents/rules/openspec.md`、`.agents/rules/documentation.md`、`.agents/rules/architecture.md`、`.agents/rules/data-permission.md`、`.agents/rules/plugin.md`、`.agents/rules/backend-go.md`、`.agents/rules/database.md`、`.agents/rules/testing.md`、`.agents/rules/i18n.md`

### 审查结论

- 严重：0。
- 警告：0。
- 状态机：只有带完整活动租约的请求可以完成、重试或死信；到期完成失败且不覆盖后续 owner。
- 未知结果：目录调用失败不保存原文、不改写来源、不创建新幂等键，只调度同源重放；测试 owner 验证重放载荷保持一致。
- 并发与性能：`SKIP LOCKED + Limit(1)`在两个并发 worker 下只产生一次目录调用；来源读取保持固定两次集合查询。
- 生产边界：该子任务自身没有作业注册、根 worker 或无托管 goroutine；后续生产接线仍复用同一执行器，没有绕过租约状态机。
- 外部阻断：完整`i18n`仍仅被禁止修改的`apps/lina-vben`宿主通知预览页 6 个既有缺失键阻断，不影响本任务无文案的实现判断，也不授权越界修改宿主。
