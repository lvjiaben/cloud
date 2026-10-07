# 第 10 阶段电子合同管理页面验证记录

## 记录范围

- 日期：2026-08-18
- 对应任务：`10.12.f`
- 本记录验证合规插件自己的合同列表、详情、人工可选合同发起、提交签署、重新发起、解除和文件证据状态管理页面。
- 页面、接口客户端、双语资源和插件专属测试均位于`apps/lina-plugins/idc-compliance/`；`apps/lina-vben/apps/web-antd/`状态为空。
- 本任务不证明发票与红字页面`10.12.g`、整体验收`10.12.h`、父任务`10.12`或完整电子合同 E2E 任务`10.17`完成。

## 问题原因与修正边界

现有电子合同后端已经具备发起、提交签署、重新发起、解除和证据归档能力，但缺少供购买方后台人员使用的安全合同目录、详情读取接口和完整管理页面。历史旧 Portal 的页面与测试不能证明当前插件前端交付完成，也不能把业务代码写入 Lina 官方管理后台宿主`web-antd`。

本轮把后台业务页面继续闭环在`idc-compliance/frontend/`，补齐服务端安全读取接口、独立精确权限和查询索引。代码生成工具按接口生成时曾在插件现有组合式控制器中追加重复构造器和未实现占位文件；这些重复内容已清理，最终控制器继续使用插件已有装配方式，没有修改宿主来绕过插件独立构建。

## 当前实现

| 能力 | 当前实现与边界 |
| --- | --- |
| 合同目录 | 使用默认`20`条、最多`100`条的服务端游标分页；支持合同号、客户号、订单号和稳定状态筛选，不执行精确总数统计 |
| 安全详情 | 固定上限读取签署流程、权威事件、文件、归档任务和替换关系；超出上限时明确显示历史被截断，不返回私有存储地址、供应商标识、签署链接或合同正文 |
| 人工可选合同 | 后台可为符合条件的订单选择“不需要合同”或先生成不可变预览；不签合同不是强制项，也不会改变订单开通门槛；生成预览与提交腾讯签署是两个独立动作 |
| 重新发起 | 内容不变时在原合同上追加新的七天签署流程；内容、模板或签署人变化时先生成有关联关系的新合同预览，核对后再单独提交，原合同和原证据继续保留 |
| 解除合同 | 仅已完成合同可操作；必须填写原因，可选条款，固定沿用原签署双方，不要求最近五分钟重新登录，提交前再次确认且不接受浏览器指定生效时间 |
| 文件证据 | 页面展示不可变文件元数据、收集状态和归档任务；未购买付费报告时明确说明原因，不把“未购买”显示成系统失败 |
| 权限 | 查看、人工发起、重新发起和解除分别使用`query`、`create`、`reissue`、`terminate`精确权限；没有查看权限时合同页签完全隐藏 |
| 插件装配 | 页面、接口客户端、状态和双语资源均由合规插件独立产物提供；`web-antd`没有 IDC 页面、依赖、路由或语言包改动 |

任务直接复核文件包括：

- `backend/api/admin/v1/electronic_contract_list.go`
- `backend/api/admin/v1/electronic_contract_get.go`
- `backend/api/admin/v1/electronic_contract_replacement_option_list.go`
- `backend/internal/controller/admin/admin_v1_electronic_contract_list.go`
- `backend/internal/controller/admin/admin_v1_electronic_contract_get.go`
- `backend/internal/controller/admin/admin_v1_electronic_contract_replacement_option_list.go`
- `backend/internal/service/electroniccontractpreparation/electroniccontractpreparation_management.go`
- `backend/internal/service/electroniccontractpreparation/electroniccontractpreparation_operations.go`
- `backend/internal/service/contractsigner/contractsigner_directory.go`
- `frontend/pages/electronic-contract-management-client.ts`
- `frontend/pages/components/electronic-contract-manager.vue`
- `frontend/pages/electronic-contract.vue`
- `manifest/i18n/zh-CN/electronic-contract.json`
- `manifest/i18n/en-US/electronic-contract.json`
- `manifest/sql/022-electronic-contract-management-directory.sql`
- `hack/tests/e2e/TC004-electronic-contract.ts`
- `hack/tests/pages/ElectronicContractPage.ts`
- `hack/tests/support/electronic-contract-mock.ts`

## 性能与数据边界

- 合同目录使用基于`created_at`和`id`的稳定游标，不读取全表总数。
- 目录通过一次有界查询连接模板信息，不在返回行循环中逐条查询。
- 详情使用固定数量的有界集合查询，流程最多`50`条、事件最多`200`条、文件最多`50`条、归档任务最多`50`条、替换关系最多`20`条。
- 新增三个可重复执行的 PostgreSQL 索引，分别覆盖全合同目录、按状态目录和按合同读取归档任务；不修改表字段、业务数据、软删除或时间维护规则，也不需要重新生成 DAO。
- 当前安装是购买方独立运行的一套本地系统，合同数据固定在当前安装的本地商户范围内，不引入中央业务网关、中央结算、平台销售注册或软件提供方业务回调。

## 浏览器与构建验证

当前隔离数据库`cloud_phase10_e2e_20260818`、后端`http://127.0.0.1:9999/`和管理前端`http://127.0.0.1:6670/`继续用于真实宿主装配验证。完整执行当前`TC004-electronic-contract.ts`，结果为`21/21`通过，包含本任务新增的六个管理场景，并同时回归开通、模板和自动签既有场景：

- 没有合同查看权限时不显示合同管理页签。
- 合同详情显示权威流程、不可变文件和未购买报告的明确原因。
- 可选合同先明确“不影响开通”，生成预览后再单独提交。
- 内容不变时在同一合同追加新的七天签署流程。
- 内容变化时保留原合同，先生成新版本预览再单独提交。
- 解除合同必须填写原因并再次确认，原合同证据继续保留。

人工复核以下最终截图，未发现原始翻译键、内部状态码、遮挡、重叠或异常横向溢出：

- `temp/20260817/171443-contract-management-evidence-zh-cn.png`
- `temp/20260817/171506-contract-changed-reissue-preview-zh-cn.png`

## 其他验证

| 验证 | 结果 |
| --- | --- |
| `pnpm run build`与`pnpm run build:check`（插件`frontend/`） | 通过；插件可独立构建 |
| `LINAPRO_SOURCE_PLUGINS=1 pnpm --filter @lina/web-antd typecheck` | 通过；仅验证宿主能够消费插件产物，没有向宿主写入 IDC 业务代码 |
| `pnpm test:validate` | 通过；217 个 E2E 文件、17 个范围、编号与清单一致 |
| 完整`TC004-electronic-contract.ts` | 通过；`21/21` |
| `go test ./... -count=1`（设置真实 PostgreSQL 测试 DSN） | 通过；包含合同管理服务、控制器、生命周期和迁移验证 |
| PostgreSQL 全量迁移与重复执行定向测试 | 通过；新索引可在隔离 schema 执行且重复迁移无冲突 |
| `go vet ./...`与插件 Go lint | 通过；零问题 |
| `make.cmd i18n.check` | 通过；运行时、消息和前端键覆盖均为零违规 |
| `make.cmd plugins.check` | 通过；2486 个插件文件扫描零发现 |
| `openspec validate add-idc-server-operations-platform --strict` | 通过 |
| `git diff --check` | 通过；仅显示共享工作区既有换行提示 |
| `git status --short -- apps/lina-vben/apps/web-antd` | 空；没有 IDC 业务改动 |

## 影响分析

- 前端：有影响。范围只在合规插件后台页面、插件接口客户端、双语资源和插件专属测试；没有修改`web-antd`。
- 接口与后端：有影响。新增安全有界的合同目录、合同详情和替换选项读取接口，复用既有写操作与本地腾讯网关。
- 数据权限：有影响。读取与各类写操作继续使用互相独立的精确权限；详情先锁定目标合同，再在当前本地安装范围读取其证据。
- 数据库：有影响。只新增三个幂等查询索引，迁移和重复执行均通过真实 PostgreSQL 验证。
- `i18n`：有影响。中文、英文资源、浏览器文案和仓库键覆盖均通过验证。
- 缓存一致性：无影响，没有新增或修改缓存。
- 开发工具与跨平台：无生产工具或脚本改动；使用既有 Windows 代码生成入口后清理了不适用于组合式插件控制器的重复产物，最终 Go 编译、测试和 lint 通过。
- 文档：有影响。新增本验证记录并更新`tasks.md`精确子任务状态。

## Lina 审查报告

**变更：** `add-idc-server-operations-platform`

**范围：** 任务级`10.12.f`

**范围来源：** 最新共享工作区状态、未跟踪插件目录展开结果、任务上下文和上述直接复核文件。

**插件本地规范：** `idc-compliance`根目录不存在额外`AGENTS.md`，适用项目顶级规范。

**已读取规则文件：** `AGENTS.md`、`.agents/rules/user-preferences.md`、`.agents/rules/openspec.md`、`.agents/rules/documentation.md`、`.agents/rules/architecture.md`、`.agents/rules/plugin.md`、`.agents/rules/frontend-ui.md`、`.agents/rules/testing.md`、`.agents/rules/i18n.md`、`.agents/rules/backend-go.md`、`.agents/rules/api-contract.md`、`.agents/rules/data-permission.md`和`.agents/rules/database.md`。

### 发现的问题

- 已修复：新增读取接口的说明与示例不完整；现已补齐字段说明、示例和路径参数标记，并通过接口与双语扫描。
- 已修复：两个读取控制器缺少空依赖保护；现统一返回服务不可用错误，不会在异常装配下崩溃。
- 已修复：合同目录和归档任务的新查询路径缺少精确索引；现新增三个幂等索引，并通过隔离 PostgreSQL 全量迁移和重复执行验证。
- 已修复：页面曾直接显示`month`、流程用途和事件原因等内部码；现统一显示中英文可读文案，完整浏览器回归重新通过。
- 未发现阻塞问题或遗留警告。

### 规则域结论

- OpenSpec、插件边界、架构、前端 UI、接口、数据权限、数据库、后端和测试：通过。
- `i18n`：通过，中英文资源、运行时扫描和前端键覆盖均已验证。
- E2E 质量审查：已触发；权限隐藏、成功、失败说明、不可逆操作二次确认、原证据保留和权威状态均有有效断言。
- 性能：通过；列表无精确总数、无逐行查询，详情查询固定有界，新查询路径有对应索引。
- 缓存一致性：无影响。
- 开发工具：无生产工具改动；代码生成临时重复产物已清理。

### 摘要

- 严重：0
- 警告：0
- 结论：未发现阻塞问题，允许只勾选`10.12.f`。
