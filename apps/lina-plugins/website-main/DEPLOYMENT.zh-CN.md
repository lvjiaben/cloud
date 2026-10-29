# 官网部署

先执行`pnpm package`构建`dist/website-main.wasm`，通过现有插件管理上传、安装及启用，不加载模拟数据。在官网配置选择模板，保留原共享资料和旧模板设置。当前要求`idc-portal >=0.1.14`及`idc-core >=0.1.0`，公告页面使用现有`content-notice`公开接口。

独立普通文件交付使用`pnpm build:files`，或使用`go run ./hack/tools/build/main.go -output <新目录>`。目录包含页面、网关和资源校验清单，不包含宿主、业务后端、测试数据或插件二进制；发布`WASM`与普通文件分别验证。运行`node gateway/server.mjs`前设置`WEBSITE_API_ORIGIN`和`WEBSITE_CUSTOMER_URL`，可用`HOST`、`PORT`指定监听地址。

客户中心必须信任实际官网来源，登录使用已有会话桥；代理只接受当前官网办理所需的公开读取和已认证账户交易契约。订单确认后进入客户中心账单，不能在官网另建付款状态。项目开发由`make dev`统一管理，不能长期依赖临时预览进程。

公开品牌首屏缓存只保存公开身份资料，默认在`.public-brand-cache`，可用`WEBSITE_BRAND_CACHE_DIR`指定共享可写目录；有效期三十秒、每十秒刷新，失效时仍提供正常页面。不得存放凭据或私有业务数据。

安装和运行验证以当前集成记录为准；不覆盖旧版本包，不清理业务数据或修改宿主。许可来源见[资源许可](docs/source-and-license.md)。
