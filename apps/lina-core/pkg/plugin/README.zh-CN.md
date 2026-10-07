# LinaPro 插件公开契约

`apps/lina-core/pkg/plugin`承载`lina-core`稳定的插件侧契约。源码插件、动态插件 guest、动态插件构建器以及宿主集成代码都应通过这里的公开边界接入。宿主侧编排和持久化实现位于`apps/lina-core/internal/service/plugin`。

本包拥有插件内核、源码插件贡献 API、动态插件传输、core-owned 能力目录、通用 capability descriptor 接收、owner-aware 动态路由、授权、审计、缓存失效和生命周期治理。由业务插件拥有的非核心领域契约应位于该 owner 插件的`backend/cap/<domain>cap`目录。

## 公开组件

| 组件 | 功能职责 |
| --- | --- |
| `capability` | 定义统一的`capability.Services`目录以及用户、文件、i18n、任务、租户、组织、存储、缓存、锁、manifest、route、plugins 和业务上下文等 core-owned 能力子包契约。源码插件从`pluginhost`注册器和回调输入中接收该目录；动态插件只通过桥接传输访问已发布的能力子集。动态插件 i18n 资源由宿主管理，不发布`i18n`host service。 |
| `pluginhost` | 定义源码插件的声明期契约、运行期 service 访问、生命周期回调、hook 注册、HTTP 路由贡献、定时任务贡献、菜单过滤、权限过滤、托管静态资源常量和通用 capability descriptor 接收入口。它不得成为 plugin-owned 能力的领域专属 provider facade owner。 |
| `pluginbridge` | 提供动态插件 guest SDK、动态插件声明、core-owned host-service client 和 owner-aware host-call envelope。guest 代码在发现或构建阶段使用`pluginbridge.Declarations`，在运行阶段使用`pluginbridge.Services`。plugin-owned 领域 guest SDK 位于 owner 插件的`backend/cap/<domain>cap/bridge`或等价公开包。 |

## 领域能力

`capability.Services`是 core-owned 领域能力运行期目录。源码插件通过`pluginhost`注册器和回调输入返回的`capability.Services`消费这些入口；动态插件声明已经显式发布为动态`hostServices`的匹配入口，再通过`pluginbridge.Services`调用。每个 core-owned 领域只暴露一个插件可见`Service`；方法级契约承载风险、授权、数据权限、上下文、性能和缓存治理。领域方法依赖标准业务`ctx`获取当前用户、租户、权限和数据权限信息；动态`hostServices`授权保留在 dispatcher 内部校验。`I18n()`保留为源码插件运行期能力；动态插件 i18n 资源由宿主发现、合并、缓存和分发。插件生命周期归属`Plugins().Lifecycle()`，插件启用状态查询归属`Plugins().State()`；租户插件治理和租户过滤上下文归属`Tenant()`。宿主内部 scope helper 不通过普通插件可见入口暴露。

plugin-owned 非核心能力不归属`capability.Services`。owner 插件在`apps/lina-plugins/<plugin-id>/backend/cap/<domain>cap`下发布普通消费契约、provider `SPI`、动态 guest SDK、descriptor helper 和版本策略。`lina-core`接收 owner descriptor，建立`owner + service + version + method`索引，校验依赖和授权快照，分发 owner-aware 动态调用，记录审计 envelope，并在生命周期变化后失效运行期缓存。`AI`是第一组 plugin-owned 能力族，归属`apps/lina-plugins/ai-core/backend/cap/aicap`。

### 统一`Service`与动态插件

插件可见领域不再提供独立管理目录。源码插件通过`pluginhost`输入返回的`capability.Services`目录上的`Services.<Domain>()`或窄化注入的`*cap.Service`执行读取、写入、执行和治理动作。插件生命周期和启用状态分别通过`Services.Plugins().Lifecycle()`和`Services.Plugins().State()`访问；租户插件治理和租户过滤上下文分别通过`Services.Tenant().Plugins()`和`Services.Tenant().Filter()`访问。`capability.Services`不再提供独立租户服务视图，也不暴露顶层`PluginLifecycle()`、`PluginState()`、`TenantPluginGovernance()`、`TenantFilter()`或`TenantTableFilter()`方法。同进程源码插件和宿主 adapter 需要约束 GoFrame 查询时，使用`tenantspi.ApplyPluginTableFilter(ctx, Services.Tenant().Filter(), model, qualifier)`或从注入的`tenantcap.Service.Filter()`派生过滤上下文；普通`tenantcap.Service`和动态插件只暴露可序列化租户过滤上下文。动态插件只能在治理方法已经注册、声明、授权并由 dispatcher 分发时，按普通领域`hostServices`调用这些插件和租户治理方法。

动态插件只能使用已经注册到动态`host service`目录、由插件清单声明、经宿主授权并由`WASM host-service`分发器处理的具体方法。如果确实需要让动态插件使用某个受治理动作，应在统一领域中发布窄化、版本化的动态方法，而不是引入平行管理服务。

| 领域能力 | 职责边界 | 运行期与校验路径 |
| --- | --- | --- |
| `APIDoc` | 解析本地化 API 文档中的路由文本和标题 operation key。 | 通过能力目录或`apidoc`host call 提供；校验关注路由和 operation key 载荷。 |
| `Auth` | 通过`Token()`、`Authz()`和`ExternalLogin()`子能力处理租户 token 选择或切换、代用户 token、权限投影与外部身份登录。**安装授权后源码与动态插件同权同信**：宿主盖章调用方 pluginID；源码 ownership 用`ProvideExternalIdentity`，动态用`hostServices`下`auth`的`resources.ref`（provider ID）；方法 wire 为`external_login.login_by_verified_identity`。token/session 铸造仍在宿主。SPA handoff 由`extlogin-core`闭环。 | 运行期检查启用、方法授权与 provider ownership。外部身份引擎为`extlogin-core`；未装引擎时登录 fail-closed。 |
| `AI` | 由`ai-core`以 plugin-owned 方式拥有；聚合文本、图片、向量、音频、视觉、文档、安全审核和视频等类型化 AI 子能力，并提供方法级状态投影。 | 源码插件 import `ai-core/backend/cap/aicap`；动态插件声明`owner: ai-core`、`service: ai`和`version: v1`。`lina-core`只治理 descriptor 发现、依赖检查、授权快照、owner-aware 路由、审计和缓存失效。 |
| `Users` | 提供用户基础读取、有界列表、可见性、受治理写入、状态与凭证，以及`CreateFromExternal`（从外部身份最小权限建号）。动态`users`含`users.create_from_external`等已发布方法。 | 宿主保持可见性边界。`CreateFromExternal`无操作员、最小权限；需在`hostServices`中声明授权后可用。 |
| `BizCtx` | 投影当前业务请求上下文。 | 作为只读运行期上下文桥接当前用户、租户、语言和请求元数据。 |
| `Dict` | 解析字典值标签、列出有界字典候选，并校验类型化值可见性。 | 宿主校验保持在可见字典类型和值范围内。 |
| `Files` | 提供宿主文件中心投影、有界搜索、可见性确认、内容读取、受治理上传，以及从插件存储创建`sys_file`记录。 | 宿主校验避免插件探测或使用可见边界之外的文件 ID。上传会创建宿主文件中心元数据，插件私有对象仍由`Storage()`持有。 |
| `HostConfig` | 按宿主优先级链读取宿主配置值，并通过`SysConfig()`提供受治理的`sys_config`投影和写入。`SetValue(ctx, key, value, options)`写单键；`BatchSetValue(ctx, items, options)`在一次事务与一次 runtime-config revision 中写多键（多字段 settings 保存应优先批量）。`options.SystemManageable` 首次插入且为 nil 时默认 false/插件闭环。 | 动态`get`和单 key `sys_config`方法声明必须列出`resources.keys`。该能力独立于插件作用域业务配置。仅在插件入口维护的业务配置 MUST 传`options.SystemManageable: gconv.PtrBool(false)`。 |
| `I18n` | 为源码插件读取`locale`并翻译消息。 | 源码插件通过`pluginhost`输入中的`capability.Services`接收该能力；动态插件不接收`i18n`host service，因为其 i18n 资源由宿主管理。 |
| `Jobs` | 读取定时任务元数据、搜索有界任务候选、校验任务可见性，并执行受治理的运行期任务动作。 | 声明期任务契约通过`pluginbridge.Declarations.Jobs().Register(...)`提交；运行期服务不暴露`Register`。 |
| `Notifications` | 列表和读取类型化通知消息投影、按业务来源批量读取消息、校验可见性、删除消息、更新已读状态并发送受治理通知。 | actor 作用域的读取、删除和已读状态调用不需要资源声明；`messages.send`需要`resources[].ref`边界。 |
| `Plugins` | 暴露当前插件投影、插件注册表投影、租户插件分页、插件启用状态、provider 启用状态、插件作用域配置和受治理插件生命周期编排。 | 运行期检查插件可见性、插件启用/provider 状态、插件作用域配置来源隔离、生命周期前置条件，以及已发布治理方法的动态`hostServices`授权。 |
| `Route` | 暴露当前执行的动态路由元数据。 | 用于运行期路由分发，不暴露宿主 router 内部实现。 |
| `Sessions` | 读取当前在线会话投影、搜索在线会话、批量读取在线会话投影、校验会话可见性，并批量读取用户在线状态。 | 宿主校验保持会话和用户可见性在调用方边界内。 |
| `Storage` | 提供插件私有对象存储操作，用于插件自有附件、业务二进制对象、导入导出临时文件和卸载清理，并支持显式批量元数据、游标列表和批量删除。 | 源码插件通过`pluginhost`输入中的`capability.Services`获得插件作用域`Storage()`；动态声明使用`service: storage`和`resources.paths`；写入不会创建`sys_file`记录，也不会暴露 provider key 或本地路径。 |
| `Cache` | 提供插件作用域缓存读取、写入、删除、多键读取/写入/删除、自增和过期操作。 | 动态声明使用`resources[].ref`；运行期分发校验 namespace、key、value 大小和正 TTL 载荷。缓存仍是非权威插件运行期数据。 |
| `Lock` | 提供插件可见的分布式锁获取、续租和释放。 | 动态声明使用`resources[].ref`；运行期分发校验锁资源和租约载荷。 |
| `Manifest` | 读取插件作用域 manifest 或 artifact 资源，包括有界多资源读取和路径列表。 | 动态声明使用`resources.paths`；源码和动态路径都通过插件作用域资源视图解析。 |
| `Org` | 提供可选组织投影，例如用户组织档案、有界部门树、部门搜索、岗位选项、可见性校验、部门分配、部门名称和岗位 ID。 | provider 可用性显式暴露；组织 provider 缺失时 fallback service 返回安全中性值。 |
| `Tenant` | 提供可选租户上下文、租户信息、租户批量/搜索投影、可见性确认、成员校验、可访问租户列表、租户插件治理和插件自有表租户过滤上下文。 | provider 可用性显式暴露；租户生命周期写入和成员关系替换留在租户 owner 或宿主内部 SPI。同进程 Go 调用方持有 GoFrame model 时，可以把普通过滤上下文传给`tenantspi.ApplyPluginTableFilter(...)`。动态插件只接收可序列化过滤上下文，并必须通过`RecordStore`或 owner 侧过滤完成租户隔离。 |

### 插件分发治理

插件 manifest 和生命周期回调快照包含`distribution`，宿主会将其归一化为`managed`或`builtin`。省略时视为`managed`。`builtin`是仅适用于源码插件的治理模式，用于随宿主编译交付的项目组成能力：宿主会在启动时自动安装、启用和安全升级，普通插件管理写操作会被拒绝，插件管理列表也会在筛选、分页和总数计算前排除它们。可信宿主集成仍可通过只读详情查询检查这些系统必需能力。动态插件必须保持`managed`语义，不能自声明内建治理。

### 业务扩展点与前端表面

插件清单把四个维度分开保存：`type`只表示运行方式（`source`或`dynamic`），`distribution`只表示生命周期管理方式（`builtin`或`managed`），`business_extensions`表示业务扩展点，前端表面表示展示能力。第四个维度再分为独立治理的`admin_surfaces`和`customer_surfaces`。宿主不得根据插件名称猜测任何维度。

内置业务拥有方使用`role: owner`发布版本化`business_extensions`。托管提供方使用`role: provider`，明确写出拥有方插件 ID，并在`dependencies.plugins`中声明带版本范围的依赖。批量发现会先一次性建立全部拥有方索引，再只接受拥有方、扩展点、版本和方法集合都匹配的提供方；它处理已经加载的一整批清单，不会为每个插件单独查一次数据库。首批扩展点标识为`product-fulfillment`、`sms-channel`、`payment-method`和`website-template`；未来业务拥有方仍可发布新的安全 kebab-case 标识。

提供方扩展可以分别声明独立版本的`interface_configuration`、`product_configuration`和`customer_configuration`。声明了`interface_configuration`的提供方还必须把`interface_configuration_cardinality`声明为`multiple`或`singleton`：前者允许管理员创建多条有名称的接口实例，后者只允许保存一份全局设置。这个数量规则只约束设置记录，不限制插件自己拥有的业务数据和增删改查功能。首版字段类型固定为`single_line`、`multi_line`、`integer`、`decimal`、`boolean`、`single_select`、`multi_select`、`date`、`datetime`、`secret`、`tree_select`、`address`和`file`十三种。文本格式只允许宿主拥有的`phone`、`email`、`domain`和`url`校验。`secret`只能用于接口配置；商品和客户配置只允许`product-fulfillment`提供方声明。文件字段只能引用安全的宿主`upload_policy`，不能填写上传网址。

选择字段只能二选一：使用有界双语`static_candidates`，或者使用已经进入该扩展授权`methods`的版本化`dynamic_candidates`方法。条件只能在有界`and`/`or`树中使用`equal`、`not_equal`、`contains`、数值比较、`empty`和`not_empty`；结果只能是`show`、`hide`、`require`、`disable`、`filter_candidates`、`requires`和`excludes`。源码`YAML`和动态包`JSON`中不认识的嵌套键都会被拒绝，因此脚本、表达式、任意正则、上传地址和未声明动态方法不能藏进清单。

```yaml
type: dynamic
distribution: managed
dependencies:
  plugins:
    - id: commerce-core
      version: ">=0.1.0 <1.0.0"
business_extensions:
  - point: product-fulfillment
    version: v1
    role: provider
    owner: commerce-core
    methods: [fulfill, query, colors.list]
    interface_configuration_cardinality: multiple
    interface_configuration:
      version: v1
      fields:
        - key: api_secret
          type: secret
          label: { en-US: API secret, zh-CN: 接口密钥 }
          required: true
    product_configuration:
      version: v1
      fields:
        - key: model
          type: single_line
          label: { en-US: Model, zh-CN: 型号 }
    customer_configuration:
      version: v1
      fields:
        - key: color
          type: single_select
          label: { en-US: Color, zh-CN: 颜色 }
          dynamic_candidates:
            version: v1
            method: colors.list
customer_surfaces:
  - key: my-products
    version: v1
    title: 我的产品
    slot: customer.products
    route: my-products
    entry: customer/index.html
    bridge_capabilities: [products.query-own]
public_assets:
  - source: frontend
    mount: customer
```

发布版本快照会同时冻结插件申请和宿主接受的完整业务扩展声明，包括三类配置方案，以及彼此独立的后台/客户中心表面。动态 WASM 包必须嵌入与`plugin.yaml`完全一致的完整声明；运行分发方式、业务扩展点、嵌套配置或前端表面不一致时会被拒绝。发现证据返回隔离副本形式的安全配置声明，并用完整规范化声明计算摘要，所以字段、候选、条件、动作或方案版本发生变化都会形成不同的冻结证据。

业务扩展拥有方可以定义一个有界、不可执行的`provider_metadata`对象，不需要把自己的业务方案写进宿主。宿主只理解`type`、`version`和可转换为`JSON`的`values`，负责规范化、深拷贝、冻结、授权，并把完整对象计入声明摘要。源码业务拥有方在装配声明时，为精确扩展点和契约版本注册类型化校验器：

```go
if err := declarations.BusinessExtensions().RegisterProviderValidator(
    "example-extension",
    "v1",
    validateExampleProviderDeclarations,
); err != nil {
    return err
}
```

校验器收到的是该提供方发布版本完整规范化声明组的隔离副本，因此可以检查必填元数据、类型化字段、方法授权和整个发布版本内的唯一性，又不会改动宿主快照。源码提供方和动态提供方走同一条清单校验路径。缺少拥有方注册、契约版本不支持、拥有方重复注册或校验器拒绝时，安装和升级都会失败关闭。宿主不会猜测元数据的业务含义。

业务拥有方通过受治理的`plugins.business_extensions.list`发现可用于新绑定的候选。宿主只做一次有上限的注册表与活动发布版本权威读取，只返回插件身份、显示名称、发布/契约版本、允许方法及发布/声明摘要；任何候选快照无法确定时都会整体拒绝。允许方法为空时仍可选择和绑定提供方，只有业务拥有方以后调用某个具体方法时才校验该方法授权。`plugins.business_extensions.exact.get`按冻结证据核对一个保留的历史声明，`plugins.business_extensions.exact.batch_get`则按输入顺序用一次权威发布版本查询核对有上限的一组声明；两者都不代表插件当前还能运行。源码业务拥有方真正提交新绑定时，仍必须调用`HTTPRegistrar.BusinessExtensions().WithNewBusinessAdmission`：宿主持有并持续续租提供方生命周期锁后重新核对活动发布版本，并让这把锁覆盖拥有方的本地事务回调；停止新业务、停用、卸载和切换活动发布版本都与该回调串行。后台任务通过`JobsRegistrar.BusinessExtensions()`获得同样按拥有方绑定、缺失即拒绝的门面；绑定不存在时，任务注册器绝不会退回到无归属的通用服务。续租一旦失败，宿主会取消回调上下文并拒绝本次准入，让正确使用上下文的数据库事务回滚。拥有方不会拿到锁名，也不能读取插件注册表。

源码业务拥有方只能通过绑定自身身份的`BusinessExtensionRegistrar.InvokeProviderMethod`调用提供方具体方法。每次调用必须带齐冻结的发布与声明证据、一个已授权方法、明确的`new-business`或`existing-business`用途，以及版本化且有上限的`JSON`对象。需要解析接口运行状态的调用可以额外携带一个有界、高熵的`runtimeContext.contextId`；源码和动态提供方只能取得同一个不透明编号，接口实例、修订号、配置值、凭据、秘密和令牌仍留在调用方插件身份由宿主权威填写的宿主服务之后。不需要接口状态的调用省略`runtimeContext`。同一次提供方调用最多执行八个机密外联步骤，并共同受总时限和总字节数限制。步骤必须严格递增；重放、并发、跨调用使用、过期或超出预算都会在供应商处理器开始前被拒绝，提供方调用结束后拥有方立即撤销上下文。宿主在持有同一提供方生命周期锁时重新核对活动发布版本和运行状态：停止新业务后拒绝新业务调用，但已有冻结业务仍可继续精确调用；插件已停用、缺失、证据不符或方法未授权时一律失败关闭。源码提供方通过`Providers().ProvideBusinessExtensionMethods`显式注册一个方法分发器。动态提供方通过现有隔离`WASM`执行器接收同一信封，固定使用`BusinessExtensionBridgeRequest`请求类型；宿主不会发起公开`HTTP`回环，也不会提供原始令牌、身份快照、秘密句柄或私有清单状态。请求和响应都有严格的字节数、对象字段数、数组项数、深度、字符串、总复杂度和五秒执行上限。

后台当前发布`admin.catalog-products`、`admin.resource-operations`、`admin.orders-finance`、`admin.notification-marketing`、`admin.compliance-governance`、`admin.support-content`和`admin.extension-management`插槽。登录后客户中心发布`customer.products`、`customer.finance`、`customer.account`、`customer.security`、`customer.compliance`和`customer.support`插槽。后台表面必须声明非空宿主权限。宿主统一生成`/plugins/{plugin-id}/...`后台路由和`/account/plugins/{plugin-id}/...`客户路由，因此插件不能覆盖`/system`、`/auth`或其他宿主路由。动态表面入口必须是插件内相对路径并被`public_assets`覆盖，远程网址和路径越界会直接被拒绝。后台表面进入现有按员工权限过滤的菜单树；客户表面只在登录后返回，并且只能挂到已有客户业务分组。清单声明不会自动获得页面桥能力，页面桥授权、身份与数据范围校验和隔离页面加载仍是彼此独立的运行期门禁。

公开的`GET /api/v1/plugins/dynamic`发现接口使用可选认证：未携带`Authorization`时只返回不含客户页面的运行状态；携带访问令牌时必须经过与受保护接口相同的在线会话校验。有效客户及员工协助客户的会话保留目标客户、租户和真实员工上下文。无效身份直接拒绝，不得提升或静默信任。动态页面声明及其`public_assets`覆盖范围一起从同一个不可变的已生效版本快照投影；待升级包不能补齐缺失或无效的已生效声明。发现页面不等于获得页面桥能力或业务数据访问权。

### 隔离页面桥

托管动态页面运行在不含`allow-same-origin`的隔离 iframe 中，不接收访问令牌、宿主状态或自定义请求头。页面使用`linapro.plugin-page-bridge`版本`1.0`消息协议；每条消息都必须精确匹配当前 iframe、插件 ID、表面 key 和协议版本。`host.view.v1`是所有受治理表面的宿主基础能力，只提供`ready`、高度同步、统一错误、标题和插件页内路由同步，不需要写入`bridge_capabilities`，也不能访问业务数据。

业务调用必须同时满足三层绑定：表面的`bridge_capabilities`声明能力；活动动态路由或宿主捕获的源码拥有方 DTO 路由通过`pageBridgeCapability`和`pageBridgeHost`绑定同一能力；宿主后端再为当前登录会话、用户、租户、提供方发布版本、服务端确定的拥有方发布版本、后台或客户宿主、表面、表面版本、能力、HTTP 方法和实际路径签发两分钟有效的防篡改票据。页面请求不能选择`ownerPluginId`，只有宿主找到唯一准确路由后才会把它返回。签发前后台表面会同时重新检查表面权限和拥有方路由权限；客户表面继续拒绝超级管理员身份直接进入、租户模拟和跨商家上下文。动态插件自有客户路由保留本人数据范围要求；源码拥有方客户路由必须通过`RouteGroup.BindCustomerPageBridge`注册权威身份回调，使用原始已认证上下文返回与当前客户一致的宿主用户编号。缺失证明、校验失败或身份不符都会拒绝。无后台角色的真实客户及有效安全协助目标因此可以访问，但不会获得员工权限，校验结果也不会跨请求缓存。拥有方路由随后仍须执行普通后台权限与数据范围或当前客户对象所有权检查，包括签发后的身份撤销。浏览器只携带不透明票据。缺少任意一层、插件停用、票据过期或被修改、能力不一致、方法或路径变化、身份变化以及跨表面调用都会拒绝。插件页面不能取得票据内容、提供任意请求头、自选另一个拥有方命名空间，也不能用查询能力调用未绑定的写路由。

页面最多同时执行 8 个桥请求；在途重复`requestId`会拒绝，单次请求 30 秒后取消。切换插件、切换表面、重试或卸载页面时，宿主会取消旧请求并丢弃旧页面迟到的响应。插件必须发送`ready`后宿主才移除骨架屏，以免脚本或子资源仍在加载时出现白屏。

## 动态插件专属能力

`Runtime`、`Network`和`RecordStore`是`pluginbridge.Default()`或`pluginbridge.New()`返回的`pluginbridge.Services`目录上的动态插件专属入口。它们不属于`capability.Services`，因为源码插件已经运行在宿主进程内，可以使用宿主原生等价能力，或者使用源码插件数据访问接缝，而不是 guest host-service 包装。

| 能力 | 公开入口 | 边界原因 |
| --- | --- | --- |
| `Runtime` | `pluginbridge.Default().Runtime()`或`pluginbridge.New().Runtime()` | 动态插件需要通过`WASI host-service`客户端写日志、读写状态、读取时间、生成 UUID 和读取节点身份；源码插件直接使用宿主原生日志和运行期上下文。 |
| `Network` | `pluginbridge.Default().Network()`或`pluginbridge.New().Network()` | 动态插件需要经由 host-service 授权访问受治理的出站 HTTP；源码插件使用宿主原生 HTTP client 或注入的领域 service。 |
| `RecordStore` | `pluginbridge.Default().RecordStore()`或`pluginbridge.New().RecordStore()`，以及`pkg/plugin/pluginbridge/recordstore` | 动态插件需要 guest 侧 facade 封装 data host-service 协议和类型化查询计划；源码插件使用自有 DAO 或 provider 接缝。 |

新增能力只有在源码插件和动态插件共享同一个稳定 core-owned 领域契约时，才应进入`capability.Services`。仅服务动态插件的 host-service client 和 guest SDK 应留在`pluginbridge`下。plugin-owned 非核心能力必须从 owner 插件的`backend/cap/<domain>cap`边界发布公开契约和 guest SDK，不得在`lina-core`中新增领域包、codec 或 dispatcher 分支。

### `Storage()`与`Files()`边界

| 场景 | 使用能力 | 边界 |
| --- | --- | --- |
| 插件保存自有附件、生成的导出文件、业务二进制对象或导入临时文件。 | `Storage()` / 动态`service: storage` | 插件传入 logical object path。宿主在委托给当前 storage provider 前按插件 ID 和租户加作用域。对象不会进入宿主文件中心列表，也不会创建`sys_file`元数据。 |
| 插件在记录删除或卸载清理时删除、列出、读取元数据或清理自有对象。 | `Storage()` / 动态`service: storage` | 清理通过 logical prefix 执行`Delete`或有界`List`后删除。插件不得直接删除宿主上传根目录、provider 根目录或文件中心记录。 |
| 插件引用用户已经上传到宿主文件中心的文件。 | `Files()` / 动态`service: files` | 插件只获得`filecap.FileInfo`信息和宿主文件 ID 的可见性校验。响应不得暴露`DAO`、`DO`、`Entity`、provider object key 或本地绝对路径。 |
| 插件命令从请求中接收宿主文件 ID。 | `Files().EnsureVisible` / `files.visible.ensure` | 命令执行写入前必须先校验全部 ID。不存在和不可见使用相同拒绝语义，避免泄露资源存在性。 |
| 插件需要上传内容并登记到宿主文件中心。 | `Files().Upload` / `files.upload` | 宿主通过文件 owner 写入，让`sys_file`记录租户、上传人、场景、hash 和存储元数据。动态直传有大小上限；更大的动态 payload 应先使用`Storage().Put`。 |
| 插件已经把对象写入私有存储，并需要宿主文件中心记录。 | `Files().CreateFromStorage` / `files.create_from_storage` | 宿主从插件作用域`Storage()`对象复制到文件中心存储。动态插件还必须为源路径声明`storage.get`。该操作不会移动或删除源对象，也不会暴露 provider key 或本地路径。 |
| 浏览器/客户端需要绕过宿主中转上传或下载云对象。 | 文件中心`direct-upload/*`与`direct-download` HTTP API；插件侧`Storage().CreateDirectPut` / `ConfirmDirectPut` / `CreateDirectGet` | 宿主签发中立`DirectAccess`（`presigned_url` / `form_post` / `temporary_credentials` / `proxy`），密钥永不下发；object key 由宿主按租户/插件作用域分配；local 或不支持时 mode=`proxy` 回退中转。文件中心仍须`complete`后才写`sys_file`。 |
| 大对象需要云 Multipart 或宿主分片中转。 | 文件中心`direct-upload/init` 自动策略 + `direct-upload/part-url` / `upload/chunked/*`；云 Provider 可选实现`storagecap.MultipartUploadProvider` | init 按大小阈值与能力探测返回中立`strategy.channel`（`direct`/`proxy`）与`strategy.encoding`（`single`/`multipart`）。直传分片使用 part 级短时访问；中转分片由宿主拼装或经 Provider UploadPart。源码插件可在 Provider 支持时调用 Service Multipart 方法；动态插件一期仍用`Put`（guest 传输分片），不暴露公共 Multipart API。 |

`Storage()`provider 选择不依赖主配置项。宿主在恰好一个 storage provider 插件可服务时使用该插件，没有可服务 provider 时回退到内置本地 provider，多个 provider 插件同时可服务时拒绝 storage 调用。官方云后端（`storage-cos`、`storage-oss`、`storage-aws`、`storage-s3`等）通过`storagecap.Provide`注册，并在宿主稳定目录**系统设置**（`menu_key=setting`）下提供凭证配置页。文件中心对象内容写入/读取/删除与插件`Storage()`共用同一套 provider 选择规则（0→local，1→云，≥2→冲突）；列表与检索仍基于`sys_file`。客户端直连同样遵守该选择规则，且永不把永久 AK/SK 暴露给前端。

## 插件配置来源

源码插件通过`Services.Plugins().Config()`读取当前插件业务配置，动态插件通过`plugins.config.get`读取当前插件业务配置。这些入口都限定在插件作用域内，不暴露任意宿主配置 key，也不会读取兄弟插件配置。

配置来源优先级按配置段生效：

| 优先级 | 来源 | 运行期行为 |
| --- | --- | --- |
| 1 | 宿主主静态配置段`plugin.<plugin-id>` | 整个配置段作为该插件的有效配置来源。段内缺失的子 key 返回不存在或调用方默认值，不再回退到文件来源。 |
| 2 | 宿主配置根下的生产文件`plugins/<plugin-id>/config.yaml` | 仅在`plugin.<plugin-id>`不存在时使用。 |
| 3 | 开发期文件`apps/lina-plugins/<plugin-id>/manifest/config/config.yaml` | 仅在宿主静态配置和生产文件来源都不存在时使用。 |
| 4 | 动态 artifact 默认配置`manifest/config/config.yaml` | 作为当前动态插件执行上下文的最终回退来源。 |

`manifest/config/config.example.yaml`只作为模板，不会作为运行期默认值加载。`HostConfig()`仍然是独立的宿主配置能力；非 root 宿主 key 按当前`sys_config`快照、宿主静态配置、宿主默认值、缺失`nil`的顺序读取。动态`hostconfig.get`调用仍必须在`hostServices`中声明`resources.keys`授权。源码插件调用`HostConfig().Get(ctx, key, defaultValue)`时必须显式传入默认值；传入`nil`表示在宿主优先级链读取后保持缺失 key 返回`nil`的语义。

## 普通消费契约、Provider SPI 与 Guest SDK

插件侧包使用三类独立边界，确保每类调用方只导入自己可以安全依赖的契约：

| 边界 | 包形态 | 目标调用方 | 不得包含 |
| --- | --- | --- | --- |
| Core-owned 普通消费契约 | `pkg/plugin/capability/<domain>cap` | 通过`pluginhost`输入中的`capability.Services`调用的源码插件、通过生成或 bridge-backed client 调用的动态插件，以及宿主 adapter | GoFrame HTTP request 对象、provider factory 注册入口、宿主私有实现状态，或 GoFrame 数据库 builder |
| Core-owned 源码插件 Provider SPI | `pkg/plugin/capability/<domain>cap/<domain>spi` | 实现宿主领域 provider 的源码插件，以及宿主能力装配代码 | 动态插件 guest SDK import 或 WASM host-service wire 契约 |
| Core-owned 动态插件 Guest SDK | `pkg/plugin/pluginbridge`及其动态插件专属子包 | WASM guest 代码和动态插件构建器 | Provider SPI import 或源码插件注册 API |
| Plugin-owned 普通消费契约 | `apps/lina-plugins/<plugin-id>/backend/cap/<domain>cap` | 依赖 owner 插件的源码插件、owner adapter，以及通过显式注入使用 owner 契约的宿主模块 | owner 插件`backend/internal`、DAO、DO、Entity、controller、provider 密钥、私有缓存或宿主 dispatcher |
| Plugin-owned Provider SPI 与 Guest SDK | `apps/lina-plugins/<plugin-id>/backend/cap/<domain>cap/spi`和`apps/lina-plugins/<plugin-id>/backend/cap/<domain>cap/bridge` | owner 插件 provider 注册 helper、源码 provider adapter，以及调用 owner-aware host service 的动态 guest | core `pluginhost`领域专属 facade、core `pluginbridge`领域专属 codec、owner 内部 service import 或绕过授权 |

Core-owned provider factory 声明归属`pluginhost.Declarations.Providers()`。源码 provider 插件通过`ProvideTenant`和`ProvideOrg`等领域方法声明 factory。宿主启动装配负责持有 provider manager 实例，并把共享 manager 注入宿主能力 service；普通`capability`包不保留包级 provider 注册表。plugin-owned provider 使用 owner helper 将 typed factory 包装为通用 capability descriptor。`pluginhost`只接收这些 descriptor，不得为非核心领域新增`ProvideAIText`等`Provide<Domain>`facade。

## 宿主领域实现

`apps/lina-core/internal/service/plugin`是宿主侧插件领域组件。根包提供统一门面，覆盖插件发现、管理列表、安装、启用、停用、卸载、运行期升级、源码插件升级、运行期路由分发、前端资源托管、依赖检查和能力装配。

硬依赖`dependencies.plugins`检查按两条生命周期轴执行：

- **安装轴**（安装/卸载/升级版本契约）：正向检查要求依赖已安装且版本满足；反向检查保护所有**已安装**下游依赖方。
- **运行轴**（启用/禁用）：启用正向检查要求依赖已安装、**已启用**且版本满足；禁用反向仅由**已启用**下游依赖方阻断。已安装但已禁用的下游不会阻断禁用，但仍会阻断卸载。

宿主不会自动安装、启用、禁用或卸载依赖链。

## 声明期能力与运行期能力

### 声明期能力

声明期能力是插件的静态声明和注册输出。宿主在业务执行前使用这些内容构建治理状态。

源码插件通过`pluginhost.Declarations`表达声明期契约，包括`Assets()`、`Lifecycle()`、`Hooks()`、`HTTP()`、`Jobs()`和`Access()`。

### 路由注册：`group.Bind` 绑定控制器对象

源码插件与宿主 HTTP 路由注册时，`group.Bind` **默认传入控制器对象**（例如`group.Bind(controller.NewV1(svc))` 或 `group.Bind(ctrl)`），由 GoFrame 自动发现对象上带路由元数据的方法。

同一中间件组内应避免无必要地罗列`ctrl.Method1, ctrl.Method2, ...`。

当**同一控制器的方法必须落在不同中间件组**（例如公开登录 vs 鉴权后管理接口）时，宿主沿用拆分注册：公开组与受保护组分别`Bind`对应方法，而不是再套一层 Public/Protected 控制器封装。

### 生命周期前置 Hook（目标 vs 全局）

源码插件`Lifecycle()`支持两类前置回调：

| 类型 | 注册示例 | 输入 | 语义 |
|------|----------|------|------|
| 目标插件 | `RegisterBeforeInstallHandler` / `RegisterBeforeEnableHandler` 等 | `SourcePluginLifecycleInput` | 仅当**本插件**被装/启/禁/卸时调用，可否决自身操作 |
| 全局前置 | `RegisterGlobalBeforeInstallHandler` / `RegisterGlobalBeforeEnableHandler` / `RegisterGlobalBeforeDisableHandler` / `RegisterGlobalBeforeUninstallHandler` | `SourcePluginGlobalLifecycleInput`（含`TargetPluginID`） | 当**其他插件**被装/启/禁/卸时调用，由 owner 做跨插件治理（例如同 kind 单例） |

编排规则：

- 安装、启用、禁用、卸载在写状态前会聚合**目标 Before\*** 与已显式注册的**全局 GlobalBefore\***。
- 启用路径提供`BeforeEnable`/`AfterEnable`；`AfterEnable` 为 best-effort，不回滚已成功启用。
- 全局参与者列表只包含注册了对应全局 Hook 的插件，未注册者不参与、无空调用。
- 宿主不识别业务领域（如邮件 kind）；领域冲突逻辑由 owner 插件在全局 Hook 内实现。
- 不得通过“向所有插件广播自管 `BeforeInstall`”模拟全局拦截，以免误触发自身安装语义。

动态插件通过`plugin.yaml`、WASM 自定义 section、`pluginbridge.Declarations.Routes().Group(...)`、`pluginbridge.Declarations.Jobs().Register(...)`以及嵌入的`protocol`契约表达声明期契约，例如路由、任务、生命周期处理器、后端资源、前端资源、SQL、i18n 资源和`hostServices`。

### 停止新业务、停用与卸载

这三个动作是不同阶段，不能互相代替：

- `accepting`表示插件可作为新接口、新商品版本、购买和履约任务的候选；`stopped`只阻止这些新业务，插件仍保持启用，已有客户菜单和服务操作继续可用。
- 停用和卸载前必须先进入`stopped`，再由宿主向注册了`RegisterBusinessImpactHandler`的可信源码业务 owner 批量查询在售商品版本、在途任务、活动服务和待执行历史动作。
- 宿主只按当前生效 release 已冻结并授权的 provider 扩展声明确定必须参与检查的 owner，不读取待安装产物来削弱保护；缺少任一必需 owner 的处理器时按检查失败处理。
- owner 只返回四类有界汇总数量，不返回对象详情；任一查询失败、分类越界或数量不合法时，宿主都会拒绝停用和卸载。
- 停止/恢复新业务、停用和卸载共用按插件划分的跨实例生命周期锁，停售检查与最终状态写入之间不能被另一个请求重新开放新业务。
- `force`只能在宿主允许时跳过插件自己的生命周期回调否决，不能跳过停售、业务占用、依赖或历史证据保护门禁。
- 卸载默认保留数据。声明了业务扩展点的插件不能通过卸载删除历史商品、订单、配置快照、接口引用、履约证据和审计记录；动态产物丢失时也不能用“清理数据”绕过该保护。

稳定的内建业务 owner 注册影响检查的入口如下：

```go
plugin.Lifecycle().RegisterBusinessImpactHandler(handler)
```

新商品绑定、购买和履约写入路径必须调用宿主的`CanAcceptNewBusiness`权威门禁；只在页面隐藏候选不构成安全控制。

### 运行期能力

运行期能力是在插件业务逻辑执行时可用的 service。

源码插件通过`pluginhost`输入返回的`capability.Services`目录访问运行期能力。插件生命周期和状态通过`Services.Plugins().Lifecycle()`和`Services.Plugins().State()`暴露。租户插件治理和租户过滤上下文通过`Services.Tenant().Plugins()`和`Services.Tenant().Filter()`暴露。同进程表过滤通过显式`tenantspi`helper 调用完成，不作为单独的源码插件服务目录方法或租户服务镜像暴露。

动态插件通过`pluginbridge.Services`访问已发布的运行期能力。调用会通过`pluginbridge/protocol`编码，经由`WASI host call`传输，由派生的`HostCapabilities`和已确认的`HostServices`授权，再由`apps/lina-core/internal/service/plugin/internal/wasm`分发执行。动态插件不接收顶层兼容快捷入口或`I18n()`，但可以在`hostServices`授权后调用`Plugins()`和`Tenant()`下已经注册的治理方法。

每次 guest 执行时，宿主会构建请求级`HostServices`授权快照，每次 host call 仍按该快照校验`service`、`method`和资源身份。owner-aware plugin-owned 调用还会先校验`owner`、`version`、调用插件的`dependencies.plugins`声明、owner 插件启用状态、descriptor 注册状态和 method 授权，再分发到 owner handler。

依赖供应商凭据的操作不得使用普通`network.request`桥。源码业务拥有方只为精确的提供方插件、发布版本、声明、接口实例、业务扩展方法和用途签发一个短时运行上下文。动态提供方只能取得这个不透明编号，也只能把它交给清单已明确声明的拥有方宿主服务。只有当前业务扩展调用携带完全相同的上下文编号、拥有方和提供方插件时，宿主才允许把该编号作为运行资源；上下文、拥有方、插件或方法不一致，来自普通路由，或者企图拿它冒充静态资源声明时，都会在进入拥有方处理器前失败关闭。拥有方服务在可信宿主进程内解析接口地址和凭据，按自身有界契约执行，并只返回安全投影。例如`idc-core/supplier-interface-runtime@v1`可以完成一次有上限的同源登录会话，密码和访问令牌都不会返回动态插件。旧`WithConfidentialOutboundContext`仍可作为资源操作拥有方的专用实现，并继续执行一次性使用、过期、重放拒绝和响应字段白名单。运行上下文在外围调用结束时撤销，不写数据库也不进入缓存。

## 动态插件 Host Service 声明

最小结构：

```yaml
hostServices:
  - service: runtime
    methods:
      - log.write
```

资源作用域结构：

```yaml
hostServices:
  - service: storage
    methods: [get, put, put.init, put.chunk, put.commit, put.abort]
    resources:
      paths:
        - reports/
  - service: data
    methods: [list, get]
    resources:
      tables:
        - plugin_acme_demo_report
  - service: hostconfig
    methods: [get]
    resources:
      keys:
        - i18n.default
  - service: network
    methods: [request]
    resources:
      - url: https://*.example.com/api
  - service: notifications
    methods: [messages.send]
    resources:
      - ref: inbox
        attributes:
          channel: inbox
```

owner-aware plugin-owned 结构：

```yaml
dependencies:
  plugins:
    - id: ai-core
      version: ">=0.1.0"

hostServices:
  - service: ai
    owner: ai-core
    version: v1
    methods:
      - text.generate
      - text.method_status.get
```

未声明`owner`的行表示 core-owned service。plugin-owned service 必须声明`owner`和`version`；宿主会合并 core-owned 静态 catalog 和 owner descriptor 投影，用于校验、授权、升级预览和运行期分发。下面的生成 catalog 来自宿主 catalog 实现，并且只列出 core-owned services。

## 可声明 Host Services

<!-- BEGIN generated:host-services -->
| Service | 资源声明 | 派生能力 | Methods |
| --- | --- | --- | --- |
| `runtime` | 无 | `host:runtime` | `log.write`<br/>`state.get`<br/>`state.get_many`<br/>`state.set`<br/>`state.set_many`<br/>`state.delete`<br/>`state.delete_many`<br/>`info.now`<br/>`info.uuid`<br/>`info.node` |
| `storage` | `resources.paths` | `host:storage` | `put`<br/>`put.init`<br/>`put.chunk`<br/>`put.commit`<br/>`put.abort`<br/>`get`<br/>`delete`<br/>`delete.batch`<br/>`list`<br/>`list.cursor`<br/>`stat`<br/>`stat.batch` |
| `network` | `resources[].url` | `host:http:request` | `request` |
| `confidentialoutbound` | 无 | `host:confidential-outbound` | `execute` |
| `data` | `resources.tables` | `host:data:read`<br/>`host:data:mutate` | `list`<br/>`get`<br/>`batch_get`<br/>`create`<br/>`update`<br/>`delete`<br/>`transaction` |
| `cache` | `resources[].ref` | `host:cache` | `get`<br/>`get_many`<br/>`set`<br/>`set_many`<br/>`delete`<br/>`delete_many`<br/>`incr`<br/>`expire` |
| `lock` | `resources[].ref` | `host:lock` | `acquire`<br/>`renew`<br/>`release` |
| `hostconfig` | `resources.keys` | `host:hostconfig` | `get`<br/>`sys_config.get`<br/>`sys_config.value.set`<br/>`sys_config.reset` |
| `manifest` | `resources.paths` | `host:manifest` | `get`<br/>`get_many`<br/>`list` |
| `apidoc` | 无 | `host:apidoc` | `route_text.resolve`<br/>`route_texts.resolve`<br/>`route_title_operation_keys.find` |
| `auth` | 无 | `host:auth:token`<br/>`host:auth:external_login`<br/>`host:auth:authz` | `token.tenant.select`<br/>`token.tenant.switch`<br/>`token.impersonation_token.issue`<br/>`token.impersonation_token.revoke`<br/>`external_login.login_by_verified_identity`<br/>`authz.permissions.batch_get`<br/>`authz.permissions.batch_has`<br/>`authz.permissions.has`<br/>`authz.users.platform_admin.check`<br/>`authz.role_permissions.replace` |
| `users` | 无 | `host:users` | `users.current.get`<br/>`users.batch_get`<br/>`users.resolve.batch`<br/>`users.list`<br/>`users.visible.ensure`<br/>`users.create`<br/>`users.create_from_external`<br/>`users.update`<br/>`users.delete`<br/>`users.status.set`<br/>`users.password.reset`<br/>`users.assignment.roles.replace` |
| `bizctx` | 无 | `host:bizctx` | `current.get` |
| `dict` | 无 | `host:dict` | `dict.refresh`<br/>`dict.type.get`<br/>`dict.type.batch_get`<br/>`dict.type.list`<br/>`dict.type.visible.ensure`<br/>`dict.type.keys.visible.ensure`<br/>`dict.type.create`<br/>`dict.type.update`<br/>`dict.type.delete`<br/>`dict.value.get`<br/>`dict.value.batch_get`<br/>`dict.value.labels.resolve`<br/>`dict.value.list`<br/>`dict.value.visible.ensure`<br/>`dict.value.values.visible.ensure`<br/>`dict.value.create`<br/>`dict.value.update`<br/>`dict.value.delete`<br/>`dict.value.by_type.delete` |
| `files` | 无 | `host:files` | `files.batch_get`<br/>`files.list`<br/>`files.visible.ensure`<br/>`files.upload`<br/>`files.create_from_storage`<br/>`files.metadata.update`<br/>`files.delete`<br/>`files.delete_many` |
| `jobs` | 无 | `host:jobs` | `jobs.batch_get`<br/>`jobs.list`<br/>`jobs.visible.ensure`<br/>`jobs.create`<br/>`jobs.update`<br/>`jobs.delete`<br/>`jobs.run`<br/>`jobs.status.set`<br/>`jobs.register` |
| `notifications` | 除`messages.send`使用`resources[].ref`外无需资源声明 | `host:notifications` | `messages.batch_get`<br/>`messages.list`<br/>`messages.by_source.batch_get`<br/>`messages.visible.ensure`<br/>`messages.send`<br/>`messages.delete`<br/>`messages.by_source.delete`<br/>`messages.mark_read`<br/>`messages.mark_unread` |
| `plugins` | 无 | `host:plugins` | `plugins.current.get`<br/>`plugins.batch_get`<br/>`plugins.registry.list`<br/>`plugins.tenant.list`<br/>`plugins.business_extensions.list`<br/>`plugins.business_extensions.exact.get`<br/>`plugins.business_extensions.exact.batch_get`<br/>`config.get`<br/>`plugins.state.enabled.check`<br/>`plugins.state.provider_enabled.check`<br/>`plugins.state.enabled_authoritative.check`<br/>`plugins.lifecycle.tenant_plugin_disable.ensure`<br/>`plugins.lifecycle.tenant_plugin_disabled.notify`<br/>`plugins.lifecycle.tenant_delete.ensure`<br/>`plugins.lifecycle.tenant_deleted.notify` |
| `route` | 无 | `host:route` | `metadata.get` |
| `sessions` | 无 | `host:sessions` | `sessions.current.get`<br/>`sessions.list`<br/>`sessions.batch_get`<br/>`sessions.users.online.batch_get`<br/>`sessions.visible.ensure`<br/>`sessions.revoke`<br/>`sessions.revoke_many` |
| `org` | 无 | `host:org` | `capability.available`<br/>`capability.status`<br/>`org.assignment.user_profiles.batch_get`<br/>`org.department.tree.list`<br/>`org.department.batch_get`<br/>`org.department.list`<br/>`org.post.batch_get`<br/>`org.post.options.list`<br/>`org.department.visible.ensure_many`<br/>`org.post.visible.ensure_many`<br/>`org.department.create`<br/>`org.department.update`<br/>`org.department.delete`<br/>`org.post.create`<br/>`org.post.update`<br/>`org.post.delete`<br/>`org.assignment.by_user.replace`<br/>`org.assignment.by_user.cleanup` |
| `tenant` | 无 | `host:tenant` | `capability.available`<br/>`capability.status`<br/>`tenant.context.current`<br/>`tenant.context.info`<br/>`tenant.context.platform_bypass`<br/>`tenant.directory.batch_get`<br/>`tenant.directory.list`<br/>`tenant.membership.validate`<br/>`tenant.membership.list_by_user`<br/>`tenant.directory.visible.ensure_many`<br/>`tenant.plugins.enabled.set`<br/>`tenant.plugins.defaults.provision`<br/>`tenant.filter.context` |
<!-- END generated:host-services -->

## 维护说明

当插件公开契约或动态 `host service` 描述符发生变化时，需要同步更新本目录下的`README.md`和`README.zh-CN.md`。

### Host Service 载荷与常量

- 新增 core-owned host service 方法必须使用统一 JSON envelope（`HostServiceJSONRequest` / `HostServiceJSONResponse` 或空载荷）。不得为新增方法引入 dedicated binary codec。
- 存量 dedicated codec 方法集合已冻结；catalog 治理测试会拒绝名单外的 dedicated 扩张。
- `service` / `method` 线值常量只维护在 `pluginbridge/protocol/hostservices/wire_constants.go`。catalog 必须引用这些常量（禁止重复写 wire 字符串字面量）；catalog 测试会校验。
- WASM 分发侧新增 JSON 方法应优先复用 `decodeCapabilityJSONRequest` 与 `capabilityJSONResponse` helper。
