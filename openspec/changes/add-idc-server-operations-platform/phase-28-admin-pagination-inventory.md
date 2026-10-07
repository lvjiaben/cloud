# 第二十八阶段：后台分页专项清单

## 范围与状态

本清单是`FB-110`的只读源码盘点及渠道实施记录，不替代[阶段主文](phase-28-vben-pagination-validation.md)。盘点时间为`2026-09-04`；不含客户中心、生成产物、测试页和无表格的卡片列表。以下按业务源码文件去重，共`56`个：正式后台非树形表格`37`个、默认隐藏技术页面`8`个、树形表格`4`个、另外交付的独立表格入口`7`个。一个文件中的多张表或详情子表不重复计文件数。表格中的“分页现状”保留实施前基线，最终收口结果以本文件末尾和[阶段主文](phase-28-vben-pagination-validation.md)为准。

## 已有通用能力

- 宿主`packages/effects/plugins/src/vxe-table/use-vxe-grid.vue`提供完整官方分页布局；`web-antd/src/adapter/vxe-table.ts`已规定默认每页`10`条，可选`10/20/50/100`。
- `vben-adapter/frontend/src/isolated/runtime.ts`原来固定关闭分页，独立插件无法传入总数及翻页回调；由公共适配器任务增加公开分页参数，业务负责真实查询。
- `idc-core/frontend/pages/components/vben-data-grid.vue`原来固定关闭分页，营销页面使用该包装组件后仍在外部手写翻页。
- 只换分页按钮不能修复数据问题：部分接口只有游标，部分查询固定截断`100/200/500`条，部分响应把已加载数组长度当总数。
- 浅色主题沿用灰色表头，数据、空态、搜索、工具栏、分页为白色，外层冷灰页面不变。本任务不重画分页或修改全局主题。

## 正式后台非树形表格：37 个文件

以下插件路径均相对`apps/lina-plugins`，宿主路径相对`apps/lina-vben/apps/web-antd/src/views`。

| 文件 | 分页现状与数据来源 |
| --- | --- |
| `idc-core/frontend/pages/customer-management.vue` | 手写下一页；`customer-client.ts`调用`admin/customers`游标接口，当前页长度不能当总数 |
| `idc-core/frontend/pages/catalog-management.vue` | 手写游标翻页；`catalog-client.ts`调用`admin/catalog/products` |
| `idc-core/frontend/pages/commerce-orders.vue` | 手写游标翻页；`commerce-client.ts`调用`admin/commerce/orders` |
| `idc-core/frontend/pages/assisted-quotes.vue` | 主列表游标；`assisted-quote-client.ts`调用`admin/commerce/assisted-quotes`；单笔报价明细另属有界详情 |
| `idc-core/frontend/pages/product-fulfillment-tasks.vue` | 手写游标翻页；`product-fulfillment-client.ts`调用`admin/product-fulfillment/tasks` |
| `idc-core/frontend/pages/financial-management.vue` | 账单、流水、调账三表游标；`financial-client.ts`调用`admin/billing/*` |
| `idc-core/frontend/pages/components/refund-management-panel.vue` | 退款列表游标；`refund-client.ts`调用`admin/billing/refunds` |
| `idc-core/frontend/pages/components/customer-overview-drawer.vue` | 客户订单、账单、流水三个历史表复用商务/财务接口及客户筛选，不是单笔明细 |
| `idc-core/frontend/pages/marketing-management.vue` | 六张历史表由`marketing-client.ts`提供游标；概览最近活动、最近发放另为有界摘要 |
| `idc-core/frontend/pages/components/marketing-insights-panel.vue` | 活动效果为聚合；反滥用案件、缺失成本列表固定截断，需拥有方补全分页与准确总数 |
| `idc-core/frontend/pages/components/credit-management-workspace.vue` | `use-credit-workspace.ts`只读首批数据，当前页长度冒充总数且覆盖官方布局 |
| `idc-core/frontend/pages/catalog-automation-interfaces.vue` | 已使用完整 Vben 查询分页；沿用拥有方总数 |
| `idc-resource/frontend/pages/provider-capacity-observations.vue` | `provider-capacity-client.ts`调用`admin/resource/provider-capacity-observations`游标接口 |
| `idc-resource/frontend/pages/provider-health.vue` | `health-client.ts`调用`admin/resource/channel-health`游标接口 |
| `idc-resource/frontend/pages/provider-tasks.vue` | `provider-task-client.ts`调用`admin/resource/tasks`游标接口 |
| `idc-resource/frontend/pages/server-batch-renewal.vue` | 候选列表调用`admin/resource/server-renewals`游标；预览、结果另属有界明细 |
| `idc-resource/frontend/pages/server-accessories.vue` | 服务主表调用拥有方`admin/resource/server-services`游标；附加资源预览属单笔明细 |
| `idc-resource/frontend/pages/provider-configuration.vue` | 渠道与资源池快照；服务分别读取`101/501`条，超出`100/500`报错，不是完整分页 |
| `idc-resource/frontend/pages/resource-overview.vue` | 资源池及财务列表由`admin/resource/overview`固定`LIMIT 500`，不能以数组长度作总数 |
| `idc-compliance/frontend/pages/enterprise-verifications.vue` | 企业认证列表游标；`enterprise-verification-client.ts` |
| `idc-compliance/frontend/pages/components/electronic-contract-manager.vue` | 合同主列表游标；模板预览、签署流程子表为有界详情 |
| `idc-compliance/frontend/pages/tax-invoice.vue` | 开票申请及红冲任务游标；分配、行项目子表为有界详情 |
| `idc-compliance/frontend/pages/components/electronic-contract-template-manager.vue` | 接口已有真实总数，但固定每页`20`且外置 Ant 分页禁用条数选择；请求 DTO 仅有页码 |
| `idc-compliance/frontend/pages/components/electronic-contract-auto-sign-manager.vue` | 接口已有真实总数，但固定每页`20`且外置 Ant 分页；请求 DTO 仅有页码 |
| `idc-payment/frontend/pages/payment-channels.vue` | 支付渠道配置完整数组，未接官方分页 |
| `idc-payment/frontend/pages/components/payment-operations.vue` | 健康、对账批次、差异、资金四表；接口部分固定`100/500`条，没有总数与随机页查询 |
| `idc-channel/frontend/pages/channel-operations.vue` | 八表本地分页；四业务列表截断`200`，三财务列表截断`100`；能力列表为固定完整元数据 |
| `idc-portal/frontend/pages/customer-external-menus.vue` | 安装级完整配置集合，适合对已取得的有界集合做真实本地分页 |
| `idc-zjmfmigrate/frontend/pages/migration.vue` | 冲突接口看似返回总数，实际服务固定`Page(1,100)`，控制器返回当前长度 |
| `content-notice/frontend/pages/notice-management.vue` | 已有完整 Vben 服务端分页及真实总数 |
| `org-core/frontend/pages/post-management.vue` | 已有完整 Vben 服务端分页及真实总数 |
| `staff-access/frontend/pages/staff-access.vue` | 已有完整 Vben 服务端分页及真实总数 |
| `system/user/index.vue` | 已有完整 Vben 服务端分页 |
| `system/role/index.vue` | 已有完整 Vben 服务端分页 |
| `system/role-auth/index.vue` | 已有完整 Vben 服务端分页 |
| `system/file/index.vue` | 已有完整 Vben 服务端分页 |
| `system/plugin/index.vue` | 已有完整 Vben 服务端分页 |

## 默认隐藏技术页面：8 个文件

宿主`system/config/index.vue`、`system/job/index.vue`、`system/job-group/index.vue`、`system/job-log/index.vue`、`system/dict/type/index.vue`、`system/dict/data/index.vue`已有服务端分页，但按已确认菜单规则默认隐藏。`tenant-core/frontend/pages/tenant-management.vue`同样已有分页且默认隐藏。`tenant-core/frontend/pages/tenant-plugin-management.vue`虽然打开分页，却调用不接分页参数的完整数组接口，翻页会重复；不以该隐藏页面冒充本轮商家验收范围。

## 树形表格：4 个文件

宿主`system/menu/index.vue`、`components/tree/src/menu-select-table.vue`，以及插件`org-core/frontend/pages/dept-management.vue`、`idc-core/frontend/pages/catalog-categories.vue`属于树形列表。应保持父子关系、选中关系及展开语义，不能直接截断树节点来凑普通分页。菜单相关入口按产品规则隐藏；树形方案独立处理。

## 另行交付的独立表格入口：7 个文件

- `idc-channel/frontend/pages/admin-mount.js`：完整渠道后台八表，使用同一公开 API；必须与正式 Vue 页同时实现分页。
- `idc-channel/frontend/pages/mount.js`：渠道旧独立管理表面，仍是可交付入口，不能删除入口逃避扫描。
- `idc-resource/frontend/src/ProviderConfiguration.vue`：独立资源配置表面，与正式配置页同源数据约束。
- `idc-payment/frontend/src/App.vue`：独立支付配置及运营表面，与正式页同源数据约束。
- `idc-core/frontend/src/finance/FinanceWorkspace.vue`：独立信用工作区，首批截断问题与正式信用组件相同。
- `idc-core/frontend/src/server-batch/ServerBatchWorkspace.vue`：批量操作预览，完整集合受批量上限约束，不是历史库全量。
- `yunsu-ssl/frontend/src/shared/page.ts`：动态 SSL 正式交付表面，`admin/service-entitlements`目前游标加载更多，无总数。

## 无表格及明细排除说明

工单工作台、内容管理独立工作区、通知投递记录中的卡片列表和宿主站内信`List`不计入上述表格文件数；这不是认定其分页无需治理。合同单笔流程、开票单笔分配、批量预览和报价行项目等完整有界子集合可以本地分页，但禁止为历史列表一次拉全库。

## 渠道专项实施方案

1. 渠道账户、商品发布、令牌、调用审计四表由`idc-channel`在相同权限及渠道条件下执行真实计数和有界数字页查询，稳定排序；不再用`LIMIT 200`的长度充当总数。既有客户令牌和下游开放 API 不在本次行为变更范围。
2. 渠道账单、信用账单、还款记录由`idc-core/backend/cap/channelcommercecap`公开分页契约，`channel_commerce`拥有方实现同渠道/付款客户范围计数和查询；`idc-channel`不跨插件查表。主代理已确认允许本专项修改这两个限定拥有方文件及对应测试，其他财务实现保持原样。
3. 能力列表为固定完整的协议元数据（适配器及操作能力），不是数据库业务历史，保留真实本地分页。
4. 请求默认第一页、每页十条，候选`10/20/50/100`，单页最多一百；无效输入拒绝。页面及独立入口使用官方完整分页条；每页条数变化回第一页。明确的临时读取失败保留已确认数据及页码；`401/403`必须清空已失授权的数据与一次性密钥。旧隔离入口的宿主桥把错误统一为`bridge_request_failed`，无法分辨权限拒绝与临时故障，因此按原安全规则清空，而不假定为`503`；本任务不扩大宿主桥范围。渠道或页码切换均隔离旧回读，不让旧请求覆盖新上下文。
5. 不新增缓存：每次读取权威数据，计数与行使用相同筛选边界，不存在节点本地缓存失效问题。权限标签和渠道身份来源不变，不增加付款、退款、停服等业务动作。
6. 验证覆盖空列表、跨页、不足整页、超过原截断上限、无效页/条数、渠道隔离和失败恢复；执行 Go 编译与测试、所属模块`lint.go`、API 生成核对、前端类型与 E2E。未完成验证前不标记`FB-110`完成。
7. `005-channel-list-pagination.sql`属于新增数据库资产，渠道插件版本必须从`v0.1.1`提升到`v0.1.2`，通过正式插件升级事务应用；不得用手工执行 SQL、卸载重装或重置数据库代替升级验证。

## 执行记录

- 已完成静态分类和本清单限定的渠道分页实现；正式与独立渠道页面均已完成真实浏览器验收。
- 渠道实现已接入四张拥有方列表、三张核心财务列表和固定能力元数据分页；旧独立商品目录同时保留原已批准渠道、已发布商品条件补齐分页，未扩大可见范围。
- 已新增并通过渠道真实 PostgreSQL 测试：`225`条记录跨过原`200`截断，验证页码、最后五条、空尾页、渠道隔离、字面量候选搜索、目录仅展示已发布商品及暂停渠道门禁；四项分页索引在测试隔离 schema 连续执行两次通过。核心财务测试使用`125`条跨过原`100`截断，验证三个总数与独立页、渠道/付款主体条件和多笔分配不会重复计数。
- 新增真实 GoFrame HTTP 绑定验证，覆盖默认第一页每页十条、最小值、最大值、零、负数、超限和三种财务页参数。正式及独立表格均拒绝缺少或无效的总数，不把业务数组长度当总数；只有完整能力元数据使用本地真实长度。独立渠道选择器随账户当前页更新并保留已选渠道，不把最初十条当作全部候选。
- 渠道全部`backend/...`测试、渠道与核心两个模块的`lint.go`及全仓`i18n.check`已通过；正式 Vue 页独立`vue-tsc`覆盖已通过。新增测试`TC008f`覆盖完整分页、大小切换、失败旧页保留及重试、三财务页独立；`TC008g`覆盖独立运行时同样分页，最终串行 Chrome 已通过。
- `ctrl`已执行并核对接口；生成器因既有合并控制器文件重复生成`12`个空占位壳，并向两个已实现`_new.go`追加重复声明。仅移走本次新生成壳至根目录`temp/20260904/125111-channel-ctrl-duplicate-scaffolds`保留证据，移除本次追加的重复空声明，保留原业务方法和依赖注入，随后编译及 lint 通过；未回退任何原有未提交实现。
- 命中并已读取文档、OpenSpec、架构、插件、后端、API、数据库、数据权限、前端、测试、i18n及缓存规则。涉及 i18n 的新 DTO 描述、错误文案跟随所属插件资源；无 SQL 重置、缓存新增、第三方供应商调用或宿主框架边界变更。
- 首轮真实 Chrome 已验证旧独立`TC004`，发现`TC008f`测试误把官方条数选择器同节点当作后代，已修正 POM；原断言及分页结果保留。`TC003`初次独立运行时布局过程中点击未发请求，增加官方分页就绪和按钮进入视口的等待后复测。首轮 trace 保存于`temp/20260904/140000-channel-pagination-regression`；因录制和浏览器关闭阶段卡顿，后续小批次关闭 trace/video，继续真实 Chrome 与显式截图验证。
- 自审补充写后回读序号检查：创建/撤销令牌成功后的慢查询不能覆盖同渠道后来切换的页码；`TC008c`补回归。`TC008b`保持原权限拒绝清空断言，`TC008g`补旧隔离入口不可区分拒绝清空及恢复，不能以权限放宽换取失败保留。

## 本专项实际修改文件

以下是`FB-110`渠道专项的精确文件范围，不把整个未跟踪插件目录算作本次改动。路径均相对仓库根目录。

### 新增

- `openspec/changes/add-idc-server-operations-platform/phase-28-admin-pagination-inventory.md`
- `apps/lina-plugins/idc-channel/backend/internal/service/channel/channel_page.go`
- `apps/lina-plugins/idc-channel/backend/internal/service/channel/channel_page_test.go`
- `apps/lina-plugins/idc-channel/backend/api/admin/v1/channel_page_http_test.go`
- `apps/lina-plugins/idc-channel/manifest/sql/005-channel-list-pagination.sql`
- `apps/lina-plugins/idc-core/backend/channel_finance_page_test.go`
- `apps/lina-plugins/idc-channel/frontend/pages/channel-request.ts`
- `apps/lina-plugins/idc-channel/frontend/pages/channel-request.test.ts`

### 修改

- `apps/lina-plugins/idc-channel/backend/internal/service/channel/channel.go`
- `apps/lina-plugins/idc-channel/backend/internal/service/channel/management.go`
- `apps/lina-plugins/idc-channel/backend/internal/service/channel/channel_errors.go`
- `apps/lina-plugins/idc-channel/backend/api/admin/v1/channel.go`
- `apps/lina-plugins/idc-channel/backend/internal/controller/channel/channel.go`
- `apps/lina-plugins/idc-channel/go.mod`
- `apps/lina-plugins/idc-channel/go.sum`
- `apps/lina-plugins/idc-channel/manifest/sql/scaffold.json`
- `apps/lina-plugins/idc-channel/manifest/i18n/en-US/error.json`
- `apps/lina-plugins/idc-channel/manifest/i18n/zh-CN/error.json`
- `apps/lina-plugins/idc-channel/manifest/i18n/zh-CN/apidoc/index.json`
- `apps/lina-plugins/idc-channel/frontend/pages/channel-operations.vue`
- `apps/lina-plugins/idc-channel/frontend/pages/channel-table-runtime.js`
- `apps/lina-plugins/idc-channel/frontend/pages/admin-mount.js`
- `apps/lina-plugins/idc-channel/frontend/pages/mount.js`
- `apps/lina-plugins/idc-channel/hack/tests/support/channel-workspace-mock.ts`
- `apps/lina-plugins/idc-channel/hack/tests/pages/ChannelVbenPage.ts`
- `apps/lina-plugins/idc-channel/hack/tests/e2e/TC008-channel-vben-tables.ts`
- `apps/lina-plugins/idc-channel/hack/tests/e2e/TC003-channel-management.ts`
- `apps/lina-plugins/idc-channel/hack/tests/e2e/TC004-channel-capability-page.ts`
- `apps/lina-plugins/idc-core/backend/channel_commerce.go`
- `apps/lina-plugins/idc-core/backend/cap/channelcommercecap/contract.go`

### 生成流程核查

以下五个文件被生成流程触及并显式核查，构造器保留原实现；另行生成的十二个重复空壳只移至上述临时目录保存，不算交付代码。

- `apps/lina-plugins/idc-channel/backend/api/account/account.if.go`
- `apps/lina-plugins/idc-channel/backend/api/admin/admin.if.go`
- `apps/lina-plugins/idc-channel/backend/api/channel/channel.if.go`
- `apps/lina-plugins/idc-channel/backend/internal/controller/admin/admin_new.go`
- `apps/lina-plugins/idc-channel/backend/internal/controller/channel/channel_new.go`

## 本专项验证命令与审查状态

截至本专项代码冻结，以下命令已通过；Go 测试从各插件目录执行，设置`GOWORK=off`。数据库测试从本机宿主配置读取连接到`LINAPRO_IDC_CORE_TEST_DSN`，未输出或写入文档任何连接秘密。

```powershell
# idc-channel
go test ./backend/... -count=1
# idc-core
go test ./backend -run 'TestChannelFinancePage' -count=1
# 仓库根目录
go -C hack/tools/linactl run . lint.go dir=apps/lina-plugins/idc-channel plugins=1
go -C hack/tools/linactl run . lint.go dir=apps/lina-plugins/idc-core plugins=1
go -C hack/tools/linactl run . i18n.check
openspec validate add-idc-server-operations-platform --strict
# apps/lina-vben
pnpm exec vue-tsc --noEmit --project D:/golang/cloud/temp/20260904/120000-channel-tsconfig.json --pretty false
```

三个独立 JS 文件`admin-mount.js`、`mount.js`、`channel-table-runtime.js`分别通过`node --check`。API 生成过程已执行`go -C hack/tools/linactl run . ctrl dir=apps/lina-plugins/idc-channel/backend`，核对及重复壳处理详见执行记录，不以生成命令成功代替编译。

最终 Chrome 集合为`TC003-channel-management`一项、`TC004-channel-capability-page`一项、`TC008-channel-vben-tables`七项，共九项。第一次运行在修复测试定位前未通过，不计为完成；主线在正常重启后以单 worker 串行运行最终集合。本专项临时配置`temp/20260904/130700-channel-playwright.config.ts`继承正式配置，仅关闭 trace/video 并限制动作超时，未修改公共测试配置、业务断言或权限门禁。

任务级`lina-review`范围为新增八个、修改二十二个、生成核查五个，共三十五个文件；已检查根仓库状态并展开本插件未跟踪文件，按任务范围排除其他既有实现。`idc-channel`与`idc-core`根目录均不存在插件本地`AGENTS.md`。已读取规则为`user-preferences`、`documentation`、`openspec`、`architecture`、`plugin`、`frontend-ui`、`testing`、`backend-go`、`api-contract`、`database`、`data-permission`、`i18n`、`cache-consistency`、`dev-tooling`；使用`goframe-v2`、`vben`、`karpathy-guidelines`、`lina-feedback`、`lina-e2e`与`lina-review`执行对应检查。

后端计数、查询边界、索引幂等、类型与 i18n 验证已通过。主线`52`项综合回归曾在本专项发现`TC008b`正式请求丢失 HTTP 状态，补丁及单元验证如下；随后五十三项综合回归中的渠道九项全部通过，最新正常重启及`v0.1.2`升级后`TC008`七项又一次通过。数据权限沿用原身份和权限，计数与列表同筛选；不新增缓存、不跨插件查表、不改宿主源码、不进行第三方供应商调用。

## 综合回归发现与后续补丁

- `TC008b`失败证据位于`temp/20260904/140000-vben-current-regression/apps-lina-plugins-idc-chan-c6850-token-operations-and-errors-admin-plugin/error-context.md`：收到明确`403`后表格仍保留十行。根因是宿主`RequestClient.request()`只向调用方抛出响应正文，页面无法再读取`response.status`；认证拦截器重放也调用该方法，因此仅改用底层实例仍有缺口。
- 渠道私有`channel-request.ts`沿用同一个已配置的请求客户端、认证与提示拦截器和成功解包逻辑。每次请求使用独立闭包，在`transformResponse`最前面记录真实 HTTP 状态，在每次`transformRequest`开始清空此前状态；发生失败时只保留实际观察到的有效 HTTP 状态和原始原因，不读取业务`code`，不修改共享错误对象，不增加自动重试。正式页面只替换本地请求函数，既有`401/403`清空和临时故障保留逻辑不变。
- `channel-request.test.ts`使用真实`RequestClient`、官方认证刷新拦截器及`Axios`响应替身，共`13`项验证：首次`401/403/503`、业务码与 HTTP 区分、刷新后`401/403/503`、刷新成功及请求编码、重放网络和超时失败不继承旧`401`、刷新失败及错误对象不污染、并发隔离、调用方变换与解析失败。所有断言通过；没有只用手写请求替身模拟宿主丢状态行为。
- 当前正式页面和 helper 独立类型检查通过，新增测试也显式纳入类型检查通过。`TC008b`保留原零行断言，`TC008f`保留`503`旧页与重试断言，`TC008c`保留慢请求隔离断言；这些断言已在串行真实浏览器回归通过，不以单元测试代替 UI 验收。

```powershell
# 工作目录：apps/lina-plugins/idc-channel
node D:/golang/cloud/apps/lina-vben/node_modules/vitest/vitest.mjs run --config D:/golang/cloud/temp/20260904/153000-channel-request-vitest.config.mjs
# 工作目录：apps/lina-vben
pnpm exec vue-tsc --noEmit --project D:/golang/cloud/temp/20260904/153100-channel-request-tsconfig.json --pretty false
```

本地临时 Vitest 配置仅解析既有测试依赖，不改宿主构建或插件生产依赖；Windows 上运行进程工作目录须与配置的插件根目录一致。该配置及专项类型配置保留于根`temp/20260904`，不计入交付文件。请求补丁没有新文案、语言资源、HTTP 契约、后端、数据库、身份或权限放宽；不新增服务实例或共享缓存。测试使用每例独立客户端并恢复替身。

## 已安装实例索引部署与收口

最终渠道九项已包含在主线五十三项综合回归中全部通过，结果目录`temp/20260904/155500-vben-final-regression`；`TC008b`明确拒绝清空、`TC008f`临时故障保留及`TC008c`慢请求隔离均保留原断言。请求十三项已接入插件根目录`pnpm test`，新增相对路径配置`vitest.requests.config.mjs`和脚本，并核查自动生成的空依赖锁文件；不再依赖本机绝对路径的临时配置。

- 首次只读查询确认信用六个索引已存在、渠道四个索引缺失。根因是新增`005-channel-list-pagination.sql`后清单版本仍为`v0.1.1`，同版本启动不会产生新的升级发布。
- 已先把版本门禁同步到本记录，再将`idc-channel`提升到`v0.1.2`并通过插件后端全量测试。正常重启后，内置源码插件按正式启动对账事务升级到`v0.1.2`；未调用普通插件手工升级、未卸载、未重装、未加载演示数据，也未重置数据库。
- 升级后只读检查当前`cloud_preview_19120`的目标索引为`10/10`，缺失集合为空；`sys_plugin_migration`中第`005`号资产对应的升级记录为`succeeded`，插件保持`desired_state=enabled`与`current_state=enabled`。重启后的渠道`TC008`七项全部通过，结果目录为`temp/20260904/final-channel-v012`。
