# 第十阶段隐私治理验证记录

## 验收范围

本记录验收任务`10.13.a`至`10.13.h`。实现覆盖六类分别版本化的个人信息保护影响评估、三项独立后台权限、收集前告知、客户同意撤回、按资料类型配置的留存与人工确认删除、跨境硬关闭、AI安全投影白名单、泄露响应、本地脱敏支持导出，以及后台和客户页面。

后台业务页面只位于`apps/lina-plugins/idc-compliance/frontend/`，通过源码插件机制装入 LinaPro 管理后台。登录后的客户页面只位于`apps/lina-vben/apps/web-customer/`。`apps/lina-vben/apps/web-antd/`没有本任务业务差异，公开官网和旧 Portal 均未承接本任务代码。

## 业务结果

- 后台权限分为查看、配置、泄露事件处理三项，接口和页面按钮按对应权限关闭。
- 个人实名、企业认证、电子合同、税务发票、本地敏感资料导出和 AI 安全投影分别维护草稿、已发布和已替代版本；发布必须由一名有权后台人员再次确认并写入审计。
- 客户只能查看当前登录账号的告知、最新同意决定和本人申请。撤回只追加一条影响未来处理的决定，不改写历史证据。
- 每类资料必须有后台配置的正整数留存天数。评估或留存规则缺失时，新收集和 AI 投影都关闭；到期只生成待删除任务，删除前再次检查合同、税务、争议、审计和法律保全。
- 跨境处理由数据库约束和服务校验共同保持关闭，没有普通开关。AI 只接受已发布评估中明确列出的安全字段；原始材料、私有地址、秘密引用和可还原身份字段不进入投影。
- 创建泄露事件立即废止旧敏感访问代次并阻止导出。流程按发现、遏制、评估、待确认通知、已确认通知、修复、解决顺序推进；系统不会自动对外通知。
- 支持资料必须另行确认支持约定，只生成本地最少脱敏统计和摘要，不开放远程登录。

## 主要实现位置

- 数据库：`apps/lina-plugins/idc-compliance/manifest/sql/024-privacy-governance.sql`
- 后端服务：`apps/lina-plugins/idc-compliance/backend/internal/service/privacygovernance/`
- 接口与控制器：`apps/lina-plugins/idc-compliance/backend/api/{admin,account}/v1/`和`backend/internal/controller/{admin,account}/`
- DAO 生成配置与结果：`apps/lina-plugins/idc-compliance/hack/config.yaml`、`backend/internal/{dao,model}/`
- 插件后台：`apps/lina-plugins/idc-compliance/frontend/pages/privacy-governance.vue`
- 客户后台：`apps/lina-vben/apps/web-customer/src/views/account/privacy/index.vue`
- 双语：插件`frontend/locales/`、`manifest/i18n/`和客户应用`src/locales/langs/{zh-CN,en-US}/privacy.json`
- 浏览器回归：插件`TC006-privacy-governance.ts`和客户应用`TC009-privacy-authorization.ts`

## 验证结果

| 门禁 | 当前工作区结果 |
| --- | --- |
| 合规插件全量后端测试 | `GOWORK=off go test ./backend/... -count=1`通过 |
| 核心插件全量后端测试 | `GOWORK=off go test ./backend/... -count=1`通过 |
| 隐私治理真实 PostgreSQL | 使用显式本地隔离测试库执行`TestPrivacyGovernancePostgreSQLAssessmentRetentionAndGate`，通过且未跳过 |
| 核心同意撤回真实 PostgreSQL | `TestIdentityProjectionPostgreSQLPrivacyWithdrawalIsAppendOnlyAndCustomerScoped`通过且未跳过 |
| SQL 迁移 | 随机隔离 schema 的迁移与重复执行通过；新增十张隐私治理表和真实唯一约束 |
| DAO 生成 | 标准`make.cmd dao dir=apps/lina-plugins/idc-compliance/backend`通过，十张表的 DAO、DO、Entity 已生成 |
| Go 静态检查 | 合规插件与核心插件分别执行`make.cmd lint.go ... plugins=1`，均为零问题 |
| 仓库双语检查 | `make.cmd i18n.check`通过；运行时、错误码、前端键和新增中文接口文档均通过 |
| 插件前端 | `pnpm run build`和`pnpm run build:check`通过 |
| 客户前端 | 结构对齐、双语、类型检查和生产构建通过；生成`dist/`与`dist.zip` |
| 插件管理端 Chrome | `TC006-privacy-governance.ts`为`4/4`通过 |
| 客户后台 Chrome | `TC009-privacy-authorization.ts`为`4/4`通过 |
| OpenSpec | `openspec validate add-idc-server-operations-platform --strict`通过 |

## 审查结论

- 数据权限：管理接口只使用三项精确权限；客户接口不接收客户号，所有权只来自当前登录上下文；真实数据库测试覆盖其他客户不可见。
- 事务与数据库：隐私多步写操作由插件 DAO 的`TransactionWithDatabase`封装，并继续使用宿主注入的本地数据库；服务代码不能取得或使用原始事务对象。复杂的锁定、聚合和跨资料类型扫描保留参数化 SQL，普通写入没有使用`g.Map`，并由真实 PostgreSQL 和并发版本条件验证。
- 性能：列表全部有上限；评估字段和事件时间线使用集合查询，没有逐行查询；客户概览最多返回一百条本人申请。
- 安全：浏览器和接口均不返回原始材料、私有对象地址、秘密引用或远程访问入口；支持包内容带 SHA-256 校验。
- 前端归属：`git diff --name-only -- apps/lina-vben/apps/web-antd`为空；没有通过修改宿主绕过插件构建问题。
- 其他规则域：没有缓存、中央业务网关、中央结算、平台销售注册、软件提供方业务回调或官网变化；没有提交或推送 Git。

共享宿主当前仍有三个早于本任务的重复控制器声明，位于`joblog_new.go`、`plugin_new.go`和`role_new.go`。本任务没有修改或恢复这些文件，因此没有把宿主全量编译误写为通过；插件独立编译、真实数据库、两套前端构建和真实浏览器回归已经覆盖本任务范围。
