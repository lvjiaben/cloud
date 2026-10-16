# 官网公开接口说明

默认`Nuxt`官网通过受限的同源`/x/`代理访问`NUXT_API_ORIGIN`（默认`http://127.0.0.1:6030`）。开发和生产使用同一个代理，只接受`HTTP(S)`地址，不开放后台接口。

| 插件 | 公开路径 | 用途 |
| --- | --- | --- |
| `idc-portal` | `GET /api/v1/public/brand` | 当前启用官网的公共信息、渲染器和模板字段 |
| `idc-core` | `GET /api/v1/public/catalog/*` | 已发布商品和购买配置 |
| `idc-core` | `GET /api/v1/public/content/*` | 公开内容和下载信息 |
| `idc-core` | `GET /api/v1/public/commerce/*` | 公开购物车和交接信息 |

完整品牌地址为`/x/idc-portal/api/v1/public/brand`。响应使用统一的`{ code, message, data }`包裹，品牌在`data.brand`。

匿名`POST`仅开放购物车的`carts`、`cart-items`，商品的`purchase-configuration/evaluate`、`purchase-configuration/quote`、`campaign-exposures`，以及内容的`download-tickets`。其余写入方法返回`405`，后台和其他插件路径返回`404`。授权头不透传；购物车所有权继续由后端保护的`Cookie`控制。

管理员在“系统设置 → 官网配置”选择已安装且启用的官网插件，保存公共信息和模板自己的字段。停用插件后不输出其配置。公共页面不缓存品牌投影，保存后刷新即可生效。旧品牌修订接口保留，公开官网只读取新的官网配置。

目前默认官网支持`public-standard-v1`与`public-minimal-v1`，新布局需要实现并注册对应渲染器，不能只声明任意名称。登录、购买和账户交给客户后台。金额使用服务端币种和最小单位字符串，时间为`Unix`毫秒；接口失败时不得猜测价格或用硬编码内容冒充权威数据。
