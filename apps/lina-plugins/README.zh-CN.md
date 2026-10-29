# 插件开发与安装规范

这里是当前插件的统一开发入口；每个插件的实际能力、约束和验证边界见下方对应说明。业务分类只服务管理页展示，可随业务增加，不能取代扩展点、运行类型或安装许可。

## 契约与安装门禁

- 清单中的`id`必须与目录一致，语义版本、运行类型、分发方式、依赖、公开资源和页面声明必须合法。内置插件自动装配且不可卸载，普通动态托管插件通过正式生命周期接口安装、更新和卸载。
- 每个提供方声明对应的`point`、契约`version`、`owner`和`role=provider`，并声明带版本约束的拥有方依赖。方法、字段及元数据须满足拥有方公开校验器；缺少校验器或校验失败时拒绝安装与更新。分类名称、README存在、代码可编译均不能替代这些检查。
- 接口配置由提供方声明结构，秘密由业务拥有方保存；商品字段、购买字段和客户资源列表由实际插件定义，不把服务器字段强加给SSL、域名或未来产品。菜单和页面通过已发布的挂载位置接入，身份及对象归属仍由后端校验。
- 停止新业务只阻止新绑定和销售，保留已有服务；停用和卸载需通过业务影响与依赖检查。禁止直接改库、删包、跳过授权或伪造成功。

| 业务 | 公开开发契约与校验入口 | 安装要求 |
| --- | --- | --- |
| 商品交付及云产品 | [`productfulfillmentcap`](idc-core/backend/cap/productfulfillmentcap/declaration.go) | `product-fulfillment@v1`及其有类型元数据；按自身能力声明字段及动作 |
| 服务器接口 | [`resourcecap`](idc-resource/backend/cap/resourcecap/dynamic_provider_contract.go) | `resource-provider@v1`，机密出站及真实协议边界；通常同时声明商品交付 |
| 支付 | [`paymentcap`](idc-payment/backend/cap/paymentcap/dynamic_provider_contract.go) | `payment-method@v1`；不把配置描述插件当成独立账务执行器 |
| 短信及通知 | [`channelcap`](idc-notify/backend/cap/channelcap/channel.go) | 当前是源码适配器SPI；动态`sms-channel`验证器尚未发布，动态安装继续拒绝 |
| 官网 | [开发指南](idc-portal/docs/website-plugin-development.zh-CN.md)、[`websitecap`](idc-portal/backend/cap/websitecap/contract.go) | `website-template@v1`，真实包内页面、静态资源和公开配置 |
| 用户中心 | [开发指南](idc-portal/docs/customer-plugin-development.zh-CN.md)、[`customertemplatecap`](idc-portal/backend/cap/customertemplatecap/contract.go) | `customer-template@v1`，独立构建及客户身份边界 |
| 迁移及系统功能 | [迁移插件](idc-zjmfmigrate/README.zh-CN.md)、[宿主公开插件接口](../lina-core/pkg/plugin/README.zh-CN.md) | 声明实际路由、权限和受治理宿主服务，不虚构不需要的业务扩展点 |

`product-fulfillment`、`resource-provider`等属于公开契约标识；管理页分类不限定这些契约的业务用途。未来产品可以提供新的拥有方规范和验证器，再增加对应展示分类。

## 开发、验证与交付

1. 先读取仓库入口、目标插件的本地规则及本插件双语说明；修改其他插件前定位它的公开契约，不依赖其私有实现。
2. 使用每个插件实际提供的锁文件及构建入口；独立官网和用户中心保留仓库外构建能力。动态安装包为`.wasm`，不是源码目录或未装配元数据的普通Go WASM。
3. 从仓库根执行`go run ./.agents/tools/plugincheck/main.go`，审计全部清单、双语说明及提供方公开契约，并检查错误元数据会被拒绝。工具只创建自己的临时工作区和日志，不改共享`go.work`或运行实例。
4. 执行`make plugins.check`、对应插件Go测试和`make lint dir=apps/lina-plugins/<id>`；前端行为补相应E2E。上述审计不替代真实接口、资金、权限、打包产物及失败恢复验收。
5. 正式安装、更新仍经过公开生命周期接口和服务端验证器，上传成功不等于安装或更新完成。真实第三方写入必须有针对测试对象的授权。

## 插件索引

| 插件说明 | 运行类型 | 分发 | 声明的业务扩展点 |
| --- | --- | --- | --- |
| [`content-notice`](content-notice/README.zh-CN.md) | `source` | `builtin` | `—` |
| [`customer-default`](customer-default/README.zh-CN.md) | `dynamic` | `managed` | `customer-template` |
| [`customer-tdesign`](customer-tdesign/README.zh-CN.md) | `dynamic` | `managed` | `customer-template` |
| [`idc-ai`](idc-ai/README.zh-CN.md) | `source` | `builtin` | `—` |
| [`idc-baota`](idc-baota/README.zh-CN.md) | `dynamic` | `managed` | `product-fulfillment, resource-provider` |
| [`idc-channel`](idc-channel/README.zh-CN.md) | `source` | `builtin` | `—` |
| [`idc-compliance`](idc-compliance/README.zh-CN.md) | `source` | `builtin` | `—` |
| [`idc-core`](idc-core/README.zh-CN.md) | `source` | `builtin` | `product-fulfillment` |
| [`idc-dcim`](idc-dcim/README.zh-CN.md) | `dynamic` | `managed` | `product-fulfillment, resource-provider` |
| [`idc-notify`](idc-notify/README.zh-CN.md) | `source` | `builtin` | `sms-channel` |
| [`idc-payment`](idc-payment/README.zh-CN.md) | `source` | `builtin` | `payment-method` |
| [`idc-payment-alipay`](idc-payment-alipay/README.zh-CN.md) | `dynamic` | `managed` | `payment-method` |
| [`idc-payment-epay`](idc-payment-epay/README.zh-CN.md) | `dynamic` | `managed` | `payment-method` |
| [`idc-payment-unionpay`](idc-payment-unionpay/README.zh-CN.md) | `dynamic` | `managed` | `payment-method` |
| [`idc-payment-wechatpay`](idc-payment-wechatpay/README.zh-CN.md) | `dynamic` | `managed` | `payment-method` |
| [`idc-peer`](idc-peer/README.zh-CN.md) | `dynamic` | `managed` | `product-fulfillment, resource-provider` |
| [`idc-portal`](idc-portal/README.zh-CN.md) | `source` | `builtin` | `customer-template, website-template` |
| [`idc-resource`](idc-resource/README.zh-CN.md) | `source` | `builtin` | `resource-provider` |
| [`idc-v10`](idc-v10/README.zh-CN.md) | `dynamic` | `managed` | `product-fulfillment, resource-provider` |
| [`idc-zjmf-finance`](idc-zjmf-finance/README.zh-CN.md) | `dynamic` | `managed` | `product-fulfillment, resource-provider` |
| [`idc-zjmfcloud`](idc-zjmfcloud/README.zh-CN.md) | `dynamic` | `managed` | `product-fulfillment, resource-provider` |
| [`idc-zjmfmigrate`](idc-zjmfmigrate/README.zh-CN.md) | `dynamic` | `managed` | `—` |
| [`org-core`](org-core/README.zh-CN.md) | `source` | `builtin` | `—` |
| [`plugin-manager`](plugin-manager/README.zh-CN.md) | `source` | `builtin` | `—` |
| [`shared-services`](shared-services/README.zh-CN.md) | `source` | `builtin` | `—` |
| [`tenant-core`](tenant-core/README.zh-CN.md) | `source` | `builtin` | `—` |
| [`vben-adapter`](vben-adapter/README.zh-CN.md) | `source` | `builtin` | `—` |
| [`website-default`](website-default/README.zh-CN.md) | `dynamic` | `managed` | `website-template` |
| [`website-html`](website-html/README.zh-CN.md) | `dynamic` | `managed` | `website-template` |
| [`website-minimal`](website-minimal/README.zh-CN.md) | `dynamic` | `managed` | `website-template` |
| [`website-qycn`](website-qycn/README.zh-CN.md) | `dynamic` | `managed` | `website-template` |
| [`yunsu-ssl`](yunsu-ssl/README.zh-CN.md) | `dynamic` | `managed` | `product-fulfillment` |
