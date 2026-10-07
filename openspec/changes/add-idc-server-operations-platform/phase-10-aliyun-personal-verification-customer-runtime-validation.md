# 第十阶段阿里云个人实名客户运行时验证记录

## 完成范围

任务`10.2.d.1`至`10.2.d.4`已完成客户个人实名纵向运行时：独立敏感个人信息同意证据、客户本地状态、发起与权威重查`API`、服务端固定回跳、客户与系统操作者审计隔离、主节点有界恢复作业，以及`apps/lina-vben/apps/web-customer/`中的中英文页面、付款准备返回链和真实浏览器回归。旧 Portal 中英文页面、测试和截图继续保留为最终统一迁移前的行为基线，不再作为本任务目标交付证据。

`10.2.d.4a`中的合规插件`TC002`已由仓库标准 runner 实际执行 2/2 通过。`10.2.d.4b`在客户应用内按连续编号新增`TC004-personal-verification.ts`，4/4 通过；客户应用全套`TC001`至`TC004`共 23/23 通过。用例覆盖中文桌面、英文移动端、单独同意、伪造浏览器通过参数、供应商跳转失败关闭、权威回跳重查、付款准备恢复、身份明文不进入浏览器存储及最新截图审查。

## `web-customer`目标页面与付款返回链

| 边界 | 实现结果 |
|------|----------|
| 中英文告知 | `/account/identity`完整展示目的、必要性、处理信息、接收方责任、处理区域、保留边界和客户权利，明确技术账号由当前安装实例的 IDC 商家配置和承担 |
| 身份明文 | 姓名和身份证号只保留在当前表单引用与一次请求局部变量中，请求前清空输入，最终清空局部缓冲；浏览器存储、回跳参数、普通案件、任务和审计不保存明文 |
| 返回状态 | `verification_return=1`只在本地状态为`verifying`且存在所属尝试编号时调用权威重查；`status=verified`、`passed=true`等浏览器参数完全不参与状态决策 |
| 付款上下文 | 订单页生成白名单`/account/orders/{orderNo}`返回路径和受限付款准备编号，只写当前标签页`sessionStorage`；认证通过后调用`resumePayment`重检原上下文，不再次结账或创建订单 |
| 安全跳转 | 供应商继续地址必须为带真实主机名的 HTTPS；非 HTTPS、空主机、加载错误、未登录或企业客户走失败关闭，不提供前端强制通过 |
| 插件边界 | `web-customer`只通过统一请求封装调用合规插件`account`接口和核心插件付款准备接口，不直连数据库、供应商、管理接口或旧 Portal；合规不可用只关闭实名旅程，不在浏览器复制认证真相 |

## 客户接口与权威边界

| 边界 | 实现结果 |
|------|----------|
| 所有权 | 四个客户接口只接入宿主`Auth + Tenancy`；`identitycap`从当前登录主体推导活动个人客户，请求不接受客户号、租户号或他人对象号 |
| 本地读 | `GET /account/compliance/personal-verification`仅使用两次有界 PostgreSQL 查询返回最新案件和尝试；未发起时返回`unverified`，不构建阿里云客户端也不执行供应商查询 |
| 单独同意 | `POST /account/compliance/personal-verification-consents`只接受明确`true`、服务端当前政策版本和`zh-CN/en-US`。同意追加到`core customer_consent`，原始幂等键不持久化；证据精确绑定固定政策标识、版本、语言、客户和当前操作者 |
| 发起 | `POST /account/compliance/personal-verifications`只消费最新有效的同一政策同意编号；姓名和身份证号在控制器与服务缓冲中清理，不进入常规行、审计或任务 |
| 权威重查 | `PUT /account/compliance/personal-verifications/{attemptNo}/authoritative-state`先验证该尝试属于当前客户，再主动查询阿里云；浏览器查询参数不参与结果决策 |
| 回跳 | 启动装配从`LINAPRO_IDC_CUSTOMER_PUBLIC_ORIGIN`构造固定`/account/identity?verification_return=1`，并继续拒绝请求体、Host、Referer 和调用方回跳值 |

## 审计与恢复作业

`core`的源码能力运行时一次注入身份服务、当前客户审计 writer 和可信系统审计 writer。客户发起与权威查询使用当前请求者；恢复作业使用固定系统身份`plugin:idc-compliance:personal-verification-recovery`，不解析登录会话、请求体或任务载荷中的操作者。

合规插件以阻塞模式向`pluginhost.Jobs`注册一个主节点`@every 1m`作业，每批最多 50 条。恢复只扫描`initialization_unknown/ready/pending`和超过两分钟的`preparing`；只对已持久化`certifyId`的`ready/pending`调用`query`，不进入`Initiate`。它继续处理同批其他项，但会返回首个项级失败供宿主记录与重试，不会用“成功作业”掩盖持久化或审计失败。

## 自动化验证

| 验证 | 结果 |
|------|------|
| `GOWORK=off go test ./backend/api ./backend/internal/controller/account ./backend/internal/runtimeconfig ./backend/internal/service/cron ./backend/internal/service/personalverification -count=20 -shuffle=on` | 通过；覆盖中文`apidoc`键完整性、四个客户接口语义隔离、固定回跳、本地读零供应商查询和主节点恢复注册 |
| 合规插件设置隔离`DSN`后执行`GOWORK=off go test ./backend/internal/service/personalverification -run '^TestPersonalVerificationPostgreSQL' -count=3` | 通过；随机 schema 覆盖`unverified/verifying/verified`本地读、权威通过、终态事务回滚、恢复项错误传播和`init`零重放 |
| 核心插件设置隔离`DSN`后执行`GOWORK=off go test ./backend/internal/service/identityprojection -run '^TestIdentityProjectionPostgreSQL' -count=3` | 通过；覆盖同意精确幂等、原始键不持久化、跨客户隔离、固定政策范围、最新授权和业务号分配 |
| 两插件分别执行`GOWORK=off go test ./backend/... -run '^$' -count=0` | 通过；全部后端包完成编译 |
| 分别执行`make.cmd lint.go dir=apps/lina-plugins/idc-core plugins=1`与合规插件同项命令 | 通过；两插件`golangci-lint`和定向`staticcheck`零问题 |
| `make.cmd plugins.check` | 当前工作区通过；扫描 13 个清单、2323 个插件文件，零治理发现 |
| `openspec validate add-idc-server-operations-platform --strict` | 通过 |
| `make.cmd i18n.check` | 当前工作区通过；运行时硬编码、消息覆盖和前端键覆盖均无问题 |
| `pnpm --filter @lina/web-customer typecheck` | 通过 |
| `pnpm --filter @lina/web-customer i18n:check` | 通过；中英文各 9 个客户应用语言文件键集合一致 |
| `pnpm --filter @lina/web-customer check:alignment` | 通过；客户应用继续保持 Vben/Vite 母版结构，不引入 Nuxt |
| `pnpm exec tsc --noEmit -p apps/web-customer/tests/tsconfig.json` | 通过；客户应用页面对象、假服务和 E2E 严格类型检查零错误 |
| `pnpm exec vitest run apps/web-customer/src/api/idc/personal-verification.test.ts apps/web-customer/src/api/idc/customer-commerce.test.ts` | 2/2 通过；接口路径、方法、不透明付款上下文和参数最小化符合约束 |
| `pnpm --filter @lina/web-customer build` | 通过；Vite 生产构建完成并生成`dist.zip`，只有既有静态/动态导入提示，没有构建失败 |
| 客户应用执行`pnpm exec playwright test --config apps/web-customer/tests/playwright.config.ts` | 23/23 通过；其中个人实名`TC004`为 4/4，且既有工单、通知、售前和合同经办人 19 个场景无回归 |
| `GOWORK=off go test ./backend/internal/runtimeconfig ./backend/internal/controller/account -count=1` | 通过；覆盖新客户公开来源变量、固定回跳和客户控制器编译 |
| `make.cmd lint.go plugins=1 dir=apps/lina-plugins/idc-compliance` | 当前工作区通过；`golangci-lint`和定向`staticcheck`零问题 |
| `pnpm test:validate` | 通过；治理器识别 17 个范围内 217 个 E2E 文件，编号与目录规则无问题 |
| Portal 执行`pnpm run build` | 通过；`vue-tsc`零错误，Vite 生产构建生成`v0.1.7`版本化资产与独立`IdentityCenter`异步块 |
| Portal 执行`GOWORK=off go test ./backend/... -count=20 -shuffle=on` | 通过；嵌入、路由和交付契约 20 轮随机顺序稳定 |
| `make.cmd lint.go plugins=1 dir=apps/lina-plugins/idc-portal` | 通过；`golangci-lint`与定向`staticcheck`零问题 |
| 两插件各自 E2E `tsconfig`执行`tsc --noEmit` | 通过；新增页面对象、假服务、`TC002`和`TC009`严格类型检查零错误 |
| 合规插件配置执行`TC002-personal-verification-account.ts` | 2/2 Chromium 通过；覆盖四类 API 请求最小化、无客户/租户/回跳字段和未登录失败关闭 |
| 仓库标准 runner 执行合规插件模块 | 23/23 通过；其中`TC002-personal-verification-account.ts`为 2/2 |
| 旧 Portal 公开配置执行`TC009-personal-verification.ts` | 历史 5/5 Chromium 结果只保留为行为迁移基线，不作为`web-customer`完成证据 |
| `web-customer`关键截图 | 已人工审查`temp/20260816/084621`至`084631`生成的中文单独同意、认证通过、权威待确认、不安全跳转、英文移动端顶部与同意区；桌面层级清晰，移动端无横向溢出、悬浮按钮遮挡、原始翻译键、技术错误或身份明文 |

## 影响分析

- `i18n`：有影响。后端继续维护个人实名状态字典和完整简体中文`apidoc`，`web-customer`新增同键的`zh-CN/en-US`页面语言资源；仓库级和客户应用级双语检查均通过。
- 数据权限：有影响。所有客户读写在目标存在性查询前由当前主体派生客户所有权；后台权限不与客户自助入口复用，系统作业使用固定可信操作者。
- 性能：客户本地读取固定两次投影查询；同意、发起和权威查询均为单目标固定路径。恢复批次上限 50，只在该有界工作集中按尝试执行带版本围栏的独立权威查询。
- 缓存一致性：无影响。同意、案件、尝试和`core`实名投影全部以 PostgreSQL 为权威，不新增进程缓存。
- SQL：无新增迁移。同意复用核心插件既有追加式`customer_consent`和业务号账本；状态与恢复复用`10.2.a/10.2.c`已验证迁移与索引。
- 开发工具：无影响。没有修改根构建、代码生成、CI 或 runner 脚本。

## 目标交付门禁结果

- `web-customer`已经提供`/account/identity`目标页面、个人实名与付款准备 API 客户端、中英文资源、应用内`TC004`和构建产物；标准 runner、全仓双语、E2E 治理、测试 TypeScript、应用类型检查和结构对齐均通过。
- 后端部署变量已经改为`LINAPRO_IDC_CUSTOMER_PUBLIC_ORIGIN`，旧 Portal 变量名不再作为运行时配置入口。
- `web-antd`已通过跟踪与暂存差异双重检查，当前没有本任务产生的任何差异；旧 Portal 没有删除、回滚或继续扩写，仍等待最终统一迁移门禁。
- 因此`10.2.d.3`、`10.2.d.4b`、`10.2.d.4`、`10.2.d`和`10.2`具备完成条件；下一项严格进入`10.3.d.4`企业认证客户页面。

## 强制审查结论

本切片按`lina-review`从`git status --short`和未跟踪文件展开结果确定任务级范围，共复核 17 个个人实名迁移相关源码、语言、路由、测试、运行配置、证据和任务文件。实际读取`AGENTS.md`、`.agents/rules/user-preferences.md`、`openspec.md`、`documentation.md`、`architecture.md`、`plugin.md`、`api-contract.md`、`backend-go.md`、`frontend-ui.md`、`testing.md`和`i18n.md`，并使用`goframe-v2`与`lina-e2e`检查 Go 和 E2E 边界。合规插件和客户应用根目录均不存在额外`AGENTS.md`。

审查发现并闭环以下问题：

- 收紧最新敏感信息同意的有效性检查，要求同时匹配本次个人实名固定政策标识与版本，防止其他敏感信息同意被误当成实名授权。
- 回跳源和供应商 H5 继续地址同时补充真实主机名检查，拒绝`https://:443`类似看似 HTTPS 但没有主机的边界值。
- 恢复批查补充投影`status_code`，使`Queried`指标反映真实可查询尝试；项级持久化或审计失败现在处理完有界批次后向宿主返回首个错误，避免作业误报成功。
- 客户应用英文场景最初被假服务固定中文覆盖，已让公共假服务按场景返回确定语言，并以全量 23/23 回归证明既有中文场景未受影响。
- 英文移动端同意区最初被悬浮设置按钮遮住部分文字，已把偏好入口移到顶部栏并重新执行截图审查，功能入口仍然保留。

任务级审查结果为严重问题 0、警告 0，未发现阻塞问题。当前改动没有新增缓存、数据库迁移、数据权限算法、运行期依赖注入、开发工具或脚本；客户接口均为单目标有界请求，不产生列表瀑布或`N+1`。剩余风险仅是生产阿里云账号和真实身份材料不在本地自动化中触网验证，该限制已由既有官方假服务契约、后端状态机和购买方运行时失败关闭门禁覆盖，不阻塞本地任务完成。
