# 第七阶段在线退款验证记录

> **`FB-52`失效声明：**本记录把订单项与服务器作为通用退款的并列目标，并把逐台停机、删除和`service_instance`影响写进核心模型，原结论已经失效，不能作为当前在线退款完成证据。现行退款先按订单项、数量和金额工作；只有冻结插件契约声明持续管理且已经产生长期权益时才允许选择权益并投影业务拥有方影响。服务器停机和数据删除只属于服务器插件。下文仅用于迁移对账和删除旧实现，不保留旧字段、接口或页面兼容层。

## 当前结论

任务`7.8.a`和`7.8.b`已经完成。客户退款范围、活动服务器影响、固定原因、自动/人工审核、高额重新登录和原渠道失败后的余额确认规则已经同步到长期偏好、设计与退款规范。

`core`新增了退款政策版本、退款申请、客户选择项、逐台服务器影响、原付款来源分配、服务操作锁、追加式决定和耐久执行命令共 8 张表。订单项选择和单台服务器选择分开保存；退款金额按选择项保存，停机、数据删除、终止要求和服务版本按服务器逐台冻结。

任务`7.8.c`已经完成。`core`现已提供客户退款影响预览、申请、原渠道失败后的同币种余额确认，以及后台人工审核、高额批准重新登录凭证和逐币种退款规则配置接口。申请、审核、拒绝/同意余额退款和规则配置均在同一权威事务内追加统一业务审计，审计失败会一起回滚。

任务`7.8.d`已经完成。`core`通过版本化`billingcap`能力发布退款执行领取和结果回报；`payment`只能领取已经批准且通过全部来源预检的原渠道退款命令，并保存渠道退款尝试、验签回调、主动查询和不可变证据。支付宝已经实现退款和退款查询，微信支付已经实现退款、退款查询和退款回调验签解密，易支付的自动退款、退款查询和退款回调继续明确关闭。

任务`7.8.e`和`7.8.f`已经完成。`web-customer`现已提供“我的退款”目录、订单退款入口、逐项金额与服务器影响预览、申请提交、原路退款结果和组合付款余额确认；`idc-core/frontend`现已提供退款目录、冻结明细与服务器影响详情、普通人工审核及高额批准的重新登录凭证流程。客户页面继续由`web-customer`拥有，后台页面和接口客户端继续由插件`frontend/`拥有，`web-antd`没有新增 IDC 业务源码、依赖、路由或多语言资源。

真实 PostgreSQL、权限与数据范围、并发防超退、版本冲突、自动/人工审核、高额批准、活动服务器影响和组合付款整笔暂停都已通过回归。客户`TC013`共 4 条、插件后台`TC010`共 3 条均通过；独立插件前端构建、客户类型检查与生产构建、双语治理、插件治理和 OpenSpec 严格校验均通过。因此父任务`7.8`可以完成。退款成功后的跨领域账本、服务、佣金和税务联动仍属于下一项`7.9`，退款日对账和差异处理仍属于`7.10`，本记录不提前完成这些任务。

## 关键边界

- 客户身份由当前活动客户主体派生，客户请求不接收后台客户编号。
- 退款申请、资格、影响、审核和余额确认由`core`拥有。
- 原付款来源按不可变付款分配拆分；渠道付款交给`payment`，余额和信用由`core`处理，人工收款与迁移资金进入人工执行。
- 活动服务退款冻结逐台服务版本和影响，并建立同一服务只能有一张活动退款锁的数据库门禁。
- 历史退款项、服务器影响、来源分配、政策版本和决定禁止修改或删除。
- `web-antd`没有新增 IDC 业务源码或依赖。
- 组合付款只要一个原付款来源明确不支持退款，整笔申请和全部未执行来源都暂停。客户拒绝后继续暂停；客户同意后，只为不支持的来源新增同币种余额指令，其余来源仍保留原路退款指令，再统一放行。
- `payment`收到的执行命令不包含退款资格、审批人、审核规则或客户可改金额；这些业务事实始终由`core`拥有。支付插件只核对已经成功的原支付尝试、通道、币种和金额。
- 渠道返回“处理中”或网络结果不明确时，支付插件只把本地尝试保存为处理中，不向`core`伪报成功，也不重新创建第二笔退款。后续使用同一退款单号主动查询；到固定第 8 次仍无法确认时冻结为`result_unknown`并交人工核对。
- 微信退款回调必须使用准确通道的凭据完成官方签名校验和报文解密，并与本地退款单号、通道、币种和金额全部匹配后才更新。重复的同一回调只复用原结果；终态冲突不会覆盖已保存事实。

## 验证结果

在本机配置的 PostgreSQL 中创建随机隔离 schema，执行依赖迁移后连续执行两次`052-online-refund.sql`：

```text
go test ./backend -run TestOnlineRefundMigrationPostgreSQL -count=1
ok lina-plugin-idc-core/backend
```

退款输入、订单项/服务器目标隔离、其他原因说明、原付款来源拆分、超额拒绝和服务影响分类单元测试：

```text
go test ./backend/internal/service/billing -run 'TestNormalizeRefund|TestAllocateRefund|TestRefundServiceImpact' -count=1
ok lina-plugin-idc-core/backend/internal/service/billing
```

新增迁移加入现有财务测试装配后，真实 PostgreSQL 财务全量回归：

```text
go test ./backend/internal/service/billing -count=1 -p=1
ok lina-plugin-idc-core/backend/internal/service/billing 35.948s
```

中英文退款字典与错误资源均通过 PowerShell JSON 解析。

客户自动部分退款、员工高额人工审核以及组合付款余额确认真实 PostgreSQL 回归：

```text
go test ./backend/internal/service/commerce -run 'TestOnlineRefundPostgreSQL(SubmitsAutomaticPartialRefund|RequiresFreshLoginForHighAmountApproval|HoldsMixedFundingUntilBalanceConsent)' -count=1 -p=1 -v
PASS
ok lina-plugin-idc-core/backend/internal/service/commerce 13.389s
```

客户和后台退款 API、控制器及生产装配编译门禁：

```text
go test ./backend/api/account/... ./backend/api/admin/... ./backend/internal/controller/account ./backend/internal/controller/admin ./backend -run TestDoesNotExist -count=1 -p=1
ok
```

支付插件渠道退款、主动查询、微信回调和结果未知保护的真实 PostgreSQL 回归：

```text
$env:LINAPRO_IDC_PAYMENT_TEST_DSN='pgsql:postgres:***@tcp(127.0.0.1:5432)/cloud?sslmode=disable'
go test ./backend/... -count=1 -p=1
ok lina-plugin-idc-payment/backend
ok lina-plugin-idc-payment/backend/internal/provider/alipay
ok lina-plugin-idc-payment/backend/internal/provider/epay
ok lina-plugin-idc-payment/backend/internal/provider/wechatpay
```

上述回归覆盖：支付宝退款单号与原交易号查询、微信退款查询、微信退款回调验签解密、易支付能力关闭、退款处理中不提前回报、主动查询确认、重复回调单次应用、金额币种不符拒绝，以及查询达到固定上限后进入`result_unknown`。

`core`退款 owner、版本化能力和组合付款整笔暂停回归：

```text
go test ./backend/cap/billingcap/... ./backend/internal/service/billing -count=1 -p=1
ok lina-plugin-idc-core/backend/cap/billingcap
ok lina-plugin-idc-core/backend/cap/billingcap/bridge
ok lina-plugin-idc-core/backend/cap/billingcap/spi
ok lina-plugin-idc-core/backend/internal/service/billing

go test ./backend/internal/service/commerce -run 'TestOnlineRefundPostgreSQL(SubmitsAutomaticPartialRefund|RequiresFreshLoginForHighAmountApproval|HoldsMixedFundingUntilBalanceConsent)' -count=1 -p=1 -v
PASS
```

当前工作区静态与治理门禁：

```text
.\make.cmd lint.go dir=apps/lina-plugins/idc-payment plugins=1
0 issues

.\make.cmd i18n.check
violations: 0

.\make.cmd plugins.check
findings: 0

openspec validate add-idc-server-operations-platform --strict
Change 'add-idc-server-operations-platform' is valid
```

客户退款页面与 B 方案组合付款回归：

```text
pnpm --filter @lina/web-customer i18n:check
21 locale files / 2 locales passed

pnpm --filter @lina/web-customer check:alignment
passed

pnpm --filter @lina/web-customer typecheck
passed

pnpm --filter @lina/web-customer build
passed

pnpm exec playwright test apps/lina-vben/apps/web-customer/tests/e2e/TC013-online-refund.ts --workers=1
4 passed
```

`TC013`覆盖逐项部分退款、活动服务器停机与数据删除警告、原路退款结果，以及组合付款任一来源不支持时整笔暂停；客户拒绝后继续等待，客户同意后只有不支持部分转同币种余额，其余来源仍原路退款并统一放行。中文桌面页和英文`390px`移动页截图已经逐张检查，没有原始翻译键、金额截断、弹层溢出或伪造成功提示。

插件后台退款目录与审核回归：

```text
cd apps/lina-plugins/idc-core/frontend
pnpm run build
vue-tsc passed
vite build passed
bundle verification passed

pnpm exec playwright test apps/lina-plugins/idc-core/hack/tests/e2e/TC010-online-refund-management.ts --workers=1 --project=chromium
3 passed
```

`TC010`覆盖英文冻结明细与服务器删除影响、普通人工驳回只提交摘要证据，以及高额批准先取得与同一审核意图绑定的重新登录凭证。截图审查发现并修复了英文界面仍优先显示中文明细的问题，最终英文截图显示`Production cloud server`和完整删除警告，没有原始翻译键、遮挡或加载提示残留。

当前插件后端、真实数据库与治理复核：

```text
$env:GOWORK='off'
go test ./backend/... -count=1
passed

$env:LINAPRO_IDC_CORE_TEST_DSN='pgsql:postgres:***@tcp(127.0.0.1:5432)/cloud_idc_phase10_e2e_v2?sslmode=disable'
go test ./backend/internal/service/commerce -run 'TestOnlineRefundPostgreSQL' -count=1
ok 6.964s

go test ./backend -run TestOnlineRefundMigrationPostgreSQL -count=1
ok 0.679s

.\make.cmd lint.go dir=apps/lina-plugins/idc-core plugins=1
0 issues; staticcheck passed

.\make.cmd i18n.check
violations: 0

.\make.cmd plugins.check
findings: 0

pnpm -C hack/tests test:validate
222 E2E files; 17 scopes; passed
```

真实 PostgreSQL 用例覆盖客户所有权、后台精确权限与可表示数据范围、并发申请防超退、乐观版本冲突、自动与人工审核、高额重新登录、活动服务影响冻结、来源拆分、组合付款整笔暂停、客户拒绝保持等待和同意后的原路/余额混合放行。详情固定使用两次集合查询装配选择项与服务器影响，列表在数据库完成过滤、排序和最多 100 条的 keyset 分页，不存在逐行补查。

## 7.8.e 与 7.8.f 审查记录

- 审查范围：客户退款 API 客户端、路由、页面、双语和`TC013`；插件后台退款 API 客户端、页面、权限清单、双语和`TC010`；`core`退款列表/详情 API、控制器、服务、SQL、真实 PostgreSQL 测试；本验证记录与`tasks.md`精确状态。
- 审查修复：英文后台详情原先无条件优先取中文明细，已改成当前语言优先、另一语言兜底，并由`TC-10a`和英文截图证明；退款返回 DTO 缺少的接口文档说明和示例已经补齐；英文截图助手已经同时等待中英文菜单加载提示消失。
- E2E 质量审查：本任务改变客户退款和后台审核的可观察流程，因此触发 E2E 质量审查。客户用例覆盖申请、影响警告、原路结果和 B 方案余额同意；后台用例覆盖英文翻译、不可变影响、拒绝、高额重新登录和批准。用例分别闭环在`web-customer`和插件自有目录，可单独运行，不依赖其他用例残留。
- 权限与数据范围：客户目录、详情、申请和确认都从当前活动客户主体派生，不接收客户编号；后台列表、详情和审核使用互不替代的精确权限，并在读取目标前拒绝模拟身份、不支持数据范围及非活动员工。单商家首版只接受可正确表达的全量或商家范围，不会把未知范围放大全量。
- 性能：客户与后台列表均采用最多 100 条的 keyset 分页；后台详情对明细与服务器影响各执行一次集合查询，再以内存映射装配，没有随返回行数增加的查询；前端一次列表、一次按需详情，不逐行补查。
- SQL：`052-online-refund.sql`是本迭代唯一退款迁移，使用完整插件表前缀、存在性保护、幂等索引与触发器重建、追加历史和禁止删除门禁；真实 PostgreSQL 迁移测试通过。退款证据不使用软删除，也不手工模拟 GoFrame 软删除。
- 显式依赖：客户与后台控制器复用启动期注入的退款服务；退款服务的数据库、当前主体读取、授权、重新登录和业务审计依赖均由插件启动装配逐项传入，没有在请求路径临时创建关键服务图。本轮前端修正没有新增运行期依赖。
- 缓存：退款、版本、权限结果、来源执行和余额确认继续以 PostgreSQL 与宿主权威服务为准，本任务不新增缓存或失效路径，因此没有缓存一致性影响。
- i18n：客户和插件后台都维护`zh-CN/en-US`运行时资源及中文`apidoc`资源；接口英文源文本、运行时键覆盖和截图均通过。客户填写的自由原因说明按原文展示，不伪造翻译。
- 工具与跨平台：本任务没有修改`Makefile`、`make.cmd`、构建脚本或运行工具；新增 E2E 继续使用仓库已有跨平台入口，测试截图位于根`temp/<日期>/`。
- 宿主边界：`git status --short -- apps/lina-vben/apps/web-antd`为空。`web-antd`只作为插件页面运行宿主参与构建和浏览器验证，没有 IDC 源码、路由、语言包或业务依赖改动。
- 最终审查结果：严重问题 0，警告 0。审查中发现的英文显示、接口文档和截图等待问题均已修复并重新验证。外部支付宝/微信真实商户小额退款仍属于商家凭据和供应商环境验收，不以本地测试伪造；它不影响本地 B 方案、权限、资金围栏和页面交付结论。

## 7.8.d 影响与审查记录

- 依赖来源：`billingcap`由`idc-core`拥有，`payment`只在插件启动装配中通过`billingspi.Freeze()`取得版本化工厂，构造一个`paymentattempt.Store`后显式注入`paymentflow.Service`；渠道适配器、凭据访问器和数据库同样在启动装配创建并由路由与托管任务复用，没有在请求或任务处理中临时创建服务图。
- 数据权限：退款资格、客户归属、审批和金额均在`core`完成。公开回调不使用后台人员权限，而是外部供应商回调的窄例外；它只能按准确通道凭据验签，并在数据库中核对既有退款尝试的通道、单号、币种和金额，不能创建退款或读取客户业务数据。
- 数据库：本任务复用`payment`既有退款尝试和不可变证据表，没有新增或修改 SQL 文件。领取、更新和重复回调均有状态围栏；真实 PostgreSQL 隔离测试覆盖迁移、并发安全投影和幂等结果。
- 性能：每次托管任务只领取一条有界退款命令，插件任务每轮最多处理 8 条；没有按动态列表逐项查库或跨插件循环调用，不存在本任务新增的`N+1`路径。
- 缓存：本任务不新增缓存、进程内权威状态或缓存失效路径；所有退款尝试、查询次数、终态和证据均以 PostgreSQL 为准。
- `i18n`：新增退款回调文档使用英文源文本，并补齐中文`apidoc`资源；渠道内部诊断不再写死中文，最终`make i18n.check`为零违规。
- 开发工具与跨平台：没有修改构建、代码生成、脚本、`Makefile`或运行工具，无跨平台工具影响。
- 前端与 E2E：`7.8.d`只改变后端内部执行、供应商回调和主动查询，没有新增页面、按钮或客户操作；因此不触发页面 E2E 和截图审查。客户与后台页面及浏览器验证由`7.8.e`、`7.8.f`完成。
- 插件边界：`idc-core`和`idc-payment`根目录均不存在插件本地`AGENTS.md`；支付插件只导入核心插件公开的`backend/cap/billingcap`，没有导入其他插件`backend/internal`或跨插件查表。`web-antd`、`web-customer`和官网均未在本任务修改。

本任务完成后的`lina-review`已读取`AGENTS.md`、`.agents/rules/user-preferences.md`、`.agents/rules/openspec.md`、`.agents/rules/documentation.md`、`.agents/rules/architecture.md`、`.agents/rules/plugin.md`、`.agents/rules/api-contract.md`、`.agents/rules/backend-go.md`、`.agents/rules/database.md`、`.agents/rules/data-permission.md`、`.agents/rules/testing.md`和`.agents/rules/i18n.md`。审查覆盖任务级`core`退款 owner、`payment`渠道适配/尝试/回调/查询、API 文档、双语资源和测试文件，共 27 个文件；严重问题 0，警告 0，未发现阻塞问题。剩余风险是未使用商家真实支付宝/微信商户号执行供应商沙箱或生产小额退款，该外部验收不由本地自动测试伪造；当前使用官方微信 SDK、签名报文测试、支付宝 RSA2 响应测试和真实 PostgreSQL 流程作为本地证据。

## 7.9 退款成功联动与人工接管

任务`7.9`已经完成。原实现只把渠道命令改成成功、失败或结果未知，没有把权威成功结果写入账本和账单退款分配，也没有形成整单完成围栏、佣金与税务联动事件、结果未知核对任务或资金已退后的服务回收接管。这会造成渠道已经退款但本地账单仍显示未退，或者服务仍运行却没有高优先级处置入口。

本次由`core`补齐以下闭环：

- 每个渠道或本地来源只有取得权威成功结果后，才在同一个`PostgreSQL`事务中写入唯一平衡退款流水、原账单明细、原支付分配和不可变退款分配；重复成功报告恢复原事实，不重复记账。
- 整单只有在全部当前有效命令都成功且都有退款分配时才进入`succeeded`。成功后发布`billing.refund.succeeded`、`billing.refund.commission_reversal_requested`和`billing.refund.tax_adjustment_requested`耐久事件；逐来源另发布`billing.refund.source_posted`。
- B 方案执行顺序固定为先完成所有渠道预检；仍有原路渠道来源时，先取得渠道权威成功，再执行本地余额或同币种余额替代。渠道明确拒绝进入`manual_confirmation`，结果不明进入`result_unknown`，两者都创建优先级`100`的`refund_result_reconciliation`任务并停止后续来源，不自动把整单标成失败或释放冲突锁。
- 退款成功且需要终止服务时，保持服务当前真实运行状态和活动退款锁，为每台受影响服务创建优先级`100`的`refund_service_recovery`任务及`billing.refund.service_recovery_required`事件。没有资源回收要求时才释放退款锁。
- 本地退款`worker`由宿主主节点每`10`秒托管执行，每轮最多处理`20`条；事件服务和任务服务都由启动装配显式创建并注入，没有在业务构造函数内另建隐藏服务图。

真实`PostgreSQL`回归：

```text
$env:LINAPRO_IDC_CORE_TEST_DSN='pgsql:postgres:***@tcp(127.0.0.1:5432)/cloud_idc_phase10_e2e_v2?sslmode=disable'
$env:GOWORK='off'
go test ./backend/internal/service/commerce -run 'TestOnlineRefundPostgreSQL' -count=1 -v
PASS
ok lina-plugin-idc-core/backend/internal/service/commerce 10.175s
```

该回归覆盖渠道成功后的账本和账单分配幂等、结果未知不入账、渠道明确拒绝保持整单人工确认、B 方案在渠道预检前不执行本地来源、客户同意后的混合原路/余额完成、服务仍运行时保持锁并生成最高优先级回收任务，以及完整退款事件数量。

当前工作区全后端和治理门禁：

```text
$env:GOWORK='off'
go test ./backend/... -p 1 -count=1
passed

.\make.cmd lint.go dir=apps/lina-plugins/idc-core plugins=1
0 issues; staticcheck passed

.\make.cmd i18n.check
violations: 0

.\make.cmd plugins.check
findings: 0

openspec validate add-idc-server-operations-platform --strict
Change 'add-idc-server-operations-platform' is valid
```

全后端连接公共测试库时，内容中心的保护代码曾主动拒绝数据库`cloud_idc_phase10_e2e_v2`，因为它只允许在专用数据库`linapro_idc_codegen`内创建和清理随机隔离`schema`。改用该专用库单独执行`go test ./backend/internal/service/contenthub -count=1 -v`后全部通过；没有修改或绕过内容中心的安全保护。退款真实数据库回归始终使用原公共测试库并全部通过。

## 7.9 Lina 审查报告

- 变更与范围：`add-idc-server-operations-platform`任务级`7.9`，审查退款`owner`、账本和账单联动、本地`worker`、托管作业、启动装配、真实`PostgreSQL`测试及本节`OpenSpec`记录，共`18`个任务相关文件。
- 范围来源：`git status --short`、未跟踪文件展开、OpenSpec 任务上下文和`openspec status --change add-idc-server-operations-platform --json`。共享工作区其他未提交内容不属于本任务，没有覆盖或回滚。
- 已读取规则：`AGENTS.md`、`.agents/rules/user-preferences.md`、`.agents/rules/openspec.md`、`.agents/rules/documentation.md`、`.agents/rules/architecture.md`、`.agents/rules/data-permission.md`、`.agents/rules/plugin.md`、`.agents/rules/backend-go.md`、`.agents/rules/database.md`、`.agents/rules/testing.md`和`.agents/rules/i18n.md`；同时使用`goframe-v2`和`lina-review`。插件根目录不存在独立`AGENTS.md`。
- 审查修复：发现退款`owner`和本地`worker`在构造函数内临时创建事件与任务服务，已改为启动装配逐项显式注入；发现渠道明确拒绝会把整单直接标成失败并释放锁，已按用户确认的 B 方案改为人工确认并停止后续来源；发现本地来源可能早于渠道预检执行，已增加预检和渠道成功围栏；服务影响读取增加`1001`行探测和`1000`行硬上限。
- 架构与插件：`core`继续拥有退款资格、账本、账单和完成事实；`payment`继续只报告渠道权威结果。跨领域后续处理使用耐久事件，不导入其他插件内部实现，不修改`lina-core`、`web-antd`、`web-customer`或官网。
- 数据权限：本任务没有新增人员或客户接口。托管`worker`只消费已经由客户隔离、员工精确权限、审核和版本门禁形成的耐久命令，不能用任意编号创建或扩大退款，因此没有新增数据权限入口。
- 数据库与性能：没有新增`SQL`迁移、表或生成文件，复用`7.8`既有表；所有资金、账单、事件和任务写入同一事务。每轮本地`worker`最多`20`条，每次只领取一条；服务影响最多读取`1001`行并拒绝超过`1000`行，不存在动态无上限列表或逐服务补查。
- 测试：真实 PostgreSQL 用例各自创建和清理场景，顺序无关；新增成功、未知、明确拒绝、B 方案暂停和服务回收异常路径。托管作业单元测试覆盖仅主节点注册、固定批量和错误向上传递。任务没有新增页面或改变现有页面交互，独立 E2E 收口仍按`7.13`执行，因此本项不提前使用旧页面测试勾选`7.13`。
- `i18n`：只新增内部状态、任务类型和事件名，没有新增用户可见文案、接口文档或语言资源，确认无`i18n`资源影响；全仓`i18n`门禁仍通过。
- API、缓存与工具：没有新增或修改 HTTP API、DTO、缓存、失效路径、Makefile、脚本或代码生成输入；无需 API、缓存或跨平台工具补偿验证。
- 审查结论：严重问题`0`，警告`0`，未发现阻塞问题。剩余外部风险仅为商家真实支付宝或微信账号的生产小额退款验收，本地没有伪造该外部证据；佣金具体冲回由任务`12.5`消费本次事件，退款日对账由`7.10`继续实现，均未提前勾选。

## 后续门禁

下一步先返回当前最早且已经具备执行条件的任务`6.13`，补齐多次部分退款、合并/拆分账单退款和信用各阶段退款等真实测试，再处理`6.14`。任务`1.7`最后一个子项仍需等待各插件功能开关全部落地，`4.10.b.3`仍需等待效果权威事件完备，`6.9.b.2`仍需安装后隔离支付通道；这些依赖项不得伪造完成。完成第六阶段可执行缺口后，才继续`7.10`，且不能用`7.8`页面或`7.9`后端联动证据提前勾选后续任务。
