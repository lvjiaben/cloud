# 第四阶段商品目录数据基础验证记录

> **`FB-52`失效声明：**本记录把服务器分类、固定配置、资源池绑定和 Provider 库存写成通用商品数据底座，原结论已经失效，不能作为当前目录完成证据。分类只由商家维护且不决定商品种类；通用商品只保留资料、版本、不透明插件配置、计费价格、简单库存和一个接口实例绑定。下文仅用于旧表、旧字段和开发数据迁移对账。

## 完成范围

本轮完成任务`4.1.a`的实现与任务级验证，在`idc-core`建立商品目录权威数据底座：服务器分类、商品、发布版本、配置组、配置项、兼容规则、计费规则、逐币种价格、资源池绑定、本地库存快照和成本快照共 11 张表。分类和商品身份支持可恢复软删除；已发布版本、库存和成本使用显式生命周期或不可变历史。由于下述仓库级强制门禁仍失败，任务清单暂不勾选。

父任务`4.1`暂不勾选。实现和本轮新增的中英文字典已经通过针对性验证，但仓库级`i18n`门禁仍被禁止修改的宿主通知预览页 6 个既有缺失键阻断；该阻碍与商品目录变更无关，待宿主拥有方修复后再完成父任务。

## 数据与性能不变量

- 首版商品类型只允许`cloud_server`和`bare_metal`；`IP`、磁盘、备份、镜像、流量和带宽是配置组件，不是独立商品。
- 发布版本同一时刻最多一个；分类父级、商品当前版本、配置项价格等可空关系使用真实外键，不使用空字符串逃避引用完整性。商品当前版本、配置组与配置项、兼容规则两端、计费规则和配置项价格还使用带版本号的复合外键，数据库会拒绝跨商品或跨版本拼接。
- 价格按币种保存最小货币单位整数，币种只接受三位大写代码；基础价和配置项价格使用分层部分唯一索引，禁止同一计费维度重复价格。
- 商品只保存资源拥有方的不透明资源池编号，不建立跨插件数据库外键。Provider 原始响应和凭据禁止进入配置投影；成本与内部编号不属于公开目录字段。
- 报价、商品列表、版本配置、资源池路由、最新库存和最新成本均具备匹配过滤与排序路径的组合索引。真实 PostgreSQL 在禁用顺序扫描后证明逐币种报价查询使用`idx_idc_catalog_price_quote`，数据装配不要求逐项查库或访问上游。
- 只有分类和商品两张可恢复配置表使用`deleted_at`；版本、价格、绑定、库存和成本不使用软删除抹去历史。

## 验证证据

| 命令或检查 | 结果 |
|---|---|
| 商品迁移对代码生成数据库连续重放两次 | 通过；DDL 与 Seed DML 幂等 |
| `TestCatalogFoundationMigrationPostgreSQL -count=20 -shuffle=on` | 通过；每轮使用独立 schema，覆盖 11 表、15 类字典、外键、发布唯一性、跨版本兼容规则/价格/当前版本拒绝、币种、价格维度、库存数量、成本来源和报价索引 |
| `GOWORK=off go test ./backend/... -count=1 -shuffle=on` | 通过；`idc-core`全部后端包回归通过 |
| `make.cmd dao dir=apps/lina-plugins/idc-core/backend` | 通过；11 组 DAO、DO、Entity由项目生成器维护，可空字段使用显式指针映射 |
| `make.cmd lint.go dir=apps/lina-plugins/idc-core plugins=1` | 通过；`golangci-lint`与`staticcheck U1000`均为 0 问题 |
| `make.cmd plugins.check` | 通过；扫描 11 个插件清单和 565 个文件，0 项治理问题 |
| 中英文`dict.json`解析与本轮字典消息覆盖 | 通过；JSON 有效，本轮 15 类字典双语键完整 |
| `make.cmd i18n.check` | 本轮资源通过，仓库门禁失败；仅报告宿主`notice-preview-modal.vue`既有 6 个缺失键，本任务无权修改宿主 |

## 影响分析

- 架构：数据闭环在`idc-core`商品子域，没有修改框架宿主或跨插件内部实现。资源池引用遵守唯一拥有方边界。
- 数据权限：本轮只有固定`tenant 0`的数据模型和内部迁移测试，没有新增 API、列表、动作或用户输入编号入口；后续服务和 API 仍需单独接入权限、数据范围和审计。
- 缓存：没有新增缓存或失效路径。草稿和权威状态仍以 PostgreSQL 为准；只有后续已发布公开商品可进入可重建版本缓存。
- API、前端与 E2E：没有新增路由、DTO、页面或用户交互，因此不触发 E2E；字典资源为后续双语页面准备，但当前没有运行时页面引用。
- SQL：`010-catalog-foundation.sql`是当前迭代唯一文件，只含幂等 DDL 和必需 Seed DML，无 Mock、无手写自增主键。代码生成数据库清理测试只允许数据库名精确等于`linapro_idc_codegen`且只删除明确列出的商品表。
- DI 与开发工具：没有新增运行期依赖、服务构造或启动装配，也没有修改脚本、构建、CI 或跨平台入口；只复用现有`make.cmd dao`和 lint 入口。

## 后续边界

本阶段只建立权威数据与性能底座，不把任何商品自动发布。下一步实现完整 Provider 快照的只读商品规格投影和待审核草稿生成；销售文案、显式价格、税费、履约和风险规则未补齐前必须失败关闭。

## Lina 审查报告

**变更：** `add-idc-server-operations-platform`
**范围：** 任务级，任务`4.1.a`
**审查文件数：** 55
**范围来源：** `git status --short`、`git ls-files --others --exclude-standard`、任务上下文和本轮`make dao`生成清单
**已读取规则文件：** `AGENTS.md`、`.agents/rules/openspec.md`、`.agents/rules/documentation.md`、`.agents/rules/architecture.md`、`.agents/rules/data-permission.md`、`.agents/rules/plugin.md`、`.agents/rules/backend-go.md`、`.agents/rules/database.md`、`.agents/rules/testing.md`、`.agents/rules/i18n.md`

### 发现的问题

- **严重** `apps/lina-vben/apps/web-antd/src/views/system/message/notice-preview-modal.vue:16`：仓库级`make.cmd i18n.check`被该禁止修改宿主页及同文件第 42、45 行引用的 3 个键在中英文资源中同时缺失所阻断，共 6 项。该问题不由本任务引入，本轮插件消息覆盖和运行时扫描均通过，但`.agents/rules/i18n.md`和`lina-review`要求当前工作区强制门禁通过，因此任务`4.1.a`不能标记完成。

审查期间另发现并已修复两项任务内问题：测试失败分支忽略数据库关闭错误；新增中文 fixture 发生代码页乱码。README 中“发布后不可变”的超前承诺也已改为与当前显式生命周期实现一致。修复后的 20 轮数据库测试、全量插件测试和 Go lint 均重新通过。

### 规则域结论

- OpenSpec 与文档：任务状态没有越过失败门禁，中英文 README 镜像同步，严格校验通过。
- 架构与插件：通过。商品权威数据闭环在`idc-core`，没有修改宿主，没有跨插件数据库外键；插件根目录不存在本地`AGENTS.md`。
- 数据库：通过。当前迭代单一`010`文件只含幂等 DDL 和 Seed DML，不写自增主键；11 张表、复合外键、软删除边界和高频索引均由真实 PostgreSQL 验证。
- 后端 Go 与测试：通过。44 个 DAO/DO/Entity 文件均由`make dao`生成；测试按独立 schema 自包含清理，成功和关键拒绝路径均有覆盖。
- 数据权限：无运行时影响。本轮没有 API、服务或用户可输入业务编号入口；固定`tenant 0`模型不构成权限绕过。
- 缓存一致性：无影响。本轮未新增缓存、失效、订阅或跨实例派生状态。
- API、前端与 E2E：未命中。没有新增 DTO、路由、页面或用户可观察工作流，因此未触发 E2E 质量审查与截图验证。
- 开发工具与跨平台：无影响。没有修改根工具或脚本，只调用现有 Windows 入口；插件`hack/config.yaml`只维护生成器表清单和可空字段映射。
- `i18n`：任务资源通过，仓库强制门禁发现 1 个外部严重问题，阻塞任务勾选。

### 验证证据

- `TestCatalogFoundationMigrationPostgreSQL -count=20 -shuffle=on`：通过。
- `GOWORK=off go test ./backend/... -count=1 -shuffle=on`：通过。
- `make.cmd lint.go dir=apps/lina-plugins/idc-core plugins=1`：通过，0 问题。
- `make.cmd plugins.check`：通过，扫描 565 个文件，0 问题。
- `openspec validate add-idc-server-operations-platform --strict`：通过。
- `make.cmd i18n.check`：失败；任务内扫描与消息覆盖通过，宿主前端既有 6 个缺失键阻断。

### 摘要

- 严重：1 个外部门禁问题。
- 警告：0。
- 结论：任务实现未发现剩余阻塞问题，但强制仓库门禁未通过，任务保持未完成；继续后续实现不代表豁免该门禁。
