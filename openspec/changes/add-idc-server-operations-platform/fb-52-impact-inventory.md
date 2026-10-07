# FB-52 全仓影响清单与保护方案

## 盘点结论

本清单是任务`0.1`的实施依据。当前实现不是某个页面字段写错，而是旧服务器商品假设已经从商品目录扩散到交易、服务、资源、插件清单、前端和测试。纠偏必须替换旧模型本身，不能在固定`cloud_server`、`bare_metal`和`resource_pool`字段外再包一层通用接口。

当前 OpenSpec 进度为`190/301`，第一个真实未完成任务是`0.1`。第 12 阶段及以后所有依赖旧模型的任务保持冻结。约定服务当前监听`19120`、`15666`、`16677`和`16680`；`9120`由其他项目占用，本变更不得访问、重启或占用该端口。

## 参考源码结论

用户提供的智简魔方财务源码只作为业务拆分参考，不复制实现和表结构。

| 参考文件 | 可借鉴的业务拆分 | 明确不能照搬的部分 |
|---|---|---|
| `app/common/logic/Product.php` | 商品、定价、自定义字段、配置组和上游商品可以分别读取与组合 | `getProducts`在商品循环内继续查询字段、价格、配置组、选项和子选项，存在明显`N+1`；同步逻辑把`type`、`server_group`和上游字段写回商品 |
| `app/common/logic/ConfigOptions.php` | 配置组、配置项、子选项、价格和联动规则是独立概念 | 初始化直接读取操作系统和服务器配置，配置项仍与服务器类型和上游商品耦合 |
| `app/common/logic/Provision.php`、`app/admin/controller/ProvisionController.php` | 自动开通模块可声明配置、元数据和动作 | 接口实例仍以服务器组为中心，并在控制器写死`dcim`、`dcimcloud`、重装系统等服务器动作 |
| `app/common/logic/PluginLogic.php` | 插件类型和插件目录可以独立发现 | 旧系统按插件目录类型建立多套管理方式，不符合 LinaPro 统一插件治理 |
| `app/common/logic/Menu.php`、`app/admin/controller/MenuController.php` | 客户中心与官网菜单是可配置表面 | 菜单缺少现代宿主插槽、权限、隔离桥和生命周期约束 |

采用的结论是“商品、配置、自动化模块和菜单表面可以分离”；拒绝复制固定商品类型、服务器组中心、同步覆盖历史商品、逐项查询和多套插件管理器。

## 工作区保护基线

当前主仓存在大量未提交内容，其中`apps/lina-plugins`表现为旧 Git 子模块记录删除、同路径目录重新出现为未跟踪内容。它已经不是可安全单独重置或切换的独立仓库。本变更采用以下强制保护方式：

- 不执行`git reset`、`git checkout --`、`git clean`、递归删除、批量覆盖或全仓格式化。
- 不恢复`.gitmodules`，不把`apps/lina-plugins`重新当作子模块，也不在该目录执行会回退父仓状态的 Git 命令。
- `AGENTS.md`、`.agents/rules/database.md`、`.agents/rules/plugin.md`、根构建入口、`hack/tests`、`hack/tools/linactl`、`apps/lina-vben`工作区配置、整个`apps/lina-plugins`、`web-customer`、`lina-websites`和活跃 OpenSpec 都视为已有成果保护区，只允许为 FB-52 做精确补丁。
- 修改前再次比较目标文件状态和内容；出现其他对话新增变化时，先重新合并语义，不覆盖整文件。
- 插件目录当前没有插件根级`AGENTS.md`；后续每次修改插件前仍需重新检查，若新增则先读取。
- `.contributing`存在，但 FB-52 的通用宿主变更仍必须限于插件清单、发现、隔离和生命周期等框架级能力；IDC、服务器、证书、手机、短信和支付供应商业务不得进入`lina-core`或`web-antd`。
- 未经用户要求不提交、不推送、不创建`PR`，FB-52 完成前不归档 OpenSpec。

## 插件宿主与清单影响

### 当前结构错误

现有`plugin.yaml`已经分别保存`type`和`distribution`，但业务扩展点、后台表面、客户中心表面尚未成为独立的版本化声明。大部分页面仍共用一份`menus`；动态前端主要依赖`public_assets`和菜单查询参数，不能表达客户中心独立菜单、允许挂载槽位、页面桥能力和完整隔离策略。

宿主主要影响路径如下：

- `apps/lina-core/pkg/plugin/catalog/`：清单解析、类型、校验和清单快照。
- `apps/lina-core/pkg/plugin/capability/capmodel/capmodel_manifest.go`：对外清单投影。
- `apps/lina-core/internal/service/plugin/internal/governance/`：清单治理。
- `apps/lina-core/internal/service/plugin/internal/integration/`：菜单、权限、启用快照和修订同步。
- `apps/lina-core/internal/service/plugin/internal/frontend/`：`public_assets`、动态页面绑定和前端产物验证。
- `apps/lina-core/internal/service/plugin/internal/management/`：插件管理列表投影，目前需要明确只返回`managed`。
- `apps/lina-core/internal/service/plugin/internal/lifecycle/`：安装、升级、启用、停用和卸载，目前缺少独立“停止新业务”和业务影响排空契约。
- `apps/lina-core/internal/service/plugin/internal/upgrade/`：升级时清单、菜单和权限同步。
- `apps/lina-core/internal/service/plugin/internal/testutil/`及同层测试：测试清单仍主要围绕单一`menus`和旧 ID 示例。

精确缺口位于`catalog_manifest_types.go`的`Manifest`和`ArtifactManifest`、`catalog_manifest_validate.go`、`catalog_manifest_cache.go`，以及公开投影`capmodel_manifest.go`、`manifestcap.go`、`plugincap.go`、`pluginhost/internal/manifestview/manifestview.go`和`api/plugin/v1/plugin_list.go`。`pluginhost_slots.go`当前的`ExtensionPoint`是登录、生命周期、HTTP、Jobs 和菜单过滤等技术 Hook，不能冒充商品履约、短信、支付和官网模板等业务扩展点。

### 必须新增的清单边界

后续`0.2`至`0.6`必须把以下内容分别建模和校验，不能用插件名称推断：

1. 运行类型：`source`或`dynamic`。
2. 分发方式：`builtin`或`managed`。
3. 版本化业务扩展点：至少商品履约、短信渠道、支付方式和官网模板。
4. 后台表面与客户中心表面：分别声明菜单、页面、标题、图标、权限、状态、宿主槽位和桥能力。

动态页面必须通过版本化、按插件、能力和方法授权的页面桥或接口代理访问数据。现有`embedded-mount`只能作为待替换资产，不能向第三方代码交出宿主原始令牌、全局对象或状态容器。

现有后台页面壳`apps/lina-vben/apps/web-antd/src/views/system/plugin/dynamic-page.vue`会把`accessStore.accessToken`交给插件模块，并用动态`import`直接在宿主 JavaScript 与 DOM 上下文执行第三方 ESM。这是明确的安全缺口，`0.4`必须删除该令牌传递和同进程导入路径。`web-customer`当前也没有客户插件表面清单、动态页面壳或隔离桥。

## 插件生命周期与管理页影响

当前清单中`tenant-core`仍为`source + managed`，必须改为强制安装、强制启用、不可停用和不可卸载的`source + builtin`。`idc-core`、`idc-payment`、`idc-notify`等稳定业务拥有方继续是内置源码插件，但不能出现在插件管理列表。

宿主`plugin_list.go`、`internal/management/management.go`和`api/plugin/v1/plugin_list.go`目前明确保留或默认展示`builtin`，`plugin_list_test.go`中的`TestListIncludesBuiltinByDefault`还把错误行为固化为测试。它们必须在`0.6`改为管理列表只返回`managed`，同时保留宿主内部发现内置插件的能力。

`idc-zjmfcloud`当前是`source + builtin`，必须迁为可安装、升级、停止新业务、停用和卸载的托管动态服务器连接器。`content-notice`和`idc-zjmfmigrate`当前虽为`managed`，也必须遵守新的管理列表和生命周期契约。

生命周期需要新增独立的“停止新业务”状态和影响检查投影。停止后只禁止新接口实例、新商品版本绑定、新购买和新履约任务；已有服务菜单、后台处置、查询、续期、售后、下载和物流跟踪仍保留。停用或卸载前必须批量检查在售版本、在途任务、活动服务和仍需插件执行的历史动作，不能按对象逐项查询。

## 官方插件 ID 迁移清单

现有 15 个官方目录全部带`linapro-`前缀，必须一次性改名，不建立别名、重定向或兼容层。

| 当前 ID | 目标 ID | 当前分发判断 |
|---|---|---|
| `content-notice` | `content-notice` | 稳定业务拥有方，最终分发以对应业务设计为准 |
| `idc-ai` | `idc-ai` | 内置源码拥有方 |
| `idc-channel` | `idc-channel` | 内置源码拥有方 |
| `idc-compliance` | `idc-compliance` | 内置源码拥有方 |
| `idc-core` | `idc-core` | 内置源码拥有方 |
| `idc-notify` | `idc-notify` | 内置源码拥有方 |
| `idc-payment` | `idc-payment` | 内置源码拥有方 |
| `idc-portal` | `idc-portal` | 内置源码拥有方 |
| `idc-resource` | `idc-resource` | 服务器领域内置源码拥有方 |
| `idc-zjmfcloud` | `idc-zjmfcloud` | 托管动态服务器连接器 |
| `idc-zjmfmigrate` | `idc-zjmfmigrate` | 托管迁移插件 |
| `shared-services` | `shared-services` | 内置源码拥有方 |
| `staff-access` | `staff-access` | 内置源码拥有方 |
| `tenant-core` | `tenant-core` | 强制内置源码插件 |
| `vben-adapter` | `vben-adapter` | 内置前端契约插件 |

旧前缀目前至少出现在`apps/lina-plugins`的 2212 个文件、`apps/lina-core`的 109 个文件、`web-customer`的 61 个文件、`lina-websites`的 21 个文件、`hack`的 21 个文件和活跃 OpenSpec 的 186 个文件。`0.7`必须覆盖目录、Go module/import、清单依赖、嵌入注册、路由、权限、菜单键、`i18n`命名空间、资源 URL、构建清单、测试夹具、文档和所有`plugin_linapro_*`表名，并用全仓零命中治理扫描收口。

## 商品与定价影响

### SQL 主链

- `manifest/sql/010-catalog-foundation.sql`：商品分类和商品基础、固定商品类型、版本、配置组与配置项，是旧模型入口。
- `014-catalog-inventory-routing.sql`、`016-catalog-resource-pool-candidate-binding.sql`：把商品和版本直接绑定服务器资源池。
- `015-catalog-management-read-indexes.sql`、`017-catalog-public-read-indexes.sql`：索引继续使用`product_type_code`。
- `020-marketing-foundation.sql`：营销范围含`product_type`，需要改为通用商品、分类或插件声明的可治理范围。

### API、DAO、服务和前端主链

- `backend/api/admin/v1/catalog_category_create.go`、`catalog_category_list.go`、`catalog_product_create.go`、`catalog_product_types.go`和`catalog_product_detail_types.go`暴露固定类型或服务器资源池字段。
- `backend/api/public/v1/catalog_product_list.go`和`catalog_types.go`把旧类型带到官网商品接口。
- `backend/internal/service/catalog/catalog_management*.go`、`catalog_public_read_persistence.go`、`catalog_purchase_quote.go`、`catalog_copy.go`、`catalog_inventory_routing.go`和`catalog_resource_pool_*`继续执行分类匹配、固定类型和资源池绑定。
- `catalog_management.go`、`catalog_management_persistence.go`和`catalog_management_read.go`把有效商品类型固定为`cloud_server`与`bare_metal`；`catalog_copy.go`和`catalog_publication_preview.go`强制分类类型等于商品类型；`catalog_component.go`直接解析处理器、内存、磁盘和带宽。
- `backend/cap/poolsourcecap/`把服务器资源池能力发布给商品核心，后续应由服务器插件契约拥有。
- `frontend/pages/catalog-management.vue`、`catalog-categories.vue`、`components/catalog-category-create-modal.vue`、`src/catalog/CatalogWorkspace.vue`和相关 API 客户端包含旧商品类型、服务器配置与资源池交互。
- `frontend/pages/catalog-client.ts`把商品类型锁为`bare_metal | cloud_server`，并固定处理器、内存和带宽组件；`components/catalog-configuration-option-create-modal.vue`继续以服务器规格组织配置表单。

重写后分类只保留商家目录语义；商品和版本只保存通用商业内容、计费、价格、上下架、履约方式、可选单接口实例绑定和冻结配置声明。`CPU`、内存、系统、磁盘、带宽、`IP`、地区、线路、域名、证书、`CDN`、手机和物流字段不得进入通用商品表或 DTO。

## 购物车、报价、订单、账单、服务与履约影响

SQL 主链为：

- `022-commerce-foundation.sql`：购物车、配置快照、报价、订单项和履约单元保存`product_type_code`、资源池或服务器配置假设。
- `026-commerce-assisted-order.sql`、`036-trial-qualification.sql`和`037-financial-settlement-fulfillment.sql`：协助报价、试用和付款后履约沿用旧快照。
- `030-service-renewal-foundation.sql`：`service_instance.product_type_code`被限制为`cloud_server`或`bare_metal`，并通过复合外键把服务继续绑定资源池和旧订单项快照。
- `031-service-renewal-execution.sql`、`032-service-expiry-scheduling.sql`和`033-billing-directory-indexes.sql`：续期、到期和财务目录建立在旧服务实例上。
- `038-service-provision-execution.sql`：服务开通任务仍面向服务器资源 Provider。
- `056`至`067`系列 SQL：服务器操作、附件、换`IP`、批量、变配和转移属于应迁入服务器插件的专业能力，不能删除，但不再属于通用商品核心。

主要 Go 路径包括`backend/internal/service/commerce/`、`catalog/catalog_purchase_quote.go`、`billing/billing_financial_settlement*.go`、`provisioning/provisioning_persistence.go`、`fulfillment/`、`renewal/`、`lifecycle/`和`serveroperation/`。对应 DAO/DO/Entity 包括`commerce_cart_item`、`commerce_configuration_snapshot*`、`commerce_quote_*`、`commerce_order_item`、`commerce_fulfillment_unit`、`service_instance`和`service_price_snapshot`。

通用快照必须只冻结商品版本、履约方式、插件 ID、插件契约版本、接口实例、配置声明版本和经过服务端校验的不透明配置值。账单只保存商业说明、数量、币种、金额、税费和来源快照，不理解插件字段。服务核心表示通用权益与生命周期引用，服务器运行状态和服务器动作由服务器插件拥有。

现有安全门禁值得保留：结算只建立`pending_payment`订单、账单和履约意图，`fulfillment_consumer.go`只消费`FinanciallySettled`事件，Provider 任务只在履约与业务操作都就绪后创建。重写必须把它提升为三种履约方式共同使用的权威通用门禁，并增加“未付款时插件适配器构造和调用次数均为零”的测试。

## 服务器能力与智简魔方连接器影响

服务器能力不能删除，必须迁移到`idc-resource`拥有的服务器插件契约。主要资产位于：

- `apps/lina-plugins/idc-resource/backend/cap/resourcecap/`：现有服务器`Provider`、操作、附件、主体和能力契约。
- `backend/internal/service/provisionsource/`、`renewalsource/`、`serveroperationsource/`、`resourcetask/`、`inventorysnapshot/`和`poolcandidate/`：服务器开通、续期、操作、任务、库存和资源池。
- `manifest/sql/`中的渠道、资源池、映射、任务和证据表。
- `frontend/pages/provider-configuration.vue`、`provider-tasks.vue`、`provider-health.vue`及服务器页面。
- `apps/lina-plugins/idc-zjmfcloud/backend/internal/provider/zjmfcloud/`：当前源码 Provider 实现和服务器专用动作。

迁移后，通用商品履约扩展点只把冻结输入交给插件。服务器插件内部再使用版本化服务器`Provider`契约、资源池、负载与故障处理。智简魔方连接器以托管动态插件消费该契约，不再编译进宿主源码插件集合。

## 三个前端边界和测试影响

- 后台业务页面继续放在对应插件`frontend/`，通过受治理后台表面装配；禁止向`web-antd`写 IDC 页面、路由、状态、多语言或业务依赖。
- `apps/lina-vben/apps/web-customer/src/router/routes/modules/account.ts`和现有服务器 API、页面只能作为服务器插件迁移资产。客户中心需要按插件声明动态增加`SSL`证书、手机订单和云服务器等不同入口，不能继续把全部服务套入`views/account/servers/`。
- `web-customer`当前把`/account/products`重定向到`/account/servers`，通用商业 API 仍使用`serverQuantity`；其`src/plugins/page-registry.ts`会编译期扫描插件`frontend/pages/*.vue`并按插件 ID 与文件名推导路由，不能继续用目录扫描猜测后台页与客户页。
- `apps/lina-websites/sites/default`只负责公开商品浏览和权威购物车交接，需要消费通用商品与插件购买配置投影，不拥有登录后服务页。
- 默认官网`app/components/portal/catalog-client.ts`、`ProductCatalog.vue`和`ProductConfigurator.vue`也锁定服务器类型或`serverQuantity`，必须重写为通用公开商品与插件配置投影。
- 现有服务器测试、固定`cloud_server`/`bare_metal`夹具、`linapro-`路径断言和单一`menus`测试只能作为迁移回归输入，不能证明通用模型完成。
- 必须新增服务器、`SSL`和手机三类真实插件形态的契约、购买、付款或审核、异步履约、独立菜单、列表、详情和操作测试；动态页面 E2E 还要覆盖首次/再次进入、中英文、明暗主题、窄屏、慢资源、来回切换、刷新、权限变化和停止新业务。

## 开发数据库保护与迁移方案

项目没有线上兼容负担，但当前开发库中的测试和浏览器验证数据必须保护。实施采用“先快照映射、再幂等迁移、后对账、最后清理”的方式：

1. 在任何破坏性 SQL 前记录当前插件版本、表结构修订和服务器商品、版本、购物车、报价、订单、账单、履约、服务、资源映射及任务的数量和关键业务号摘要。
2. 新表和新字段先以可重复 SQL 建立；迁移以稳定业务号作为幂等键，把旧服务器字段写入服务器插件配置快照，把旧资源池引用写入服务器插件接口实例或内部池映射。
3. 历史商品版本、订单项、客户配置、接口引用、履约证据和审计只迁移，不覆盖；迁移不得触发新开通、扣费、通知或供应商调用。
4. 输出迁移前后逐类型数量、金额、业务号、活动服务和在途任务对账；无法唯一映射的记录进入明确人工接管清单，不猜测。
5. 本地测试允许通过受控初始化重建，但必须先保存可重放 fixture，并明确区分“迁移验证库”和“全新安装库”。禁止直接清空当前正在运行的开发库。
6. 只有新模型、服务器回归、三类商品测试和对账全部通过后，才删除旧字段、旧表、旧路径和旧测试。

## 跨领域影响判断

- `i18n`：有重大影响。官方 ID、菜单、状态、错误、表单和插件页面都会变化，必须同步宿主或插件自身中英文资源，禁止显示原始 key、枚举和`JSON`。
- 缓存一致性：有重大影响。插件扩展点、菜单、页面、接口实例和已发布商品版本都需要权威 revision、事务后失效、多实例重新读取和最大陈旧边界。
- 数据权限：有重大影响。后台扩展点和接口实例按员工权限及数据范围过滤；客户页面和服务必须在查询对象前校验当前客户所有权；插件之间不得查私有表。
- 开发工具：`0.1`本身无脚本改动；`0.7`会影响插件发现、构建、嵌入、代码生成、E2E 清单和跨平台入口，届时必须单独记录 Windows、Linux 和 macOS 验证。
- 性能：扩展点、插件菜单、商品列表和快照装配必须批量查询。禁止按插件、菜单、商品、配置项或订单项逐条补查，测试需包含查询次数或等价静态证据。
- 测试：`0.1`是治理盘点，不改变运行行为，以文件扫描、OpenSpec 严格校验和`lina-review`作为门禁；从`0.2`开始每项行为变化必须增加单元、PostgreSQL 集成或 E2E。

## 后续执行边界

下一项只能开始`0.2`：先扩展统一清单和发现模型。不得越过`0.2`提前重命名目录或删除旧商品字段，也不得继续第 12 阶段。`0.2`完成实现、测试、严格校验和审查后，才能依次进入后台/客户表面、动态隔离桥、生命周期、内置隐藏、去前缀和商品重写。

## 任务级审查记录

- 审查范围：本清单、`proposal.md`、`design.md`、`tasks.md`、`coverage.md`、两份受影响增量规范、FB-52 纠偏记录，以及仓库中的插件宿主、十五个官方插件、商品交易链路、客户中心、默认官网、SQL 和测试入口。
- 审查结论：没有遗漏会阻止进入`0.2`的代码域；旧模型的保留迁移、重写和最终清理边界已逐项区分，未把任何历史服务器实现当成通用模型完成证据。
- 保护结论：没有修改、删除、覆盖或格式化现有生产代码和他人未提交内容；开发数据库只记录迁移方案，未执行清库或破坏性迁移。
- 验证结论：`0.1`只改变治理文档，不改变运行行为，因此不需要 Go、前端、SQL 或浏览器回归；严格 OpenSpec 校验和重复项扫描通过后方可勾选。
