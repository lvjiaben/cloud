# LinaPro 客户后台

`@lina/web-customer`是独立的客户侧 Vben 应用。它与`@lina/web-antd`同级，不是管理后台中的普通页面，也不是 Nuxt 官网。

本应用以`apps/lina-vben/apps/web-antd/`为母版。Vben、Vue 3、Vite、Ant Design Vue、`src/`目录、启动装配、路由、请求、状态、偏好、主题、布局、国际化、组件、页面容器、类型检查和构建写法必须持续与`web-antd`保持一致。

只允许包名和应用标识、开发端口`6032`、存储命名空间、客户业务路由、客户业务接口、品牌值、客户页面和构建产物边界不同。接受结构变更前，必须在本目录运行`pnpm check:alignment`。

公开官网不放在这里。官网开发、默认 Nuxt 官网、`SDK`、契约、起步模板、文档和可安装官网包统一属于`apps/lina-websites/`。
