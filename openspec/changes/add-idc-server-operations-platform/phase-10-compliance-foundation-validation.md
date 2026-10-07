# 第十阶段合规数据底座验证记录

## 实现范围

本记录覆盖任务`10.1`。`idc-compliance`由`v0.1.0`提升到`v0.1.1`，新增一个只包含 DDL 的`001-compliance-foundation.sql`，并通过插件自己的`hack/config.yaml`和标准`make dao`入口生成 DAO、DO 与 Entity。本轮没有连接阿里云、腾讯电子签、诺诺或其他外部服务，没有新增`HTTP API`、页面、后台任务或生产运行时依赖。

`idc-compliance`独占 13 张业务表：认证案件与企业材料；合同、签署流程、签署方与合同文件；`tax_invoice`、原应收账单分配、税目明细与发票文件；红字任务与不可变红字分配；只追加的外部证据。`idc-core`继续只保存认证/合同业务门禁投影和不透明合规编号，不保存敏感材料或第二份税务发票真相。

## 数据边界与不变量

- 13 张表都使用完整的`plugin_idc_compliance_`前缀、内部`BIGINT IDENTITY`主键和稳定不透明业务编号，约束、索引、触发器与函数名称均不超过 PostgreSQL 的 63 字节标识符上限。插件内部关系使用`ON DELETE RESTRICT`外键；客户、主体、订单、服务、`billing_invoice`、退款操作和退款分配只保存不透明编号，没有跨插件数据库外键。
- 数据库禁止原始人脸图片或视频、证件内容、合同正文、发票原件和第三方原始报文列。企业材料、合同/发票文件及外部证据只保存私有对象引用、媒体元数据和`SHA-256`完整性摘要；身份与发票号码的精确查询只预留带密钥摘要字段，明确禁止原文和无盐公开哈希。
- 法律和审计记录不使用`deleted_at`。企业材料使用显式可用、隔离、替换、删除待办、已删除和法律保全状态。归档合同与发票文件的对象引用、摘要、归属、类型、大小和不可变标志由数据库触发器锁定，物理删除始终拒绝；留存期限、法律保全和销毁生命周期仍可显式推进。
- `external_evidence`和`red_letter_allocation`严格只追加。每条红字分配同时关联红字任务、原税务发票分配和`core`退款分配不透明编号，使部分开票、合并开票和多次退款仍能逐次追溯。
- 合同、税务发票、税目和红字金额使用最小货币单位整数并校验含税金额等于未税金额加税额。币种强制使用大写三字符代码；订单币种与计税币种不同时，换算分子、分母、日期和来源必须同时存在且均有效。
- 活动认证案件、供应商流程编号、开票幂等摘要、活动红字任务和第三方事件编号均有唯一约束。状态、客户、订单、服务、轮询、原账单分配、退款分配、留存扫描及聚合证据具备对应组合或部分索引，后续列表可先分页再按业务编号集合化装配，不需要逐行跨插件查询。

## 测试与生成证据

| 命令或检查 | 结果 |
|---|---|
| `TestComplianceFoundationMigrationPostgreSQL -count=5 -shuffle=on` | 通过；每次创建随机隔离 schema，全部迁移连续执行两遍并自动清理 |
| `TestComplianceMigrationIdentifiersFitPostgreSQL -count=1` | 通过；迁移内全部约束、索引、触发器和函数名称均不超过 63 字节 |
| `TestComplianceFoundationCodegenPostgreSQL -count=1` | 通过；先确认数据库名称精确为`linapro_idc_codegen`，只重建合规插件表 |
| `make.cmd dao dir=apps/lina-plugins/idc-compliance/backend` | 通过；生成 13 组公开 DAO、内部 DAO、DO 与 Entity，共 52 个生成文件 |
| `GOWORK=off go test ./backend/... -count=20 -shuffle=on` | 通过；覆盖插件注册、清单版本及全部生成包编译 |
| `make.cmd lint.go plugins=1 dir=apps/lina-plugins/idc-compliance` | 通过；`golangci-lint`和`staticcheck U1000`均为 0 问题 |
| `make.cmd plugins.check format=json` | 通过；扫描 11 份清单、11 份配置和 1460 个文件，发现 0 个问题 |

真实 PostgreSQL 行为测试覆盖 13 张表、零软删除列、零跨插件外键、关键索引和 4 个不可变触发器，并实际拒绝非零租户、重复活动实名、企业材料挂到个人案件、跨案件材料替换、合同文件跨合同引用签署流程、红字明细跨税务发票引用原分配、非法摘要、小写币种、归档文件内容篡改、文件物理删除、红字分配修改及外部证据修改/删除。同时验证归档文件可以合法进入法律保全或删除待办，而内容与归属继续不可变。

## 规则域影响

- 架构、插件与数据库：合规事实完整闭环在源码内建插件中，没有修改`apps/lina-core/`、`apps/lina-vben/`或根`hack/`。迁移文件名、完整表前缀、DDL 数据分类、幂等执行、内部外键、索引和无软删除语义均通过真实 PostgreSQL 验证。
- 后端 Go：只有自包含 PostgreSQL 测试和 GoFrame 生成文件；DAO、DO 与 Entity全部由官方生成入口维护，没有手工编辑。没有新增构造函数、服务、日志、错误或运行期依赖，因此 DI 来源、共享实例和缓存后端无影响。
- 数据权限：本轮没有新增读取、写入、下载、执行或插件宿主服务入口，数据库尚不可由用户直接访问，因此没有运行时数据权限接入点。后续`10.2`至`10.12`的客户与后台接口必须分别接入当前客户所有权或直接员工、精确权限和目标数据范围，不能把内部 DAO 暴露为跨插件契约。
- `i18n`与字典：没有新增页面、菜单、API 文档、业务错误或用户可见运行时文案，现有`en-US`/`zh-CN`资源无需新增键。存储层状态使用稳定代码和数据库约束；用户可见字典及双语标签由引入对应 API/UI 的任务注册，当前切片没有运行时字典消费。
- 缓存、公开 API、前端与 E2E：均无变化。本轮没有用户可观察页面、接口联动或端到端工作流，因此不触发 E2E 质量审查；真实 PostgreSQL 集成测试是本任务更直接的行为证据。
- 开发工具：只配置插件自己的 GoFrame DAO 表白名单、完整`removePrefix`和可空字段映射，没有修改代码生成器、根脚本或默认开发路径。Windows 已通过`make.cmd dao`；Linux/macOS继续使用同一 Go 工具链的插件`make dao`入口。
- 文档：英文`README.md`与中文`README.zh-CN.md`同步表达相同范围和边界；OpenSpec 设计、长期用户偏好和本验证记录均使用中文并保持技术标识格式一致。

## Lina 审查摘要

任务级审查范围来自`git status --short`、展开后的合规插件未跟踪文件、OpenSpec 任务上下文和本轮代码生成清单；插件根不存在局部`AGENTS.md`。仓库中宿主与其他阶段已有脏文件不属于任务`10.1`，未修改也未作为本任务结论依据。

审查中发现并修复五类问题：第一，最初的文件触发器锁死整行，导致归档后无法合法设置法律保全或留存处置；现只允许治理元数据推进并永久锁定内容、归属和不可变标志。第二，身份与发票号码查询字段最初按普通`SHA-256`命名，存在低熵标识被枚举的风险；现统一为带密钥查询摘要并在列注释和文档中禁止无盐公开哈希。第三，只有红字任务头和发票分配累计值无法证明一次红冲对应哪笔原分配；现新增不可变`red_letter_allocation`逐次保存完整追溯链。第四，单列外键可能把企业材料、合同文件或红字分配挂到错误的父聚合；现改用带案件、合同或税务发票编号的复合外键，并加入负向实库用例。第五，8 个初始数据库对象名超过 PostgreSQL 的 63 字节上限；现缩短为唯一可读名称并加入静态回归测试。

修复后复核未发现剩余阻断问题。任务范围内严重问题 0 个、警告 0 个。个人/企业认证服务、外部适配器、中央合规网关、合同与发票状态机、权限/API、页面、字典和 E2E 仍由`10.2`至`10.19`分别交付，不属于本数据底座完成声明。

**已读取规则：**`AGENTS.md`、`.agents/rules/openspec.md`、`.agents/rules/documentation.md`、`.agents/rules/architecture.md`、`.agents/rules/data-permission.md`、`.agents/rules/plugin.md`、`.agents/rules/backend-go.md`、`.agents/rules/database.md`、`.agents/rules/testing.md`、`.agents/rules/i18n.md`、`.agents/rules/dev-tooling.md`与`.agents/instructions/markdown-format.instructions.md`。
