# 默认用户中心模板

本插件是独立动态用户中心模板。公共品牌资料（Logo、名称、标题、公司及联系方式）由系统统一保存；主题颜色等模板专属设置分别保存，切换模板不会丢失。

将整个插件目录复制到任意目录后，仅需标准 Node.js 22+、pnpm 10.30.3 和 Go 1.25+：先运行 `pnpm install --frozen-lockfile`，再运行 `pnpm run build` 和 `pnpm run package`，将 `dist/customer-default.wasm` 上传安装。构建脚本只访问本插件目录和公共 npm 注册库，无需宿主源码或仓库工具。

默认前端位于 `frontend/customer/`，携带构建和运行必需的 29 个 Vben 组件包及客户业务页面闭包；依赖清单见 `frontend/customer/vendor/DEPENDENCIES.json`，第三方 MIT 许可保存在 `vendor/vben/LICENSE`。这里只包含必要依赖，不是整个宿主仓库。所有客户业务仍通过已文档化的公开 HTTP 接口访问，副本独立维护。

框架无关网关为 `frontend/server.mjs`：配置 `LINAPRO_BACKEND_PROXY_TARGET` 与 `CUSTOMER_PORT` 后用 Node.js 启动。网关读取当前启用模板，代理公开资源与认证接口，支持刷新深层地址。第三方模板可使用原生 JavaScript、jQuery、Vue、React 等，不绑定 Vben。

安装后在“系统配置 → 用户中心配置”选择并保存。停用模板不删除客户订单、余额、服务或工单。部分宿主版本存在模板停用/卸载的 owner 授权检查缺陷；本插件不修改宿主，也不绕过生命周期检查。
