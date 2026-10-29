# 官网报价费用明细验收

`UX13`修正两套官网配置器只展示周期标签和总额、丢弃已有费用明细的问题。陕西商品的实际只读报价为本次`CNY 11.00`，原页面未解释其中的一次性费用，容易被理解成每月收费。

页面现直接展示报价拥有方返回的商品与配置费、一次性费用、税费和本次合计；税费明确标注合计已包含，含税价格不会被浏览器再次加税；最少购买多个周期时显示本次包含的周期数。没有在浏览器计算售价、税额或续费价，也没有给公开商品目录增加未授权的价格投影。需要客户身份才能确认的条款只显示原登录核价说明及入口，避免将未确认的零金额显示为已确认报价。旧响应缺少明细时不会编造零费用。

配置变更继续撤销旧报价，购物车仍按所选条款的服务端周期和修订重新校验，下单、收款与开通行为均未改变。两个独立模板保留各自源码、构建、资产与路由。

证据位于`temp/20261021/`：

- `012300-website-default-quote-ssr-build.log`及`012300-website-minimal-quote-ssr-build.log`：初步类型、双语、页面、SEO及SSR构建通过。修正文字模板原先不同的标题键后通过，没有以类型错误构建作为验收依据。
- `012500-default-quote-e2e.log`、`012500-minimal-quote-e2e.log`各四项通过。截图发现嵌套报价卡继承了商品详情的大内边距，窄屏币种与小数被挤压换行；已在组件内部限定报价卡内边距、金额宽度，并新增短金额单行断言。
- 最终`013000-default-quote-readable-ssr.log`、`013000-minimal-quote-readable-ssr.log`含前端类型、双语、SEO、页面及SSR构建通过；同前缀`static.log`和`package.log`完成两个独立静态包。
- `013200-default-quote-readable-e2e.log`、`013200-minimal-quote-readable-e2e.log`各四项通过，覆盖CNY费用`1.00＋10.00＝11.00`、超安全整数的JPY精确值、KWD三位小数、三个购买周期、中文/英文、窄屏金额可读性及未确认价格保护。测试拦截写入路径并断言没有购物车写入。
- `012800-default-quote-regression-e2e.log`三项、`012800-minimal-quote-regression-e2e.log`两项通过，保留商品接管及隔离加购、身份核价恢复和配置改变使报价失效的业务断言。
- `013000-product-quote-final-test-types.log`、`013200-website-quote-i18n.log`、`013000-website-quote-openspec.log`分别通过测试类型、完整运行时多语言及严格规范验证。
- 升级预览确认只改变模板版本，无SQL、依赖阻断、权限扩大或扩展声明变化。初步模板升级后再以真实陕西报价核对含税展示，最终两个模板`v0.1.11`已启用正常，见`014400-website-default-installed-proof.json`、`014400-website-minimal-installed-proof.json`。独立SSR进程已从最终构建切换并通过健康检查，旧哈希静态资源保留，网关及官网模板选择保持原值。

最终`014000-*-quote-tax-ssr.log`、`014000-*-quote-tax-static.log`、`014000-*-quote-tax-package.log`完成两套模板构建。`014200-default-quote-tax-e2e.log`、`014200-minimal-quote-tax-e2e.log`各四项通过，CNY场景进一步核对商品与配置`1.00`、一次费`10.00`、已计入税费`0.11`和原总额`11.00`；KWD场景保留另一税费构成，由服务器决定总额，页面不求和。最终类型、多语言和规范验证分别见`014200-website-tax-final-test-types.log`、`014200-website-tax-final-i18n.log`、`014200-website-tax-final-openspec.log`。

本次同时将相关官网测试截图路径修正为从用例文件定位项目根目录，保留当前日期，避免依赖运行目录和历史固定日期。

按`lina-review`审查两个配置器、两个清单、测试及截图辅助与增量规范。复用前端、测试、多语言、插件、金额及官网视觉规则。触发并完成`E2E`质量审查：断言明确核对费用各字段、精确格式、周期数、目标语言、隐藏未确认金额及零写入；关键桌面、窄屏截图已视审。没有修改业务API、数据库、缓存、付款逻辑、实际商品或宿主。未发现阻塞问题，真实收款与资源创建没有执行。

最终含税窄屏截图已视审：`temp/20261020/165338-public-purchase-website-default-quote-details-mobile-CNY.png`；此前英文KWD和文字模板JPY桌面及窄屏亦已核对。实际Chrome只读选择陕西镜像并请求报价，商品费`CNY 1.00`、一次费`CNY 10.00`、税费`CNY 0.11`及本次总额`CNY 11.00`来自真实拥有方；未加入购物车、未下单或付款。
