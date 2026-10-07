# 第四阶段商品草稿持久化发布请求验证记录

> **`FB-52`失效声明：**Provider 容量快照登记商品草稿发布请求的方向已经废止。本记录只用于清理旧请求表、接口、作业和开发数据迁移对账，不能作为当前商品模型完成证据。

## 实现范围

本轮完成`4.14.c.3.a`的代码、SQL 和真实 PostgreSQL 验证，但因仓库级`i18n`外部阻断不勾选任务。

新增`plugin_idc_resource_catalog_draft_publication`请求表和`plugin_idc_resource_catalog_draft_publication_event`追加式事件表，保存一个不可变终态计划库存快照对应的一条本地发布请求及状态转换历史。数据库复合外键要求来源始终为`inventory_kind=plan`且当前状态为`complete/superseded`；请求初建时只能来自`complete`终态，后续新快照发布会以`ON UPDATE CASCADE`把旧请求的来源状态同步为`superseded`，保持旧不可变来源可重放。请求号和快照号格式、唯一来源、状态、尝试预算、租约字段和成功结果字段均有检查约束。领取索引只覆盖`pending/retry_wait`，租约回收索引只覆盖`leased`，不会扫描已投递历史。

`inventorysnapshot.Stage`只在终态`plan`页更新成功后，在同一个 PostgreSQL 事务插入`pending`请求和首条`enqueued`事件。普通区域、线路、镜像和库存快照不会入队；任一插入失败会令整个终态提交回滚。事件形状约束禁止把自由文本错误、原始响应或明文租约令牌写入历史，后续只允许记录租约令牌摘要。事务不调用`catalogcap`，因此不会把跨插件不确定性带入库存发布。

迁移同时登记`idc_resource_catalog_publication_status`字典，覆盖`pending`、`leased`、`retry_wait`、`delivered`和`dead_letter`，资源插件中英文运行时资源按宿主字典键约定补齐类型名称、说明与五个状态标签。隔离迁移测试没有宿主字典表时安全跳过种子；真实插件安装存在宿主字典表时幂等写入。当前没有管理页面或 HTTP API，字典尚不形成用户可见交互。

## 未实现范围

- 领取、完成、重试、死信与单次执行器已经由`4.14.c.3.b`补齐；本记录只证明请求登记数据底座。
- 没有`pluginhost.Jobs`生产 worker，也没有模块停用、在途排空和恢复续跑装配。
- 没有跨插件调用，因此没有超时未知结果；后续只能以同一不可变快照重试并依赖核心精确幂等收敛。

## 验证证据

| 命令或检查 | 结果 |
|---|---|
| `LINAPRO_IDC_RESOURCE_CODEGEN_DSN=... go test ./backend -run '^TestResourceFoundationCodegenPostgreSQL$' -count=1` | 通过；只允许名为`linapro_idc_codegen`的专用数据库，重建 16 张资源表、重复执行全部迁移，并在宿主字典表存在时断言 5 个状态种子齐全 |
| `make.cmd dao dir=apps/lina-plugins/idc-resource/backend` | 通过；生成发布请求 DAO、DO、Entity，未手改生成文件 |
| `LINAPRO_IDC_RESOURCE_TEST_DSN=... go test ./backend -run '^TestResourceFoundationMigrationPostgreSQL$' -count=20 -shuffle=on` | 通过；覆盖迁移幂等、终态计划请求与首条事件原子写入、非计划不入队、连续计划快照替换、旧来源状态级联与精确重放、数据库拒绝错误来源、自由文本错误码和安全读取 |
| `GOWORK=off go test ./backend/... -count=1` | 通过；资源插件全部后端包回归通过 |
| `make.cmd lint.go dir=apps/lina-plugins/idc-resource plugins=1` | 通过；`golangci-lint`与`staticcheck U1000`均为 0 问题 |
| `make.cmd plugins.check` | 通过；扫描 594 个文件，0 项插件治理问题 |
| `make.cmd i18n.check` | 本次插件运行时扫描与消息覆盖通过；仓库前端键检查被宿主通知预览页 6 个既有缺失键阻断 |

## 影响分析

- SQL：新增 2 张表、1 个父表复合唯一约束、3 个索引和 1 组状态字典；迁移可重复执行。没有删除、改写或回填既有库存数据。
- 架构：发布请求由库存真相拥有方`resource`持久化；跨插件命令仍由`core`拥有。当前局部直接插入即可保证终态原子性，没有新增多余接口或全局定位器。
- 数据权限：没有 HTTP、列表、详情或执行入口；固定单安装实例的`tenant_id=0`，来源由复合外键锁定为完整计划快照。后续 worker 只能读取平台级待办，不接受用户传入资源范围。
- 性能：每个终态计划快照额外固定 2 次插入；唯一约束和部分索引支持幂等来源、有界领取和按请求读取事件历史，没有按候选项逐行查询或写入。
- 缓存：无影响，没有缓存、订阅或失效状态。
- API、前端与 E2E：无影响，没有新增路由、DTO、页面或用户可观察行为，未触发浏览器验证。
- `i18n`：新增内部状态字典并补齐中英文类型名称、说明和五个状态标签，插件运行时扫描和消息覆盖通过；仓库门禁失败仅来自禁止修改的宿主通知预览页既有缺失键，因此本任务保持未勾选。

## Lina 审查报告

**变更：**`add-idc-server-operations-platform`
**范围：**任务级，`4.14.c.3.a`
**已读取规则：** `AGENTS.md`、`.agents/rules/openspec.md`、`.agents/rules/documentation.md`、`.agents/rules/architecture.md`、`.agents/rules/data-permission.md`、`.agents/rules/plugin.md`、`.agents/rules/backend-go.md`、`.agents/rules/database.md`、`.agents/rules/testing.md`、`.agents/rules/i18n.md`

### 审查结论

- 严重：0。
- 警告：0。
- SQL 幂等性与分类：DDL、索引、约束和字典种子均幂等；属于全新结构与内建枚举种子，不包含用户数据、凭据、成本或上游原始响应。
- 已修复风险：初版表只按`snapshot_no`关联，无法由数据库证明来源是终态计划快照；现改为`snapshot_no + inventory_kind + status_code`复合外键，并补非计划来源拒绝测试。后续执行器回归又发现 complete-only 外键会阻止新计划快照把旧快照转为`superseded`；现允许`complete/superseded`并使用`ON UPDATE CASCADE`保持父子状态一致，连续快照和旧来源重放已纳入 20 轮真实 PostgreSQL 回归。事件租约摘要最初使用`CHAR(64)`导致空值读回被空格填充，现改为`VARCHAR(64)`。错误字段最初只有长度边界，现由数据库限制为大写机器码并验证拒绝自由文本。
- 外部阻断：宿主`notice-preview-modal.vue`引用的 3 个键在中英文各缺失一次。根目录无`.contributing`，本任务禁止修改`apps/lina-vben`，因此不能越权清除该门禁。
- 剩余工作：生产`pluginhost.Jobs`生命周期、模块停用、在途排空与恢复续跑由`4.14.c.3.c`继续承接。
