# 第十阶段企业认证页面验证记录

## 当前结论

任务`10.3.d`已经完成。合规插件提供中英文管理列表、脱敏案件抽屉、单次原件下载和审核决定；`apps/lina-vben/apps/web-customer/`在固定路由`/account/enterprise-verification`提供中英文首次认证、服务端草稿恢复、加密材料上传、待审核、退回重提和已认证资料变更复核。客户当前状态最多返回八条本人案件安全材料元数据，不包含文件名、对象引用、摘要、密钥、内容或下载地址。

仓库标准 runner 已实际执行管理端`TC003-enterprise-kyc.ts`，批准、驳回、单次原件访问和中英文三个场景 3/3 通过；客户应用连续编号`TC005-enterprise-verification.ts`定向 4/4、全量 27/27 通过。人工审查覆盖中文桌面草稿、待审核、退回、资料变更保留旧投影和英文`390 × 844`移动端。旧 Portal 源码和历史`TC010`仍按迁移规则保留，但只作为迁移基线，不再承担目标页面功能，也未在本任务删除。

## 页面与接口边界

### 管理端

- 动态路由为`/idc/compliance/enterprise-verifications`，菜单只要求列表权限；详情、开始审核、批准、驳回和原件访问分别按既有独立精确权限显示和执行。
- 列表使用无精确总数的服务端键集游标，前端只保存当前页、前后游标和本页指标，不请求总数也不逐行读取详情。
- 抽屉只展示脱敏法定资料和最多八条材料元数据。批准请求只发送案件号、观察版本和随机幂等键，不允许页面提交任何法定值。
- 原件按钮先申请近期完整登录、会话和目标绑定的一次性访问上下文，再立即以`POST`消费；浏览器创建 Blob 下载后马上撤销对象地址。

### 客户端目标实现与旧基线

- 固定客户路由`/account/enterprise-verification`现由 Vben/Vite`web-customer`拥有；页面、接口客户端、路由、状态和双语资源均未写入`web-antd`，也未继续扩展旧 Portal。
- 首次认证按“单独同意、法定资料与证据、商家人工审核”三步展示；PDF、JPEG 和 PNG 原件以两步式预约及原始字节写入提交，浏览器文件名不发送。
- 页面内存只在当前填写和请求期间持有法定值与文件对象。上传后的服务端状态刷新不会卸载当前表单；失败时保留内存中的已填内容但禁止继续写操作，用户重新读取权威状态后才可重试。`sessionStorage`只保存随机重试键；成功提交后清空法定输入，重新进入页面依靠本人当前案件和安全材料元数据恢复。
- 已认证企业只能先提交类别化法定资料变更申请，再接受新告知并创建精确绑定的复核草稿。变更申请不含拟替换法定值，原已认证投影在复核中和驳回后继续展示。
- 客户浏览器测试证明企业名称、统一社会信用代码、代表人证件号、上传文件名和材料编号均不进入`localStorage`或`sessionStorage`；客户请求不接受客户号，页面也不会把浏览器参数当作审核结果。

## 自动化验证

| 验证 | 结果 |
|---|---|
| 合规与核心插件`GOWORK=off go test ./backend/... -count=20 -shuffle=on` | 通过；两插件全部后端包连续二十轮随机顺序成功 |
| 两插件`GOWORK=off go vet ./backend/...` | 通过 |
| 两插件`make.cmd lint.go plugins=1 dir=...` | 通过；`golangci-lint`零问题且`staticcheck`通过 |
| 两插件 Windows 与 Linux amd64、`CGO_ENABLED=0`构建 | 通过 |
| 企业认证真实 PostgreSQL 提交、审核和材料访问用例 | 通过；随机临时 schema 覆盖加密提交、本人当前材料投影、单次原件访问、批准、驳回和重复登记围栏 |
| 核心客户真实 PostgreSQL 用例 | 通过；包含类别化法定资料变更、旧投影保留及事务回滚 |
| Portal`pnpm run build` | 通过；`vue-tsc`和 Vite 生产构建成功 |
| Portal`TC010-enterprise-verification.ts` | 通过，3/3；覆盖首次提交与浏览器零明文、变更复核保留旧投影、英文移动端无横向溢出 |
| `web-customer`企业认证接口单元测试 | 通过，1/1；覆盖合规账户边界、两步原始字节上传、类别化变更与提交路径 |
| `web-customer`类型、双语和 Vben 对齐检查 | 通过；`vue-tsc`、测试 TypeScript、10 组双语文件和宿主结构对齐均无错误 |
| `web-customer`生产构建 | 通过；Vite 生产构建成功并生成`dist.zip`，仅保留既有静态/动态导入提示 |
| `web-customer`定向`TC005-enterprise-verification.ts` | 通过，4/4；覆盖首次提交、退回重提、类别化变更与旧投影保留、英文移动端 |
| `web-customer`全量真实 Chrome 回归 | 通过，27/27；`TC001`至`TC005`全部成功 |
| 管理端两个 Vue SFC 独立编译与`TC003`用例发现 | 通过；Vue 脚本和模板编译成功，Playwright 发现 3 个相互独立的管理场景 |
| `make.cmd plugins.check` | 通过；扫描 1583 个文件，零治理发现 |
| `make.cmd i18n.check`（当前复核） | 通过；运行时扫描、宿主与插件消息覆盖、宿主与插件前端键覆盖均无问题 |
| `pnpm test:validate`（`hack/tests`） | 通过；217 个 E2E 文件、17 个范围、CI 分片有效 |
| 管理端测试 TypeScript 与插件独立前端构建 | 通过 |
| 仓库标准 runner 执行合规插件模块 | 23/23 通过；其中`TC003-enterprise-kyc.ts`为 3/3 |

管理端截图审查覆盖中文脱敏详情、单次下载后批准、驳回客户可处理原因和英文队列，未出现原始登记标识、内部翻译键或永久材料地址。`web-customer`最新截图为`customer-enterprise-verification-draft-zh-cn`、`pending-zh-cn`、`rejected-zh-cn`、`change-draft-zh-cn`和`mobile-en-us`；人工审查未发现内部翻译键、明文证件号、遮挡或横向溢出。旧 Portal 截图只保留为历史迁移基线。

## 影响分析

- 接口：有影响。新增本人企业状态和类别化法定资料变更入口；当前状态的材料集合硬上限为八且不接受客户号。
- 后端：有影响。核心插件新增窄化企业账户工作流适配，合规插件新增本人当前案件与材料安全投影。
- 数据库：有影响。新增本人当前企业案件读取索引；查询为一个案件加一个最多八条材料的集合化读取，无`N+1`。
- 数据权限：有影响。客户状态从当前主体派生；管理页面继续使用五个独立权限并由服务层复核直接员工、租户与目标数据范围。
- 前端与`i18n`：有影响。插件管理页与`web-customer`客户页提供完整中英文文案及响应式布局；全仓覆盖通过。旧 Portal 仅保留为待后续统一清理的迁移基线。
- 缓存一致性：无影响。未引入缓存，PostgreSQL 继续作为案件和审核权威状态。
- 开发工具：无源码影响。仅执行既有代码生成、构建、测试、lint 和治理命令；`ctrl`追加的重复空构造骨架已从插件自定义构造文件精确移除。
- `AI`：无调用影响。企业法定资料、文件原件、脱敏前值和原件访问内容继续禁止进入模型。

## Lina 审查报告

**变更：**`add-idc-server-operations-platform`的`10.3.d`企业认证页面纵向切片。

**范围：**任务级，共 53 个生产、测试、迁移、配置和 OpenSpec 文件；代码生成涉及的接口与 Controller 文件一并纳入。

**范围来源：**`git status --short`、`git ls-files --others --exclude-standard`、`openspec status --change add-idc-server-operations-platform --json`和`10.3.d`任务上下文。三个插件根目录均不存在插件本地`AGENTS.md`，因此使用项目顶层规范。

**已读取规则文件：**`AGENTS.md`、`.agents/rules/openspec.md`、`.agents/rules/architecture.md`、`.agents/rules/data-permission.md`、`.agents/rules/plugin.md`、`.agents/rules/api-contract.md`、`.agents/rules/backend-go.md`、`.agents/rules/database.md`、`.agents/rules/frontend-ui.md`、`.agents/rules/testing.md`、`.agents/rules/i18n.md`、`.agents/rules/documentation.md`和`.agents/instructions/markdown-format.instructions.md`。

### 发现的问题

- **严重**`apps/lina-vben/apps/web-antd/src/views/system/message/notice-preview-modal.vue:16`：仓库级`make.cmd i18n.check`被宿主页面在中英文各缺少三个运行时键阻断；另外两个引用位于同文件第 42、45 行。该文件属于禁止在本插件任务修改的宿主范围，插件新增运行时和消息覆盖本身均通过。依据：`.agents/rules/i18n.md`和`lina-review`强制门禁。
- **严重**`apps/lina-core/internal/controller/role/role_new.go:13`：宿主`role`、`plugin`、`joblog`控制器分别存在重复`ControllerV1`与`NewV1`声明，另见`apps/lina-core/internal/controller/plugin/plugin_new.go:16`和`apps/lina-core/internal/controller/joblog/joblog_new.go:13`，导致标准管理端服务不能编译启动，进而无法实跑`TC003`及完成管理端截图审查。这些文件均为任务开始前已有的用户工作区宿主改动，且本仓库无`.contributing`，本任务不得修改。依据：`AGENTS.md`宿主边界和`.agents/rules/testing.md`。

审查中发现的任务内问题已闭环：审批确认从`ant-design-vue`静态确认框迁移到`useVbenModal`；Portal 只把`available`材料计入可提交集合；管理端补充驳回原因与客户可处理状态的独立 E2E 场景；两个新 TC 文件的标签和页面交互已按`lina-e2e`收口到 POM。闭环后未发现新的任务内阻塞问题。

### 规则域结论

- OpenSpec：通过。顶级`10.3.d`和`10.3.d.3`保持未勾选，没有把外部宿主阻断伪报为完成。
- 架构与插件：通过。企业法定资料仍由核心插件拥有，合规插件仅通过`identitycap.EnterpriseAccountService`稳定投影契约消费；没有跨插件导入`internal`实现，也未修改宿主。
- 数据权限：通过。客户接口不接受客户号并从可信上下文派生当前企业；管理列表、详情、审核与原件访问继续执行独立权限和目标范围校验。
- API 与性能：通过。本人当前状态最多执行一个最新案件查询和一个最多八条材料的集合查询；管理列表使用有界游标，不存在逐行详情或`N+1`查询。
- 后端 Go 与依赖注入：通过。新增企业账户依赖由核心插件启动装配创建，经`identitycap.Runtime`传递到合规组合层、服务和 Controller，复用启动期图；没有请求期临时`New()`。
- 数据库：通过。`005-enterprise-verification-account-state.sql`仅包含幂等`CREATE INDEX IF NOT EXISTS`，组合索引顺序匹配租户、客户、企业类型和最新案件排序；无 Seed、Mock、自增主键或软删除影响。
- 前端与测试：任务内通过，宿主实跑仍阻断。管理端使用`Page`、`useVbenVxeGrid`、`useVbenDrawer`、`useVbenModal`和`requestClient.download`；Portal 三个真实 Chromium 场景通过，管理端三个场景通过发现和编译。
- `i18n`：插件范围通过，仓库总门禁被上述宿主六个既有缺键阻断。
- 缓存一致性：无影响。本切片未新增或修改缓存，数据库继续是案件、材料和审核状态的权威来源。
- 开发工具跨平台：无源码影响。只执行仓库既有生成、lint、构建与测试入口；未修改根工具链。
- 文档：通过。OpenSpec 使用中文并通过严格校验；未新增目录级 README，无镜像文档影响。

### 验证证据

- 两插件`GOWORK=off go test ./backend/... -count=1`：通过。
- 两插件`GOWORK=off go vet ./backend/...`：通过。
- 两插件`.\\make.cmd lint.go plugins=1 dir=<plugin>`：通过，`golangci-lint`零问题且`staticcheck`通过。
- `.\\make.cmd plugins.check`：通过，扫描 1583 个文件，零发现。
- Portal`pnpm run build`：通过。
- Portal`TC010-enterprise-verification.ts`：真实 Chromium 3/3 通过。
- 管理端 Vue SFC 独立编译：通过；`TC003-enterprise-kyc.ts --list`发现 3/3 场景。
- `openspec validate add-idc-server-operations-platform --strict`：通过。
- `.\\make.cmd i18n.check`：失败，仅包含上述宿主六个既有缺键。
- 管理端标准服务与`TC003`实跑：未完成，被上述三个宿主重复 Controller 阻断。

### 摘要

- 任务内严重：0。
- 任务内警告：0。
- 外部严重阻断：2。
- 结论：允许按用户指示继续后续插件阶段，但不允许勾选`10.3.d`、声称仓库级门禁通过或归档变更。

## 后续阻断复核与目标边界纠正

以上 Lina 审查报告保留实施当时的真实历史。当前共享工作区已经由对应所有者范围解除重复控制器和通知翻译阻断；本次未修改`web-antd`业务源码或依赖。重新执行全仓双语门禁、E2E 治理、测试 TypeScript、合规插件独立前端构建和仓库标准模块 runner 均通过，管理端`TC003`实际结果为 3/3。

同时，最新前端边界确认要求登录后客户页面由`web-customer`拥有。旧 Portal 页面和`TC010`只能作为迁移清单，不能据此完成父任务。该阻断现已由`web-customer`目标页面、连续编号`TC005`、生产构建、全量 27/27 真实浏览器回归和截图审查解除，因此`10.3.d.4`、`10.3.d`及`10.3`可以精确完成。

## `web-customer`迁移完成审查

本次最终审查范围为企业认证客户 API、API 单元测试、账户路由、企业认证页面、中英文资源、客户 POM、接口假服务和`TC005-enterprise-verification.ts`，以及本验证记录与对应任务状态。`web-antd`工作区差异和暂存区差异均为空；合规插件后台页面未被删除或迁入客户应用；旧 Portal 未删除。

### 发现与修复

- 首轮浏览器回归发现语言文件名与页面命名空间不一致，界面直接显示翻译键；已统一为`enterpriseVerification`命名空间，并由中英文与移动端场景验证。
- 首轮材料上传回归发现服务端状态刷新会卸载表单，导致已填敏感资料丢失；已改为刷新期间保持当前表单只在页面内存中，不写浏览器存储，上传与退回重提场景验证通过。
- 最终安全复核发现读取权威状态失败后旧页面仍可能显示写按钮；现已保留已填内容但禁用同意建档、资料变更、材料上传和提交，重新读取成功后才恢复。

### 最终结论

- 严重问题：0。
- 警告问题：0。
- OpenSpec：通过。只完成`10.3.d.4`、`10.3.d`和`10.3`，未借企业页面迁移勾选后续合同、发票、第十五阶段或`FB-10`。
- 架构与前端归属：通过。客户页面只在`web-customer`，管理页面继续在合规插件`frontend/`，`web-antd`无业务差异，官网无影响。
- API、数据权限与性能：通过。客户请求不接受客户号；状态由当前登录主体派生，材料最多八条且一次随案件安全投影返回，没有逐材料查询。
- 隐私与失败处理：通过。浏览器存储检查排除法定名称、登记标识、代表人证件号、文件名和材料编号；只保存随机重试键。失败时不自动重发供应商写操作，也不允许基于旧状态继续写。
- `i18n`与测试：通过。10 组客户双语文件、全仓运行时文案扫描、前端键覆盖、E2E 治理、客户测试 TypeScript、定向 4/4 和全量 27/27 均通过。
- 构建与交付：通过。Vben 对齐检查和 Vite 生产构建成功，生成`dist.zip`；本任务未修改发布工具、安装包规则或旧 Portal 清理边界。
- 缓存、数据库、后端 Go、开发工具与`AI`：本次客户迁移无源码影响。既有服务端权威状态、加密材料和禁止原始合规材料进入`AI`的边界不变。
