# 功能删除与管理入口收拢验收

## 范围与业务结论

本次只处理用户确认的电子合同、独立隐私治理、员工企业微信和资源入口调整。保留实名认证、企业认证、发票、其他通知、必要的材料留存与人工删除。工作区存在其他会话并行改动，未提交、推送或重启共享后台。

- 电子合同页面、客户入口、模板与签署人、供应商签署调用、合同规则、履约门禁及合同退款补偿已经移除。变配退款、风控、正常下单付款和交付继续保留。
- 材料留存策略、分页到期待办、法律保全和人工确认删除收进企业认证管理抽屉；客户授权记录与撤回收进客户认证页面。到期扫描不等于批准删除。
- 员工企业微信绑定、扫码、通知投递、本人测试及专用身份来源已经移除。升级迁移停用相关配置并保留历史投递与同意证据。
- 服务器、证书与关联任务放入客户产品管理；供应商全局运维放入接口配置。产品任务在数据库按所属产品过滤后计数分页。没有可靠产品关联的历史日志不会伪造归属，仍在具体供应商接口下查看。
- 历史合同与法律证据不在本次物理销毁范围。保留通用网关查询、回调与发票依赖的历史投影迁移，已删除的电子签客户端与业务路由不会重新装配。

## 验证证据

| 范围 | 结果 |
| --- | --- |
| `idc-core`生产构建、全包无数据库测试与官方`make lint` | 通过；实际源码插件汇总与注销提供者装配测试通过 |
| `idc-core`合同删除相关服务、能力、控制器与装配测试 | 通过；包括普通退款、续费、客户汇总和服务器操作 |
| 核心真实`PostgreSQL`回归 | 普通原子结账、渠道结账、付款风控、商品导入、履约风控与订单分页通过；商品、客户管理、履约包全量数据库测试通过 |
| `idc-notify` | 全量`Go`测试、数据库服务测试、官方`lint`、前端构建及退役迁移重复执行通过 |
| `idc-compliance`及本地网关 | 两模块真实`PostgreSQL`全量测试和官方`lint`通过；一次性数据库官方`make dao`生成通过 |
| 材料留存 | 新装与重复迁移、保留触发器、无评估时的材料策略门禁、分页总数、保全拒删、解除后人工确认删除通过 |
| 资源与动态产品插件 | 归属隔离与分页数据库测试、路由/服务测试、三个模块官方`lint`、两动态插件构建与前端单测通过 |
| 前端 | 合同删除页面及资源内嵌页面的定向`vue-tsc`通过；客户中心完整类型检查、翻译结构检查及十九项相关客户端单测通过 |
| 规范与翻译 | `openspec validate add-idc-server-operations-platform --strict`通过；运行时语言覆盖和前端语言键覆盖通过 |

## 浏览器验收

页面行为使用隔离的接口响应驱动真实浏览器；数据库规则由独立数据库集成测试验证，不将页面模拟响应当作真实供应商结果。

- `idc-core/TC004c/e`：普通订单与缺少明细订单正常查看，无电子合同入口及合同请求。
- `idc-core/TC039e/f/g`：客户产品精确管理旧服务器、动态权益服务器及证书；动态服务器详情失败重试、续费预览与确认固定当前权益，任务按产品过滤；只有任务权限及完全无管理权限的角色分别覆盖。
- `idc-resource/TC007j/k`和`TC015a`：接口任务与日志固定当前接口、未打开页签不读取；仅任务或日志权限不请求接口配置；旧独立路由退役，嵌入页分页保持在视口内。资源相关九例通过，最后布局调整后三例复验通过。
- `idc-notify/TC001a/h`：企业微信渠道删除、其他渠道及投递记录保留，中文和英文检查通过。
- `idc-compliance/TC003f`：完整权限与仅材料权限两变体通过，覆盖期限更新、设置与解除保全、保全时不可删除、二次确认后删除状态；窄权限变体企业认证查询严格为零。
- `customer-default/TC008a`：认证页按需加载授权，失败可重试，确认前无撤回请求，确认后显示已撤回，无独立隐私与合同入口。

关键截图已视觉审查，位于`temp/20261022/`：`112729-order-contract-retired.png`、`114117-workspace-product-server-management.png`、`114126-workspace-product-certificate-management.png`、`114136-provider-interface-scoped-logs.png`、`114248-material-retention-opened.png`、`114250-material-retention-confirmed.png`，以及`verification-consent-results/`内的`114013-verification-consent-retained.png`。

## 审查结论与验证边界

首轮按`lina-review`检查并修复删除合同贡献者后启动完整性计数未同步、服务器结算合同门禁、客户合同页签、权限白名单、注销阻断文案和失效翻译键。二轮实际链路复核又发现下文问题，首轮通过证据不代替这些新增修复的验收。宿主目录`apps/lina-core/`、`apps/lina-vben/`和根`hack/`未纳入本次修改。

以下区分已通过检查与整体验收边界；`REMOVE-5`因范围外全局扫描及共享运行环境尚未统一加载验收保留未完成：

- 二轮已补齐`financeops/operations_postgres_test.go`测试替身的`AuthorizeBalanceAdjustmentCreate`方法；并发会话补齐商品配置和恢复配置文档后，当前`idc-core`全包无数据库测试已通过，见`/tmp/removal-round2-core-final.log`，不将未配置数据库时跳过的集成测试计为数据库通过。
- 全工作区`make i18n.check`的当前阻断仅剩单语言插件`customer-tdesign`的`15`个文件、`152`项`frontend-property-han`；`website-qycn`自身扫描项已全部修复。`customer-tdesign`明确声明`i18n.enabled: false`，现行语言规则允许其使用中文。负责人核查确认扫描器不读取插件单语言声明，统一命令不透传插件配置，默认白名单位于禁止修改的根`hack/`，因此当前允许范围内没有有效修复入口。这是扫描边界与单语言规则不一致，不是缺失翻译键或`152`个业务缺陷；语言资源覆盖及前端键覆盖仍通过，全仓扫描仍不得记为通过。当前证据为`temp/20261022/230000-tdesign-i18n-boundary.log`及`openspec/changes/add-tdesign-customer-template/review.md`末节。
- 首轮`commerce`数据库套件十分钟超时后，二轮扩展超时完成整个套件：顶层用例`141`项通过、`1`项显式基准测试跳过、`1`项并发更新前旧详情断言失败，耗时约`1151`秒；当前源码该失败用例独立复验通过。原长跑结果仍保留失败记录，不冒称原套件全绿。

共享后台未重启，动态插件未重新安装。页面验收加载当前源码或冻结的当前前端产物；运行中旧菜单仍可能来自后台旧清单快照。加载本次插件清单与产物后才会反映全部入口变化。

## 二轮实际链路复核

本轮范围内代码、数据库回归和页面验证已完成；整体验收边界及构建产物残余分别记录，不将局部通过等同已部署。

- 启动完整性：新增`idc-core/hack/tests/startup-registration/startup_registration_test.go`，使用官方构建工作区实际导入全部源码插件，验证八种客户汇总和五种注销检查均有唯一真实提供者；已通过。该测试仅执行插件声明与注册，不启动服务或第三方调用。
- 企业微信迁移：发现退役迁移误删历史绑定审计表，以及真实数据库测试仍期待八种通知渠道；已保留不可变审计历史，定向移除其活动绑定外键，并在入队前拒绝退役渠道。真实旧版完整表结构升级、审计整行及其他外键保留、重复迁移、七种保留渠道及全通知数据库测试通过。
- 工作台：移除`operations/workbench.go`遗留合同异常、待处理合同统计和合同补偿表水位查询；数据库回归不再创建合同表，并确认刷新成功且不发布合同队列。整个`operations`真实数据库测试通过，记录位于`/tmp/removal-round2-operations-postgres.log`。
- 材料留存：已修复已有待办占满扫描首批、确认删除未重检当前期限，以及法律保全无页面入口；扫描采用一次候选查询和一次批量写入。补齐个人与企业认证成功快照的到期人工处置，保留认证结果和历史证据；修正个人快照与企业上传字节数的冲突约束。仅有材料权限的角色也能进入抽屉，且企业认证列表请求数为零。当前数据库、保全/解除/确认删除及两种权限页面回归通过。
- 客户产品管理：已按`service_entitlement`真实归属补齐动态产品详情与续费，保留旧服务器任务查询；供应商配置使用无业务权限容器，内部各项请求仍按原精确权限执行，原菜单只保留授权节点而不再贡献独立路由。新增拥有方过滤、续费正向确认及跨产品拒绝数据库回归通过。
- 核心数据库：整个`commerce`套件使用独立测试数据库模式运行完成。长跑编译快照中`TestProductFulfillmentPreparationRecoveryPreservesHistoryPostgreSQL`的旧详情断言失败；检查时该断言已被并发会话修改，当前源码该用例真实数据库复验通过，见`/tmp/removal-round2-commerce-recovery-recheck.log`。未启用需要显式请求的基准压测。

### 共享范围缺口复验

早期`/tmp/removal-round2-core-tests.log`显示两组接口文档覆盖失败，已记录交协调会话处理，未修改商品目录文件。当前`/tmp/removal-round2-core-final.log`已全部通过，下述缺口均已解除。

- `backend/api/configuration_workspace_apidoc_i18n_test.go`：`CatalogCustomerFieldSelection`缺少`productChoiceBindings`、`informationOnly`、`candidateLabels`中文说明；`CatalogProductVersionConfiguration`缺少`pendingAutomationFields`、`directoryRelationships`、`customerChoiceRules`、`merchantPriceTargets`、`productChoiceFields`、`customerProductFields`、`upstreamOptionsStatus`中文说明。完整键前缀为`plugins.idc_core.api.admin.v1.`。
- `backend/api/admin/v1/product_fulfillment_diagnostics_test.go`此前缺少恢复配置的`generationLength.dc`与`generationCharacters.dc`；该组及商品配置十个中文说明均已通过最终复验。

本轮核心无数据库测试日志位于`/tmp/removal-round2-core-tests.log`；真实插件注册测试日志位于`/tmp/removal-round2-startup-registration.log`；全量商务数据库运行记录位于`/tmp/removal-round2-commerce-postgres.jsonl`。临时日志不作为可持久保存的测试资产，相关新增回归保留在各插件测试目录。

最终证据另包括`/tmp/removal-round2-core-final.log`、`/tmp/compliance-round2-final-pg.log`、`/tmp/compliance-round2-permissions-e2e.log`、`/tmp/resource-round2-verified-pg.log`、`/tmp/resource-round2-verified-e2e.log`和`/tmp/resource-round2-layout-e2e.log`。二轮关键截图位于`temp/20261022/`：`131837-material-retention-only-held.png`、`131839-material-retention-only-confirmed.png`、`131937-workspace-native-server-renewal-result.png`和`132247-provider-configuration-task-only.png`，已视觉审查翻译、操作反馈和分页布局。各自创建的独立预览进程均已清理，冻结产物与证据保留。

### 构建产物写入偏差

二轮合规界面验证构建虽设置`VITE_ARCHIVER=false`，仍意外生成宿主路径`apps/lina-vben/apps/web-antd/dist.zip`，文件大小`3366592`字节，修改时间`2026-10-22 21:04:25.486400869 +0800`。这是构建产物写入宿主的边界偏差，不能以无源码差异视为完全未写宿主。

已按宿主边界规则核查精确撤回条件：该文件创建时间为`2026-10-19 15:03:40.572687386 +0800`，早于本轮；此前`/tmp/embed-resource-ui-build.log`、`/tmp/embed-resource-ui-dev-build.log`、`/tmp/embed-resource-ui-final-build.log`也记录写入同一路径。本轮覆盖了已有产物，未取得可信的覆盖前摘要或快照；找到的更早备份不能证明是覆盖前内容。因此无法安全还原，未通过删除既有文件或套用旧备份伪造恢复。已停止该构建路径，残余覆盖交协调会话统一处理；后续页面验证使用隔离临时预览，构建显式禁用归档插件，不重启共享后台。

## 用户追加的两轮复查

本轮沿用`lina-feedback`的`openspec-existing`路径，限定原删除功能及其保留能力；商品目录和交易实现由对应会话负责。已知单语言扫描及宿主菜单类型门禁不重复排查，既有`dist.zip`残余不通过删除原文件伪造恢复。以下新增行为已完成两轮复查，原有通过证据不替代新增反证；范围外门禁与共享运行环境边界保持如上。

编译协调：合规删除实现已完成原子修改，首次扫描锁序修复后在`2026-10-22T16:06:47Z`再次以`GOWORK=off go -C apps/lina-plugins/idc-compliance test ./backend/internal/service/privacygovernance ./backend -run ^$ -count=1`确认编译通过，当前已无先前的中间态缺失函数或未使用变量；可继续依赖该插件的完整构建。证据`/tmp/compliance-build-readiness.txt`。材料生产后端与迁移已于`2026-10-22T16:11:07Z`冻结，`311`个生产`Go/SQL`文件摘要见`/tmp/compliance-backend-final-fingerprints.txt`；此后旧版约束验证只修改测试，后端可供交易会话统一构建。

### 第一轮：实现、升级与失败路径

- 企业微信：真实数据库复现越界未知任务升级后被发送工作器误标取消。退役迁移改为将越界非终态转人工核对，保留原结果分类、服务商引用和查询证据，清理租约与重试时间，并记录一次退役审计。失败反证见`/tmp/wecom-round3-migration-reproduce.log`。
- 材料留存：发现多重保全解除单项后错误显示可删除；另发现外部删除成功但后续数据库失败会丢失确认记录。已补有效保全重算、先提交确认意图再删除原件及失败后的有权恢复；最终并发交叉复验通过。
- 产品管理：发现上下文切换后晚到预览仍可打开旧弹窗，旧附件请求亦缺少目标与卸载失效保护；插件目录读取失败会被永久缓存为未安装。已补上下文代次、操作时权限重验和插件状态刷新；另补同页两个产品的入口响应顺序保护，当前五项受影响浏览器用例通过。
- 根范围检查：当前核心非生成生产代码未发现电子合同门禁、合同补偿表查询或独立客户合同/隐私路由残留；续费代码中的`entitlement_contract_no`表达插件版本契约，并非电子签合同，不作关键词式删除。保留历史合同表及其外键证据，不恢复合同工作器。

### 第二轮：危险交叉路径对抗验证

- 企业微信已通过：新装七种渠道、真实旧版审计外键、重复迁移、五种越界非终态与三种终态、旧工作器回写围栏、连续发送/查询工作器、正常站内信、人工结案及禁止重发均覆盖。迁移审计不重复；当前真实数据库定向测试和`notify`官方`lint`通过，见`/tmp/wecom-round4-adversarial.log`、`/tmp/wecom-round4-notify-lint.log`。第二轮未发现新问题。
- 材料基础故障恢复与已有待办的并发验证通过；只读交叉审又发现首次扫描插入任务的窗口，真实数据库反证确认可能出现`confirmed`任务与新增有效保全并存，既无法完成也无法解除保护。失败证据`/tmp/compliance-first-scan-before.log`，已统一为先锁目标、后锁任务，定位只读后在锁内重读并核对目标；当前包编译通过，首次窗口、已有待办的保全/审批先后顺序及并发确认/恢复均通过真实数据库`-race`复验，见`/tmp/compliance-first-scan-fixed-race.log`；当前官方`lint`为零问题，见`/tmp/compliance-first-scan-fixed-lint.log`。只读交叉审确认已无原反向锁序。
- 产品第二轮发现证书刷新撤回动作能力后旧表单仍能提交，已收起过期表单并重验当前动作。修复构建的首次浏览器检查又捕获首次挂载关闭未连接抽屉导致列表加载中断，已修复连接前调用；失败记录`/tmp/resource-append-green-e2e.log`保留。当前`TC039g/h/i/j`五例通过，见`/tmp/resource-append-fixed-e2e.log`，覆盖目录失败重试、停用、同页入口乱序、证书能力撤回以及详情到续费确认正向链。根代理已视觉审查`temp/20261022/161109-workspace-certificate-action-withdrawn.png`、`161114-workspace-management-entry-late-response.png`。
- 材料页面新增关闭重开晚响应反证：旧策略值可覆盖新策略值，见`/tmp/compliance-stale-load-before.log`；请求序号、分页快照与关闭失效修复后，完整权限及仅材料权限两项页面用例通过，见`/tmp/compliance-stale-load-fixed.log`。根代理已视觉审查`temp/20261022/161200-material-retention-only-recoverable.png`中的失败提示及继续删除操作。未使用真实第三方发送或删除用户真实材料，未重启共享`6030/6031`；不能据隔离验证声称共享运行实例已加载新增迁移。

### 最终审查与证据边界

本轮按`lina-review`完成限定范围审查，企业微信迁移、删除恢复和资源管理入口未发现剩余阻塞问题。新增留存故障测试见`/tmp/compliance-adversarial-final-pg.log`，真实旧状态约束升级与重复迁移见`/tmp/compliance-historical-constraints-upgrade.log`。生产后端与迁移冻结后指纹未变化。新增页面构建明确剔除归档插件，宿主既有`dist.zip`摘要未再变化；前述历史覆盖残余仍保留，不声明已撤回。

页面验收使用受控接口响应，数据库使用真实`PostgreSQL`隔离模式及外部存储替身，未验证真实第三方故障或进程硬退出。新增验证不包括共享后台重启与安装新清单；`REMOVE-5`的全局门禁和统一运行验收边界仍未解除。

资源补充证据：六项真实`Vue`生命周期测试见`/tmp/resource-append-lifecycle-test.log`，仅列表权限不能直取详情或接管的真实数据库回归见`/tmp/resource-append-direct-scope-pg.log`，官方`lint`及定向组件类型检查见`/tmp/resource-append-lint.log`、`/tmp/resource-append-final-types.log`。新增测试通过`test:management`接入正常命令。资源与合规自有预览均已停止；本轮最终`OpenSpec strict`和文档差异检查通过。
