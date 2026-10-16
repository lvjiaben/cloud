## 实现与验证

- [x] 官网 provider 契约、安装依赖、双模板分发及实际空库安装验证
- [ ] 统一公共配置与各模板独立保存，GET/PUT API 和停用门禁验证
- [x] 官网配置菜单、动态字段及系统配置入口移除，通过 `apps/lina-plugins/idc-portal/hack/tests/e2e/TC001-brand-settings.ts` 和真实浏览器验证（包含加载/保存失败重试、刷新后模板字段恢复）
- [x] Nuxt 按已选 renderer 渲染并通过类型检查及真实模板切换验证
- [ ] 本轮范围 lina-review、Go lint、编译和 OpenSpec 严格检查

## 当前证据

2026-10-14：idc-portal `GOWORK=off go test ./backend/... -count=1` 通过，含持久恢复、配置隔离、停用与非法嵌套内容；`make lint dir=apps/lina-plugins/idc-portal plugins=1` 零问题。两个 WASI provider 编译成功。真实安装和页面切换仍待统一实例验证，不提前勾选。

官网 UI 证据：隔离空库后端 `16030`、源码前端 `16031` 下，TC001 a～c 三项通过（`temp/20261014/171500-website-results`），d 旧入口移除通过（`temp/20261014/172800-website-results`）。测试使用服务实际翻译资源，配置接口受控模拟覆盖模板字段隔离、保存启用、刷新恢复、加载失败恢复、未安装空态与保存失败保留重试。桌面、390px 移动端、失败、空态、系统配置截图已多模态检查，无原始翻译键、文字截断或表单重叠。真实菜单确认归属“系统设置”；`web-antd typecheck` exit 0（其 tsconfig 不直接覆盖插件目录，插件实际编译由源码 Vite 运行验证）。真实官网 provider 安装及无模拟保存联动仍待完成，不勾选整项。

## 本轮范围审查（lina-review）

已检查新 websitecap、siteconfiguration、路由 DI、公共品牌桥接、两个 provider 清单及 Nuxt renderer。无宿主修改；provider 使用权威批量发现（单次最多五页），没有逐插件查询。保存在生命周期准入锁内完成，前端输入不能覆写其他模板配置；非法嵌套字段、危险图片地址、未知 renderer 被拒绝。Go tests、三个 module 定向 lint、Nuxt typecheck 和 diff 空白检查通过。

尚未完成：隔离空库真实 provider 安装、管理 API 保存后官网渲染、TC001 与真实浏览器验收。存储沿用 portal 文件方案，单管理实例写入；多实例需要共同挂载 LINA_SITE_STORAGE，不能宣称跨实例并发更新已覆盖。旧品牌修订 API 仍保留但公共显示改用官网配置页的数据，旧发布内容不再影响新官网。

安装验收发现空方法声明在宿主声明冻结时存在 nil/空列表深比较差异。插件侧采用正式 `capabilities.describe` 唯一只读契约，owner/provider 显式匹配，guest 严格验证冻结身份和运行上下文，仅返回公开模板描述；不修改宿主、不绕过授权。新增 API 使用独立文档化 DTO 与 controller 映射，保持 JSON 字段和 GET/PUT 路由。

正式生产链路补验：`Nuxt`完整`pnpm run build`（含类型、双语、SEO 检查）通过，构建产物以`node-server`运行在`16682`并代理隔离后端`16030`。真实`data.brand`响应中的“简洁官网验收标题”已出现在首页服务端 HTML；品牌与商品目录返回`200`，后台及其他插件路径返回`404`，品牌`PUT`返回`405`。正式匿名配置`evaluate POST`到达后端并收到业务参数校验，不再被代理误拦截。`schema`小写字段投影已统一，新增回归覆盖标题、图片、颜色、安全菜单与非法图片/颜色；最新 portal 全测试和 lint 通过。双语公开 API 文档同步真实响应包裹、生产代理和新官网配置逻辑。

真实图片和导航补验：默认模板通过后台保存`https://example.test/banner.png`与“商品|/products”“帮助|/pages/help”，正式`16682`品牌响应和首页 SSR HTML均出现保存的标题、图片地址和两个菜单。图片地址仅核对配置投影与渲染，不宣称示例远端图片可访问。停用实测暴露官网 owner 未注册业务影响处理器，现已在 portal 插件侧补齐公开处理器并新增回归，最新全测试、lint、严格检查通过；正常停用/恢复流程待重新构建后的真实验收。

正式 UI 回归补验：TC001 a～e 在隔离实例 16030/16031 全部五项通过（`temp/20261014/182800-website-results`）。fixtures 与真实 schema 统一小写字段；e 不模拟官网接口，通过 UI 保存两个实际已安装官网，断言公开 renderer、标题、图片及公共网站名称，刷新后恢复各模板独立内容，并 finally 恢复原公共资料、两模板配置与已选模板。动态字段 description 透传短 placeholder，菜单示例断言为“每行一个：商品|/products”。质量审查覆盖成功、加载/保存失败、未安装空态、旧入口删除、实际投影及清理；截图多模态检查未见翻译键或表单重叠。停用仍受公开 lifecycle 业务影响检查 code50 阻断（owner=unknown），已定位交由 provider 处理，不绕过门禁。

生命周期剩余阻塞：重新构建后 owner 缺失已修，但真实停用暴露宿主`SyncManifest`重建 release snapshot 时丢失`AuthorizedBusinessExtensions`。启用另一个插件可导致目标冻结快照授权为空，后续停用在调用 owner handler 前返回`ownerPluginId=unknown`。公开重新启用目标只能临时恢复其授权，不能作为耐久修复；本轮禁止修改宿主，未直写系统表或绕过冻结授权。停用/恢复整体能力仍不勾选，最终验收需明确这个限制。

## 最终范围审查结论

本轮`lina-review`覆盖安装工具及官网 owner/provider、配置保存与公开投影、后台页面和生产代理。最新后端测试和定向`lint`、公开完整构建、官网生产构建及TC001全部五项通过；插件配置采用权威候选与生命周期准入，公共接口只投影选中模板，未新增宿主修改。安装级数据权限和单管理实例文件写入限制已在设计及安装指南明确。

阻塞发现：`apps/lina-core/internal/service/plugin/internal/store/store_release.go:310`重建快照时调用`store_authorization.go:169`仅继承宿主服务授权，未继承业务扩展授权；`lifecycle/lifecycle_business.go:174`在官网owner处理器前拒绝空业务授权，返回`ownerPluginId=unknown`。真实跨插件操作已复现，公开重新启用仅临时恢复，无耐久补偿扩展点。依据宿主禁止修改规则，保持宿主不变，不直写系统表、不取消授权、不勾选停用和整体审查完成项。先前“待真实安装/页面保存”记录是阶段性历史，现已由上述实测补齐；停用恢复仍未通过。

最终阻塞定位（仅只读宿主源码，未修改）：`apps/lina-core/internal/service/plugin/internal/store/store_release.go:310`重建快照，`:324`只复用 hostServices；`store_authorization.go:190`仅携带`AuthorizedHostServices`，没有携带业务授权。`lifecycle_status.go:57`先检查业务已排空，`:61`才同步和执行后续动态/global 生命周期；`lifecycle_business.go:182`发现声明存在但授权为空即报`unknown`，在`:132`调用 owner 之前失败，因此 owner/global handler无法补偿。真实复现文件`temp/20261014/181500-authorization-loss-repro.json`：公开重新启用 minimal 后 impact 成功，再启用 default，minimal impact/disable均报`unknown`；重新启用只是临时恢复，升级/重装也不能防下一次其他插件状态操作擦掉授权。

额外发现：`businessdiscovery/businessdiscovery.go:55`将名为`AuthorizedBusinessExtensions`的字段从`businessExtensions`原始声明读取，因此官网保存/发现成功不能证明真正的`authorizedBusinessExtensions`冻结授权有效，不能据此宣布生命周期验收通过。

已再次评估正式 owner-aware hostServices 登记 schema 替代路线。公开 descriptor 能校验 caller、依赖、方法授权与 owner 启用；但动态`pluginbridge/contract/contract_lifecycle.go`没有`BeforeEnable/AfterEnable`操作，`capability/plugincap/plugincap.go:109`的`PluginInfo`没有停止新业务状态或 release/declaration授权证据，普通`Registry.BatchGet`不能替代持锁的`BusinessExtensionRegistrar.WithNewBusinessAdmission`。后者必须匹配 BE 声明，不能在移除声明后复用。因此注册schema＋批量installed/enabled检查虽能持久显示，却不能等价保留冻结声明治理、停止新业务和原子准入。本轮没有移除声明、直写系统表、伪造授权或修改宿主；当前公开缺口阻止该替代方案安全实施。

## 用户纠正后的官网选择验收（2026-10-14）

业务依据为`.agents/memory/preferences/72-customer-website-visual.md`的“官网插件与配置”及本变更`specs/website-configuration/spec.md`。插件可用状态与官网展示选择分开；历史停用授权问题仍独立遗留，不是保存切换官网的验收依赖。保留上方联合任务和整体审查任务的未完成状态，不将本次选择验收解释为生命周期缺陷已修复。

- [x] 验证多个官网插件可用、仅一个官网对外展示；未保存及保存失败保留原官网，成功后唯一更新使用中，切回恢复独立配置。
- [x] 完成本次偏好、规范、安装说明、双语提示及`TC001`增强范围的`lina-review`，通过对应回归、翻译检查和规范检查。

沿用隔离后端`16030`、源码前端`16031`，设置`E2E_WEBSITE_REAL=1`执行`TC001-brand-settings`，五项全部通过（`50.0s`）。`TC-1a/c`覆盖未保存、失败重试和唯一使用中标识；`TC-1e`真实保存两个已安装模板，核对公开品牌的`renderer`、标题、图片及切回恢复，候选仍包含两个模板，最后恢复原公共资料、模板配置和选择。报告为`temp/20261014/continue-website-report/index.html`，结果目录为`temp/20261014/continue-website-results`。

桌面、窄屏与真实切回截图已检查，包括`temp/20261014/1791972546194-website-loaded.png`、`temp/20261014/1791972547341-website-mobile.png`和`temp/20261014/1791972583788-website-real-independent-restored.png`。页面无表单重叠或原始翻译键；运行实例仍加载旧提示文案，新双语提示仅完成`make i18n.check dir=apps/lina-plugins/idc-portal plugins=1`静态验证，需重新加载插件翻译资源后生效，不将截图作为新文案运行生效的证据。

本次范围审查未发现阻塞问题：测试断言实际选择结果、公开投影、失败保留与清理恢复；文档与规范匹配最新确认。未修改生产逻辑、接口、权限、缓存、运行期装配或宿主。既有后端实现已区分编辑选择与生效选择，并保留各模板配置；本次无需重复后端编译和生命周期操作。`openspec validate website-plugin-configuration --strict`及本次路径空白检查通过。

## 插件侧构建修复与重新加载验收（2026-10-14）

- [x] 清除四个`idc-core`控制器构造文件末尾的生成器重复空声明，保留带依赖注入的业务构造函数；补齐现有币种读写权限的共享目录，新增漏登回归，并将注册测试的发布契约版本更新为`v0.1.100`。
- [x] 对当前源码完成四个控制器包、权限目录和插件启动注册包测试，定向`make lint`及完整源码插件构建；重新启动隔离后端并加载新翻译。

构建时另有192个重复空骨架已由并发任务移除，本任务没有删除这些文件。上述四个构造文件仅移除重复尾部，其他会话已有的业务与绑定改动保留。权限目录只登记清单和路由已声明的`idc:admin:currency:read`与`idc:admin:currency:write`，不新增权限授予或数据访问例外。测试日志为`temp/20261014/reload-controller-tests.log`，静态检查为`temp/20261014/reload-idc-core-lint.log`，完整构建为`temp/20261014/reload-build-next.log`；均通过。

重载翻译后，`TC001-brand-settings`五项通过（47.4秒），报告为`temp/20261014/reloaded-website-report/index.html`。真实保存切回截图`temp/20261014/1791973456054-website-real-independent-restored.png`已显示“可安装多个官网，在这里选择一个展示。”；窄屏与加载失败截图也已多模态检查，未发现原始翻译键、表单重叠或提示截断。故前述“新提示仍待加载”阶段记录已补齐。最后再次重载包含权限目录修复的隔离后端，翻译接口就绪返回`200`，常规开发后端未操作。

本轮`lina-review`覆盖上述局部构建修复、权限目录补齐、版本测试及重载证据，未发现新增阻塞问题。控制器依赖仍由原启动装配创建并传入，没有新增服务实例。页面选择与保存流程复用前述端到端回归，无新增界面行为。生命周期冻结授权缺口继续保持未完成：公开`BusinessExtensionRegistrar`仅提供发现、准入和调用，没有恢复授权入口；拒绝发生在插件处理器之前。本轮未修改宿主、未移除业务扩展声明、未直写宿主表，未将临时重新启用冒充耐久修复。

## 用户授权同步官方宿主后的复测（2026-10-14）

- [x] 按本轮明确授权检查官方上游并合并四项最新宿主修复，保留本地独有能力和未提交改动；重新构建隔离实例并实测授权丢失场景。

官方仓库`https://github.com/linaproai/linapro.git`的`main`为`75395c8a10ff54817dd688169613bad2161ce0fd`，提交时间为2026年9月7日，最新稳定标签为`v0.5.0`。本地不是官方历史的直接分支，且已有扩展能力；未直接覆盖官方全树。合并`3aea1898`之后的四项更新：认证后检查租户插件启用状态、菜单翻译投影复用、侧栏顶栏位移修复及`Windows`进程句柄清理诊断，共14个源码／测试文件。两处冲突分别保留本地`lockSidebarMenuOpen`与上游菜单投影，采用上游查询结果不受清理失败覆盖的处理。所有同步前文件备份及逐文件计划在`temp/20261014/upstream-sync-75395c8a/`，没有暂存、提交或推送；授权边界记录于偏好主题74，仅限本次上游同步。

验证通过：新增认证租户检查、宿主生命周期及启动绑定包测试；宿主`make lint`、完整源码插件构建；前端5项菜单投影单测和`web-antd typecheck`；`Windows`进程测试交叉编译及当前平台进程测试；侧栏动画两项端到端测试及截图检查；官网`TC001`五项真实回归（44.6秒）。前端类型检查首次被终止，重试通过。宽范围测试没有全部通过：`TestInstallRepairsEmptyReleaseArchive`卸载被依赖门禁拒绝，`TestBusinessLifecyclePostgreSQLSchema`未限定数据库`schema`导致返回8列；上游菜单三项用例依赖本项目不存在的默认“工作台／分析页”或首次尚未就绪的偏好按钮，均保留失败证据，不宣称全回归通过。日志分别为`temp/20261014/upstream-host-tests.log`、`upstream-runtime-targeted.log`、`upstream-layout-tests.log`、`upstream-host-lint.log`、`upstream-frontend-typecheck-retry.log`、`upstream-ui-e2e.log`与`upstream-website-e2e.log`。

本轮`lina-review`按官方增量和备份差异限定范围，保留本地其他会话内容；没有自行补写宿主授权逻辑、变更插件契约或新增运行期依赖。触发界面质量审查，依据上游单测、动画端到端截图及官网真实保存回归，菜单全布局端到端仍有上述验证缺口。同步结果已加载到隔离后端`16030`，前端`16031`使用合并后的源码。实测结果`temp/20261014/upstream-authorization-loss-repro.json`表明：重新启用`website-minimal`后影响检查成功，再启用`website-default`后，前者影响检查和停用仍返回`code=50`及`ownerPluginId=unknown`；已恢复`website-minimal`启用和新业务接收状态。官方更新没有修复授权丢失，原生命周期未完成项继续保留。

## 官网导航与独立模板交付（2026-10-16）

本轮限定插件及官网交付应用，严格不修改宿主。参考魔方`MenusController.php`的当前模板页面扫描与`Menu.php`头尾树投影，采用冻结声明公开页面，避免读取插件私有文件。官网候选空白的实际原因是两个官网插件均已卸载，通过正式安装启用恢复`website-default`，空态补充插件管理入口。原配置没有选择模板和网站名，正式配置接口初始化为`website-default`及“云服务平台”，未虚构公司和联系信息。

- [x] 增加当前模板公开页面发现、头部和底部导航增改删及三级树校验，后台及官网真实消费；单元测试覆盖伪造、循环、危险外链和切换过滤。
- [x] 新增框架无关`entry`与正式`public_assets`入口协议、原生`website-html`完整示例，宿主冻结发布标识生成URL，路径逃逸及编码路径拒绝。
- [x] 官网配置直接提供开发文档；候选空态说明安装启用步骤，隐藏重复的旧导航文本配置入口。
- [x] 完成`TC002`真实导航增改删、原生模板发布切换、实时接口搜索/展开/新窗口/下载与资源失败路径验收、截图审查及最终类型/规范检查。
- [ ] 实测模板非当前状态的停用再启用闭环：当前正式影响检查仍被宿主冻结授权快照拒绝，保留阻断，不绕过宿主。

阶段证据：`website-navigation-e2e.log`的导航、刷新、伪页拒绝、公共头尾投影及文档入口已通过（7.8秒）；`113000-website-navigation-dialog.png`与`113100-website-development-guide.png`已多模态检查，中文表单正常，文档抽屉宽度已按检查结果扩大。最终原生模板回归尚待统一加载最新插件owner校验代码，不将旧二进制拒绝新客户模板声明误报为业务完成。Go全包测试与插件`make lint`均通过，日志`website-backend-final.log`、`website-final-lint.log`位于`temp/20261016/`。

最新阶段：统一运行实例加载本轮owner校验后，`website-html v0.1.1`已通过正式`dynamic/package`上传及`upgrade`，均返回`code=0`。扩展`TC002`通过（10.1秒）：原生模板深链、脚本200、缺失资源404、会员登录注册域名、外链安全属性及切回内置模板。`113200-native-website.png`已多模态检查，真实商品和导航正确展示。开发文档现接入共享实时`OpenAPI`索引，接口`/api.json`实际200且含818个路径，后续验收搜索、schema展开、原文新窗和下载；不另造静态契约副本。

## 用户中心模板插件闭环（2026-10-16）
- [x] 新增框架无关 customer-template 契约、公共品牌与模板独立配置，默认应用仅复制到插件目录。
- [x] 用户中心配置管理页、实时 OpenAPI 搜索/Schema/下载及开发说明。
- [x] 独立6032网关支持版本化资源、SPA深链、压缩代理、认证和多Cookie、超时和503。
- [x] 客户应用消费统一会员导航与品牌；构建时自动生成语言资源，尊重成功空导航。
- [x] 最终动态包v0.1.4安装升级与客户真实业务页回归、截图审查、范围审查。停用/卸载的宿主owner unknown限制单独记录，不标为已通过。

末轮验收补充：新增真实后台导航增改删和“保存并启用”的状态断言，发现弹窗输入未提供可访问名称，已在插件表单配置补`aria-label`。该路径已实测通过，后续开发接口索引断言发现新`templateApi`翻译尚未进入运行发布资源；公开消息接口也确认缺键，不能以测试mock翻译替代修复。由并发主任务统一将`idc-portal`提高至`v0.1.13`并加载后再通过正式生命周期刷新。实时接口JSON约3MB，共享索引将data URI改为`Blob`对象URL且撤销旧URL，避免巨型DOM属性；接口详情按需展开，列表有界展示。


最终本轮`lina-review`限定官网导航、框架无关入口、原生模板和开发文档，不覆盖其他会话历史工作区变更。按根入口、宿主边界、前后端/API/插件/测试/i18n/文档规则核对，未发现本轮插件实现的新增阻塞问题。DI复用`registerRoutes`创建的共享`siteconfiguration.Service`及传入的`registrar.BusinessExtensions()`，不在请求路径建服务；无DAO/数据库变更，导航在一次有界候选接口中投影，无逐条接口或数据库查询。公开页面不缓存配置；新增翻译由`idc-portal v0.1.13`正式发布资源加载，未用测试mock替代生产修复。

最终证据：`TC002`全部通过（12.4秒），真实后台增改删并保存、伪造页面原配置保护、导航投影、实时API目标中文文案、展开schema、新窗口`/api.json`、下载JSON内容解析、原生模板深链及切回内置均覆盖。`113000-website-navigation-dialog.png`、`113100-website-development-guide.png`和`113200-native-website.png`均已多模态审查，中文表单、九百像素文档抽屉及真实官网布局正常，无翻译原始键。Go全包测试、portal与`website-html`的`make lint`、后台专项`vue-tsc`和官网`Nuxt typecheck`均通过，规范严格校验通过。最终正式配置恢复`website-default`、保留公共资料，清空本次验收导航及原生示例标题。

生命周期仍有实际阻断，不能标为完整可卸载交付：`123000-website-lifecycle-result.json`记录`GET /api/v1/plugins/website-html/business-impact`返回`code=50`、`PLUGIN_BUSINESS_IMPACT_CHECK_FAILED`、`ownerPluginId=unknown`；没有调用强制停用或卸载。只读定位为宿主`lifecycle_business.go`在进入插件处理器前发现发布快照有`BusinessExtensions`却没有`AuthorizedBusinessExtensions`，插件没有公开能力修复该冻结快照。本轮未修改宿主，未去掉扩展声明，未写系统表，未以重新启用掩盖耐久缺口。停用/卸载验收保持未完成，安装、启用、升级、选择和真实渲染已由正式API验证。

## 官网独立源码、真实页面与分组导航（2026-10-16）

- [x] 默认、文字、原生官网均自带独立Guest、依赖、打包工具和资源；仓库外安装构建并正式升级验收。
- [x] 默认与文字官网保留完整Nuxt SSR及静态业务应用，独立gateway根据当前模板路由；验证真正SSR和静态深链。
- [x] 当前模板页面候选展示真实路径，构建逐项核验实现；HTML示例改用真实products.html，移除虚构/products。
- [x] 导航无目标分组保存/刷新、桌面与窄屏点击和键盘展开不跳转、子项跳转与空分组隐藏；真实E2E、截图、类型和范围审查。

独立化中间验证：`website-outside-website-default.log`与`website-outside-website-minimal.log`为仓库外干净副本的`make install build`成功记录，最终发布v0.1.5；原生HTML模板v0.1.3同样脱离仓库构建。各Guest仅依赖Go标准库，正式上传及升级均code0，未引用宿主或portal的Go模块。目录内全部依赖链接仍位于插件自身目录，没有指向仓库的链接。`website-real-route-results.json`逐项记录默认与文字模板14条真实SSR路由全部200且输出h1；原生示例为实际`products.html`文件，不再声明不存在的`/products`。

`make i18n.check`因迁移的内联双语首页文案被扫描为硬编码，现已移入模板自有JSON词典；未修改扫描器或allowlist，最终`website-group-i18n.log`0违规。三个Guest的GOWORK=off测试、wasip1编译和`make lint`、portal相关Go测试与lint、后台专项vue-tsc、Nuxt完整SSR/静态构建均通过。最终group E2E见下述完成记录，中间失败不作为功能通过依据。

最终验收：`TC002a`通过，继续覆盖真实后台CRUD、危险页面拒绝、OpenAPI原文/下载及原生多页切回；`TC002b`通过（11.6秒），覆盖分组保存刷新、真实路径候选、桌面和390px头尾展开/收起、Enter/Escape、点击不跳转、商品子项跳转、仓外完整静态应用深链、HTML实际products.html和缺失文件404。测试在SSR脚本完成加载后操作，避免初始HTML尚未水合的时序误判。中间失败均为定位器/错误main结构断言或过早操作，最终业务断言完整通过，没有放宽预期行为。所有测试通过正式接口写入并在finally恢复原配置。

`website-group-1440.png`、`website-group-390.png`、`website-static-independent.png`和`website-group-editor.png`已多模态审查：分组及实际路径中文正确，默认、窄屏与静态商品应用无原始翻译键、重叠或错误消息。额外修复footer原有样式强制展开问题，使底部导航分组可真实收起。最终运行6680插件网关、6681/6682各插件独立SSR服务；临时静态验收网关6684测试后已停止。当前展示仍为website-default，保留原公共资料，验收分组已清理。

本轮`lina-review`限定上述独立官网源码/构建/网关、分组导航及实际页面清单，未发现新增阻塞问题。Guest自行实现公开v1 protobuf/JSON契约，仅允许绑定插件、owner、point、版本、摘要和上下文的describe调用；没有hostcall、秘密或私有源码依赖。packager仅读取本插件源码与静态文件，构建期核对真实页面，不修改宿主。导航验证复用原有共享服务，无新服务图、数据库或逐行远程查询；所有条目最多100且树深度最多3。静态模式与SSR部署边界已明确，未把单WASM静态应用宣称为SSR。历史宿主冻结授权影响检查阻断仍独立保留，不把安装/升级成功等同停用/卸载已通过。


## 用户中心完全独立交付（2026-10-16）
- [x] 将默认客户应用所需29个公开Vben包与17个客户业务源码文件闭包带许可复制到插件；去除外部源码、宿主node_modules和仓库构建工具依赖。
- [x] 插件自带锁文件、标准库Go protobuf guest、自有跨平台Node打包器与语言资源嵌入；构建loader使用可跟踪源码文件，不依赖被忽略的dist文件。
- [x] 整目录复制到 `/tmp/lina-independent-customer-20261016/customer-default`，完成独立安装、构建、Go测试、打包；外部符号链接数量为0。
- [x] 使用仓库外产物customer-default v0.2.0完成正式上传、升级预览、升级；运行网关读取该版本资源。
- [x] 最新vue-tsc、Go测试、定向make lint、全仓i18n.check、OpenSpec strict及gateway测试通过；TC003真实客户浏览器回归1 passed32.9s。

构建证据：`temp/20261016/customer-outside-build.log`（最初完整构建13.85s）、`customer-outside-loader-build.log`（最终可跟踪loader构建13.28s）、`customer-outside-package-final.log`（189资源，14,928,866字节）、`customer-outside-go.log`、`customer-standalone-upgrade.json`。运行与审查证据：`customer-standalone-e2e.log`、`customer-standalone-typecheck.log`、`customer-standalone-go.log`、`customer-standalone-lint.log`、`customer-standalone-i18n.log`、`customer-standalone-gateway.log`、`customer-standalone-openspec.log`及`customer-standalone-review.md`。原web-customer目录由主代理依据主题74精确备份后删除；本子任务不扩大宿主修改范围。

## 独立运行后的旧应用清理（2026-10-16）

- [x] 仓库外构建、正式安装和真实客户回归通过后，备份并精确删除旧`apps/lina-vben/apps/web-customer`。
- [x] 对比3489项宿主基线，除获授权删除的427项外，其他宿主文件零变化；旧16032无监听，6030/6031/6032/6680正常。

证据：`temp/20261016/retired-web-customer-before-delete.tar.gz`、`retired-web-customer-deletion.json`、`independent-host-check.json`。当前新版官网根路径、公开品牌接口及客户登录页均HTTP200。旧官网源码不再作为当前构建或运行入口，定位索引已同步。


## 旧官网工作区清理（2026-10-16）

- [x] 备份并移除已被独立插件替代的`apps/lina-websites`，修正当前开发文档、忽略规则和AI定位入口，验证现有启动计划及实际官网入口。

宿主旧`linactl`仍有历史官网路径引用，因禁止修改根`hack/`而保留；当前根`env.setup`、`db.init`、`build`、`dev`已经通过插件安装工具提供，不使用旧官网构建入口。历史阶段验收文件不重写为当前证据。

本轮按`lina-review`审查删除的98个旧文件及19个配套文档/规则文件，未发现当前插件与根启动入口依赖旧工作区。旧目录完整备份在忽略的`temp/20261016/retired-lina-websites/`，不进入提交。安装工具Go回归、`make dev plan=1`、修改文档链接检查、官网首页/商品/帮助三条真实HTTP检查及严格规范校验通过。未改宿主、接口、收费、权限、缓存、DI或现有页面实现；不重复运行与删除无关的业务测试。

## 废弃品牌修订链清理（2026-10-16）

- [x] 删除已不被当前官网配置和公开品牌投影消费的旧品牌修订管理接口、文件存储服务及专用翻译；保留公开品牌投影、路径安全校验和当前官网管理权限。
- [x] 更新插件中英文职责说明，完成插件后端全包测试、lint及全仓语言键检查。

旧修订写入不会影响当前官网，故删除该误导入口。现行`site/configuration`与`customer/configuration`及外链菜单能力保留。证据为`temp/20261016/cleanup-portal-lint.log`与`cleanup-i18n.log`；公开品牌投影回归与插件装配测试通过。

本轮同时清理10个插件的26个无运行消费者的历史`scaffold.json`，实际SQL保留；渠道测试改为核对真正嵌入的SQL。删除7份过时源码快照及5组与现行条款完全重复的归档，保留有独有契约或未完成工作的规范。83份现行规范仅补齐用途说明，全部Requirement和Scenario保持不变；138份现行规范严格校验通过。

本轮按`lina-review`复核清理范围，未发现阻塞问题。插件包编译、渠道lint及SQL回归通过；证据为`temp/20261016/141000-scaffold-package-tests.json`、`141000-scaffold-channel-lint.log`。宿主三处禁止修改目录相对本轮HEAD零差异；当前官网管理权限仍由`site/configuration`消费，已保留。未对仍有效的外链菜单、会员产品导航、独立模板依赖或历史资金释放代码作无依据删除。
