# 原生HTML官网示例

本插件自带HTML、JavaScript、CSS、Go标准库Guest和独立打包工具，不依赖仓库源码，也不要求Vue或React。

```sh
make install build
# 上传dist/website-html.wasm，安装或升级后启用，在官网配置选择本插件。
node gateway/server.mjs
```

网关默认监听`6680`，不需要其他渲染服务。使用非默认端口时，在进程环境中设置`WEBSITE_API_ORIGIN`和`WEBSITE_CUSTOMER_URL`。`frontend/public/index.html`和`products.html`为实际页面；插件构建核验导航中声明的HTML文件存在。新增页面时提供实际HTML文件并维护`plugin.yaml`的`pages`。公共资料、独立标题和头尾导航来自公开`brand`接口。分组导航只展开子项不跳转。此示例展示原生接口接入，完整云服务官网使用`website-default`或`website-minimal`。
