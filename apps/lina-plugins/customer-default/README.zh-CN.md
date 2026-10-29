# 默认用户中心模板

本插件是独立动态用户中心模板。公共品牌资料（Logo、名称、标题、公司及联系方式）由系统统一保存；主题颜色等模板专属设置分别保存，切换模板不会丢失。

将整个插件目录复制到任意目录后，仅需标准 Node.js 22+、pnpm 10.30.3 和 Go 1.25+：先运行 `pnpm install --frozen-lockfile`，再运行 `pnpm run build` 和 `pnpm run package`，将 `dist/customer-default.wasm` 上传安装。构建脚本只访问本插件目录和公共 npm 注册库，无需宿主源码或仓库工具。

默认前端位于 `frontend/customer/`，携带构建和运行必需的 29 个 Vben 组件包及客户业务页面闭包；依赖清单见 `frontend/customer/vendor/DEPENDENCIES.json`，第三方 MIT 许可保存在 `vendor/vben/LICENSE`。这里只包含必要依赖，不是整个宿主仓库。所有客户业务仍通过已文档化的公开 HTTP 接口访问，副本独立维护。

框架无关网关为 `frontend/server.mjs`：配置 `LINAPRO_BACKEND_PROXY_TARGET` 与 `CUSTOMER_PORT` 后用 Node.js 启动。网关读取当前启用模板，代理公开资源与认证接口，支持刷新深层地址。第三方模板可使用原生 JavaScript、jQuery、Vue、React 等，不绑定 Vben。

安装后在“系统配置 → 用户中心配置”选择并保存。停用模板不删除客户订单、余额、服务或工单。部分宿主版本存在模板停用/卸载的 owner 授权检查缺陷；本插件不修改宿主，也不绕过生命周期检查。

选配、购物车和结算属于官网，客户购买旧入口会跳转`WEBSITE_PUBLIC_ORIGIN`配置的官网，默认本地地址为`http://localhost:6680`；官方开发启动自动传入实际官网端口。`/auth/website-session`通过隐藏页面将已有会话仅交接给配置的官网来源；需要登录时使用当前标签，完成后由`/account/website-purchase`返回校验过的官网地址。确认订单后在客户账单中选择在线支付方式，支持已配置渠道的二维码和页面跳转；未实名、已取消或不可付款订单继续由原有门禁阻止付款。

认证交接是框架无关的模板协议：`/auth/website-session?nonce=...`只向`/__website/configuration`确定的官网来源发送`{type:"lina-website-session",nonce,accessToken}`。官网核对配置的客户来源、隐藏窗口和随机标识。`login=1&returnTo=...`表示当前标签发起完整客户登录，随后由`/account/website-purchase`返回配置的官网地址，令牌不进入地址。`probe=1`只在当前标签检查已有会话，不发起登录，再以表单正文交接到允许的官网网关，覆盖浏览器隔离嵌入式存储的情况。配置返回`origins`，仅本地回环官网增加同端口回环别名，生产来源保持精确匹配。第三方模板可以用任意框架实现，无需共享私有存储。

常用界面图标（Lucide、Fluent MDL2、Element Plus）随模板打包，无需连接外部图标接口。原始许可与来源保留在 `frontend/customer/public/icon-licenses.txt`，构建后同样随模板提供。

控制台分别加载全部已购服务、云服务器、最近订单、余额和待付账单，单个区域失败可以单独重试。服务总数和前五项预览来自所属客户的分页接口，完整服务入口为`/account/product-directories`，不依赖服务器插件；购买入口面向所有产品。`/account/servers`保留资源拥有方的完整分页列表，直接进入已交付服务器的状态、地址、电源、续费和显式密码查看；其他服务器服务保留独立入口。资源插件停用时隐藏对应服务器入口，普通已购服务继续可用。

订单、控制台和全部服务列表分别展示本地管理入口准备情况与真实交付、生命周期。管理入口尚未准备好或需人工处理时，保留原已付款服务和订单，服务器管理入口等待关联确认；没有此类通知的历史服务沿用原入口。进度展示不会授权重新开通或退款。
