# 官网渲染

`apps/lina-websites/sites/default`中的共享`SSR`通过`public-minimal-v1`渲染本模板：标题、介绍和独立导航，不包含首页图片。后台字段在`plugin.yaml`中声明，由`idc-portal`展示。

第三方模板可在清单声明 `entry`和`public_assets`，安装并启用后由官网入口加载自己的 `HTML`、`JS` 和样式；不限定框架。原生示例见`website-html`，完整接口和流程见后台“官网配置 → 开发文档”。
