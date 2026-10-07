# 第十阶段合同文件与证据归档验证记录

## 1. 任务结论

任务`10.9.d`已完成。合同或解除流程取得腾讯权威完成结果后，每套安装自己的托管作业会在本地处理归档，不依赖软件提供方中央业务网关、中央结算、平台操作者或远程业务回调。

本任务完成了最终`PDF`、解除文件、签署报告和短期存证元数据的归档闭环，但没有提前完成`10.9.e`整组回归，也没有勾选`10.9`父任务。客户合同页面仍由`10.12`在`@lina/web-customer`实现，本任务没有修改前端。

## 2. 自动归档规则

- 普通签署流程收到腾讯`ALL`权威终态后，托管作业自动建立最终`PDF`、签署报告和区块链存证元数据三个独立任务。
- 解除流程收到腾讯`RELIEVED`权威终态后，托管作业自动建立解除文件和区块链存证元数据任务，原合同文件和历史保持不变。
- 最终`PDF`和解除文件通过已保存的加密腾讯流程绑定读取。每个流程只接受一份最终文件，文件必须是有效`PDF`且不超过`32 MiB`。
- 商家已经购买签署报告服务，并且账号、应用和调用主体都满足腾讯要求时，系统自动申请一次签署报告。重复执行只查询原报告，不会再次申请，也不要求管理员逐份点击；可能产生的费用由当前安装所属商家承担。
- 未购买或能力不满足时不调用收费接口，任务保存明确的跳过原因，其他文件仍可继续归档。
- 腾讯区块链存证目前只提供短期二维码或查看地址，因此数据库只保存来源、供应商时间和核验结果等不含链接的元数据，不把它冒充可长期下载的证据文件。

## 3. 文件保存和完整性

可长期保存的文件写入插件私有对象存储。写入后立即重新读取，并逐项核对：

- 实际文件内容；
- 文件容量；
- `SHA-256`摘要；
- 文件类型；
- 私有可见性和不透明对象引用；
- 腾讯供应商时间。

只有重新读取核对成功后，数据库事务才会同时写入不可变合同文件记录、完成归档任务并追加业务审计。对象已存在但写入返回不确定结果时，也会先重新读取确认，避免重复覆盖。已归档的同一流程和文件类型有唯一约束，存储写入禁止覆盖。

数据库不保存合同正文、腾讯临时下载地址或签署报告地址。腾讯报告编号使用当前安装的外部引用密钥加密保存；腾讯短期地址只在一次下载过程中使用，不进入数据库、审计或日志。

## 4. 本地网关和安全边界

- 插件和同一安装内的本地合规网关继续使用用途隔离的进程签名凭据通信；最终文件读取和报告申请使用不同的精确权限范围。
- 本地网关只按当前安装、腾讯供应商、受保护路由和已记录终态处理请求。普通签署只接受`ALL`，解除只接受`RELIEVED`。
- 下载器只允许没有账号密码、片段或异常端口的`HTTPS`地址，拒绝代理、跳转、IP地址写法，以及解析到内网、环回、共享、文档、基准测试、组播或保留地址的域名；TLS最低为`1.2`。
- 文件响应带有禁止缓存和禁止浏览器猜测类型的头。插件再次核对响应种类、状态、长度、`PDF`文件头、摘要和供应商时间。
- 签署报告首次创建后固定等待至少`24`小时再查询。报告尚未生成时继续有限退避查询；权限、账号、主体或报告不存在等明确结果会停止并保存原因；不确定结果保持人工可查，不危险重发收费申请。

## 5. 托管作业、数据库和性能

插件迁移`018-contract-archive.sql`新增：

- 合同文件的供应商时间；
- 已归档文件的必填完整性约束；
- 同一流程和文件类型只能归档一次的唯一索引；
- 带租约、重试、明确跳过和不确定终态的合同归档任务表。

本地网关迁移`021-tencent-contract-artifact.sql`新增签署报告任务。报告业务身份和加密编号不可修改或删除，迁移使用存在性保护并可安全重复执行。

托管作业每分钟只在主节点运行，每批最多处理`8`个任务，使用 PostgreSQL 权威时间、`SKIP LOCKED`和两分钟租约。失败后按`1`分钟、`5`分钟、`30`分钟、`1`小时退避；查询次数的显示值封顶，但只要腾讯没有给出明确结果，任务不会被错误标成完成。任务发现、领取和状态更新都使用有界集合查询，没有按数据库总行数无限扫描，也没有列表逐行远程补查。

依赖在插件启动时一次装配：共享数据库来自宿主服务，私有对象存储来自插件注册器，系统审计来自当前插件身份运行时，本地网关客户端在启动时创建后同时供生命周期任务和归档任务使用。请求处理过程不会临时重建这些依赖。本任务没有新增缓存，缓存一致性无影响。

## 6. 主要实现范围

- 插件数据库和生成代码：`manifest/sql/018-contract-archive.sql`、`hack/config.yaml`、合同文件与归档任务的`DAO/DO/Entity`及迁移集成测试。
- 插件后端：`backend/internal/service/contractarchive/`、`backend/internal/service/cron/contract_archive.go`、本地网关签名与客户端、`backend/plugin.go`。
- 本地合规网关：`internal/migration/sql/021-tencent-contract-artifact.sql`、`internal/tencentartifact/`、`internal/tencentess/artifact.go`、供应商终态绑定、签名协议和应用路由。
- 规范记录：`.agents/rules/user-preferences.md`、本变更的`design.md`、`specs/idc-compliance-center/spec.md`、`tasks.md`和本验证记录。

没有修改`apps/lina-vben/apps/web-antd`，没有把后台业务写入宿主；也没有修改`web-customer`、公开官网、菜单、页面、按钮、可见文案或语言包。

## 7. 自动化验证

```text
LINAPRO_IDC_COMPLIANCE_GATEWAY_TEST_DSN=<本机测试库> GOWORK=off go test ./... -count=1
PASS（本地合规网关全量测试，包含真实 PostgreSQL）

LINAPRO_IDC_COMPLIANCE_GATEWAY_TEST_DSN=<本机测试库> GOWORK=off go vet ./...
PASS

LINAPRO_IDC_COMPLIANCE_TEST_DSN=<本机测试库> GOWORK=off go test ./... -count=1
PASS（合规插件后端全量测试，包含真实 PostgreSQL）

LINAPRO_IDC_COMPLIANCE_TEST_DSN=<本机测试库> GOWORK=off go vet ./...
PASS

make.cmd lint.go dir=apps/lina-plugins/idc-compliance/central-gateway
PASS（golangci-lint 0 issues；staticcheck通过）

make.cmd lint.go dir=apps/lina-plugins/idc-compliance
PASS（golangci-lint 0 issues；staticcheck通过）

make.cmd plugins.check
PASS（扫描2397个文件，0 findings）

make.cmd i18n.check
PASS（违规0；前端键覆盖和运行时消息覆盖通过）
```

真实 PostgreSQL集成测试证明：完成合同会建立三个任务；最终`PDF`归档后具有私有对象、容量、摘要和供应商时间；未购买报告能力时明确跳过收费调用；短期存证只留下不含链接的元数据；重复运行不会再次下载、申请报告或写审计。网关集成测试还证明报告编号以密文保存、首次只创建一次、到期查询后归档、重复查询刷新短期下载地址但不重复收费创建。

全仓`make.cmd lint.go`已经运行。它通过本任务涉及的合规插件和本地网关，随后在与本任务无关的`tenant-core`三个既有文件上因格式问题停止：`backend/internal/dao/config_override.go`、`backend/internal/dao/tenant.go`和`plugin_embed.go`。本任务没有修改这些文件，也没有越权替其他插件改代码；两套本次变更目标的正式检查均已单独通过。

本任务没有用户页面变化，所以没有触发浏览器、截图或页面`E2E`。文件归档整组回归仍属于`10.9.e`，客户页面回归仍属于`10.12`和`10.17`，不能用本记录提前勾选。

## 8. 官方契约依据

- 腾讯最终合同文件：<https://qian.tencent.com/developers/partnerApis/files/DescribeResourceUrlsByFlows/>
- 腾讯申请签署报告：<https://qian.tencent.com/developers/partnerApis/certificate/CreateChannelFlowEvidenceReport/>
- 腾讯查询签署报告：<https://qian.tencent.com/developers/partnerApis/certificate/DescribeChannelFlowEvidenceReport/>
- 腾讯短期区块链存证查看地址：<https://qian.tencent.com/developers/partnerApis/certificate/CreateFlowBlockchainEvidenceUrl/>

## 9. Lina审查结论

审查范围来自当前共享工作区状态、未跟踪插件目录展开、任务`10.9.d`实现清单和代码生成结果。插件根目录不存在额外`AGENTS.md`，本轮完整读取并遵守根`AGENTS.md`、`.agents/rules/user-preferences.md`、`openspec.md`、`documentation.md`、`architecture.md`、`data-permission.md`、`plugin.md`、`api-contract.md`、`backend-go.md`、`database.md`、`testing.md`、`i18n.md`和`dev-tooling.md`。

审查中已经修正签署报告编号密钥用途不一致、数据库错误被误判为业务冲突、迁移直接重跑失败、保留地址拦截不完整、数据库更新结果未检查、对象读取关闭错误被忽略和文件响应缺少安全响应头等问题。修正后已经重新通过两套全量测试、`go vet`、两套正式代码检查、插件治理和多语言治理。

- 架构和插件边界：通过；全部业务闭环在合规插件及其随安装交付的本地网关。
- 数据权限和安全：通过；任务只处理当前安装本地系统作业，没有新增用户列表或下载接口，安装、供应商、路由、终态、签名和防重放边界均有测试。
- API和性能：通过；没有新增公共HTTP API，只有安装内签名协议；批次、租约、下载大小和查询范围均有上限。
- Go、数据库和代码生成：通过；目标模块全量测试、`go vet`和正式代码检查通过，迁移可重复执行，生成文件由项目生成入口维护。
- 测试：通过；单元测试和真实PostgreSQL测试覆盖成功、跳过、等待、不确定、重复执行、密文、文件核验和不安全下载地址。
- `i18n`：无影响；没有新增用户可见文案或语言资源，全仓多语言检查通过。
- 缓存：无影响；没有新增或修改缓存。
- 开发工具和跨平台脚本：无影响；本任务没有修改构建、测试或发布工具。
- 文档：通过；业务结论已经同步到用户偏好、设计、规范、任务和本验证记录。

最终未发现阻塞`10.9.d`的问题。剩余工作是`10.9.e`要求的合同整组安全回归和双平台组件包验收，不能把本子任务的通过结果当作父任务已经完成。
