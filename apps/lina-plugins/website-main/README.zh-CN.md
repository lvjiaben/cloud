# 官网

`website-main`是从`codex/官网交付`的`240aec4`选择性迁入的独立动态官网插件。当前本地集成版本为`v0.4.3`，保留原页面、素材及中英文适配，通过当前`idc-portal`、`idc-core`和`content-notice`公开接口工作。分支中的旧业务代码、其他模板、模拟商品及历史工作区没有整体合入。

全部455个原有页面、样式、图片和字体保持分支原字节；新增购物车复用原配置页壳和样式，不另设样式表。

官网负责商品选配、逐台登录信息、购物车、核价和订单确认；登录复用现有客户中心；结算在官网内选择付款方式，调用当前账单与支付接口完成付款。提交结果未知时查询权威状态，不重复创建付款。商品统一固定显示`NEW`，无后台标记开关。公开品牌使用公共配置，支持独立头尾Logo、联系方式和备案资料，后台关闭的登录输入不会显示，密码不进入普通配置或浏览器持久存储。

使用`Go 1.25+`、`Node.js 22.6+`和锁定的`pnpm 10.30.3`：

```text
pnpm install --frozen-lockfile
pnpm package
pnpm build:files
pnpm test
```

`pnpm package`输出`dist/website-main.wasm`；普通文件构建输出`dist/site/`，目标已存在时需选择新的`-output`目录。运行构建产物测试时，将`WEBSITE_SITE_OUTPUT`设为普通文件输出目录、`WEBSITE_PUBLIC_OUTPUT`设为其`public`子目录，`WEBSITE_API_ORIGIN`指定已启动的本地后端。浏览器测试使用插件`hack/tests/playwright.config.ts`，核心本地适配用例为`TC156`、`TC157`、`TC100`和`TC133`。

通过现有插件管理上传、安装和启用，再从官网配置选择模板。共享品牌、原模板设置和业务数据保留。项目开发使用既有`make dev`统一管理服务；独立部署见[部署说明](DEPLOYMENT.zh-CN.md)。

本地实际验收和分支冲突取舍见仓库`openspec/changes/integrate-website-main-current-commerce/`。`docs/`中的历史记录来自原分支及另一环境，仅用于追溯，不表示本地运行或验收状态；当前集成说明优先。原许可、版权和来源记录保留，见[资源许可](docs/source-and-license.md)。
