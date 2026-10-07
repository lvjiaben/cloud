# 第二十八阶段：客户插件发现身份修复

## 已确认范围

用户最后确认“一起修复”，并补充确认“一起修正，保留原业务权限和隔离”。本次允许修改宿主插件发现接口的身份识别，以及客户页面桥误用后台角色的判断和对应测试。不扩大插件权限，不改变隔离属性、供应商调用或客户所有权校验；页面桥仍须先由原业务拥有方证明客户身份再签票。

## 根因与方案

客户请求已经通过正式`requestClient`携带当前访问令牌；普通客户与安全协助使用相同路径。该接口却与登录、公开语言包一起绑定到无认证的公开路由组，`Ctx`只建立空业务上下文，控制器因此始终把请求视为匿名并隐藏`customerSurfaces`。后台菜单使用另一条受保护的发现路径，故后台可打开而客户入口消失。

增加宿主中间件`OptionalAuth`，仅在没有`Authorization`时直接继续；请求提供身份时复用已有`Auth`，由启动期共享的认证服务校验签名、有效期和在线会话，并完整注入用户、租户和经办员工信息。无效或过期身份按现有认证链拒绝，不能降级为已认证。将动态插件发现单独绑定到此中间件，其他公开和管理路由不变。不接受查询参数、前端用户编号或自造上下文作为身份来源，也不新增租户切换行为。

该接口继续只输出运行状态和已发布页面声明，不输出客户业务记录；客户对象范围仍由原有页面桥和业务拥有方在调用时校验。无需给客户授予后台`plugin:query`权限，也无需改变公开响应字段。

真实协助入口验证又发现同一发现链的第二处遗漏：`runtime_dynamic.go`从已生效版本快照构建发现用清单时没有复制`PublicAssets`，动态页面构建器按原安全规则检查公开资源声明时拒绝该清单。因此可识别身份的源码渠道页面出现了，动态 SSL 页面仍缺失。补丁只从同一个已生效版本快照复制公开资源声明，保持安装、启用、版本有效性、入口覆盖和隔离校验；禁止退回扫描到的待升级候选清单或绕过公开资源检查。新增回归须同时证明有效快照正常发现，以及无效、缺失或仅候选声明不能发布页面。

## 影响判断

- 权威身份仍为同一`auth.Service`和在线会话存储，没有新增服务实例、缓存或跨节点状态。每次发现至多增加一次既有认证调用，不随插件数量增加查询。
- 不修改`API DTO`、数据库、索引、`SQL`、供应商配置、插件安装数据或业务枚举。
- `i18n`不新增运行时文案，失败沿用现有认证响应；不新增外部网络调用。
- `pkg/plugin`文档当前说明客户表面只在认证后返回，需要补充公开发现接口的可选身份规则，并同步中英文镜像。
- 开发工具无生产流程改动；验证继续使用仓库`Go`、`make`和真实`Chrome`入口，跨平台行为不变。

## 验证计划

1. 中间件单测覆盖匿名、普通客户、有效安全协助、错误格式、无效或撤销令牌，断言认证调用次数及目标用户、真实员工、租户上下文。
2. 路由绑定测试确认只有发现接口使用可选认证，公开登录不受影响，插件管理仍要求认证及权限。
3. 覆盖修改包、宿主启动绑定包、宿主静态检查；运行现有动态发现控制器投影测试。
4. 正常构建并重启后，真实客户或安全协助会话通过可见菜单打开`SSL`页面；匿名发现不含客户页面，无效身份不得取得客户页面；资源、页面桥及灰表头检查不使用资产或业务响应替身。

实现与真实验收完成前保持任务未完成。浏览器业务替身测试只能证明独立页面行为，不能替代真实身份发现验收。

## 当前实现与审查记录

本次宿主修改限定为九个文件：

- `apps/lina-core/internal/service/middleware/middleware.go`
- `apps/lina-core/internal/service/middleware/middleware_impl.go`
- `apps/lina-core/internal/service/middleware/middleware_impl_test.go`
- `apps/lina-core/internal/cmd/internal/httpstartup/httpstartup_routes.go`
- `apps/lina-core/internal/cmd/internal/httpstartup/httpstartup_routes_test.go`
- `apps/lina-core/internal/service/plugin/internal/runtime/runtime_dynamic.go`
- `apps/lina-core/internal/service/plugin/internal/runtime/runtime_dynamic_test.go`
- `apps/lina-core/pkg/plugin/README.md`
- `apps/lina-core/pkg/plugin/README.zh-CN.md`

按`lina-review`核查了已跟踪与未跟踪状态；宿主文件内原有其他反馈改动保留，本次仅添加可选身份链、对应测试与双语说明。读取并应用`AGENTS.md`、`user-preferences`以及`architecture`、`data-permission`、`plugin`、`api-contract`、`backend-go`、`testing`、`cache-consistency`、`frontend-ui`、`i18n`、`dev-tooling`、`documentation`、`openspec`规则。适配插件和SSL插件根目录均无额外`AGENTS.md`。未修改身份服务的权威来源、业务所有权或存储模型；无新增缓存、SQL、接口字段、收费、供应商请求或用户文案。

本机验证已通过：

- 在`apps/lina-core`执行`go test ./internal/service/middleware ./internal/controller/plugin ./internal/cmd/internal/httpstartup -count=1`，三包通过，耗时分别为`1.767s`、`0.619s`、`4.152s`。
- 在`apps/lina-core`执行`go test ./internal/service/auth -count=1`，通过，耗时`3.053s`。
- 根目录`make.cmd lint.go dir=apps/lina-core plugins=0`通过，`golangci-lint`和`staticcheck`零问题。
- 根目录`make.cmd lint.go dir=apps/lina-plugins/yunsu-ssl plugins=1`通过，两个静态检查零问题；插件`Go`后端测试通过，耗时`1.591s`。
- SSL前端`pnpm run build`通过；`pnpm test`七项通过，含原有重试按钮恢复失败表格的单测。
- `node scripts/validate-e2e.mjs`通过，当前`276`个测试文件；`openspec validate add-idc-server-operations-platform --strict`通过。
- 宿主修改文件`git diff --check`通过；仅有仓库已有的换行符提示。

当前静态审查未发现新的阻塞问题，但真实客户菜单验收仍待主代理正常重启后运行`yunsu-ssl/TC003`。该测试使用明确指定的本机演示客户编号，通过正式员工协助入口进入，不模拟接口或资源，并在结束时撤销本次协助会话；不创建客户或证书、不执行供应商操作。共享表格运行时仍保持不含`allow-same-origin`的隔离属性，不能通过放宽权限换取测试通过。

只读列举测试时发现SSL测试目录缺少ES模块声明，导致宿主共享fixture被错误按CommonJS转换并报`exports is not defined`。按现有插件测试目录做法，仅新增SSL自有`hack/tests/package.json`声明`type=module`，不改宿主测试框架或依赖；`pnpm exec playwright test yunsu-ssl --list`已通过，准确发现三文件九用例。该文件不进入生产前端或WASM归档，无须再次升级插件。

## 第二层快照修复验证

真实`TC003`首次运行证明`OptionalAuth`已生效：有效协助身份能获得源码渠道页面，匿名无客户页面、错误身份被拒绝；但动态SSL页面仍缺失，因此不能将客户真实验收标为完成。新增`TestListRuntimeStatesPublishesOnlyEffectiveSurfaceAssets`使用自有插件测试数据，先安装带双端页面和公开资源声明的`v1.0.0`，再放入采用不同资源前缀的`v1.1.0`候选包。未修复时双端页面都为空，准确复现问题；仅补回已生效快照的`PublicAssets`后，运行状态组测试通过，耗时`2.536s`。

该测试同时确认两个页面始终指向已生效版本及其资源前缀；故意移除有效快照的资源声明时，即使候选包合法也不得补救或发布页面。原损坏快照、缺失有效版本测试继续通过。使用既有请求级快照和克隆函数，没有新增查询或缓存；查询捕获继续确认无逐插件版本点查。测试仅清理自身唯一插件编号的治理记录和临时资源，不修改本机已安装SSL数据。

本轮SSL八个业务替身浏览器场景已通过，真实资源采用实际构建产物。重试场景将故障注入安排在首个页面桥列表响应后，避免首次上下文加载的自动恢复抢先消除错误；断言原有重试按钮恢复真实`Vben`表格、页面不刷新且无业务动作重放。其定向复跑通过，耗时`10.7s`。真实客户第九项仍须在第二层补丁正常重启后复跑。

补丁后的`catalog`、`middleware`、插件控制器和宿主路由启动包分别通过，耗时`2.308s`、`1.865s`、`0.857s`、`2.847s`；宿主`make.cmd lint.go dir=apps/lina-core plugins=0`再次通过，两类静态检查零问题。扩大运行`runtime`全包时，既有`TestInstallRepairsEmptyReleaseArchive`在最后卸载步骤被反向依赖守卫拒绝，单独重跑同样失败；该测试不调用本次修复的发现投影，不扩展修改其治理行为。所有发现投影测试均通过，但不能将`runtime`全包报告为通过。该独立验证缺口已告知主代理。

上述命令均在`apps/lina-core`执行：

- `go test ./internal/service/plugin/internal/runtime -run TestListRuntimeStates -count=1`：通过。
- `go test ./internal/service/plugin/internal/runtime ./internal/service/plugin/internal/catalog ./internal/service/middleware ./internal/controller/plugin ./internal/cmd/internal/httpstartup -count=1`：除`runtime`外四包通过，`runtime`仅既有归档修复测试失败。
- `go test ./internal/service/plugin/internal/runtime -run '^TestInstallRepairsEmptyReleaseArchive$' -count=1`：同一卸载错误独立重现。测试末尾调用`StopNewBusiness`、`Uninstall`，后者经过`ensureNoReverseDependencies`后拒绝；未经过`ListRuntimeStates`及本次补字段的清单投影路径。错误显示依赖阻断，但所列依赖者为空；其更深生命周期原因未在本次范围内确定，不能推断为本次发现修复失败。

已实际查看浅色首屏`temp/20260904/051208-yunsu-ssl-customer-first-load.png`及重试恢复`temp/20260904/051528-yunsu-ssl-runtime-retry.png`，灰色表头、白色内容和操作列正常。暗色窄屏截图抓在刷新加载过程中，后续测试已增加等待`ssl-loading`隐藏，再进行最终可读性截图检查；不能用加载中截图代替暗色验收。

## 第三层客户页面桥身份修复方案

第二层修复后，真实发现已成功，但`POST /api/v1/plugins/page-bridge/grant`返回既有拒绝码`61`，业务列表接口尚未发出。后台日志定位到`customer page bridge surface audience is denied`。只读查询确认演示客户的权威身份为有效客户，且没有后台员工角色；宿主角色服务因此给出`ScopeNone`，旧页面桥错误要求`ScopeSelf`。用户已确认一起修正，并保留原业务权限和不透明隔离。

签发前仍必须证明是真实客户，不能只因存在源码拥有方路由而绕过身份检查。最小接线如下：

1. 宿主`pluginhost.RouteGroup`新增`BindCustomerPageBridge(authorize func(context.Context) (int, error), handlers...)`。这是页面桥治理的通用注册接缝，只接受已声明客户宿主及明确能力的标准`DTO`路由；缺失回调或不匹配元数据在注册期返回错误。普通`Bind`不变，不让回调成为可访问路由。
2. 绑定快照保存启动期传入的回调。签发只对服务端唯一匹配、可用、版本有效的源码拥有方路由调用一次回调；返回的当前客户用户编号必须与经过认证的签发主体一致，错误、零值、不同主体及缺失回调都拒绝。不会把客户号或业务内部结构传入宿主。
3. `idc-core`只在既有客户服务控制器绑定处提供回调，复用启动期共享的`serviceActionAccess.Customer`，后者继续调用原`CurrentCustomer`权威校验。无后台角色的真实客户与有效协助目标可通过，没有客户身份的普通员工仍被拒绝。控制器、客户访问中间件和实际数据归属校验保持不变。
4. 动态插件自有路由保持原有`ScopeSelf`判断，不因新增接缝放宽。后台权限、商家上下文、禁止切租户、令牌绑定、有效版本、能力、方法、路径、来源与隔离校验不变，不读取候选升级清单兜底。

该接缝由宿主拥有，业务身份由原插件拥有。没有新服务图、业务导入、缓存、SQL、权限授予或用户文案；回调引用既有共享实例，每次签发仅一次有界身份检查，无按返回行数增长的查询。运行期错误继续在既有页面桥边界包装，不改变错误契约。开发工具与跨平台脚本无变更。

计划验证：注册期缺失与错误元数据拒绝；源拥有方无角色真客户及有效协助成功、普通员工和不匹配主体拒绝；无回调、停用、错误能力或路径拒绝；动态自有路由旧边界回归；插件启动绑定、客户权威检查及宿主相关包编译与静态检查。真实`TC003`在主代理统一重启后复验，期间不改验收成功断言。

### 第三层实现与验证记录

本层修改为八个宿主文件、三个业务拥有方文件和本记录。此前九个宿主文件中的双语说明继续更新，其余新增接线范围为：

- `apps/lina-core/pkg/plugin/pluginhost/pluginhost_route_registrar.go`
- `apps/lina-core/pkg/plugin/pluginhost/pluginhost_source_route_binding.go`
- `apps/lina-core/internal/service/plugin/internal/integration/integration_extensions_http_registrar.go`
- `apps/lina-core/internal/service/plugin/internal/integration/integration_extensions_http_registrar_test.go`
- `apps/lina-core/internal/service/plugin/internal/runtime/runtime_route_page_bridge.go`
- `apps/lina-core/internal/service/plugin/internal/runtime/runtime_route_page_bridge_test.go`
- `apps/lina-plugins/idc-core/backend/plugin.go`
- `apps/lina-plugins/idc-core/backend/internal/service/customer/customer_access_test.go`
- `apps/lina-plugins/idc-core/backend/internal/service/serviceaction/service_action_access_test.go`

`idc-core`根目录无附加`AGENTS.md`。已按`lina-review`收集当前已跟踪和未跟踪状态，并逐项核查上述增量。`DI`链为启动期`serviceActionAccess`共享实例，经`bindRouteControllers`参数传入，绑定回调调用原`Customer`方法，最终使用原`identityprojection.CurrentCustomer`；不存在请求期新建服务、伪造`ScopeSelf`或浏览器传入身份。回调仅附着到本次明确绑定的标准客户路由，不泄漏给普通`Bind`，也不注册成独立路由。

新增与扩展测试证明：无角色真实客户与有效协助目标签发成功；原上下文完整传递；每次签发重新检查身份；身份错误、零值、不同用户、无回调、错误路径或能力、商家上下文、超级管理员均拒绝。动态自有路由对`ScopeNone`仍拒绝。客户实际路由中间件在先前身份已通过后，遇到撤销、解绑或改成员工身份时重新读取权威主体并拒绝业务处理；服务访问测试同时确认`CurrentCustomer`的失败不复用旧结果。

当前验证结果：

- 在`apps/lina-core`执行`go test ./internal/service/plugin/internal/runtime ./internal/service/plugin/internal/integration -run 'TestSourceRouteRegistrar|TestIssuePageBridgeGrant|TestAuthorizePageBridge|TestVerifyPageBridge|TestListRuntimeStates' -count=1`：分别`1.510s`、`1.469s`通过。
- 在`apps/lina-plugins/idc-core`使用`GOWORK=off`执行`go test ./backend/internal/service/serviceaction ./backend/internal/service/customer ./backend -count=1`：分别`0.660s`、`1.992s`、`2.660s`通过。首次未设置`GOWORK`被工作区排除插件的检查阻止，随后使用插件既有本地`replace`正确执行，未修改工作区或依赖。
- 宿主`internal/controller/plugin`、`internal/cmd/internal/httpstartup`整包分别`0.622s`、`2.302s`通过。`pkg/plugin/pluginhost`首次扩大运行时，既有`1ms`生命周期超时测试出现调度时序失败；不改该测试或生产逻辑，整包独立重跑`0.507s`通过。
- 扩大运行`integration`整包时，既有`TestDynamicPluginInstallAndUninstallManageMenusFromManifest`和`TestDynamicPluginRoutePermissionsMaterializeHiddenMenus`在卸载步骤遇到空依赖者拒绝；用`go test ./internal/service/plugin/internal/integration -run '^(TestDynamicPluginInstallAndUninstallManageMenusFromManifest|TestDynamicPluginRoutePermissionsMaterializeHiddenMenus)$' -count=1`独立重现。此路径不签发页面桥，和前述卸载守卫验证缺口一并上报，未在本次范围修改生命周期，不声称宿主全包通过。
- 根目录`make.cmd lint.go dir=apps/lina-core plugins=0`最终`4.084s`通过；`make.cmd lint.go dir=apps/lina-plugins/idc-core plugins=1`为`16.243s`，均为两类静态检查零问题。
- `openspec validate add-idc-server-operations-platform --strict`及本次已跟踪文件`git diff --check`通过。没有新增运行时文案、翻译键或接口文档字段；不修改`i18n`资源。

静态审查未发现本层接线阻塞问题，但完整交付仍等待主代理统一重启后的真实`TC003`。本层未自行重启或构建生产资源，也未修改本机客户、角色、证书、插件配置或供应商状态。

### 正常重启后的真实入口验证

主代理完成当前后台与客户中心生产构建后，通过根目录`.\make.cmd dev plugins=1 skip_wasm=1`正常重建并启动三个服务。未卸载、重装插件，未重置数据库；SSL仍为已经安装启用的`0.3.4`。随后独立执行真实`TC003/TC-3a`通过，耗时`11.7s`，结果位于`temp/20260904/155000-ssl-real-final`。

测试通过正式员工协助进入演示客户中心，从真实发现菜单打开SSL页并调用原证书业务接口；无接口或静态资源替身。匿名发现不返回客户表面，无效身份与主动结束后的身份被拒绝。客户没有新增后台角色，页面没有`allow-same-origin`，没有创建证书、修改供应商配置或调用真实签发服务。本次协助会话由测试结束流程撤销。综合表格回归的最终结果另在阶段总审查记录。
