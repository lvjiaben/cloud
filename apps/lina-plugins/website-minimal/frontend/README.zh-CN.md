# 官网渲染

本插件在`site/`内独立维护`Nuxt`源码、依赖锁、`SSR`渲染器和静态构建，展示标题、介绍和独立导航，不包含首页图片。后台字段在`plugin.yaml`中声明，由`idc-portal`展示。旧`apps/lina-websites`工作区已移除。

构建打包见[插件说明](../README.zh-CN.md)。渲染入口为`site/.output/server/index.mjs`，插件网关为`../gateway/server.mjs`。根`make dev`统一构建并启动各应用服务。

第三方模板可声明`entry`和`public_assets`，交付自己的`HTML`、`JavaScript`和样式，不限定框架。完整契约见`website-html`及后台“官网配置 → 开发文档”。
