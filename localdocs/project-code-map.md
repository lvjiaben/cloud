# 项目结构与业务链路地图

更新日期：`2026-10-10`。用途：作为后续逐项修正业务逻辑和页面的源码定位底稿。

## 阅读范围与结论边界

已完成全仓非忽略源码的结构扫描、全部`25`个插件清单元数据与服务目录梳理，并阅读宿主启动装配、前端入口与插件页面注册、系统配置聚合页面，以及交易结账、财务结清、事件投递、通用履约和资源履约交接的关键实现。其他领域已建立目录与接口定位索引，尚未逐函数审查。

这份记录描述当前代码，不确认现有业务设计合理，也不表示全部功能测试通过。后续以用户明确的问题和最新确认的业务结果为准，展开对应源码、完整规范、数据与真实页面验证。不能把注释、文件名、已有测试或已勾选任务单独作为业务正确性的证明。

结构扫描结果：`apps/`下有`8068`个源码文件，其中`1598`个测试文件、`263`个`SQL`文件、`637`个`Vue`文件；`hack/`下有`285`个源码文件。扫描包括生成的数据模型，不包括依赖和临时构建输出。已提取`50`个源码插件页面入口和`898`处接口路径字面量，不等于实际启用的路由数量。

- [模块、服务、页面与大文件清单](project-source-index.md)：便于人工查阅。
- [机器可检索源码索引](project-source-inventory.json)：包含插件类型、服务、公开能力、控制器分组、页面、接口声明位置、客户中心页面和官网页面；记录了扫描时的`Git HEAD`，并包含当时工作区改动。
- [业务偏好索引](../.agents/memory/preferences/index.md)：只用来定位已确认条款，不在本文件复制业务规范。

## 整体源码布局

| 层级 | 目录 | 当前职责与定位重点 |
| --- | --- | --- |
| 宿主后端 | `apps/lina-core/` | `GoFrame`服务；认证、角色权限、菜单、租户接缝、配置、存储、字典、国际化、调度、插件运行时 |
| 管理后台 | `apps/lina-vben/apps/web-antd/` | `Vue/Vben`管理壳、路由权限、请求适配、插件页面注册；大量业务页面不在此目录 |
| 客户中心 | `apps/lina-vben/apps/web-customer/` | 独立`Vue/Vben`应用；客户商品、购物车、订单、财务、服务器、认证、合同、工单等 |
| 共享前端 | `apps/lina-vben/packages/` | 通用组件、布局、偏好、请求、表单、表格、国际化和插件沙箱 |
| 业务插件 | `apps/lina-plugins/` | `25`个插件；业务后端、前端页面、清单、数据库迁移、翻译及测试按插件归属维护 |
| 公开官网 | `apps/lina-websites/sites/default/` | 独立`Nuxt`官网，公开商品、购买配置、内容、品牌外观与客户中心跳转 |
| 官网公共包 | `apps/lina-websites/packages/` | `site-contracts`、`site-sdk`、`site-packager`；协议、接口消费、官网包交付 |
| 开发与测试 | `hack/tools/linactl/`、`hack/tests/` | 开发启停、构建、生成、检查；宿主与插件共用的浏览器测试基础设施 |
| 业务记录 | `openspec/`、`.agents/memory/preferences/` | 规范、活跃任务及已确认业务条款；与实现交叉核对 |
| 本地资料 | `localdocs/` | 本地图谱及已有外部参考笔记；参考资料不自动成为业务需求 |

本轮此前已验证的本地地址：后端`6030`、管理后台`6031`、客户中心`6032`、官网`6680`。这是本地开发配置，不是部署端口契约。官网`nuxt.config.ts`的后端默认值仍为`http://localhost:9999`，当前启动需通过`NUXT_API_ORIGIN`对齐真实后端。

## 插件责任地图

| 插件或插件组 | 当前代码责任 | 反馈时重点区分 |
| --- | --- | --- |
| `idc-core` | 客户、目录、商品、报价、订单、账本、授信、营销、服务权益、通用履约、工单、内容、运营与商家治理 | 是最大业务聚合点；同一目录中存在多个领域，不能视为单一服务 |
| `idc-resource` | 资源接口、容量、资源池、服务器投影、上游任务与服务器操作接缝 | 与商品销售、客户服务权益、通用商品自动化分开追踪 |
| `idc-payment` | 支付渠道、支付与退款尝试、回调、查询、对账和资金异常 | 与`idc-core/billing`的账本、结清事实分开追踪 |
| `idc-compliance` | 个人／企业认证、合同、税务开票、隐私与合规证据 | 另有`central-gateway`独立模块，排查供应商调用时不能遗漏 |
| `idc-notify` | 通知事件、模板、渠道、投递、重试、偏好和证据 | 与站内内容、工单业务和宿主消息能力分开 |
| `idc-channel` | 下游令牌、商品发布、渠道请求、兼容适配与回调投递 | 与销售代理／佣金服务`idc-core/saleschannel`、`commission`不是同一个领域 |
| `idc-portal` | 品牌修订、客户外部菜单、客户入口桥接等 | 不拥有独立官网和客户中心应用源码；品牌配置存储在文件运行时目录 |
| `idc-ai` | 运营洞察、检索、诊断、预测和草稿辅助入口 | 具体可写能力、权限与配置需逐接口核对 |
| `idc-zjmfcloud`、`idc-dcim`、`idc-v10`、`idc-zjmf-finance` | 托管动态服务器连接器 | 协议与认证互不通用；看各自`backend`处理器和资源拥有者契约 |
| `idc-baota`、`yunsu-ssl` | 宝塔网站、证书等动态自动化连接器 | 属于各自产品履约，不能根据服务器逻辑推定行为 |
| `idc-payment-alipay`、`idc-payment-wechatpay`、`idc-payment-epay`、`idc-payment-unionpay` | 动态支付适配器 | 渠道协议实现与统一支付事实拥有者分开 |
| `idc-zjmfmigrate` | 动态迁移检查与迁移业务 | 看独立迁移状态、映射和受治理的数据访问 |
| `tenant-core`、`org-core`、`staff-access` | 租户、部门岗位、业务员工资格与范围 | 登录用户、客户、员工资格和租户归属是不同对象 |
| `shared-services`、`content-notice`、`vben-adapter` | 人机校验和消息、公告、前端适配契约 | 属于复用能力，不应复制进每个业务插件 |

上述责任由清单、目录和关键源码推导；各插件具体方法是否完整实现、是否启用、是否正确授权，需在对应反馈中验证。

## 启动、页面与接口怎样连接

### 后端入口

`apps/lina-core/main.go`加载引导配置并运行命令树；`internal/cmd/cmd_http.go`及`internal/cmd/internal/httpstartup/`负责服务启动。`httpstartup_runtime.go`装配认证、缓存、租户、插件运行时、文件和调度等共享服务；`httpstartup_routes.go`绑定宿主控制器。

源码插件通过自身`plugin_embed.go`嵌入资源，`backend/plugin.go`注册插件、控制器、能力和任务。`idc-core/backend/plugin.go`同时包含大量依赖装配和任务注册，排查服务“没有启动”“没有注入”“接口存在但后台任务不执行”时必须检查此处。

动态插件以`main.go`导出`WASI`调用入口。例如`idc-zjmfcloud/main.go`创建`pluginbridge.GuestRuntime`，连接`ConfidentialOutbound`和资源能力客户端。动态插件入口、宿主桥、业务能力拥有者与真实上游请求是不同层。

### 管理后台与系统配置

管理后台`src/main.ts → bootstrap.ts → router`初始化应用。源码插件页面由`virtual:lina-plugin-pages`提供模块，`src/plugins/page-registry.ts`读取页面的`pluginPageMeta`并建立路由映射，`src/views/system/plugin/dynamic-page.vue`负责呈现。

系统配置聚合入口是`idc-core/frontend/pages/system-configuration.vue`，目前分业务与品牌、自动化、合规、通知、支付、迁移。它通过`components/configuration-surface.vue`在抽屉中装配源码页面，或根据菜单元数据呈现`PluginSandboxView`。所以问题可能出现在聚合入口、抽屉上下文、目标页面、沙箱绑定或接口权限中的任一层。

“菜单出现”“页面注册成功”“插件启用”“按钮可操作”“接口授权通过”不能相互替代。清单、运行时菜单和页面静态注册要分别检查。

### 客户中心与公开官网

客户中心的静态页面映射主要在`src/router/routes/modules/account.ts`；动态产品目录还涉及`src/router/product-directory-routes.ts`和`src/plugins/customer-surface-routes.ts`。客户业务请求适配集中在`src/api/idc/`。

官网采用`Nuxt`文件路由，页面在`sites/default/app/pages/`，主要实现位于`app/components/public/`。`PublicSiteShell.vue`读取公开品牌接口并呈现站点名、导航、配色、标识和页脚；官网商品与购买配置另有`catalog-client.ts`及`purchase-configuration-client.ts`。后台能保存品牌不代表官网所有字段已正确消费，需同时验证公开接口响应包裹、`SSR`访问、代理、缓存与页面字段使用。

宿主请求默认走`/api/v1`；前端`pluginApiPath()`构造`/x/<plugin-id>/api/v1/...`。接口索引中的路径多为相对路由，不能直接当完整请求地址。

## 主要业务链路

### 商品到服务开通

```mermaid
flowchart LR
  A[后台商品与配置] --> B[商品版本与购买配置]
  B --> C[官网或客户中心选购]
  C --> D[购物车与报价]
  D --> E[本地事务结账]
  E --> F[订单／应收账单／库存预留]
  F --> G[财务结清事实]
  G --> H[事务事件与异步消费]
  H --> I[通用履约与前置条件]
  I --> J[人工处理或插件自动化]
  J --> K[资源拥有者／供应商任务]
  K --> L[服务结果投影]
```

- 商品与配置：`idc-core/backend/internal/service/catalog`、`catalogdraft`、`configurationcontract`、`productautomation`；前端从`catalog-management.vue`、`catalog-client.ts`及相关编辑组件进入。
- 报价与结账：`commerce_quote*.go`、`commerce_checkout.go`。已读结账入口从当前客户身份派生归属，在本地事务中处理订单；零金额结账另调用财务结清服务。
- 财务权威：`billing/billing_financial_settlement.go`持有结清服务，声明并处理`commerce.order.financially_settled`事件；不可把支付渠道回调或前端成功提示直接理解为履约成功。
- 事件交接：`composition/financial_settlement_delivery_job.go`在主节点注册周期任务；`fulfillment_worker.go`认领事件并记录投递或失败；`fulfillment_routed_consumer.go`先判断已有附属操作意图，再交给通用商品履约。
- 通用履约：`productfulfillment`区分人工、审批后插件、付款确认后插件等策略，并有查询优先、有限重试和人工处理路径。
- 服务器交接：`productfulfillment_worker_resource.go`通过`provisionresourcecap`创建／读取资源任务，`idc-resource`再承担服务器供应商执行与状态投影。

这里记录的是已识别调用链，尚未对每个事务、金额、库存、幂等与失败分支做完整业务验收。

### 支付、退款与续费

支付创建／回调／查询入口：`idc-payment/backend/internal/service/paymentflow/service.go`。持久支付事实与渠道配置分别在`paymentattempt`、`channelconfig`；账单、余额、授信、退款权益联动在`idc-core/billing`。退款还涉及`composition/refund_*`、合同与佣金补偿任务。

续费入口主要在`idc-core/renewal`，定时处理在`composition/renewal_*`，资源侧接缝在`idc-resource/renewalsource`等目录。排查时应区分报价、收费、权益期限、资源续期和失败恢复，逐项确认用户实际要求。

### 身份、权限、工单和内容

宿主认证与角色权限在`lina-core/internal/service/auth`、`role`、`datascope`、`middleware`；客户业务身份在`idc-core/principal`、`customer`、`onboarding`等服务；业务员工资格在`staff-access`。个人／企业认证实现位于`idc-compliance`，投影与业务门禁还经过`idc-core/identityprojection`、`identityworkflow`。

工单主服务是`idc-core/supportdesk`，上游交接为`supportupstream`，定时规则在`composition/support_*`；内容为`contenthub`，包含附件上传、扫描、可见范围和下载。通知投递由`idc-notify`承担。页面上的状态、内部状态及第三方状态需分别核实。

## 从用户描述到代码的快速定位

下表中后台页面和服务默认相对于`apps/lina-plugins/idc-core/`；其他拥有者明确写出。

| 用户描述 | 后台入口／前端 | 后端与客户侧定位 |
| --- | --- | --- |
| 系统配置、配置入口找不到 | `frontend/pages/system-configuration.vue`、`components/configuration-surface.vue` | 目标插件清单、源码页面注册表、运行时菜单 |
| 商品分类、商品编辑、价格、库存 | `catalog-categories.vue`、`catalog-management.vue`、`catalog-client.ts` | `catalog`、`catalogdraft`、`configurationcontract`；客户`views/account/products/` |
| 自动化接口、上游配置 | `catalog-automation-interfaces.vue` | `productautomation`、`idc-resource`与目标动态连接器 |
| 客户中心服务菜单、购买分类 | `product-directories.vue` | `productdirectory`；客户`product-directories`页面和动态目录路由 |
| 客户资料、联系人、员工归属 | `customer-management.vue`、`components/customer-overview-drawer.vue` | `customermanagement`、`customer*`、`staff-access` |
| 报价、代下单、购物车、订单 | `assisted-quotes.vue`、`commerce-orders.vue` | `commerce`；客户`views/account/commerce/`和`api/idc/customer-commerce.ts` |
| 账单、余额、退款、信用额度 | `financial-management.vue`及财务组件 | `billing`、`financeops`、`idc-payment`；客户`finance`页面和财务请求适配 |
| 付了钱没有开通、履约审核 | `product-fulfillment-tasks.vue` | `billing`结清、`outbox`、`fulfillment`、`productfulfillment`及任务注册 |
| 服务器、重装、续费、换机、换地址 | `idc-resource/frontend/`及`server-operation-security.vue`、`ip-change-policy.vue` | `serveroperation`、`renewal`、`serviceaction`、`idc-resource`；客户`servers`页面 |
| 实名、合同、发票 | `idc-compliance/frontend/` | `idc-compliance/backend/internal/service/`及`central-gateway`；客户`identity`、`contracts`、`tax-invoices` |
| 工单、部门、状态、超时 | `support-desk.vue`、`support-settings.vue` | `supportdesk`、`supportupstream`；客户`support`和公开售前工单页面 |
| 新闻、公告、知识库、下载 | `content-management.vue` | `contenthub`；官网`PublicContentPage.vue`、`content-client.ts` |
| 短信、邮件、消息模板 | `idc-notify/frontend/` | `idc-notify/notification`及`idc-core/notificationpolicy` |
| 优惠券、活动、佣金、代理 | `marketing-management.vue`、`sales-channel.vue` | `marketing`、`commission`、`saleschannel`；下游接口另看`idc-channel` |
| 官网品牌、页脚、导航 | `idc-portal/frontend/pages/portal-brand.vue` | `idc-portal/brand`；官网`PublicSiteShell.vue`与其他品牌消费者 |
| 数据迁移 | `idc-zjmfmigrate/frontend/` | 同插件`backend/internal/service/migrate`及动态宿主桥 |

## 已观察事实与待核实线索

1. **已知修复记录**：官网品牌页曾引用不存在的`/x-assets/idc-portal/v0.1.10/mount.js`，前一轮已替换为原生源码页面，并验证真实接口加载及模拟的保存、预览、发布、回滚。不能据此宣称真实官网渲染和所有品牌字段都已验收。
2. **配置聚合增加了定位层数**：系统配置的抽屉内还会挂载页面和动态沙箱，独立访问成功不代表抽屉中的上下文也正确。后续页面反馈应保留具体进入方式。
3. **交易状态存在多层权威**：订单、应收账单、支付尝试、财务结清、履约意图、资源任务、服务权益各自有状态。需要逐项明确业务含义，不能统一改一个展示标签就认为修好了。
4. **大文件值得优先展开**：当前`marketing-management.vue`约`2833`行、`idc-core/backend/plugin.go`约`2573`行、`financial-management.vue`约`1666`行、客户服务器页面约`1652`行。行数说明阅读和修改影响面大，不足以判定代码混乱。最大文件列表还包括纯图形资源，不能按行数机械排序重构。
5. **文档和实现可能不同步**：前轮已发现品牌页面说明和资源断言仍引用旧入口；当前活跃变更严格校验还存在`idc-provider-integration/spec.md`中需求位于`Purpose`下的警告。后续以源码、用户确认和实际证据交叉判断。
6. **官网端到端消费待核实**：已见独立官网代理配置、直接品牌读取和本地端口差异；这属于待测试链路，不在本次结构梳理中判定根因或擅自修改。

## 后续修正方式与记录维护

每次反馈建立“用户描述／实际入口 → 预期业务结果 → 当前页面与接口 → 领域状态和数据归属 → 根因 → 最小修改 → 自动化与真实页面验证”的短记录。用户对业务含义的新确认更新对应偏好主题和受影响规范；本地图谱只更新代码位置及关系。

本轮仅梳理与记录，没有新增业务修改。工作区保留前轮官网品牌修复。后续修改仍按当前`AGENTS.md`执行；检查宿主修改授权和插件本地规则，不能将这次读取授权解释为任意重构、生产写入、提交或推送授权。

当前活跃变更目录：`add-idc-server-operations-platform`、`harden-idc-runtime-delivery`、`add-plugin-market-docs`、`streamline-agent-context`。本轮没有加载无关变更全文，也没有调整其任务状态。

本文件及索引属于文档和静态定位产物；验证采用路径存在性、索引计数、链接及差异检查，不重复运行与本轮无行为变化的业务测试。
