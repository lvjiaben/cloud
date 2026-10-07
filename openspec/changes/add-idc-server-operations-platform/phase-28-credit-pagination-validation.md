# 第二十八阶段：信用完整分页验证与审查

## 结论与边界

最终复验：正式及独立信用九项已在主线五十三项综合集合全部通过；客户`TC014a`至`TC014g`七项独立运行全部通过，耗时`43.1s`。结果分别位于`temp/20260904/155500-vben-final-regression`和`temp/20260904/160000-customer-credit-final`，覆盖权限拒绝清空、还款草稿清空、页规模复位与失败恢复。最终只读检查确认当前开发库六个信用索引均已存在；连同渠道四个索引，`FB-110`目标索引为`10/10`。

本记录对应`FB-110`的信用专项：客户两张表、正式后台三张表、独立插件三张表，均已接真实 Vben 完整分页。准确总数来自`idc-core/billing`当前授权、当前筛选集合，而非前五十条或浏览器已加载条数。没有修改审批、额度、费用、还款或客户与员工的权限含义；没有修改服务器、SSH、工单及其他财务表。

独立入口仍复用唯一的`vben-adapter`运行时，旧 Vue 组件只通过 DOM 挂载及普通数据更新，不新增第二套运行时。资源加载失败后的原“重试”能重新挂载恢复，页面不需要刷新。正式页面直接使用宿主`useVbenVxeGrid`，独立入口使用同一官方组件的隔离适配。

## 当前验证证据

- 真实 PostgreSQL：`TestCreditNumericPagesPostgreSQLCountAuthorizedFilteredRows`、`TestCreditDirectoriesPostgreSQLKeepCustomerOwnershipAndManagementScope`、`TestCreditPageOffsetRejectsMixedAndUnboundedRequests`及`TestManagementCreditAuthorizer*`通过。测试读取本机配置后把连接串仅置入环境变量，未输出秘密；全部使用随机隔离 schema，不重置开发数据。
- 数据库场景：125 条申请、第 3 页、100 条页规模、状态与币种筛选、当前客户覆盖外部传入客户号、超出末页空集合但保留准确总数、三类员工列表无权时不返回结果、非法数字页与游标混用拒绝。账户和账单验证页 1、2、3 及筛选；分页与总数同一只读重复读快照。
- SQL`108-credit-directory-pagination-indexes.sql`：六个索引在隔离 schema 内重复执行通过；没有 DML、字段变更或 DAO 结构变化。旧测试迁移列表补入`003-customer-account.sql`，解决原先`070`依赖客户联系表但 fixture 漏建的问题，生产迁移未变。
- `go test ./backend/internal/service/billing ./backend/internal/controller/account ./backend/internal/controller/admin -run '^TestCredit|^TestManagementCredit' -count=1`：通过。独立模块执行时设置`GOWORK=off`；未设置数据库环境时 PostgreSQL 用例跳过，因此本条不替代上一条真实数据库验证。
- 插件`pnpm run build`：类型检查、生产打包、独立产物校验通过。正式信用 Vue 另用`temp/20260904/120100-credit-tsconfig.json`显式包含该页面执行`vue-tsc --noEmit --skipLibCheck`，通过；普通插件测试配置不覆盖全部正式页面，不据此声称全插件页面类型检查通过。
- 客户`pnpm run typecheck`：通过。主线程另完成当前客户与后台生产构建及插件构建，均通过。
- `make.cmd lint.go dir=apps/lina-plugins/idc-core plugins=1`：最后补强员工拒绝用例后再次执行通过整模块 golangci-lint 与 U1000 检查，零问题，耗时 37.397 秒。
- 插件`TC011a`至`TC011g`共七项通过：审批、英文状态、灰表头白表格、有界高度、失败保留、独立产物、两种入口各三表的 123 条完整分页。`TC011f/g`追加页脚进入视口与`pageerror`零断言后再跑两项通过。
- 客户`TC014a`至`TC014e`共五项通过：申请私有材料、余额还款、信用支付、英文手机端及两表 123 条完整分页。`TC014e`追加页脚进入视口与`pageerror`零断言后单独重跑通过。
- E2E 新增断言固定验证每页`10/20/50/100`、真实`total=123`、直接末页 13、失败回退及原入口重试、空态`total=0`。没有把首次结果长度当成总数，没有循环加载全部游标。
- 页面回归先前出现 Playwright 录制关闭阶段卡住，终止仅本次测试会话后以根`temp/20260904`临时配置关闭 trace/video 串行重跑；没有删测试断言。客户最后一次补图被同 URL 页面重载打断，重新在稳定环境执行通过；没有添加忽略失败或自动放宽断言。

页面 E2E 的 123 条数据与故障由页面级 HTTP 替身提供，使用真实页面组件和独立生产产物；它不替代后端真实 PostgreSQL 的 125 条验证。本地服务正常重建后的新字段接口冒烟与 SQL 迁移由主线程最终收口。

E2E 结果目录：`temp/20260904/125500-credit-pagination`记录前四项通过后录制关闭卡住；`130000-credit-rest`记录其余三项通过；`130600-credit-visual`记录强化后的正式与独立完整分页两项通过；`130400-customer-credit`记录客户五项通过；`125940-customer-credit-visual`记录强化后的客户分页通过。

## 截图多模态审阅

已实际打开并逐张查看以下截图，不是只检查文件存在：

- `temp/20260904/045820-credit-formal-complete-pagination.png`：正式后台表格内部滚动，灰表头、白普通行与白分页；总数 123、每页选择和完整翻页按钮可见。页面外层冷灰与宿主白色菜单未被改动。
- `temp/20260904/045827-credit-standalone-complete-pagination.png`：独立财务入口使用真实 Vben 表格和完整分页；表头灰色，数据与分页白色，保留鼠标悬停反馈，没有原生手写上一页/下一页区域。
- `temp/20260904/050004-customer-credit-complete-pagination.png`：客户信用账单列、原有余额还款入口、固定操作列、123 总数与分页均可见，内容避开左菜单。
- `temp/20260904/050006-customer-credit-pagination-empty.png`：零数据仍保留表头、空态、准确零总数和禁用翻页状态；没有数据区变灰或分页被隐藏。

早期取图只截到表体，未包括下方页脚，已补`scrollIntoViewIfNeeded`及`toBeInViewport`再取图；这是证据取景修正，不是用截图替代分页行为检查。普通行临时灰色是保留的悬停反馈，不是灰色数据背景。

## Lina 任务级审查

范围来自主仓库的`git status --short`、未跟踪文件展开和本任务实际编辑记录，不把整个未跟踪`idc-core`目录当成已审查。当前在`apps/lina-plugins`执行`git rev-parse --show-toplevel`仍返回主仓库，不能声称存在独立插件子仓库。插件根`AGENTS.md`不存在。已读取项目入口及命中规则：用户偏好表格条款、`documentation`、`openspec`、`architecture`、`data-permission`、`plugin`、`api-contract`、`backend-go`、`database`、`frontend-ui`、`testing`、`i18n`、`dev-tooling`；使用`lina-feedback`、`lina-e2e`、`vben`、`goframe-v2`、`karpathy-guidelines`及`lina-review`执行当前专项。

- 数据权限：通过。总数与页面在同一拥有方共用筛选，员工精确权限先于读取，当前客户身份覆盖外部客户号；没有新增跨插件查表或放宽身份。
- 性能与 SQL：通过。当前页加一次计数，固定数据库调用次数；无逐行网络或数据库查询，无全量浏览器切片。索引为纯结构补充、可重复执行。
- API 与生成边界：新增请求`page`和响应`total`，方法、路由、控制器接口签名不变。主线程在隔离副本执行控制器生成核对，没有让生成器覆盖工作区未提交代码。
- 前端：专项及统一回归通过。页码、页规模与数据只在成功后一起确认，服务故障回退并重试失败参数，过期异步响应不能覆盖新页；无搜索字段时不增加空搜索表单。页规模变化统一回到第一页；`TC011f/g`和客户`TC014e`从后续页直接切换规模并断言第一页，已在统一回归通过。
- `i18n`：沿用官方分页多语言，新增英文 API 元数据同步中文`apidoc`。中文、英文及英文手机端页面测试通过；最终`i18n.check`由主线程统一执行。
- 缓存与依赖注入：无新缓存、无持久化分页偏好、无新服务装配与身份来源；共享 loader 的失败重试归其拥有方实现，本文件不修改其源码。
- 开发工具与跨平台：未修改启动、构建工具；临时测试配置仅在根`temp/20260904`。`pnpm`刷新 file 依赖后将其路径规范化回正斜杠，避免 Windows 路径污染跨平台清单。
- 代码审查未发现本专项阻塞问题；统一`i18n.check`、OpenSpec 验证、浏览器回归和正常启动后的真实索引检查均已补齐。此记录仍不表示外部供应商真实联调已完成。

## 本专项实际文件范围

以下路径相对项目根；未列入的其他未提交文件不属于本专项编辑范围。

```text
apps/lina-plugins/idc-core/backend/api/account/v1/credit_application.go
apps/lina-plugins/idc-core/backend/api/account/v1/credit_statement.go
apps/lina-plugins/idc-core/backend/api/admin/v1/credit_application.go
apps/lina-plugins/idc-core/backend/api/admin/v1/credit_account.go
apps/lina-plugins/idc-core/backend/api/admin/v1/credit_statement.go
apps/lina-plugins/idc-core/backend/internal/controller/account/account_v1_credit_application_list.go
apps/lina-plugins/idc-core/backend/internal/controller/account/account_v1_credit_statement_list.go
apps/lina-plugins/idc-core/backend/internal/controller/admin/admin_v1_management_credit_application_list.go
apps/lina-plugins/idc-core/backend/internal/controller/admin/admin_v1_management_credit_account_list.go
apps/lina-plugins/idc-core/backend/internal/controller/admin/admin_v1_management_credit_statement_list.go
apps/lina-plugins/idc-core/backend/internal/service/billing/credit_directory.go
apps/lina-plugins/idc-core/backend/internal/service/billing/credit_directory_application.go
apps/lina-plugins/idc-core/backend/internal/service/billing/credit_directory_account.go
apps/lina-plugins/idc-core/backend/internal/service/billing/credit_directory_statement.go
apps/lina-plugins/idc-core/backend/internal/service/billing/credit_directory_pagination.go
apps/lina-plugins/idc-core/backend/internal/service/billing/credit_directory_pagination_test.go
apps/lina-plugins/idc-core/backend/internal/service/billing/billing_postgres_integration_test.go
apps/lina-plugins/idc-core/manifest/sql/108-credit-directory-pagination-indexes.sql
apps/lina-plugins/idc-core/manifest/i18n/zh-CN/apidoc/account-credit.json
apps/lina-plugins/idc-core/manifest/i18n/zh-CN/apidoc/admin-credit-management.json
apps/lina-plugins/idc-core/frontend/package.json
apps/lina-plugins/idc-core/frontend/pnpm-lock.yaml
apps/lina-plugins/idc-core/frontend/src/finance/api.ts
apps/lina-plugins/idc-core/frontend/src/finance/use-credit-workspace.ts
apps/lina-plugins/idc-core/frontend/src/finance/FinanceWorkspace.vue
apps/lina-plugins/idc-core/frontend/src/runtime/IsolatedVbenGrid.vue
apps/lina-plugins/idc-core/frontend/pages/components/credit-management-workspace.vue
apps/lina-plugins/idc-core/hack/tests/support/credit-management-mock.ts
apps/lina-plugins/idc-core/hack/tests/support/standalone-workspace.ts
apps/lina-plugins/idc-core/hack/tests/e2e/TC011-credit-account.ts
apps/lina-vben/apps/web-customer/src/api/idc/customer-credit.ts
apps/lina-vben/apps/web-customer/src/views/account/credit/index.vue
apps/lina-vben/apps/web-customer/tests/support/customer-credit-api-mock.ts
apps/lina-vben/apps/web-customer/tests/e2e/TC014-credit-account.ts
openspec/changes/add-idc-server-operations-platform/phase-28-credit-pagination-plan.md
openspec/changes/add-idc-server-operations-platform/phase-28-credit-pagination-validation.md
```

## 综合回归补修：请求状态保留

综合浏览器回归发现客户请求层在异常时只抛出`error.response.data`，导致页面收不到真实`HTTP`状态。后台正式和独立信用表均通过插件自己的`fetch`请求，不受此问题影响。仅改用`requestClient.instance`仍不足：官方令牌刷新拦截器重放请求时会重新调用`client.request`，继续丢失状态。

客户业务适配层新增`requestWithHttpStatus`，继续使用同一个`RequestClient`及既有认证、租户、语言、提示与响应解包拦截器。每次调用建立独立闭包，`transformResponse`在原有转换前捕获真实状态；每次请求及重放的`transformRequest`先清空旧状态，防止刷新成功后网络错误被当成旧的`401`。异常只在捕获了有效传输状态时附加`response.status`，原始异常或响应内容保留在`cause`和`response.data`，不读取业务`code`推测状态，不修改原始错误，不二次解包成功数据。

接线仅覆盖当前信用页的概览、两类列表及原有材料和申请还款调用，以及客户工单的`listCustomerTickets`。工单详情和写入接口、其他页面使用的`settleInvoiceFromCredit`不变。方法、参数、文件内容、幂等键、权限和金额规则没有变化。

- `pnpm exec vitest run apps/web-customer/src/api/idc/request-with-http-status.test.ts`：九项通过，使用真实`RequestClient`与官方令牌刷新拦截器，覆盖首次拒绝、刷新重放拒绝、刷新后网络失败、`503`、正常响应与请求转换、业务码不冒充状态、再次`401`、自定义转换数组和并发隔离。
- 客户中心`pnpm run typecheck`：通过。
- `TC011h/i`的申请行定位由包含匹配改为真实单元格精确文本匹配，避免`0001`误命中`0001_2`等多行；未用第一条匹配掩盖问题。
- 生产构建与客户`TC014f/g`、工单权限拒绝和核心`TC011h/i`的重新执行由主线程统一进行；这里不把单元测试通过当作浏览器闭环。

任务级`lina-review`范围为新请求帮助函数、其单元测试、信用`API`、工单列表`API`、信用`POM`和本文六个文件，按主仓未跟踪文件逐项展开。已读取用户偏好、前端、测试、数据权限、接口契约、缓存、架构、插件、国际化、文档和`OpenSpec`规则。没有新增共享缓存、数据库写入、后端接口或第三方依赖；不改变已有认证刷新次数及身份边界，不新增用户文案或语言资源，也不修改开发工具。单元测试各自创建客户端与内存适配器，无真实网络、数据库或全局状态残留；浏览器验证仍是本补修的剩余门禁。

## 最终小修：权限失效与页规模复位

### 根因与范围

原分页读取的统一异常分支把`401/403`与`503`当作相同故障，保留了已不应继续展示的旧行、准确总数和关联详情。正式后台三表和独立入口三表共用同一组合式函数，客户中心两表另有同类分支。此次只修正这八张信用表及其关联详情、材料和弹窗状态；不开始下一组财务账单、流水或调账分页。

- `401`使当前信用工作区的全部读取代次失效，清空全部目录、总数及关联数据；客户概览、还款引用和上传材料同时清空。
- `403`仅清空被拒绝的目录及其相关详情；详情或材料单独拒绝时不清空仍授权的列表。概览拒绝不解锁另一项仍在执行的业务提交。
- `503`继续保留已确认行、总数和页码，重试使用失败请求参数。
- 材料下载保留原始`HTTP`状态，读取返回`Blob`后再次检查详情代次，失效响应不会创建下载地址。后台审批表单在选择目标失效后卸载，不把私有草稿留在隐藏表单中。
- 每页条数变化按当前`design.md`统一回到第一页；直接数字跳页不变。原有三项分页测试删除了预先点击首页的步骤，直接从后续页选择新规模以验证真实请求。

### 当前验证

- `pnpm exec vitest run pages/credit-permission.test.ts`：十项通过，覆盖三个目录的`403`作用域、`401`迟到列表与详情、`503`保留和重试、详情拒绝、迟到材料、下载状态保留及页规模复位。
- `pnpm run test:typecheck`：通过。
- `pnpm run build`：核心插件生产构建通过，包含同期主线程订单变更；当前`dist/mount.js`的`SHA256`为`D278448D0F2806B7C76E0AA75D7D0368EA6AFD6C9EF88B63C432FFF8C45D9A70`。
- `openspec validate add-idc-server-operations-platform --strict`：通过。
- 新增核心`TC011h/i`和客户`TC014f/g`；修改核心`TC011f/g`和客户`TC014e`的页规模断言。这七项以及其余原回归交由主线程在统一重启后执行；此处不将未运行的浏览器用例或旧截图记为最终通过。

### 任务级审查

已使用`lina-review`，按主仓的`git status --short`和`git ls-files --others --exclude-standard`展开本轮未跟踪文件；当前迁机后的`apps/lina-plugins`没有独立`.git`，其`git rev-parse --show-toplevel`仍指向主仓，不能视为已检查独立子仓差异。插件根目录不存在`AGENTS.md`。实际读取前端、测试、数据权限、缓存一致性、插件、架构、国际化、文档及`OpenSpec`规则。清理对象是当前页面从拥有方接口取得的派生状态，权威数据与后端权限不变；没有跨实例缓存、数据库或后端接口变更，没有新增依赖注入及开发工具。沿用现有双语提示，没有新增语言键。代码级检查未发现剩余阻塞项；浏览器权限清理、页规模复位及截图审阅尚待统一回归，因此本补充不能单独作为整项反馈完成或归档依据。

本次小修实际修改或新增的十二个文件如下，其余前节文件没有因该小修再次修改：

```text
apps/lina-plugins/idc-core/frontend/src/finance/api.ts
apps/lina-plugins/idc-core/frontend/src/finance/use-credit-workspace.ts
apps/lina-plugins/idc-core/frontend/pages/components/credit-management-workspace.vue
apps/lina-plugins/idc-core/frontend/pages/credit-permission.test.ts
apps/lina-plugins/idc-core/hack/tests/pages/CreditManagementPage.ts
apps/lina-plugins/idc-core/hack/tests/support/credit-management-mock.ts
apps/lina-plugins/idc-core/hack/tests/e2e/TC011-credit-account.ts
apps/lina-vben/apps/web-customer/src/views/account/credit/index.vue
apps/lina-vben/apps/web-customer/tests/support/customer-credit-api-mock.ts
apps/lina-vben/apps/web-customer/tests/e2e/TC014-credit-account.ts
openspec/changes/add-idc-server-operations-platform/phase-28-credit-pagination-plan.md
openspec/changes/add-idc-server-operations-platform/phase-28-credit-pagination-validation.md
```
