# 第四阶段营销效果、反滥用与优惠码验证记录

## 当前结论

本轮完成了第四阶段营销效果报表、反滥用复核、客户优惠码兑换和后台批量优惠码交付的本地闭环复核。效果事实只从曝光、优惠资产、订单、实收分摊、退款分摊和订单成本快照等本地权威数据生成；报表按活动和币种分行，纯积分只保留次数，不参与金额合计。多活动订单的实收、退款和成本先按实际优惠额整数比例分摊，不能整除的余数按优惠额降序、活动号升序补齐。

本轮新增真实 PostgreSQL 回归，直接证明三种容易出错的余数分配：`101`按`60:40`分成`61/40`，成本`7`按`60:40`分成`5/2`，退款`5`按相同优惠额分成活动号较小者`3`、较大者`2`；同一批权威事实重放不新增重复记录。生产查询同时改为显式字段投影，不再使用通配字段，避免表结构扩展后把无关列带入高频对账路径。

后台批量优惠码按活动和权益规则一次生成`8`至`24`位大写字母数字码，客户输入大小写等价。持久化只保存用途隔离摘要；完整码只存在于本次生成响应并以真实`XLSX`下载，历史查询不回显。`web-customer`的“我的优惠”使用自己的页面和接口客户端，失败统一显示“优惠码无效”；设备摘要或`IP`摘要任一来源十分钟累计十次错误即建案并暂停该来源三十分钟。

反滥用案件目前已覆盖新人活动显式开关、同实名第二次领取、同设备或`IP`二十四小时三账号、优惠码暴力尝试、同实名自荐和三十天营销退款套利。案件列表、效果读取、案件处理、成本缺失列表、成本补录及批量优惠码分别使用独立精确权限。复核必须填写二至五百字原因并二次确认；确认违规只限制该客户以后参加涉及活动，不自动封号、取消已付款订单、扣余额或追回既有资金和权益。

## `AFF`同实名自荐收口

第十二阶段`AFF`已经提供权威链接和触点，原缺口现已闭环。后台归因接口删除了可由请求方伪造的`relatedIdentity`字段；服务端在归因事务中用固定一次集合查询比较推荐人与被推荐人的个人或企业已认证摘要。相同客户直接命中自荐，不同客户只有在主体类型相同且已认证摘要相同时才命中同实名自荐。

命中后本次推荐奖励立即以`rejected_self_referral`拒绝；不同客户同实名使用稳定原因码`same_identity`。拒绝归因和`signal_code=self_referral`的统一营销待复核案件在同一事务追加，案件只保存关联账号数量、稳定摘要和脱敏结论，不保存身份证号、企业登记号或认证摘要。重放由确定性归因号、案件号、事务锁和唯一约束收敛，不重复建案。仓库现已把`staticcheck`从不支持 Go 1.27 的`v0.7.0`升级并锁定到正式版`v0.8.1`；Go 1.27 下完整`make lint`、真实 PostgreSQL 回归和任务级`lina-review`全部通过，`4.10.b.3.6`、`4.10.b.3`、`4.10.b`、`4.10`和`4.11`已完成。

## `4.11`覆盖盘点

| 要求 | 当前测试证据 | 结论 |
|---|---|---|
| 配置兼容 | `catalog_configuration_test.go`覆盖固定和可配置模型，`catalog_compatibility_management_test.go`覆盖三类兼容规则，发布完整性测试拒绝停用规格引用 | 已覆盖 |
| 所有计费模型 | `catalog_billing_test.go`覆盖固定、阶梯、按量、试用、安装费、续费、上下限、舍入和整数溢出 | 已覆盖 |
| 多币种缺价 | `catalog_currency_pricing_test.go`按币种建立完整价格索引并拒绝缺价或畸形阶梯；发布测试要求所有启用币种维度完整 | 已覆盖 |
| 发布版本 | `catalog_publication_test.go`覆盖发布依赖、独立权限、价格、配置、税务完整性；真实 PostgreSQL 商城回归覆盖在售版本被替换后旧报价拒绝 | 已覆盖 |
| 优惠并发核销 | `TestPromotionReservationPostgreSQLConvergesQuotaAndLifecycle`覆盖并发配额收敛、报价范围唯一预占和生命周期释放 | 已覆盖 |
| 活动反滥用 | 营销真实 PostgreSQL 回归覆盖新人、注册摘要、错码暂停、退款套利、案件复核和限制边界；销售渠道真实 PostgreSQL 回归覆盖服务端同实名判定、拒绝归因、统一案件和重放幂等；`TC-3i`覆盖后台可见交互 | 已覆盖 |

`4.11`的六类测试要求均已有自动化证据；同实名自荐没有使用接口布尔值或测试伪造关系，而是由服务端读取实名认证权威投影后完成真实 PostgreSQL 集成回归。

## 浏览器与构建证据

| 命令或检查 | 结果 |
|---|---|
| 插件真实 Chromium `TC-3h` | 通过；批量生成后得到可解压的真实`XLSX`，表头、完整码和工作表结构正确，刷新后不再回显完整码 |
| `web-customer`真实 Chromium `TC016` | `2/2`通过；覆盖中文兑换、统一错误提示、输入转大写、成功领取和英文移动端页面 |
| 插件真实 Chromium `TC-3i` | `1/1`通过，用时`50.7s`；覆盖效果报表、防刷复核、独立权限、历史成本补录和二次确认 |
| 插件前端`pnpm run build` | 通过；类型检查、生产构建和插件产物校验均通过 |
| `web-customer`的`pnpm run typecheck`、`pnpm run i18n:check`和`pnpm run build` | 全部通过；双语键覆盖通过并生成生产产物 |

## 后端与治理证据

| 命令或检查 | 结果 |
|---|---|
| 设置本机隔离`LINAPRO_IDC_CORE_TEST_DSN`后，`GOWORK=off go test ./backend/internal/service/marketing -run 'TestEffectReconciler\|TestMarketingInsights' -count=1 -parallel=1` | 通过，用时`3.152s` |
| 设置本机隔离`LINAPRO_IDC_CORE_TEST_DSN`后，`GOWORK=off go test ./backend/internal/service/marketing -count=1 -parallel=1` | 通过，用时`13.947s`；真实 PostgreSQL 营销全包回归通过 |
| 设置本机隔离`LINAPRO_IDC_CORE_TEST_DSN`后，`GOWORK=off go test ./backend/internal/service/catalog -count=1` | 通过 |
| 设置本机隔离`LINAPRO_IDC_CORE_TEST_DSN`后，`GOWORK=off go test ./backend/internal/service/commerce -count=1 -parallel=1` | 通过；包含历史成本补录与冲回，用时`350.770s` |
| `LINAPRO_IDC_CORE_TEST_DSN='pgsql:postgres:123456@tcp(127.0.0.1:5432)/postgres?sslmode=disable'`且`GOWORK=off go test ./internal/service/saleschannel -count=1 -v` | 通过；随机隔离 schema 覆盖正常归因、重复归因、服务端同实名判定、拒绝归因、统一案件内容和重放幂等 |
| `GOWORK=off go test ./api/admin/... ./internal/controller/admin/... ./internal/service/saleschannel -count=1` | 通过；接口字段删除、生成接口、聚合控制器和销售渠道服务可编译 |
| `GOWORK=off go test ./... -count=1` | 当前改动相关包均通过；仓库根测试因既有`idc-channel/backend/api/channel/v1/channel.go`在客户/公开元数据中声明管理员权限而失败，与本轮文件无关 |
| 交接前历史记录中的`make.cmd lint.go plugins=1 dir=apps/lina-plugins/idc-core` | 当时通过；只作为历史证据，不代替当前工作区门禁 |
| Go 1.27 下重跑`make.cmd lint.go dir=apps/lina-plugins/idc-core plugins=1` | 通过；`golangci-lint v2.12.2`为`0`项问题，`staticcheck 2026.2.1 / v0.8.1`的`U1000`检查通过 |
| `make.cmd i18n.check plugins=1` | 通过；运行时文案、后端错误键和前端键覆盖均通过 |
| `make.cmd plugins.check` | 通过；扫描`3,450`个文件，`0`项问题 |
| `node hack/tests/scripts/validate-e2e.mjs` | 通过；扫描`230`个文件、`17`个执行范围 |
| `openspec validate add-idc-server-operations-platform --strict` | 通过 |

## 影响分析

- 架构与插件：营销后台页面和接口仍全部闭环在`idc-core`插件；客户兑换页面只在`web-customer`；本轮没有修改`web-antd`。
- 销售渠道接入：`saleschannel`只读取同插件客户实名认证权威投影，并向同插件已有的营销案件表追加统一案件；没有新增第二套反滥用模型。一次主体比较、拒绝归因和案件追加都在同一数据库事务内完成，没有逐账号循环查询。
- 数据权限与审计：所有管理读取和写入继续使用互不替代的精确权限；成本补录、案件处理、新人开关和批量生成均使用稳定请求号和不可变员工审计。
- 性能：效果补齐任务每类单批最多`100`条，使用集合查询和批量插入；报表和缺失成本列表有时间范围、分页与响应上限，没有逐行追加查询。
- 数据库：本轮没有新增生产迁移。新增测试只在经过固定前缀校验的随机隔离 schema 中替换最小同构表，结束由既有测试清理逻辑删除该 schema，不接触默认 schema。
- 缓存：效果和案件读取直接使用本地 PostgreSQL 权威事实，本轮没有新增缓存、订阅或失效路径，因此无缓存一致性影响。
- `i18n`：营销插件和`web-customer`均有中英文资源并通过各自覆盖检查；仓库级检查也已通过。
- 开发工具：只修改仓库根`.staticcheck-version`，继续复用现有跨平台`go install`和版本匹配流程；没有修改`Makefile`、`make.cmd`、脚本或测试 runner。Windows 上的锁定版本安装、版本核对、定向完整 lint 和`linactl`工具测试均通过，Linux 与 macOS 继续使用相同 Go 模块版本和既有跨平台入口。

## Lina 审查报告

**变更：** `add-idc-server-operations-platform`  
**范围：** 任务级，`4.10.b.3.6`与`4.11`  
**审查文件数：** `29`个业务实现与测试文件，加仓库根`.staticcheck-version`和本阶段任务、证据文档  
**范围来源：** `git status --short`、`git ls-files --others --exclude-standard`、任务清单、本轮明确修改文件和历史代码生成记录  
**已读取规则文件：** `AGENTS.md`、`.agents/rules/user-preferences.md`、`.agents/rules/openspec.md`、`.agents/rules/documentation.md`、`.agents/instructions/markdown-format.instructions.md`、`.agents/rules/architecture.md`、`.agents/rules/data-permission.md`、`.agents/rules/plugin.md`、`.agents/rules/api-contract.md`、`.agents/rules/backend-go.md`、`.agents/rules/database.md`、`.agents/rules/testing.md`、`.agents/rules/i18n.md`、`.agents/rules/cache-consistency.md`、`.agents/rules/dev-tooling.md`；同时使用`goframe-v2`和`lina-review`技能。`idc-core`插件根不存在独立`AGENTS.md`。

### 发现的问题

- 未发现阻断问题。
- **警告**：`0`。
- **审查中已修复** `apps/lina-plugins/idc-core/backend/internal/service/saleschannel/saleschannel_affiliate.go:129`：原`AttributionRequest.RelatedIdentity`仍为导出字段，同插件其他 Go 调用方可以自行构造同实名结论。现已改为包内字段，并增加 API 契约测试，确保 HTTP JSON 不再接受`relatedIdentity`。
- **审查中已修复** `.staticcheck-version:1`：原锁定版本`v0.7.0`不支持 Go 1.27 导出数据。现锁定正式版`v0.8.1`，由现有跨平台工具入口自动安装并通过 Go 1.27 完整检查。

### 规则域结论

- 后端 Go：实现、导出面、错误处理、注释、事务和测试通过审查；无新增运行期依赖或构造函数，`DI`无影响。Go 1.27 下完整`make lint`通过。
- API：删除不可信`relatedIdentity`请求字段，控制器只传不透明链接、触点和被推荐客户编号；生成接口与控制器编译通过，并新增反射契约测试防止字段回归。
- 数据库与性能：无生产 SQL 或 DAO 变更；权威链接、触点、个人/企业实名关系均在一个 PostgreSQL 事务内读取，实名比较使用一次集合查询，无逐账号`N+1`。拒绝归因与统一案件原子追加并通过确定性编号和唯一约束幂等收敛。
- 数据权限：继续使用既有精确后台权限和单商家`tenant_id=0`边界，链接所有者与触点从数据库读取，被推荐客户由现有外键约束；本轮没有新增列表、范围扩大或身份原文暴露。
- 隐私：数据库案件只保存关联数量、稳定摘要和脱敏结论；认证摘要只在事务内参与比较，不写入归因、案件、接口、日志或审计正文。
- 缓存：无影响；所有判断直接读取 PostgreSQL 权威事实，没有新增缓存、失效或跨实例状态。
- `i18n`：无新增用户可见文案或翻译资源；当前`i18n.check`通过。删除 API 字段不会产生缺失翻译。
- 前端与 E2E：本次接续未修改页面、路由或用户可见交互，未触发新增 E2E 和截图审查；既有营销/销售渠道浏览器证据保持不变。
- 开发工具：只更新仓库版本锁定文件，没有修改`Makefile`、`make.cmd`、脚本或 runner。现有工具继续通过隔离的`go install`在 Windows、Linux 和 macOS 安装同一锁定版本；Windows 上已验证安装、版本核对、定向完整 lint 和相关`linactl`工具测试，跨平台调用语义没有变化。
- 文档与 OpenSpec：已更新任务、交接、路径和阶段证据；Markdown 结构及严格校验通过。

### 验证证据

- Go 1.27 下`go test ./api/admin/... ./internal/controller/admin/... ./internal/service/saleschannel -count=1`：通过。
- Go 1.27 与真实 PostgreSQL 下`go test ./internal/service/saleschannel -count=1 -v`：通过，随机隔离 schema 覆盖同实名拒绝、统一案件和幂等重放。
- Go 1.27 下`make.cmd lint.go dir=apps/lina-plugins/idc-core plugins=1`：通过；`golangci-lint v2.12.2`为`0`项问题，`staticcheck 2026.2.1 / v0.8.1`的`U1000`检查通过。
- `go test . -run 'TestEnsureGoLintStaticcheckBinary|TestGoLintVersionOutputMatches|TestGoLintStaticcheckMatrixInput' -count=1`：通过，覆盖版本匹配、缺失安装、版本替换和矩阵输入。
- `make.cmd plugins.check`：通过，`3,450`个文件、`0`项问题。
- `make.cmd i18n.check plugins=1`：通过。
- `openspec validate add-idc-server-operations-platform --strict`：通过。
- `git diff --check`：通过；只有共享工作区已有换行提示。

### 摘要

- 严重：`0`。
- 警告：`0`。
- 业务实现问题：审查中发现并修复`1`项；工具兼容问题修复`1`项，未发现其他代码阻塞问题。
- 任务状态：`4.10.b.3.6`、`4.10.b.3`、`4.10.b`、`4.10`和`4.11`完成。

### 建议操作

1. 后续升级 Go 主版本时，先核对并升级仓库锁定的静态检查工具，再使用同一跨平台入口验证全部受影响模块。
