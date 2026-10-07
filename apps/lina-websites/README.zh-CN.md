# Lina 官网包

本工作区完全独立于`apps/lina-vben/`。这里保存默认公开官网、稳定官网契约、开发`SDK`、起步模板和中英文开发文档，用于制作可安装的官网包。

客户后台固定保留在`apps/lina-vben/apps/web-customer/`，并遵循`web-antd`的 Vben 结构。官网包只拥有公开获客和内容路由；购物车、登录、结算、支付、账户和售后通过后端权威的不透明引用交给客户后台。

官网开发者完成构建后交付自包含的预编译`.zip`。`IDC`商家在自己的管理后台上传，当前安装实例在本地完成校验、暂存、预览、发布、切换和回滚，不重新编译 LinaPro。

当前`sites/default/`源码来自此前错误合并的 Nuxt 原型，只作为迁移基线保留，不代表最终官网包契约或客户页面拆分已经完成。

开发入口：

- [第三方官网开发说明](docs/development.zh-CN.md)
- [公开接口说明](docs/api.zh-CN.md)
- [官网压缩包格式](docs/package-format.zh-CN.md)
- [Nuxt 起步模板](templates/nuxt-starter/README.zh-CN.md)
