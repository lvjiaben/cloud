# Nuxt 官网起步模板

这个模板只用于公开官网，不能用于客户后台。复制到独立仓库后，把`@lina/site-contracts`和`@lina/site-sdk`的`workspace:*`替换为平台公布的固定版本。

开发时设置`NUXT_API_ORIGIN`指向本地 LinaPro 后端，设置`NUXT_PUBLIC_SITE_URL`为官网地址。构建产物取`.output/server`和`.output/public`，随后由官方打包工具计算真实文件清单与摘要；不要直接使用示例清单中的零摘要。

官网不得拥有`/admin`、`/api`、`/x`、`/x-assets`、`/auth`、`/account`、`/cart`或`/checkout`路由，也不得保存价格、购物车、客户或支付快照。
