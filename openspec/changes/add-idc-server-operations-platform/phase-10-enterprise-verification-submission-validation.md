# 第十阶段企业认证资料提交验证记录

## 当前结论

任务`10.3.b`的企业资料上传、确认、提交、案件与材料持久化已经实现，并完成任务范围内的数据库、后端测试、静态检查和治理验证。此前唯一剩余的仓库级`i18n`阻断已由其所有者范围闭环；当前重新执行全仓双语门禁通过，因此任务`10.3.b`可以完成。

本切片只交付客户侧资料提交和持久化边界。后台案件列表、短期私有访问、逐次查看审计、批准、驳回与重审由`10.3.c`实现；管理端与`Portal`页面及完整`E2E`由`10.3.d`实现。

## 两段式上传契约

客户先创建企业认证草稿，再为每份材料创建服务端预约。预约返回不可猜测的案件号和材料号，随后使用仅包含案件号、材料号、媒体类型和原始请求体的`PUT`上传材料，最后以材料号集合提交审核。

该契约不接收客户端文件名，不使用`multipart/form-data`，也不把材料类型或幂等键放入自定义请求头。材料类型和幂等键只进入预约命令，原始字节上传只消费已存在且属于当前企业的预约，从而避免临时文件、框架表单解析和自定义敏感头进入普通日志链路。

单个案件最多确认八份材料，单份材料最大`5 MiB`。服务端通过内容嗅探只接受`PDF`、`JPEG`和`PNG`；`application/octet-stream`只表示客户端未声明类型，最终类型仍以服务端识别结果为准。

## 私有存储与加密边界

- 材料原始字节在进入私有对象存储前使用`AES-GCM`信封加密；对象必须标记为私有，写入后立即执行读取回查和`SHA-256`完整性确认。
- 安装实例的材料主密钥通过`secretcap.GeneratedManager`创建和读取，普通数据库只保存不可逆的秘密句柄、密钥编号、私有对象引用和完整性摘要，不保存密钥原文或密文。
- 同一预约和同一内容的重放生成相同信封并安全收敛；内容变化、材料类型变化、跨案件或跨客户重放全部失败关闭。
- 案件法定资料快照单独加密，调用方提供的法定名称、登记标识、法定代表人和经办人字节缓冲在返回前主动清零。
- 客户接口响应不返回私有对象引用、内容摘要、密钥句柄、密钥编号或永久访问地址。

## 数据库与状态约束

迁移`003-enterprise-verification-submission.sql`新增企业认证案件、材料、上传预约和安装实例密钥环所需结构。上传预约以服务端材料号、案件号、材料类型、幂等摘要和请求指纹形成唯一并发围栏；材料必须引用已经预约的材料号，只有私有对象写入并回查成功后才能从`quarantined`推进到`available`。

本切片同时建立以下稳定字典及中英文资源：

| 字典 | 稳定值数量 |
|------|------------|
| `idc_compliance_enterprise_verification_status` | 8 |
| `idc_compliance_enterprise_material_type` | 6 |
| `idc_compliance_enterprise_material_status` | 6 |
| `idc_compliance_enterprise_material_upload_status` | 2 |
| `idc_compliance_enterprise_applicant_role` | 2 |
| `idc_compliance_enterprise_material_encryption_state` | 1 |
| `idc_compliance_enterprise_encryption_key_status` | 2 |

迁移采用增量文件并验证连续执行两次。法律证据表不使用软删除；材料后续通过显式的待删除、已删除和法律保全状态执行留存治理，不以普通删除接口覆盖历史事实。

## 所有权、事务与依赖注入

客户接口不接受客户编号。服务通过`identitycap.CurrentEnterprise`取得当前企业，再按企业所有权查询案件、预约和材料；对跨客户编号统一返回不可见结果。客户路由使用认证和租户中间件，客户所有权由服务层再次校验，不能由请求参数绕过。

草稿创建、预约、材料确认和提交分别在数据库事务中写入状态及业务审计。审计只记录案件号、材料号、状态和有界计数，不记录法定资料、对象路径、内容摘要或文件内容。

新增依赖的装配来源如下：

| 依赖 | 所有者与创建位置 | 传递和复用 |
|------|------------------|------------|
| 当前企业与认证投影 | `idc-core`的`identitycap`运行时 | 插件启动时加载，同一运行时供客户服务复用 |
| 材料秘密 | `idc-core`的`secretcap` | `composition.LoadSecretSource`创建受限 accessor 和 manager，并注入企业认证服务 |
| 私有对象存储 | 宿主公开`storagecap` | 插件启动时取得共享后端，由服务执行私有写入、读取和完整性确认 |
| 业务审计 | `idc-core`的客户审计能力 | 从身份运行时注入，在数据库事务内追加状态变更 |
| 数据库 | 宿主`g.DB()` | 插件启动时显式注入服务，不创建第二连接池 |

## 性能与数据最小化

该流程是单案件写路径，不提供列表或聚合接口。提交时使用一次集合查询读取已选择材料，材料数量硬上限为八份；对象读取和完整性校验也最多八次，不随数据库总行数增长。数据库约束和索引覆盖案件所有权、上传预约、材料号和状态查询，不存在按返回行数增长的数据库`N+1`装配。

`AI`无影响：合规插件没有导入或调用`AI`能力，普通数据库、日志和审计均不保存可被模型读取的材料原件或明文法定资料。后续`AI`任务仍必须延续“原件永不进入模型”的冻结边界。

## 自动化验证

| 验证 | 结果 |
|------|------|
| `GOWORK=off go test ./backend/internal/service/enterpriseverification ./backend/internal/controller/account ./backend/api -count=1` | 通过；客户接口、控制器和企业认证服务编译及单元测试成功 |
| 设置隔离测试和代码生成`DSN`后执行迁移、标识符和代码生成 PostgreSQL 用例 | 通过；迁移连续执行两次，约束、索引、七类字典和双语资源一致 |
| `GOWORK=off go test ./backend/internal/service/enterpriseverification -run 'TestEnterprise' -count=20 -shuffle=on` | 通过；并发预约、幂等重放、语义冲突、所有权隔离、内容变化和敏感信息排除稳定 |
| `GOWORK=off go test ./backend/... -count=20 -shuffle=on` | 通过；合规插件全部后端包随机顺序连续二十轮成功 |
| `GOWORK=off go vet ./backend/...` | 通过 |
| `make.cmd lint.go plugins=1 dir=apps/lina-plugins/idc-compliance` | 通过；零问题 |
| `make.cmd plugins.check format=json` | 通过；扫描`1555`个文件和`11`个清单，零发现 |
| 静态扫描自定义幂等/类型请求头、危险响应字段、`AI`导入和日志调用 | 通过；任务实现范围无命中 |
| `make.cmd i18n.check`（当前复核） | 通过；运行时扫描、宿主与插件消息覆盖、宿主与插件前端键覆盖均无问题 |

## 影响分析

- 接口：有影响。新增客户企业认证草稿、材料预约、原始内容上传和提交接口；时间字段使用 Unix 毫秒，枚举使用稳定英文代码。
- 后端：有影响。新增企业认证服务、显式依赖注入和客户控制器；全插件测试、`go vet`和 lint 已通过。
- 数据库：有影响。新增增量迁移、约束、索引、字典和生成模型；幂等性及真实 PostgreSQL 已验证。
- 数据权限：有影响。客户所有权从当前主体推导，服务层再次检查案件、预约和材料归属，不暴露对象存在性。
- `i18n`：有影响。本任务新增字典、错误和接口文档资源均已覆盖中英文，当前仓库级门禁通过。
- 缓存一致性：无影响。案件、预约、材料和密钥环状态以 PostgreSQL 与私有对象存储为权威，本切片不新增缓存。
- 前端与用户可见页面：无代码影响。客户和管理页面明确属于`10.3.d`，本切片不宣称页面交付。
- `E2E`：本切片没有新增页面，服务和数据库集成测试作为当前证据；完整企业认证浏览器流程明确保留给`10.3.d`。
- 开发工具与跨平台：无工具源码影响。没有修改根`Makefile`、`make.cmd`、根`hack`或脚本；插件`hack/config.yaml`只维护本插件 DAO 表清单，Windows 使用仓库标准命令验证。
- 文档：有影响。仅新增本 OpenSpec 验证记录并更新对应任务说明，不新增目录级说明文档。

## Lina 审查报告

**变更：** `add-idc-server-operations-platform`
**范围：** 任务级`10.3.b`
**审查文件数：** 55 个实现、生成、迁移及`i18n`文件，另含任务记录、本验证记录和用户偏好记录
**范围来源：** `git status --short`、`git ls-files --others --exclude-standard`、未跟踪插件目录展开结果与任务上下文
**插件本地规范：** `apps/lina-plugins/idc-compliance/AGENTS.md`不存在，适用项目顶级规范
**已读取规则文件：** `AGENTS.md`、`.agents/rules/openspec.md`、`.agents/rules/architecture.md`、`.agents/rules/data-permission.md`、`.agents/rules/plugin.md`、`.agents/rules/api-contract.md`、`.agents/rules/backend-go.md`、`.agents/rules/database.md`、`.agents/rules/i18n.md`、`.agents/rules/testing.md`、`.agents/rules/documentation.md`和`.agents/instructions/markdown-format.instructions.md`

### 发现的问题

- **严重** `apps/lina-vben/apps/web-antd/src/views/system/message/notice-preview-modal.vue:16`、`:42`、`:45`：仓库全局前端键覆盖缺少`plugin.content-notice.preview.title`、`plugin.content-notice.fields.type`和`plugin.content-notice.fields.createdBy`的中英文资源，共六个缺键。依据：`.agents/rules/i18n.md`与`.agents/rules/openspec.md`要求`make i18n.check`通过；根目录没有`.contributing`，依据`AGENTS.md`本任务禁止修改`apps/lina-vben/`，因此不能在当前插件任务中越界修复。

审查过程中已修复任务范围内的警告：补齐七类稳定枚举字典及双语资源、完善新增文件与导出方法注释、同步企业认证接口枚举文档，并处理测试中的未检查错误。复核后没有遗留任务范围内警告。

### 规则域结论

- OpenSpec 与文档：通过。实现与`10.3.b`边界一致，验证记录为中文且链接、表格和标识符格式符合项目规范；任务因严重门禁保持未勾选。
- 架构与插件：通过。实现闭环在`idc-compliance`内，只通过公开能力依赖核心和宿主，没有跨插件内部导入或宿主源码改动。
- 接口与数据权限：通过。客户编号不由请求提供，所有权优先校验，响应不返回敏感存储和密钥元数据。
- 后端、数据库与性能：通过。事务、显式依赖注入、生成模型、迁移幂等、查询上限和索引满足当前单案件写路径。
- 测试：通过当前后端切片；页面与浏览器`E2E`未命中本切片并由`10.3.d`承接。
- 缓存一致性：无影响；未引入缓存。
- 前端 UI：无代码影响；未修改页面、路由、组件或样式。
- 开发工具：无源码影响；未修改构建、测试或代码生成工具。
- `i18n`：本插件内容通过定向覆盖，但仓库全局门禁存在一个范围外严重问题。

### 验证证据

验证命令和结果见“自动化验证”。其中全后端二十轮、真实 PostgreSQL、`go vet`、lint、插件治理均通过；`make.cmd i18n.check`已实际执行并明确失败，未使用历史假设替代证据。

### 摘要

- 严重：1 个范围外仓库级门禁问题
- 警告：0

### 建议操作

1. 由宿主通知模块负责人在允许修改宿主的独立变更中补齐三项中英文资源。
2. 修复后重新运行`make.cmd i18n.check`、任务相关测试、lint 与`openspec validate add-idc-server-operations-platform --strict`。
3. 只有全部强制门禁通过后，才能勾选`10.3.b`并按顺序进入`10.3.c`。

## 后续阻断复核闭环

以上审查报告保留任务实施当时的真实历史。当前共享工作区已经具备`.contributing`所有权边界，通知预览缺键也已由对应所有者范围修复；本次没有修改`web-antd`业务源码。重新执行`.\make.cmd i18n.check`通过，运行时违规为 0，宿主与插件的消息及前端键覆盖全部通过。结合本记录原有真实 PostgreSQL、二十轮随机后端测试、`go vet`、lint 和插件治理证据，原外部阻断已解除，`10.3.b`现可精确勾选。
