# 官网服务器选配反馈实施与审查

## 范围与业务结论

本轮按`lina-feedback`归入现有商品变更，核对用户提供的小白云页面及三张配置截图，使用`frontend-design`完善现有官网风格。修改范围为`idc-core`的购买契约、后台登录规则、购物车及履约交接，`idc-resource`的服务器登录信息消费，`idc-zjmfcloud`的商家开关声明，以及三个官网插件。没有修改宿主源码、提交、推送或归档。

用户确认已记入[商品业务偏好](../../../../.agents/memory/preferences/75-product-commerce.md)：允许自填主机名、密码，留空自动生成；一台填完加入购物车，再单独配置下一台。每份自填信息绑定自己的单台项目，编辑时空密码保留已有秘密，不生成主机名后缀。

## 根因与实际改动

- 原官网把全部操作系统铺成通用选项，区域没有独立表现，部分有界数量默认仍是输入框。现在公开契约明确操作系统与区域的展示语义，官网使用发行版卡片加版本选择、带图标区域卡片、范围滑条加精确输入及加减。保留商家指定的输入方式及原始候选标识，分类不参与收费。
- 后台原主机名、密码开关只是展开生成规则的前端状态，没有保存客户填写权限。现在分别保存真实商家开关，官网按当前商品权限显示输入，并提供密码显隐、随机生成。
- 原链路只有自动生成信息。现在通过独立登录凭据接口加密保存密码，以不透明引用关联本人、商品修订、单个购物车项目及成交快照；履约消费原主机名与原秘密句柄，重试不另生成。密码不进入通用购买值、购物车明文或浏览器存储。
- 实际商品验收补修了两处兼容问题：已物化镜像仍能识别展示语义；旧商品评估使用兼容检查后的当前声明，新增可选开关不会误报未知字段。截图复查补修了`TencentOS`不能被子串误识别为`CentOS`，保留未知系统可选。
- 沿用自动核价、清晰选中态和订单摘要。价格失败保留选择并允许重试；已提交凭据在购物车失败后可复用，编辑和重新核价保留已有秘密。

## 验证证据

证据根目录为`temp/20261028/`，文件均为本次执行产物。

| 验证 | 结果与主要日志 |
| --- | --- |
| 核心、资源、连接器相关单元测试及范围检查 | 通过，见`purchase-core-final-tests.log`、`purchase-resource-final-tests.log`、`purchase-connector-tests.log`及对应`lint`日志 |
| 登录信息数据库回归 | 通过，见`server-login-postgres.log`、`purchase-login-password-safety-postgres.log`；覆盖身份、修订、并发、回滚、空值生成及错误不回显密码 |
| 购物车、冻结与重新核价数据库回归 | 通过，见`purchase-cart-postgres.log`、`purchase-login-requote-postgres.log`；覆盖单台绑定、重试去重、编辑保留、追加快照及重新核价 |
| 资源交付数据库回归 | 通过，见`purchase-provision-postgres.log`；原主机名和秘密进入持久开通任务，重试保持身份 |
| 真实商品兼容回归 | 通过，见`purchase-materialized-presentation-tests.log`、`purchase-current-declaration-target-postgres.log`、`220200-live-login-evaluation.json` |
| 群英官网页面回归 | `34`项通过，见`230000-qycn-final-e2e.log`；包含`TC014`、配置自动核价、购物车及导航回归 |
| 默认、极简官网页面回归 | 最终`TC006`各`9`项通过，见`163500-website-default-centos-boundary-e2e.log`及极简对应日志；包含系统分类边界与准确报价标识 |
| 真实预览代理 | `13`项通过，见`191000-qycn-proxy-tests.log`；实际请求白名单、认证及拒绝路径 |
| 后台实际开关保存 | 通过，见`220500-live-admin-login.log`；实际页面勾选并保存，接口回读两项布尔值为真 |
| 已部署官网与真实购物车 | 通过，见`225000-live-purchase-cart.log`及`225000-live-purchase-result.json`；连续填写两台不同主机名，各自数量一、各自凭据引用，购物车与浏览器存储无测试密码明文 |

真实购物车验收使用原商品的独立本地副本。只为副本补齐原先缺失的日付配置价格，未改原商品售价或设置。测试项目已从购物车删除，副本已通过商品删除接口清理；没有付款或调用第三方开通。参考站仅只读访问。

数据库迁移`157-purchase-server-login.sql`已在本地执行两次确认幂等，相关`DAO`、`DO`、`Entity`和控制器通过正式生成流程产生。主机密码沿用资源拥有方共享的`secretcap`实例，构造从插件装配进入登录信息服务与履约消费者，未创建独立秘密存储。

## 最终交付

当前后端及后台已构建并部署，日志为`220000-server-login-backend-build.log`、`200000-server-login-admin-build.log`。连接器已升级`idc-zjmfcloud v0.3.21`；三个官网最终为`website-qycn v0.1.18`、`website-default v0.1.20`、`website-minimal v0.1.20`，升级日志为`232000-selector-upgrade.log`、`234500-selector-upgrade.log`。默认和极简的最终`SSR`与静态构建分别见`163300-website-*-centos-boundary-ssr.log`、`163500-website-*-centos-boundary-static.log`。

两套部署`SSR`目录各`276`文件、零符号链接，使用既有运行器重启成功；三个官网实际商品页均返回`200`，见`235000-website-runtime-check.json`。当前使用的群英模板未切换，实际商品的原设置和价格未修改。两个受影响变更的严格规范校验及本轮文件格式检查通过。

## 审查结论与限制

按`lina-review`审查本次源码、生成产物、规范和任务；已触发`E2E`质量审查，因为涉及后台开关、官网表单、失败恢复及购物车结果。双语用例明确断言目标语言文案、提交的真实候选标识、密码不落普通请求及存储、失败后选择保留、独立项目及多台编辑拒绝。已人工查看桌面、手机和失败状态截图；实际部署最终截图见`233000-live-purchase-final.png`，分类回读及测试商品清理见`233000-live-selector-screenshot.log`。未发现阻塞问题。

保留以下范围外验证限制，不将其记为通过：

- 全仓`make i18n.check`仍被既有`plugin-manager`硬编码文案阻断，见`192500-selectors-i18n.log`；本次运行文案覆盖检查通过，新增官网组件没有该扫描问题。
- 核心整包启动边界测试受共享工作区另一份`idc-resource`测试跨插件私有导入阻断，见`server-login-startup.log`。本次账号控制器编译、正式后端构建及真实启动路由已通过。
- 较宽的既有商品配置数据库测试有两项发布／准入断言失败；跳过新增子用例后仍能复现，见`purchase-evaluation-independent-existing-checks.log`。本次新增旧商品开关兼容子用例独立通过。
- 真实付款及第三方服务器采购不属于本次参考与本地验收授权。成交到持久开通参数的证据来自隔离数据库自动化测试，不宣称已实际创建服务器。
