# 源码阅读补充记录

记录日期：2026-10-10。与工作区已有的[项目源码地图](project-source-map.md)配合使用。本文件仅记录本轮实际核对的入口，不替代业务规范，不构成全项目审计结论。

## 本轮覆盖

已扫描`apps`和`hack`的可见文件树、全部25个插件的清单类型与职责、主要插件的页面和服务目录；阅读了宿主启动、管理台插件页面注册、客户路由与请求封装，以及下单、支付、交付和服务器操作的关键源码片段。未逐行读完所有源码，未运行业务测试或浏览器验收。

按`rg --files apps hack`统计，宿主1506个可见文件、插件5785个、前端工作区1623个、官网92个、工具322个。统计包含测试、资源和文档，遵循忽略规则，不表示生产源码数量或插件启用状态。

## 异步交付定位

以下路径相对于`apps/lina-plugins/idc-core/backend/internal`。

| 入口 | 本轮已核对的行为 |
| --- | --- |
| `composition/financial_settlement_delivery_job.go` | 仅主节点注册，按10秒周期、每批20条处理已结清订单；任务描述明确为生成本地服务和业务操作意图，不直接调用供应商 |
| `composition/product_fulfillment_job.go` | 分别注册商品交付、资源结果回报、取消收敛任务，均仅主节点执行，周期5秒、每批20条 |
| `composition/server_operation_job.go` | 核心服务器动作与资源拥有者的供应商任务之间的异步衔接入口 |
| `controller/account/account_v1_server_operation_create.go` | 调用`serveroperation.CreateCurrentCustomer`，传入服务编号、动作、参数、密码、证明令牌和幂等请求号；返回操作编号、状态与创建时间 |

因此，遇到付款后未开通、服务器操作卡住或页面状态不更新，需要分别追踪订单结清、核心交付、资源任务、供应商结果和前端刷新。这里记录排查层次，不预判具体故障，也不证明这些任务当前已运行。

## 页面与接口补充

- 管理台`apps/lina-vben/apps/web-antd/src/plugins/page-registry.ts`根据`virtual:lina-plugin-pages`及`pluginPageMeta`注册插件页面；业务页面通常位于插件的`frontend/pages`。
- 客户中心`apps/lina-vben/apps/web-customer/src/api/request.ts`的`pluginApiPath`生成`/x/<plugin-id>/api/v1/<path>`。页面接口适配集中在`src/api/idc`，不能仅凭菜单标题判断后端拥有者。
- 客户中心账户路由的购物车、订单列表、订单详情共用`commerce/index.vue`；多个财务入口共用`finance/index.vue`；服务器列表与详情共用`servers/index.vue`。修改前核对不同路由模式。
- 官网`apps/lina-websites/packages/site-sdk/src/index.ts`的默认接口前缀是`/x/idc-core/api/v1`，包括公开商品与访客购物车。官网展示、门户品牌配置和客户中心是不同源码入口。
- 根目录`go.work`显式列出宿主与`linactl`；插件另有独立模块。不能默认根工作区一次测试覆盖全部插件。

## 后续使用

用户逐项描述具体问题后，沿页面、请求适配、接口、控制器、业务服务、数据与异步任务定位，再读取相关业务确认及完整规范。收费、权限、数据归属和主要流程存在真实歧义时，确认后再修改依赖该答案的实现。

本轮保留已有门户改动及并行新增的地图和索引，只新增此补充记录。未修改业务代码、任务状态或业务规则，未提交或推送。
