# 官网首屏组件样式验收

## 根因与实现

`UX17`补齐两套官网的服务端样式。既有配置只加载重置及站点样式；`Ant Design Vue`组件在服务端注册了样式，却没有把组件缓存中的样式输出到首屏。禁用脚本时按钮、选择器、空态及骨架缺少对应样式；启用脚本后才由浏览器补齐。

默认和简洁模板各自增加`app/plugins/component-styles.server.ts`。每个`Nuxt`请求在自己的`Vue`应用上创建独立组件样式缓存，渲染完成后提取组件库生成的样式，保留独立哈希、优先级和水合缓存标记，通过当前请求的文档头输出。不共享品牌或请求缓存，不接收远程`HTML`，不改现有主题、数据、导航、报价或购物车契约。

## 验证

- `temp/20261021/061200-ssr-css-before-e2e.log`：修复前`TC-1v`明确复现首屏没有组件样式标签。
- `061500-website-component-css-build.log`、`061800-minimal-component-css-build.log`：两套独立类型、语言、页面、搜索优化及服务端构建通过。
- `062000-default-css-e2e.log`、`062200-minimal-css-e2e.log`：两套模板无脚本首屏样式、服务端活动与真实读取失败回归通过，各三项。
- `063000-default-style-regression-e2e.log`、`063300-minimal-style-regression-e2e.log`：两套模板水合后加购、无脚本按钮尺寸及按请求品牌色隔离通过，各三项。
- `062400-default-style-static-build.log`、`062400-minimal-style-static-build.log`及`063100-*-style-package.log`：静态后备构建和独立动态包通过。
- `063300-nightly-test-types.log`：新增及受影响测试类型检查通过。

桌面中文及移动英文截图位于`temp/20261020/185615-public-purchase-website-default-component-styles-ssr-without-script.png`、`185652-public-purchase-website-minimal-component-styles-ssr-without-script.png`、`190025-public-purchase-website-default-component-styles-ssr-mobile-english.png`及`190053-public-purchase-website-minimal-component-styles-ssr-mobile-english.png`，已检查，无截断、重叠和原始语言键。无脚本时互动配置区仍保留原有加载占位，本次不将其宣称为可在无脚本浏览器下单。

## 部署与审查

两套本地模板均为`v0.1.13`，正常启用，渲染进程健康。升级预检仅有版本变化，无新增授权、依赖、业务扩展或数据库操作。`063500-website-style-activate.log`记录本地切换；保留旧哈希资源以兼容已打开页面。`063800-website-live-style-proof.json`确认当前官网商品返回`200`、首屏包含十八个组件样式标签和真实商品名称，未出现模板不可用提示。

按`lina-review`完成本任务范围审查，未发现阻塞问题。命中前端、测试、插件、工具和文档规则；未修改宿主、引入依赖、修改缓存业务真相或后端接口。样式适配依赖当前锁定的组件库缓存与提取协议，组件库升级仍需运行上述服务端及水合回归。仅版本变化不需要重复不相关的后端业务测试。

本次触发`E2E`质量审查，覆盖原首屏缺样式复现、有效尺寸与边框、实际中英文内容、请求间品牌隔离、水合后加购写入结果及无横向溢出。测试各自管理浏览器上下文并关闭，品牌测试在结束时恢复共享公开替身。截图已进行多模态检查。
