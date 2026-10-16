# 官网插件开发指南

本页描述当前真实实现。官网模板插件负责声明可用渲染器、公开页面和专属配置；商家在“官网配置”选择一个展示，并维护公共资料、头部和底部导航。可安装多个模板，只有保存选中的模板对外展示。

## 一、先跑通现成示例

复制仓库中的 `apps/lina-plugins/website-default/`作为开发参考，换成唯一的插件 `id`。不要直接复制宿主或使用其他插件的私有表。模板是托管动态插件，依赖 `idc-portal`；开发源码、插件产物和网站应用是三个不同交付物。

每个官网插件自带源码、依赖锁、构建配置与资源：默认、文字官网在各自的`frontend/site/`，原生示例的手写源码在`frontend/public/`（随`Git`收录）。复制整个插件到仓库外也能安装构建，不需要仓库私有源码。官网不限定语言或框架。原生 `HTML/JavaScript`、`jQuery`、`Vue`、`React`均可输出静态构建产物。内置 `public-standard-v1`和 `public-minimal-v1`只是参考渲染器；任意模板使用下述 `entry`与公开资源协议，安装后可直接展示。

## 二、插件清单

```yaml
id: my-website
name: 我的官网
version: v0.1.0
type: dynamic
distribution: managed
scope_nature: platform_only
supports_multi_tenant: false
default_install_mode: global
dependencies:
  plugins:
    - id: idc-portal
      version: ">=0.1.11 <1.0.0"
business_extensions:
  - point: website-template
    version: v1
    role: provider
    owner: idc-portal
    methods: [capabilities.describe]
    interface_configuration_cardinality: singleton
    interface_configuration:
      version: v1
      fields:
        - key: herotitle
          type: single_line
          label: { en-US: Home title, zh-CN: 首页标题 }
          maximum_length: 120
    provider_metadata:
      type: website-template-contract
      version: v1
      values:
        renderer: public-standard-v1
        entry: index.html
        spa: true
        pages:
          - { id: home, nameZh: 首页, nameEn: Home, path: / }
          - { id: products, nameZh: 商品, nameEn: Products, path: /products }
```

页面只声明模板已经实现的路由，不能用声明代替实现。最多 `100`页，标识不能重复，不能声明后台、接口或客户私有路径。页面名称支持中文、英文。字段支持单行文字、多行文字、开关和整数，每个模板必须声明`1～30`项；正式清单不接受空`fields`。没有额外设置时可像示例一样声明可选的`herotitle`首页标题，留空则使用公共网站名称。禁止秘密或可执行代码。默认官网专属字段为 `herotitle`、`herodescription`、`heroimage`、`primarycolor`。旧 `menutext`仅为已有配置兼容，新的导航使用结构化 `navigation`。

动态入口参考 `website-default/main.go`：使用本插件`backend/internal/descriptor`标准库实现的公开v1 Bridge协议，并导出正式的内存分配和执行函数；修改插件标识时同步main/backend/plugin.go，不需要导入宿主或portal源码。处理器只接受宿主验证后、绑定版本和声明摘要的 `capabilities.describe`调用，不能自行伪造冻结证据。

## 三、配置与导航接口

所有接口按标准响应包裹返回 `{code,message,data}`。

| 方法 | 路径 | 用途 |
| --- | --- | --- |
| `GET` | `/x/idc-portal/api/v1/admin/site/configuration` | 管理员读取模板候选、页面、公共资料、独立配置和导航 |
| `PUT` | `/x/idc-portal/api/v1/admin/site/configuration` | 保存并选择展示模板；需要 `idc:admin:portal:brand`权限 |
| `GET` | `/x/idc-portal/api/v1/public/brand` | 匿名读取当前展示模板的公开资料及头尾导航 |

读取管理接口时，`plugins[]`仅含已安装启用并接受新配置的模板，项中有 `pluginId/name/schema/renderer/pages`。保存不接受客户端自行提交候选或任意页面路径；后端重新确认当前插件生命周期和声明。

```json
{
  "selectedPlugin": "website-default",
  "common": {
    "siteName": "我的云服务", "siteTitle": "云服务",
    "companyName": "示例公司", "contactEmail": "help@example.com",
    "contactPhone": "", "logoUrl": "/logo.png", "description": "云服务平台"
  },
  "config": { "herotitle": "稳定的云服务", "primarycolor": "#1677ff" },
  "navigation": {
    "header": [
      { "id": "home", "parentId": "", "nameZh": "首页", "nameEn": "Home", "pageId": "home", "url": "", "sortOrder": 0 },
      { "id": "login", "parentId": "", "nameZh": "登录", "nameEn": "Sign in", "pageId": "customer-login", "url": "", "sortOrder": 1 }
    ],
    "footer": [
      { "id": "docs", "parentId": "", "nameZh": "文档", "nameEn": "Docs", "pageId": "", "url": "https://example.com/docs", "sortOrder": 0 }
    ]
  }
}
```

头尾各最多 `100`项、三级。可设置`group:true`作为仅展开子菜单的分组，分组的`pageId`和`url`均留空；官网点击分组不跳转，空分组暂不展示。上级项必须存在，不能循环。模板页面以 `pageId`选择，外链使用 `url`且必须是完整 `http/https`地址。会员登录、注册使用 `customer-login`和 `customer-register`，官网渲染器通过部署配置的客户站点地址生成链接。外部链接新窗口打开，不附带令牌或客户资料。

公开 `data.brand`中，`navigation`是头部，`footerNavigation`是底部；每项为 `{label:{enUS,zhCN},href,group,customer,external,children}`。按 `children`递归渲染；`group=true`使用可键盘触发的按钮切换展开，不渲染虚假链接；`customer=true`时将固定客户路径和配置的客户站点地址拼接，`external=true`使用安全外链，否则走官网路由。空数组表示不显示，不自行塞入默认导航。切换模板后不支持的页面不会对外输出，管理员需重新选择有效页面。

## 四、公开商品与内容接口

官网只使用匿名白名单，不持有后台令牌。当前实现包括 `GET /x/idc-core/api/v1/public/catalog/products`、商品详情及 `purchase-configuration`、`public/content`和 `public/commerce`。开发文档中的接口索引直接读取当前服务`/api.json`，可按路径搜索并查看请求、响应字段；完整OpenAPI也可新窗口打开或下载。同源代理和部署边界见本文“官网应用部署与调试”及根安装说明。货币金额沿用服务返回的最小单位字符串，不能在页面重新算出一个价格代替真实报价。购买跳转客户应用完成身份、结账和付款。

## 五、构建、安装与升级

1. 在插件目录运行`make install build`，产物为`dist/{pluginId}.wasm`；所有依赖和打包工具均在本插件内，可复制到仓库外执行。Nuxt模板同时构建`frontend/site/.output`完整SSR与动态包内静态资源。静态资源基址直接读取本插件清单的ID和版本，避免旧缓存混用。
2. 在“插件管理”上传产物。正式上传接口为 `POST /api/v1/plugins/dynamic/package`，表单文件字段 `file`；管理员必须有安装权限。
3. 查看依赖 `GET /api/v1/plugins/{id}/dependencies`，安装 `POST /api/v1/plugins/{id}/install`（安装模式 `global`），再启用 `PUT /api/v1/plugins/{id}/enable`。
4. 返回“官网配置”刷新，选模板、填写资料、保存。不要通过修改数据库将模板伪装成已安装。
5. 升级时提高版本并重新构建上传，先 `GET /api/v1/plugins/{id}/upgrade/preview`，再 `POST /api/v1/plugins/{id}/upgrade`；上传新包不等于升级完成。
6. 停用 `PUT /api/v1/plugins/{id}/disable`、卸载 `DELETE /api/v1/plugins/{id}`仍由正式生命周期治理。当前选中模板不可用时，公共品牌返回空，不能偷偷回退到其他模板。选择其他模板后再移除旧模板更便于运营。

当前运行环境若影响检查返回`PLUGIN_BUSINESS_IMPACT_CHECK_FAILED`及`ownerPluginId=unknown`，表示宿主冻结的扩展授权快照缺失，操作会在模板回调前被拒绝。此时保留已安装模板，先切换展示；不要删表、改声明或绕过影响检查。本轮已验证正式上传、安装、启用、升级和切换，受此宿主问题影响的停用/卸载尚不能保证完成。

## 六、官网应用部署与调试

默认与文字模板进入插件目录执行`make install build`。生产SSR启动`PORT=6681 node frontend/site/.output/server/index.mjs`，再用进程管理器运行`gateway/server.mjs`。网关通过部署方的`WEBSITE_SSR_ORIGINS`映射选择当前模板服务，例如`{"website-default":"http://127.0.0.1:6681"}`；文字模板可独立监听6682并加入映射。完整SSR源码和部署产物均在各插件内，宿主不自动启动Node。

SSR进程使用`NUXT_API_ORIGIN`、`NUXT_PUBLIC_SITE_URL`和`NUXT_PUBLIC_CUSTOMER_URL`；网关使用`WEBSITE_API_ORIGIN`、`WEBSITE_CUSTOMER_URL`，默认监听6680。网关只转发匿名官网白名单及正式静态资源，后台路径不开放。没有SSR映射时展示WASM自带的完整浏览器应用，这种模式不能宣称具备逐请求SSR。原生HTML示例不需要额外SSR进程。

构建会逐项核对`pages`：Nuxt模板必须有实际页面路由，HTML模板必须有实际输出文件。后台下拉展示名称和实际路径，例如`商品 (/products.html)`；不能只声明一个不存在的行业页面。浏览器查看公开brand确认模板及导航，查看网页源代码确认SSR文字已输出。

配置文件保存在 `LINA_SITE_STORAGE/website-configuration.json`。当前使用原子替换和单实例锁，多管理实例需要共享存储并另行协调并发写入；此能力不宣称已有多实例并发配置合并。


## 七、任意框架静态入口（推荐）

原生示例是`apps/lina-plugins/website-html/`，不依赖`Vue`、`React`或`Vben`。该原生示例将手写`index.html`、脚本和样式放在可收录的`frontend/public/`，清单如下。框架项目可改为自己的真实构建目录，但不要把唯一的手写源码放入被忽略的`dist`：

```yaml
public_assets:
  - source: frontend/public
    mount: /
    index: index.html
# 合并到上述 provider_metadata.values，而不是另建同名字段。
# entry: index.html
# spa: true
```

在`provider_metadata.values`使用`entry: index.html`和`spa: true`，此时不需要`renderer`。后台根据已冻结的插件标识与发布版本生成`templateEntry: {url,assetBase,spa}`，不是由浏览器提交任意入口。页面声明、公共信息、独立字段与导航仍沿用前述接口。公开`brand.templateConfig`包含本模板已声明的配置字段。

构建产物使用相对资产路径，或将构建工具`base`设置为正式的`/x-assets/{pluginId}/{version}/`。官网网关插入资源`base`和`window.__LINA_TEMPLATE__.customerUrl`；业务请求必须以`/x/`开头走同源接口，不能相对`base`拼接。`spa: true`让站内深链返回入口；多页站点设`false`，实际访问`/help.html`会读取公开资产中的`help.html`。禁止使用后台路径作为模板页面。

模板无需接触后台令牌。插件资源由正式`public_assets`公开，官网网关只加载服务端当前选中模板的入口，不读取其他插件私有文件，不使用`iframe`。`POST`、`PUT`等业务请求继续走各业务接口，不会落入模板入口。

静态`HTML`本身会由服务器返回；异步加载的数据若需要搜索引擎完整抓取，应提供预渲染的`HTML`。需要真正逐请求`SSR`的框架可独立部署应用，通过反向代理消费相同接口；目前动态插件内不执行任意第三方`Node/PHP`服务。


## 整仓启动与开发交接

新人安装和六进程统一启动见根[安装说明](../../../../INSTALL.zh-CN.md)，源码地图与`AI`协作示例见[开发入口](../../../../DEVELOPMENT.zh-CN.md)。根`make dev`构建后统一运行官网和用户中心；无需手动重复启动已有端口。后台选择模板不会代替部署`SSR`渲染进程。
