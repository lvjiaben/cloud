# 第十阶段企业认证后台审核验证记录

## 当前结论

任务`10.3.c`的后台企业认证案件目录、脱敏详情、原件短期单次访问、逐次查看审计、开始审核、批准、驳回和重审闭环已经实现。任务范围内真实 PostgreSQL、全后端随机顺序二十轮、`go vet`、官方插件 lint、插件治理和 Windows/Linux 构建均通过。

此前唯一剩余的仓库级`i18n`阻断已由其所有者范围闭环；当前重新执行全仓双语门禁通过。本次没有修改`web-antd`业务源码。结合原有真实 PostgreSQL、二十轮随机后端测试、`go vet`、lint、插件治理和双平台构建证据，任务`10.3.c`可以完成。管理页面和浏览器`E2E`由`10.3.d`交付；客户企业认证页面的最终所有者已纠正为`web-customer`。

## 后台接口

| 方法与路径 | 精确权限 | 行为边界 |
|------------|----------|----------|
| `GET /admin/compliance/enterprise-verifications` | `idc:admin:compliance:enterprise-verification:list` | 返回无精确总数的稳定键集页，不返回法定明文、原件或存储元数据 |
| `GET /admin/compliance/enterprise-verifications/{caseNo}` | `idc:admin:compliance:enterprise-verification:query` | 返回一个案件、脱敏法定资料及最多八份材料元数据 |
| `POST .../{caseNo}/review-starts` | `idc:admin:compliance:enterprise-verification:review` | 以乐观版本和幂等键开始或重新开始人工审核 |
| `POST .../{caseNo}/approvals` | `idc:admin:compliance:enterprise-verification:approve` | 只批准服务端认证过的加密快照，请求不能提交或覆盖法定资料 |
| `POST .../{caseNo}/rejections` | `idc:admin:compliance:enterprise-verification:reject` | 只接受封闭、稳定且客户可操作的原因码 |
| `POST .../{caseNo}/materials/{materialNo}/access-contexts` | `idc:admin:compliance:enterprise-material:access` | 在近期完整登录有效时签发短期、会话和目标绑定的单次访问上下文 |
| `POST .../{caseNo}/materials/{materialNo}/content` | `idc:admin:compliance:enterprise-material:access` | 消费一次访问上下文，逐次审计后返回`no-store`附件 |

路由静态权限和服务层动态精确授权同时存在。服务层还要求当前直接员工、租户 0 和允许的数据范围，先授权再查询目标，不能通过存在性差异枚举案件或材料。

## 敏感原件访问边界

- 核心近期完整登录服务签发的证明经过签名，并绑定当前会话与固定长度的案件材料摘要；重放状态持久化在 PostgreSQL，同一上下文只能消费一次。
- 消费时再次执行精确权限和数据范围授权，在事务内锁定案件与材料并重查可用状态、留存状态和法律保全状态。
- 私有对象只读入有界内存，依次校验密文大小、`SHA-256`、密钥代次、认证加密信封和真实`PDF/JPEG/PNG`内容类型。
- 每次真正放行前先追加不可变业务审计。审计不记录对象路径、密文摘要、访问上下文、会话标识、法定资料或身份明文。
- 响应使用固定服务端附件名和`no-store`头，不返回私有对象地址，也不产生永久或可复用下载链接。

## 审核事务与重复登记围栏

批准请求只包含案件号、期望版本、稳定原因码和幂等键。服务在批准事务内锁定案件，重新解密并认证法定资料快照，同时集合化锁定最多八份材料并验证其与快照声明完全一致；事务外预检不能代替该最终门禁。

批准后的企业名称、登记标识和法定代表人投影由`idc-core`独占。登记标识通过核心键控摘要与数据库唯一围栏处理并发竞争。若批准事务发现另一企业已经占用相同登记标识，批准、核心投影和审计整体回滚，随后在新事务中把案件标记为`review_required`并记录重复风险，不提供强制覆盖。法定资料变更重审期间继续保留原已认证投影，只有新案件真正批准后才原子替换。

开始审核、批准和驳回均使用原始幂等键的摘要、请求指纹和乐观版本围栏；同请求安全重放，键相同但语义不同、版本过期或非法状态全部失败关闭。驳回原因来自封闭字典，不接受自由文本替代客户提示。

## 数据库与性能

迁移`004-enterprise-verification-management.sql`新增审核幂等元数据、封闭原因字典及三个企业案件目录索引。迁移和索引名称保留完整插件标识前缀，并在真实 PostgreSQL 中连续执行两次验证幂等。

案件目录默认 20 条、最大 100 条，使用`(submitted_at DESC, case_no DESC)`签名键集游标且不计算精确总数；游标绑定状态和客户筛选，使用微秒精度避免同毫秒跳行。无筛选、状态筛选和客户筛选分别有匹配索引。详情只执行一个案件查询和一个最多八条的材料集合查询，不按返回行数产生数据库`N+1`。

原件访问是单材料路径；批准时材料集合硬上限为八，不随数据库总量增长。该切片不引入缓存，PostgreSQL 和私有对象存储继续是权威状态。

## 依赖注入与所有权

| 依赖 | 所有者与创建位置 | 复用方式 |
|------|------------------|----------|
| 管理运行时 | `idc-core`发布并由合规插件在`registerRoutes`冻结工厂、使用宿主业务上下文、会话和授权服务构造 | 同一运行时供配置管理与企业审核使用，包含精确授权、业务审计和近期完整登录证明 |
| 企业法定投影 | `idc-core`的`identitycap` | 合规插件启动时通过源码 SPI 加载，批准只调用窄投影能力 |
| 材料密钥 | 核心`secretcap`，合规插件材料专属作用域 | 同一启动图一次绑定 accessor 和 generated manager，客户提交与后台审核复用 |
| 私有对象存储 | 宿主公开`storagecap` | 同一宿主实例注入客户提交与后台审核服务，不创建第二存储客户端 |
| 数据库 | 宿主`g.DB()` | 服务构造时显式注入，不创建第二连接池 |

控制器只做 DTO 映射，不自行查库、创建服务或访问密钥。合规插件不导入核心`internal`实现，不通过 HTTP、动态插件桥或跨插件表访问核心法定投影。

## 自动化验证

| 验证 | 结果 |
|------|------|
| `GOWORK=off go test ./backend/... -count=20 -shuffle=on`（合规插件） | 通过；全部后端包随机顺序连续二十轮成功 |
| `GOWORK=off go test ./backend/... -count=20 -shuffle=on`（核心插件） | 通过；包含管理运行时和近期完整登录证明测试 |
| 企业管理真实 PostgreSQL 用例`-count=5 -shuffle=on` | 通过；覆盖游标稳定性、权限隔离、单次原件访问、逐次审计、批准、驳回、重审和重复登记围栏 |
| `TestComplianceFoundationMigrationPostgreSQL` | 通过；迁移连续执行、19 个索引及字典约束一致 |
| `GOWORK=off go vet ./backend/...`（两个插件） | 通过 |
| `make.cmd lint.go plugins=1 dir=apps/lina-plugins/idc-compliance` | 通过；零问题及`staticcheck`通过 |
| `make.cmd lint.go plugins=1 dir=apps/lina-plugins/idc-core` | 通过；零问题及`staticcheck`通过 |
| Windows `GOWORK=off go build ./...`（两个插件） | 通过 |
| Linux amd64、`CGO_ENABLED=0`、`GOWORK=off go build ./...`（两个插件） | 通过 |
| `make.cmd plugins.check` | 通过；插件清单、文件边界和治理扫描零发现 |
| `openspec validate add-idc-server-operations-platform --strict` | 通过 |
| `make.cmd i18n.check`（当前复核） | 通过；运行时扫描、宿主与插件消息覆盖、宿主与插件前端键覆盖均无问题 |

仓库根级竞态测试在关闭`CGO`时被 Go race detector 的`CGO`要求阻断，开启`CGO`后又因当前 Windows 环境没有`gcc`而无法启动；这是工具链环境阻断，不替代上表两个插件二十轮随机测试、静态检查和双平台构建证据。

## 影响分析

- 接口：有影响。新增七个管理接口，时间使用 Unix 毫秒，枚举使用稳定英文代码，新增 DTO 均有完整说明和示例。
- 后端：有影响。新增独立管理服务、控制器和核心近期完整登录窄能力；插件测试、`go vet`和 lint 通过。
- 数据库：有影响。新增增量迁移、三个目录索引、审核幂等字段和封闭原因字典；真实 PostgreSQL 幂等验证通过。
- 数据权限：有影响。静态权限、直接员工、租户和数据范围均在查询或写入前校验，写路径在事务内重新授权和锁定。
- `i18n`：有影响。本任务新增错误、字典和 API 文档资源均覆盖中英文，当前仓库级门禁通过。
- 缓存一致性：无影响。本切片未引入缓存。
- 前端：无代码影响。菜单保持隐藏，页面和可观察交互由`10.3.d`承接。
- `E2E`：本切片为后端闭环，不新增页面；浏览器端质量审查和`TC003-enterprise-kyc.ts`由`10.3.d`承接。
- 开发工具与跨平台：无工具源码影响；没有修改根构建脚本，Windows 与 Linux 目标构建通过。
- 文档：有影响。更新 OpenSpec 增量规范、设计、任务记录和本验证记录。
- `AI`：无调用影响。企业原件、法定快照和查看内容继续禁止进入模型。

## Lina 审查报告

**变更：** `add-idc-server-operations-platform`

**范围：** 任务级`10.3.c`

**范围来源：** `git status --short`、未跟踪插件目录展开结果和当前任务上下文；审查覆盖约 105 个任务相关的生产、测试、生成、迁移、双语资源和 OpenSpec 文件。

**插件本地规范：** 两个相关插件根目录均无`AGENTS.md`，适用项目顶级规范。

**已读取规则：** `AGENTS.md`、OpenSpec、架构、数据权限、插件、接口、Go 后端、数据库、测试、`i18n`、文档和 Markdown 格式规则，并使用`lina-review`完成本次审查。

### 审查中已修复

- 把审核状态更新从通用映射改为生成`DO`结构，恢复 GoFrame 字段治理边界。
- 在批准事务内锁定并重新核对有界材料集合，消除事务外预检和最终投影之间的竞争窗口。
- 为客户筛选补充稳定游标索引，并把新增约束与索引统一为完整插件标识前缀。
- 把案件材料授权目标改为域分隔、固定长度摘要，消除最大长度和字符串拼接歧义。
- 补齐 DTO 示例、枚举说明、导出方法注释、错误处理和近期完整登录用途测试。

### 遗留发现

- **严重，范围外：** `apps/lina-vben/apps/web-antd/src/views/system/message/notice-preview-modal.vue`引用的`plugin.content-notice.preview.title`、`plugin.content-notice.fields.type`和`plugin.content-notice.fields.createdBy`缺少英文和中文资源，共六个键。根目录没有`.contributing`，当前插件任务禁止修改宿主页面，因此只能如实记录。

任务范围内没有遗留代码级阻断或警告。一次并行运行两个官方插件 lint 时，共享临时目录发生清理竞争；改为串行重跑后两个插件均通过，判定为工具临时目录并发碰撞，不是产品代码失败。

### 规则域结论

- OpenSpec、架构、插件、接口、数据权限、后端、数据库、性能和测试：任务范围内通过。
- `i18n`：插件新增内容通过，仓库门禁因上述宿主问题未通过。
- 前端 UI 与浏览器`E2E`：当前后端切片无代码影响，明确由`10.3.d`继续。
- 缓存一致性：无影响。
- 开发工具：无源码影响。

### 摘要

- 严重：1 个范围外仓库级门禁问题。
- 警告：0。
- 任务范围内待修复代码问题：0。

## 后续阻断复核闭环

以上审查报告保留实施当时的真实发现，不回写成当时已经通过。当前共享工作区中，通知预览缺键已由对应所有者范围闭环；重新执行`.\make.cmd i18n.check`通过，运行时违规为 0，宿主与插件的消息及前端键覆盖全部通过。本次没有修改`web-antd`业务源码或依赖。原外部阻断已经解除，因此`10.3.c`现可精确勾选。
