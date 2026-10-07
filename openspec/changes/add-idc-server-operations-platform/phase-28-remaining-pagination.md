# 第二十八阶段：剩余业务分页收口

## 范围与状态

本记录承接`FB-110`，不替代主任务状态。下文保留各轮发现和修复过程；最终浏览器、构建、数据库及索引收口结果见本节末尾和[阶段主文](phase-28-vben-pagination-validation.md)。

已核对当前代码，客户、商品、协助报价、商品履约任务、财务、退款、客户详情历史、营销、资源观测、企业认证、合同、税票、支付运营及迁移冲突仍有真实业务表显式禁用分页或使用手写游标翻页。订单主列表、信用、渠道和证书已完成的分页保留。

本代理负责核心客户、商品、报价、履约、财务、客户详情和营销二十张列表，以及合规六张、迁移冲突一张，共二十七张。支付五张已交支付代理独立实施，不计入本代理完成范围。全部资源列表（包含既有服务器附件）由接口统计代理负责；新增流量包配置与结算由主线负责。树、候选下拉、固定业务预览及有界单据明细与业务历史列表分别评估，不伪造远程总数。

## 实施约束

- 复用已有`Vben`默认完整分页布局，第一页每页十条，可选十、二十、五十和一百条。搜索及改变页规模回第一页。
- 在原拥有方查询增加可选数字页；保留仍由非表格消费者使用的游标契约，拒绝同时传数字页和游标。页大小最多一百条，页码上限与已交付订单分页一致。
- 总数在分页条件之前使用同一授权、租户和业务过滤计算；只查询当前页并批量装配关联，不能全量拉取后分页。计数与页面读取使用同一只读一致快照。
- 暂时故障保留已确认页；明确身份或权限失效清除对应列表、总数及详情，迟到请求不得回填。复用本插件已验证的请求状态保留函数。
- 不修改宿主，不重置数据库，不调用真实上游，不重启服务；浏览器由主线统一排期，未执行的验收明确保留。

## 影响判断

修改属于各插件自己的列表接口与展示适配，不改变业务状态、写操作、幂等或权限含义，不新增缓存和运行期服务依赖。计数必须沿用原权限入口；插件接口文档新增字段同步本插件`apidoc`翻译。测试配置若有调整，使用既有跨平台`Node.js`或`Go`入口。

## 验证记录

核心已接入二十张表：客户、商品、协助报价、商品履约各一张；财务账单、流水、调账和退款四张；客户详情订单、账单、流水三张；营销分群、活动、权益、积分账户、价值发放、发放调账、效果统计、反滥用案件、缺失成本九张。统一服务重启后的最终浏览器套件已补齐。

- 后端八包编译和普通测试通过：`customermanagement`、`catalog`、`commerce`、`productfulfillment`、`billing`、`marketing`、`controller/admin`和`controller/account`。
- 真实隔离 PostgreSQL 已通过客户、商品、协助报价、履约、财务和营销目录分页。覆盖二十五条以上数据的首、中、末和越界空页、每页一百条、相同筛选总数；履约还覆盖客户授权范围变化，营销效果按活动和币种分组计数，不合并币种金额。商品、履约及后来补齐的退款、缺失成本使用隔离读契约表；其余使用对应已有真实迁移夹具，不把前者描述为完整迁移验收。
- 前端私有分页状态七项、真实请求客户端状态保留十三项，共二十项单元测试通过；实际插件页面及其依赖组件的`vue-tsc`通过。检查中只对四个旧组件进行等价类型收窄，未改字段、值或交互：配置表单的分类型绑定、验证后价格表单字段、原生货币格式精度及按钮的`class`属性。
- 客户、商品、报价、财务和营销的原分页用例已改为官方页码定位；新增`TC003j`覆盖营销九表的准确总数、第三页和每页二十条。这些浏览器用例已在最终核心套件执行。
- `i18n.check`零违规。`lint.go`发现本次履约列表复杂度超限后已提取等价输入校验；主线最后指出的两个营销控制器格式问题已在本代理范围内执行`gofmt`，核心模块最终整包门禁由主线统一复测，不能用合规或迁移的门禁替代。

命令从`apps/lina-plugins/idc-core`运行，设置`GOWORK=off`；真实 PostgreSQL 的`LINAPRO_IDC_CORE_TEST_DSN`仅从本机配置读取，不输出连接秘密，不改生产表：

```powershell
go test ./backend/internal/service/customermanagement ./backend/internal/service/catalog ./backend/internal/service/commerce ./backend/internal/service/productfulfillment ./backend/internal/service/billing ./backend/internal/service/marketing ./backend/internal/controller/admin ./backend/internal/controller/account
go test ./backend/internal/service/commerce -run '^TestAssistedQuoteNumberedPagesPostgreSQL$' -count=1 -parallel=1 -v
go test ./backend/internal/service/productfulfillment -run '^TestIntentNumberedPagesPostgreSQL$' -count=1 -parallel=1 -v
go test ./backend/internal/service/marketing -run '^TestMarketingNumberedDirectoriesPostgreSQL$' -count=1 -parallel=1 -v
go test ./backend/internal/service/billing ./backend/internal/service/marketing -run '^(TestRefundNumberedPagesPostgreSQL|TestMissingCostNumberedPagesPostgreSQL)$' -p=1 -parallel=1 -count=1 -v
```

前端私有测试必须从`idc-core/frontend`运行，临时配置保留于根目录，不更改宿主构建。Windows 下在宿主目录运行同一外部测试配置会产生虚拟测试路径无法解析，已使用正确工作目录重跑通过：

```powershell
../../../lina-vben/node_modules/.bin/vitest.cmd run --config D:/golang/cloud/temp/20260904/201000-numbered-page-vitest.config.mjs
```

实际页面类型检查从`apps/lina-vben`运行：

```powershell
pnpm exec vue-tsc --noEmit --project D:/golang/cloud/temp/20260904/162500-remaining-pagination-tsconfig.json --pretty false
```

## 新补充范围

- 合规六张：企业认证、电子合同、合同模板、自动签授权、税票和红冲。模板和自动签已有总数但固定每页二十条和外置分页；其余四张缺少总数。模板原窗口计数在越界空页会丢总数，也须修复。
- 支付五张：支付渠道配置、渠道对账健康、对账批次、差异、异常资金。原查询固定截断一百或五百条；独立入口与正式页面同源，必须一并接入。
- 迁移冲突一张：原记录存储能力已经返回同权限总数，但插件丢弃并使用当前数组长度。改为传页码和页大小并返回能力的原始总数，不跨越插件查询数据库。

其中支付五张由其他代理交付。本代理合规六张与迁移一张已完成代码和下述验证，真实浏览器仍由主线串行验收；未修改角色权限、审批和资金动作，不引入缓存或后端运行依赖。

## 合规六表闭环

| 列表 | 查询和总数来源 | 当前验证 |
| --- | --- | --- |
| 企业认证 | 原授权、租户、案件筛选，在投影前克隆计数模型 | 两案件稳定页和越界总数真实数据库通过 |
| 电子合同 | 原授权与订单过滤，同事务计数和当前页 | 二十五条、其他租户、拒绝权限、四种页规模通过 |
| 合同模板 | 同一过滤公共查询，独立准确计数 | 三模板、每页一条、越界空页总数不丢通过 |
| 自动签授权 | 原企业可见范围，两查询一致快照 | 四种页规模及空页总数通过 |
| 税票 | 原客户和状态过滤 | 二十五条、四种页规模、客户及状态过滤通过 |
| 红冲任务 | 原客户和任务范围 | 二十五次真实退款入账生成任务、四种页规模通过 |

合规新增私有`numberedpage`只负责只读一致快照中的计数和页查询；其调用前仍由每个领域执行原授权。真实数据库回归发现`GoFrame Fields()`会追加投影，不可用`Fields("*")`清除旧投影再计数；企业和合同改为先构造无投影基础模型并克隆，再分别计数与投影。合同模板原窗口计数会在越界空页丢总数，已改成同一事务内独立计数，页面仍仅读取当前页。

正式六表通过插件自有请求状态保留函数区分真实`401/403`与临时失败，保留原请求客户端和刷新重放。请求序号、冻结查询条件与卸载失效避免旧响应覆盖新页面。`403`清除对应行、总数与详情，`503`保留已确认页。合同预览、发票分配等单据内有界明细不被误改成远程业务分页。

## 迁移动态交付

冲突接口传递`page/pageSize`，返回宿主记录存储能力的原始`total`；不跨插件查数据库，不以当前页长度冒充总数。现存`Vue`实现与实际发布的`frontend/dist/admin/migration`隔离入口都接完整`Vben`分页，但当前已安装动态插件只通过系统配置中的`/plugins/idc-zjmfmigrate/migration-checks`入口可达；不能将未注册的`/admin/idc/migration`称为正式已验收页面。隔离入口不读取宿主令牌或状态，仅使用原`idc-migration.admin.v1`消息桥；因桥不公开可靠`HTTP`状态，任何未知拒绝按失败关闭清除数据，不能伪称可区分`503`。切换批次和迟到响应有序号保护。

发布检查发现旧静态入口仍是原生表，并显示“最多一百条”；该入口已真正接入共享隔离运行时，不是只改正式页面。保留原列和冲突处理动作，没有筛选条件所以不添加空搜索表单。页规模十、二十、五十、一百，默认十；摘要列自适应，操作列固定右侧，高度有界，旧原生表样式及会污染运行时的通用颜色变量已隔离。

按主线授权升为`v0.2.1`，同步清单断言及中英文说明，新增明确依赖`vben-adapter =0.1.0`。没有新增`SQL`，没有清空或覆盖配置；安装与升级由主线通过正式治理入口执行，本代理只构建。

- 官方构建命令：根目录`make.cmd wasm dir=apps/lina-plugins/idc-zjmfmigrate out=temp/20260904/233000-migration-v021`。
- 产物：`D:/golang/cloud/temp/20260904/233000-migration-v021/idc-zjmfmigrate.wasm`。
- 容量：`41,552,751`字节。
- `SHA256`：`00C1BCAC00406DEE2D8566B94BD1B3CEC4AF127E627D334AAEE79A65F2BC5A3A`。
- 结果：构建通过，未上传、未安装、未重启。不能把该产物的构建成功写成已安装版本的验收成功。

## 精确验证命令

### 核心真实数据库

从`apps/lina-plugins/idc-core`运行，`GOWORK=off`。`LINAPRO_IDC_CORE_TEST_DSN`由本机配置读入进程环境，不记录或回显秘密。以下每个测试自行创建随机隔离环境并清理：

```powershell
go test ./backend/internal/service/customermanagement -run '^TestDAONumberedCustomerPagesCountsAndFilters$' -count=1 -parallel=1 -v
go test ./backend/internal/service/catalog -run '^TestCatalogManagementNumberedPagesPostgreSQL$' -count=1 -parallel=1 -v
go test ./backend/internal/service/commerce -run '^TestAssistedQuoteNumberedPagesPostgreSQL$' -count=1 -parallel=1 -v
go test ./backend/internal/service/productfulfillment -run '^TestIntentNumberedPagesPostgreSQL$' -count=1 -parallel=1 -v
go test ./backend/internal/service/billing -run '^TestManagementFinanceDirectoryPostgreSQLReadsBoundedFacts$' -count=1 -parallel=1 -v
go test ./backend/internal/service/marketing -run '^TestMarketingNumberedDirectoriesPostgreSQL$' -count=1 -parallel=1 -v
```

以上六组本轮通过；随后补齐并通过退款和缺失成本两组多页真实数据库专项，精确命令与覆盖见前文及下文。不将普通边界测试或浏览器替身描述为真实数据库专项。

### 合规真实数据库

从`apps/lina-plugins/idc-compliance`运行，`GOWORK=off`及私有`LINAPRO_IDC_COMPLIANCE_TEST_DSN`环境。最新生产代码冻结后串行复跑如下六组，全部通过：

```powershell
go test ./backend/internal/service/electroniccontracttemplate ./backend/internal/service/contractautosign ./backend/internal/service/enterpriseverification ./backend/internal/service/electroniccontractpreparation ./backend/internal/service/taxinvoicemanagement ./backend/internal/service/taxinvoiceredletter -run '^(TestElectronicContractTemplatePostgreSQLWorkflow|TestEnterpriseAutoSignAuthorizationPostgreSQL|TestEnterpriseManagementPostgreSQLRejectsAndFlagsDuplicate|TestManagementContractNumberedPagesPostgreSQL|TestTaxInvoiceNumberedPagesPostgreSQL|TestRedLetterNumberedPagesPostgreSQL)$' -p=1 -parallel=1 -count=1 -v
```

单项实际测试耗时依次为`0.40s`、`0.39s`、`0.69s`、`0.94s`、`0.36s`、`0.52s`。模板最新显式字段投影亦在本次覆盖。税票的数据库约束只允许平台范围，测试未伪造非法租户行；不把其测试描述为独立的跨租户插入证明。

### 编译、类型及单元测试

- 核心八包普通测试、核心实际页面`vue-tsc`及二十项私有前端单测通过，命令见前文。
- 合规`numberedpage`、六个领域服务、后台控制器与插件绑定编译通过；迁移`go test ./backend ./backend/internal/service/migrate ./backend/internal/controller/admin`通过，含新页界校验、准确总数透传与`v0.2.1`清单断言。
- 合规与迁移私有前端单测各二十项通过：每个插件`frontend`目录运行`../../../lina-vben/node_modules/.bin/vitest.cmd run --config D:/golang/cloud/temp/20260904/231500-idc-compliance-vitest.config.mjs`或对应`231500-idc-zjmfmigrate-vitest.config.mjs`。
- 实际合规六表及迁移页面类型通过：从`apps/lina-vben`运行`pnpm exec vue-tsc --noEmit --project D:/golang/cloud/temp/20260904/223000-compliance-pagination-tsconfig.json --pretty false`。
- 从`hack/tests`运行`pnpm exec tsc --noEmit -p ../../apps/lina-plugins/idc-compliance/hack/tests/tsconfig.json --pretty false`与`pnpm exec tsc --noEmit -p ../../temp/20260904/231600-migration-tests-tsconfig.json --pretty false`均通过。
- 根目录`node --check apps/lina-plugins/idc-zjmfmigrate/frontend/dist/admin/migration/app.js`通过。
- 根目录`make.cmd lint.go dir=apps/lina-plugins/idc-compliance plugins=1`零问题；`make.cmd lint.go dir=apps/lina-plugins/idc-zjmfmigrate plugins=1`零问题并通过动态`guest`矩阵死代码检查。
- 最新根目录`make.cmd i18n.check`零违规，覆盖新增接口文档与稳定错误资源。

临时验证配置使用既有`Node.js`、`TypeScript`、`Go`工具，没有新增宿主依赖或平台专属正式构建脚本。Windows 使用`.cmd`薄入口，其他平台使用相同命令的`make`或`linactl`入口；本机未声称执行了其他操作系统。

## 浏览器回归交接与审查状态

主线首轮`TC008a-d`四项均未通过。已读取错误上下文并审阅合同失败截图：`temp/20260904/240000-compliance-payment-pagination`。企业页只有最小高度的自尺寸容器嵌套百分比表格高度，导致高度循环；合同和税票的固定表高叠加标题、策略及搜索区，把分页挤出视口，并因容器缺少最小宽度约束扩展整页。另企业`Vxe proxy`会在失败后重新回写请求页码，覆盖已确认页。

六表及合同父页已改为受控剩余高度链、最小宽度约束和表内滚动；五个已有筛选的列表使用`formOptions`，自动签授权没有筛选，不加入空搜索表单。企业改为与其他列表一致的直接页事件读取。自动签原创建区仍完整保留，在自身区域滚动。最新实际页面`vue-tsc`及测试`tsc`通过，`TC008`保留原业务、权限和分页断言，并增加分页在视口、页面无横向扩展、多行正文及连续高度稳定验证；尚待主线重跑，不能仅凭类型检查宣布解决。

迁移测试首次发现`import.meta.url`被按公共`CommonJS`解析；仅新增插件自有`hack/tests/package.json`声明`type: module`，未改共享测试器。`pnpm exec playwright test 'TC003-conflict-pagination' --config=playwright.config.ts --list`已正常发现正式与隔离入口两项。主线已反馈通过正式治理入口升级`v0.2.1`，安装本身不替代页面回归。

第二轮主线结果位于`temp/20260904/250000-compliance-migration-revalidation`：合规`TC008a/d`通过，`b/c`失败。已审阅截图和跟踪：合同策略卡与三字段搜索的换行按钮挤压正文到七十六像素；税票在未挂载的红冲页签直接调用`formApi.setState`抛错。继续修复为短字段标签、桌面四列搜索、紧凑策略卡及自动签创建区域自身滚动；税票通过始终可用的`gridApi.setState({ formOptions })`配置隐藏页签，不绕过延迟挂载。保持正文大于九十像素、分页在视口和高度稳定断言，不降低标准规避问题。生产实际页面类型检查再次通过；最终`TC008`四项全部通过，结果位于`temp/20260904/final-compliance-pagination`。

迁移第二轮`TC003a`截图确认旧测试地址返回四百零四；用例已改为系统配置、数据迁移分类、迁移检查抽屉内的实际动态沙箱，不新增宿主路由。`TC003b`跟踪明确显示测试壳遗漏`allow-forms`，浏览器禁止提交表单，预检请求没有发送。仅将测试壳沙箱权限对齐正式宿主的现有权限，仍不允许`allow-same-origin`；增加检查通过断言以便快速定位，并给两个入口补灰色表头、白色正文和分页可见断言。未更改生产产物、预检返回或按钮门禁。插件测试类型检查通过；最终真实页和隔离产物两项全部通过，结果位于`temp/20260904/final-migration-pagination`。

退款与缺失成本两项真实 PostgreSQL 已补齐并通过，耗时分别为`0.13s`和`0.09s`，不是缺少连接配置时跳过。退款覆盖二十五条、其他客户、状态、租户及授权拒绝；缺失成本覆盖二十五条、四种页规模、空页、非缺失状态排除、其他租户排除及重复已消费绑定不重复计数。测试注册分阶段清理，包含初始化中途失败；不修改生产表，不冒称完整退款写流程验收。

以下回归已通过类型检查并由主线完成最终浏览器运行；过程中的失败截图与最终通过结果均保留：

- 核心`TC001`客户、`TC002`商品、`TC005`协助报价、`TC006`财务原用例的分页替身与断言同步；`TC003j`新增营销九表二十五条、第三页五条、每页二十条及准确查询参数断言。
- 合规`TC008a`企业、`TC008b`合同及模板及自动签、`TC008c`税票与红冲，逐表验证十条、第三页五条、二十条和准确总数；`TC008d`验证临时故障保留与权限拒绝清空。
- 迁移`TC003a`系统配置内已安装动态页二十五条分页；`TC003b`真实静态产物在与正式宿主同等权限的隔离框架中验证分页、拒绝清空和恢复。静态资源由测试原样提供，不改产物源码，网络业务由明确替身承担。

按`lina-review`执行交付预检，文件范围见[本轮文件清单](phase-28-remaining-pagination-files.md)。已检查插件根无额外`AGENTS.md`，保持插件拥有方、原授权和事务边界；接口多语言资源及静态门禁已通过。当前仍有阻塞验收的测试缺口：统一真实浏览器逐页与截图审阅、安装后的迁移新版本行为及合规首轮失败后的完整复测。不得据此标记`FB-110`整体完成或归档；主线综合审查需合并其他代理范围及最终门禁。

## 恢复事件记录

本轮曾因错误文本转换清空`idc-compliance/frontend/pages/enterprise-verifications.vue`。发现后立即停止该文件修改并通知主线，使用本轮清空前刚读取的完整四百二十八行文本恢复，没有使用`Git`覆盖用户改动。随后将删除事件的完整前像与恢复文本按统一`LF`逐字比较，相等，双方`SHA256`均为`C89DF0B1990FEEE28E885775264D4CA128B17680D9CCFAD122C6A4A6C16798CB`。证据事件时间为`2026-09-04T09:34:09.643Z`，原恢复审计位于本轮本地会话记录。恢复确认后，只通过`apply_patch`有界修改该文件，企业专项及最终六表实际页面类型检查均通过。该事件已准确恢复，但不隐藏此次操作失误。
