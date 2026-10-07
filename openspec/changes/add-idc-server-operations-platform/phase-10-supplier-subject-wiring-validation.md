# 第十阶段本地供应商主体接线验证记录

## 结论

任务`10.6.c.3b`至`10.6.c.3d`已经按本地独立运行边界实现并验证。本轮只完成合规插件、本地合规网关和插件自有管理能力接线，不新增管理页面，不装配腾讯或诺诺生产识别器，不进入第十五阶段，也不据此完成`10.6.c`、`10.6`或`FB-11`。

本记录形成时腾讯与诺诺生产识别器都未装配，并统一返回`external_confirmation_required`，不签发无用凭证、不创建等待状态、不改变当前配置，也不回退软件提供方中央业务服务。`2026-08-14`后腾讯`10.4.b`已经改为官方契约与购买方运行时门禁，腾讯识别器及新的可重试封闭原因由`10.8`实现；诺诺`10.4.c`仍处于外部证据阻断状态。

## 根因与纠正

既有本地供应商配置骨架把凭据与主体输入放在同一配置边界，数据库仍存在`merchant_subject_no`，插件和本地网关之间也没有三个按操作隔离的签名接口。这会允许普通管理请求承担不应拥有的供应商主体选择职责，且无法证明主体明文、近期认证、单次凭证、失败状态和本机调用范围各自失败关闭。

本轮将凭据、主体绑定和最后操作拆成独立状态机，并在复核中额外纠正以下底层问题：

- 管理敏感操作凭证原来使用两分钟默认值，现固定为最多五分钟，并继续受完整登录后五分钟截止点约束。
- 管理控制器的供应商服务原来是可选依赖，现改为启动期显式必需依赖。
- 凭据或主体失败状态的落库错误原来存在被忽略路径，现全部失败关闭并向上返回稳定业务错误。
- 本地网关未装配识别器时的错误码统一为`external_confirmation_required`。
- 单次操作凭证现只持久化带用途隔离的`SHA-256`摘要和截止时间；原始凭证不落库，消费仍使用宿主 PostgreSQL 权威单次围栏。

## 固定接口与权限

| 操作 | 管理接口 | 精确权限 | 最近完整登录 | 本机接口与范围 |
|---|---|---|---|---|
| 查看 | `GET /admin/compliance/suppliers/{providerCode}` | `idc:admin:compliance:supplier:query` | 不额外要求 | 无写入 |
| 保存凭据 | `PUT /admin/compliance/suppliers/{providerCode}/credential` | `idc:admin:compliance:supplier:credential:write` | 五分钟 | 无主体输入 |
| 首次绑定 | `POST /admin/compliance/suppliers/{providerCode}/subject-bindings` | `idc:admin:compliance:supplier:subject:bind` | 五分钟 | `POST /internal/v1/supplier-subject-bindings`与`supplier-subject.bind` |
| 撤销 | `POST /admin/compliance/suppliers/{providerCode}/subject-revocations` | `idc:admin:compliance:supplier:subject:revoke` | 五分钟 | `POST /internal/v1/supplier-subject-revocations`与`supplier-subject.revoke` |
| 重新开通 | `POST /admin/compliance/suppliers/{providerCode}/subject-reopenings` | `idc:admin:compliance:supplier:subject:reopen` | 五分钟 | `POST /internal/v1/supplier-subject-reopenings`与`supplier-subject.reopen` |

后台绑定和重新开通请求不接受主体编号、主体原文或供应商授权结果。未来通过外部证据门禁的识别器只能在服务端把一次性授权结果交给内部服务；内部服务把结果放入可清零字节缓冲，经固定`127.0.0.1:9088`本机签名通道交给本地网关。浏览器、客户接口和普通业务接口均不能调用本机接口。

## 状态、存储与安全投影

- 凭据状态只允许`unconfigured`、`updating`、`configured`、`retryable_failed`。
- 主体状态只允许`unbound`、`binding`、`active`、`revoked`、`reopening`、`retryable_failed`。
- 最后操作结果只允许`none`、`pending`、`succeeded`、`retryable_failed`。
- 迁移`007-local-supplier-subject-boundary.sql`删除`merchant_subject_no`，新增本地不透明绑定号、操作号、状态、操作时间以及单次凭证摘要和截止时间约束。
- 后台响应不包含主体原文、完整供应商主体编号或其掩码；只包含状态、操作时间、本地`lsb_`绑定号和本地`lsi_`操作号。
- 操作凭证绑定当前宿主用户、默认商家、宿主会话、供应商、操作类型、上一绑定、配置版本和幂等键。目标变化、会话变化、过期或重复消费均拒绝。
- 本地网关负责识别、规范化、加密和保存主体；插件数据库、日志、业务审计和普通接口不保存主体原文。测试证明成功和异常路径都会清零授权结果缓冲。
- 任何未完成、未确认或失败状态都保持`adapter_enabled=false`。网关调用失败会提交`retryable_failed`，不会伪装成功。

## 依赖来源与独立运行

`backend/plugin.go`在启动路由装配时创建唯一`localgatewayclient.Client`，从当前安装的安装标识、进程密钥环和签名窗口读取配置，再显式传给`supplierconfig.New`。装配失败会清除客户端持有的密钥副本；装配成功后该实例由宿主进程生命周期共享，不在请求路径临时创建。

`idc-core`的管理运行时创建一个共享近期认证服务，使用宿主会话、当前安装业务上下文、签名密钥和 PostgreSQL 单次消费围栏。合规插件只依赖`managementcap`、`secretcap`和本机网关窄接口，不读取其他插件的`DAO/DO/Entity`，不访问软件提供方网络服务。

本地网关进程继续独占本地数据库、供应商主体密钥和`supplieridentity`服务。三个业务路由复用安装期进程签名、随机数防重放和业务幂等围栏，但分别固定路径与操作范围。供应商生产客户端、中央销售注册、跨客户路由、中央结算和平台操作者均未引入。

## 规则影响评估

| 规则域 | 结论 |
|---|---|
| API | 新增拆分后的资源接口；方法、路径、权限标签、字段说明和毫秒时间戳符合固定契约 |
| SQL | 当前迭代只维护`007`迁移；可立即重复执行；无 Seed 或 Mock 数据；供应商配置是安装级单例，不使用软删除 |
| 数据权限 | 两个固定安装级供应商配置不存在列表或跨租户读取；宿主 Authorizer仍强制默认商家、直接活动员工、可表达数据范围和精确权限，拒绝冒充与跨安装目标 |
| 性能 | 每次操作只读取一个受唯一键约束的配置行；无动态列表、循环查询或`N+1`装配；沿用`tenant_id + provider_code`唯一索引 |
| 缓存 | 不新增缓存或派生快照；权威状态、凭证摘要、随机数和幂等围栏均在 PostgreSQL，缓存一致性规则无影响 |
| i18n | 已同步插件菜单、错误和中文 API 文档资源；英文 API 文本仍为源文本，`en-US/apidoc`保持空对象 |
| 前端与 E2E | 本任务按冻结边界不新增页面或前端交互；未触发页面截图和 E2E 质量审查，改由 API、服务、真实 PostgreSQL和本机 HTTP 自动化覆盖 |
| 开发工具 | 只更新插件`hack/config.yaml`的 DAO 可空字段映射并通过正式`make.cmd dao`生成；未修改根构建脚本，Windows 与 Linux 运行代码一致 |

## 验证证据

以下命令均基于当前共享工作区执行：

- `go test ./...`：`idc-compliance`、`central-gateway`和`idc-core`三个模块全部通过。
- `go test ./backend/internal/service/supplierconfig -run '^TestSupplierSubjectBindingProofAndFailureStatePostgreSQL$' -count=1 -v`：真实 PostgreSQL成功，证明五分钟凭证摘要持久化、单次消费、状态提交、可重试失败、明文排除和适配器关闭。
- `go test . -run '^TestComplianceFoundationMigrationPostgreSQL$' -count=1 -v`：真实 PostgreSQL迁移、约束和立即重复执行通过。
- `make.cmd dao dir=apps/lina-plugins/idc-compliance/backend`：正式生成`DAO/DO/Entity`成功，未手工修改生成文件。
- `make.cmd lint.go dir=apps/lina-plugins/idc-compliance`、`make.cmd lint.go dir=apps/lina-plugins/idc-compliance/central-gateway`和`make.cmd lint.go dir=apps/lina-plugins/idc-core`：三个模块均为零问题。
- `go vet ./...`：三个模块全部通过。
- `make.cmd plugins.check`：扫描`1625`个文件，零发现。
- `make.cmd i18n.check`：运行时扫描和宿主/插件消息覆盖通过；全仓前端键检查仍因任务范围外的`apps/lina-vben/apps/web-antd/src/views/system/message/notice-preview-modal.vue`缺少`content-notice`中英文三个键而失败，共六项。按既有宿主门禁例外如实保留，不修改宿主页，也不宣称全仓 i18n 门禁通过。
- `openspec validate add-idc-server-operations-platform --strict`：严格规范校验通过。

## 审查结论

本轮`lina-review`未发现仍属于`10.6.c.3`范围的严重问题。自动化覆盖包含普通 API 主体字段排除、五项精确权限、四个写入的近期完整登录、五分钟单次凭证、三个签名范围、签名重放与内容冲突、本地身份幂等、成功和可重试失败状态、明文缓冲清零以及适配器关闭。

唯一仓库级警告是上述无关宿主页面的六个翻译键缺失；外部阻断仍是腾讯与诺诺官方确认。二者都未被伪造为通过，也未授权装配真实供应商客户端。
