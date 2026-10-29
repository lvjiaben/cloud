# website-default

官网源码和构建工具随各官网插件提供，旧`apps/lina-websites`目录已移除。进入`apps/lina-plugins/website-default`（也可将整个插件复制到仓库外），执行：

```sh
make install build
# 将 dist/website-default.wasm 上传到后台插件管理，预览并确认升级。
# 完整SSR模式：先启动本插件的独立渲染服务，再启动官网网关。
PORT=6681 NUXT_API_ORIGIN=http://127.0.0.1:6030 node frontend/site/.output/server/index.mjs
WEBSITE_SSR_ORIGINS='{"website-default":"http://127.0.0.1:6681"}' node gateway/server.mjs
```

以上两个进程分别由进程管理器保持运行；网关默认监听6680。`website-minimal`使用同样命令，可监听6682并加入`WEBSITE_SSR_ORIGINS`映射。网关按后台当前模板选择对应SSR服务，切换后自动路由。模板的SSR服务配置`NUXT_API_ORIGIN`、`NUXT_PUBLIC_SITE_URL`和`NUXT_PUBLIC_CUSTOMER_URL`；网关配置`WEBSITE_API_ORIGIN`和`WEBSITE_CUSTOMER_URL`。不要把用户输入的地址加入SSR上游映射。

没有配置SSR服务时，网关加载动态包内的完整静态应用；商品和内容仍可操作，但属于浏览器渲染，不具备完整SSR搜索收录能力。原生HTML模板直接携带实际HTML页面，使用`make install build`打包即可。后端或WASM安装不会自动启动Node服务。

页面候选来自本插件实际路由，构建会校验声明；导航分组可只展开而不跳转。

官网负责`/products`选配、`/cart`购物车和`/checkout`确认订单。通过隐藏的`/auth/website-session`页面自动识别配置客户服务已有会话；未登录时在当前标签复用完整认证并返回官网结算；网关将`/auth/*`、`/account/*`和支付返回页交给`WEBSITE_CUSTOMER_URL`指定的客户服务，并转发受保护接口，后端继续校验客户归属。会话消息校验配置的客户来源、来源窗口和随机标识，只向配置的官网来源发送，不进入地址。本地回环官网别名使用同一配置客户来源，只接受配置官网及其同端口回环别名，生产来源保持精确匹配。浏览器隔离嵌入式存储时，当前标签自动读取客户已有会话，以表单正文交接至官网网关；网关核对客户来源并向后端验证，只使用读取即清除的三十秒`HttpOnly`交接Cookie，令牌不进入地址。已选配置通过合并后保留的公开编号提交至订单报价项校验，成功后才允许下单。确认订单后才打开客户账单及支付方式弹窗；真实渠道配置和付款结果仍由支付拥有方管理。购买流程需要同时运行官网网关和客户服务，不能只把静态文件放到任意文件服务器。
