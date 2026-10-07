# 第二十九阶段：服务器接口统计局部验证

## 范围与根因

本记录只覆盖`FB-113`已确认的本地使用统计、正式接口详情和供应商接口页两张表的分页。此前列表未提供统计字段，且接口和资源池分别固定截取一百及五百条；新建资源池还把当前接口列表当成全部候选。此次没有实现上游商品导入、发布与定价，也没有修改流量包、取消退款、宿主或现有业务权限。

统计使用用户确认的当前绑定语义：`service_instance.channel_no`当前绑定的本地实例计入总数，包含手工、迁移和已终止实例；已激活以服务权益判断，不以开关机判断。已设置只计算未删除商品的`catalog_product.current_version_no`实际生效绑定，同商品去重，不取最新草稿和历史绑定，无生效版本指针的商品不计数。

## 实现边界

复用`resourceinterfaceprojectioncap`新增只读`UsageReader`，由`idc-core`在启动声明工厂，`idc-resource`显式注入原管理授权器。数据库读取仍在数据拥有方内部，资源插件不跨插件查表。读操作先通过既有员工配置目录权限与支持的数据范围检查，再对最多一百个接口进行两次集合统计；没有逐行查询、上游请求、缓存修改、库存写入或计费行为。

接口和资源池各自返回准确总数及有界分页，支持每页十、二十、五十、一百条。统计只装配当前接口页；资源池名称联查当前池所属接口，不依赖另一张表的页。管理目录保留已删除配置，以维持原有恢复操作；资源池候选单独排除已删除接口，使用名称搜索及滚动分页，而不是截前一百条当作全部候选。

正式页面使用既有`useVbenVxeGrid`与官方完整分页条，详情使用真实`useVbenDrawer`。改变每页条数回第一页，普通翻页保留请求页；暂时失败保留已确认页，真实`HTTP 401/403`清空目录、统计及相关编辑或详情状态，并阻止旧请求迟到恢复授权数据。私有请求适配保留官方响应和令牌续期拦截器，只恢复真实传输状态，不把业务码当作`HTTP`状态。

独立入口表格仍通过同一隔离`Vben`运行时挂载，已接统计及数字分页。共享运行时`vben-adapter/frontend/src/isolated/types.ts`只提供`mountIsolatedVbenTable`，没有声明抽屉和表单接口；未新增自绘详情或原生搜索替代。独立入口已有原生编辑表单及资源池候选仍需主线统一提供真实`Vben`表单与抽屉接缝，这一缺口不能计为完成。

## 查询性能证据

服务统计命中既有`030`迁移的`idx_idc_service_resource_projection`前缀。商品统计显式限定资源来源，使用`096`的接口局部索引、`074`的接口绑定索引，再经版本和商品已有唯一键连接；额外版本连接同时验证当前商品所属关系，不改变当前生效版本语义，无新增迁移。

隔离数据库的`EXPLAIN`验证了接口、绑定、版本、商品逐段索引可用。该断言仅在测试事务中关闭顺序扫描以验证索引资格，不是生产延迟压测，也没有修改全局数据库参数。生产数据量下的实际计划仍由最终集成验证负责。

## 已执行验证

以下命令均针对当前工作区，`Go`命令使用`GOWORK=off`。数据库连接从既有本地配置安全读取，未输出连接秘密。两个数据库测试串行执行，使用随机隔离模式并清理自身数据，不接触开发业务数据。

- `idc-core`：`go test ./backend/internal/service/resourceinterfaceprojection ./backend/cap/resourceinterfaceprojectioncap/... ./backend -count=1`通过；服务包、契约包及启动包分别约零点七、零点三、二点四秒。
- `idc-core`：`go test ./backend/internal/service/resourceinterfaceprojection -run TestUsageProjectionPostgreSQL -count=1 -v`通过，测试零点一五秒。覆盖当前绑定、手工与迁移、停机及暂停、终止、历史与草稿、删除商品、其他租户和拥有方、权限拒绝、重复与超限请求及索引路径。
- `idc-resource`：`go test ./backend/internal/service/providerconfiguration ./backend/internal/controller/admin ./backend -count=1`通过，包含新增调用契约和启动装配编译。
- `idc-resource`：`go test ./backend/internal/service/providerconfiguration -run TestProviderConfigurationPagesPostgreSQL -count=1 -v`通过，测试零点五三秒。覆盖一百二十一条接口、五百二十一条资源池、四种页规模、后续页、空页、准确总数、独立名称、已删除候选过滤及单次批量统计。
- 私有请求适配：真实`RequestClient`与官方令牌续期拦截器十三项测试通过；覆盖首轮拒绝、续期后拒绝、续期后网络失败、暂时故障、业务码不冒充传输状态及不重复解包。临时配置为`temp/20260904/190100-provider-request-vitest.config.mjs`。
- 正式插件页面：`pnpm exec vue-tsc --noEmit -p D:/golang/cloud/temp/20260904/190000-provider-tsconfig.json`通过，显式包含本页，不冒称所有插件页面类型检查。
- 独立入口：`pnpm run build`通过，包含`vue-tsc`、生产打包与入口验证，产物为`idc-resource/frontend/dist/mount.js`。
- `make.cmd lint.go dir=apps/lina-plugins/idc-resource/backend plugins=1`通过，含全模块`golangci-lint`与`staticcheck`；新增核心投影及契约定向`golangci-lint`零问题。
- `make.cmd lint`实际执行后报告命令不存在。按后端规则允许的`lint.go`执行；核心整模块当次仍被并行修改的取消证据、履约列表、附属报价复杂度和取消文件格式阻断，已交主线收口，不能宣称核心整模块最终通过。
- `make.cmd i18n.check`通过；`openspec validate add-idc-server-operations-platform --strict`通过。
- `make.cmd ctrl dir=temp/20260904/190200-resource-ctrl/backend`通过。仅在复制的接口与控制器目录生成，`ProviderConfigurationList`签名一致，聚合业务控制器哈希一致；生成器新增的同名功能空桩只留在临时副本，没有回写工作区。

## 浏览器门禁

新增`TC011-provider-usage-pagination.ts`的两项用例，覆盖统计、官方详情反复打开、独立数字分页、改页规模回第一页、暂时失败保留旧页和权限失效清空后恢复。原`TC007`与`TC010`仅同步有界响应契约，后者增加统计和分页可见断言。

从`hack/tests`的唯一 Playwright 依赖执行。正常重启`6030/6031/6032`后，`TC011a/b`两项在正式 Chromium 运行时通过；覆盖统计、详情反复打开、接口与资源池独立数字分页、改页规模回第一页、暂时失败保留旧页、权限失效清空及恢复。曾从前端工作区调用另一份测试依赖触发双版本错误，改回项目测试入口后通过，未修改依赖。本轮没有另行生成截图，不把自动断言表述为人工截图审阅。

新增用例及其页面对象通过`temp/20260904/190300-provider-e2e-tsconfig.json`定向类型检查；临时配置显式复用现有`Node`类型，未修改测试依赖。

## 精确变更文件

`idc-core`范围：

- `backend/cap/resourceinterfaceprojectioncap/contract.go`
- `backend/cap/resourceinterfaceprojectioncap/spi/registrar.go`
- `backend/internal/service/resourceinterfaceprojection/resourceinterfaceprojection.go`
- `backend/internal/service/resourceinterfaceprojection/resourceinterfaceprojection_usage.go`
- `backend/internal/service/resourceinterfaceprojection/resourceinterfaceprojection_code.go`
- `backend/internal/service/resourceinterfaceprojection/resourceinterfaceprojection_usage_test.go`
- `backend/plugin.go`，只新增使用统计工厂声明。
- `manifest/i18n/en-US/error.json`
- `manifest/i18n/zh-CN/error.json`

`idc-resource`范围：

- `backend/api/admin/v1/provider_configuration.go`
- `backend/internal/controller/admin/admin_v1_provider_configuration.go`
- `backend/internal/service/providerconfiguration/providerconfiguration.go`
- `backend/internal/service/providerconfiguration/providerconfiguration_list.go`
- `backend/internal/service/providerconfiguration/providerconfiguration_usage_test.go`
- `backend/internal/service/providerconfiguration/providerconfiguration_postgres_test.go`，只补两个构造调用的只读依赖。
- `backend/plugin.go`，只新增供应商配置初始化时的只读使用统计注入。
- `frontend/pages/provider-configuration-client.ts`
- `frontend/pages/provider-configuration.vue`
- `frontend/pages/provider-request.ts`
- `frontend/pages/provider-request.test.ts`
- `frontend/src/api.ts`
- `frontend/src/request.ts`
- `frontend/src/StandaloneTable.vue`
- `frontend/src/ProviderConfiguration.vue`
- `manifest/i18n/en-US/plugin.json`
- `manifest/i18n/zh-CN/plugin.json`
- `manifest/i18n/zh-CN/apidoc/provider-usage.json`
- `hack/tests/e2e/TC007-provider-configuration.ts`
- `hack/tests/e2e/TC010-standalone-vben-tables.ts`
- `hack/tests/e2e/TC011-provider-usage-pagination.ts`
- `hack/tests/pages/ProviderUsagePage.ts`

临时类型配置、隔离生成副本和编译产物单独作为证据，不是生产接口或默认工具入口。本记录本身也纳入文档审查范围。

## 局部审查结论

使用`lina-review`，从主仓`git status --short`和展开的未跟踪目录收集范围，结合本轮精确补丁审查；当前插件目录属于同一工作树，不声称审查了独立子仓。两个插件根目录均无`AGENTS.md`。

本轮读取了顶层`AGENTS.md`、用户偏好，以及开发流程、文档、架构、数据权限、插件、后端、数据库、接口、前端、测试、国际化和开发工具规则；按`goframe-v2`、`vben`、`lina-feedback`、`openspec-apply-change`、`lina-e2e`和`karpathy-guidelines`执行。没有新增或修改缓存，没有修改宿主、生产迁移、默认工具或服务启停行为；临时工具证据实际在本机运行，不宣称其他操作系统实测。

统计、详情和两表分页检查未发现新的权限放宽或业务规则偏差，资源插件生产构建及正常重启后的浏览器回归已经完成。独立入口的公共表单适配和`FB-113`上游商品导入仍未完成，且导入发布与定价规则尚待用户确认，因此`FB-113`不得据本记录整体勾选或归档。没有执行真实上游写入、重置数据库、重新安装插件或提交代码。
