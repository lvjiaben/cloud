# 第十五阶段交付验证记录

## 本轮完成

- `apps/lina-websites/` 独立 workspace 的默认官网、契约包、SDK、Nuxt 起步模板和中英文文档均可独立构建。
- 默认官网通过 Nuxt 服务端渲染、双语资源、`robots.txt`、`sitemap.xml`、规范链接和路由隔离检查。
- `apps/lina-vben/apps/web-customer/` 通过 Vben 母版结构、独立端口与存储命名空间、双语资源、类型检查和生产构建检查。
- 新增 `TC025-account-center`，覆盖客户登录后统一分组导航、客户概览、英文移动端和横向溢出检查；两项测试均通过。
- `site-contracts` 新增纯函数清单校验器，拒绝客户/管理/API 路由、路径穿越、重复文件、错误摘要、版本和运行时字段，可供后台上传门禁与第三方打包器复用。
- `@lina/site-packager` 提供本地受治理命令：官网包安装、预览、发布、回滚、卸载，以及品牌配置（站点名、标识、配色、联系方式、导航、页脚、条款和 SEO）按修订保存、发布和回滚。命令只接受结构化 JSON 和预编译包，不执行代码片段。
- `idc-portal` 后台品牌页已补齐首页区块编辑：每个区块只允许双语标题、双语说明和可选站内公开路径，最多 12 条；保存、预览、发布和回滚复用同一严格校验，拒绝管理端、接口、客户中心路径及代码片段；默认官网 SSR 会按已发布修订输出区块、SEO 和条款链接。
- `idc-portal` 的密码找回桥接已完成并通过真实后端验证：精确 `GET /admin/auth/reset-password` 返回 `302`，只保留一个非空 `token`，目标固定为配置的客户中心 `/auth/reset-password`，请求中的 `redirect` 等参数不会影响目标地址。
- `web-customer` 的统一身份链路已完成复核：宿主登录与注册、真人验证和验证码、一次性开户意图创建/兑换、可选外部登录单次交接、刷新令牌、退出、会话失效清理和安全深层路由恢复均已接入 Vben 请求与状态层。

## 仍需单独实现的后台能力

官网包 HTTP 上传入口仍可直接复用上述 CLI 的校验与原子指针；图形化品牌表单及首页区块发布能力已在 `idc-portal` 插件内完成，本记录不把静态前端构建误当成动态官网包发布能力。

## 验证命令

```text
pnpm --filter @lina/site-contracts build
pnpm --filter @lina/site-sdk build
pnpm --filter @lina/site-default build
pnpm run check:alignment
pnpm run i18n:check
pnpm run typecheck
pnpm run build
pnpm exec playwright test tests/e2e/TC025-account-center.ts --config tests/playwright.config.ts
pnpm exec playwright test tests/e2e/TC023-customer-onboarding.ts tests/e2e/TC025-account-center.ts tests/e2e/TC026-route-contract.ts --config tests/playwright.config.ts
```

密码找回桥接验证：

```text
（在 `apps/lina-plugins/idc-portal` 目录执行）`go test ./backend/... -count=1`
GET http://127.0.0.1:19120/admin/auth/reset-password?token=abc&redirect=https://evil.test
=> 302 Location: https://127.0.0.1:16677/auth/reset-password?token=abc
```

客户身份与路由定向复核结果为`9 passed`；相关请求、开户、外部登录错误映射和路由恢复单测为`32 passed`。本轮没有修改宿主核心、腾讯电子签或诺诺适配器，也没有提交、推送或创建 PR。

客户后台全量 Chrome 验证结果为`102 passed / 1 skipped / 0 failed`；唯一跳过项是需要真实已安装系统和签名环境的充值用例，不影响本地可验证的客户概览、购物车、结算、交易、财务、服务器、合规、工单、通知、推荐、佣金、双语桌面与移动端、错误边界和模块降级。默认官网完成类型、双语、SEO、Nuxt 生产构建及`23 passed`的 Chrome 验证。基于这些证据，任务`15.5`和`15.6`已关闭。

官网购买交接补齐后，`idc-core`公开接口只返回不透明购物车编号，256 位游客所有权令牌由后端写入`HttpOnly + SameSite=Lax` Cookie，HTTPS 下同时启用`Secure`；令牌不进入 URL、日志字段、普通响应或 JavaScript 存储。客户登录进入购物车时调用无令牌参数的账户接口，由后端读取 Cookie 并复用既有事务合并，成功或凭据失效后清除 Cookie，币种冲突则保留凭据和既有后端失败语义。

验证结果：Cookie 策略单测`2 passed`，公开/账户控制器和交易服务测试通过；官网类型、双语、SEO、生产构建通过，`TC012-public-purchase`为`1 passed`；客户 API 单测`2 passed`、类型检查通过，`TC027-cart-handoff`与`TC010-commerce-checkout`合计`5 passed`。因此任务`15.8`及其购买子任务已关闭。全后端回归唯一失败来自尚未完成的`idc-channel`公开/客户接口错误声明管理员权限，归后续第 17 阶段处理，与本轮购物车改动无关。

响应式购买追加验证后，官网`TC012-public-purchase`为`2 passed`，覆盖英文桌面安全交接和中文移动配置、核价、加购、无横向溢出；客户后台`TC027-cart-handoff`为`2 passed`，覆盖登录后安全合并以及中文移动核价、结账和付款准备。结合全量客户后台中`TC025`客户概览、`TC015`服务器、`TC011`账单、`TC001`工单与通知的既有通过结果，任务`15.9`和`15.10`已关闭。

补充验证：`idc-portal` 后端测试、`web-customer` 类型检查和生产构建均通过；官网 `TC001-public-site` 为 `2 passed`，客户开户 `TC023-customer-onboarding` 为 `4 passed`，路由边界 `TC026-route-contract` 为 `3 passed`。

## 前后台运行壳隔离验证

新增客户后台模块连续编号`TC028-shell-isolation`和独立双服务配置，同时启动`web-customer`的`6032`入口与`web-antd`的`6031/admin/`入口。测试在同一个真实 Chrome 上下文中完成客户与管理员登录，并分别下发互斥运行设置：客户中心使用顶栏、深色主题和客户水印，管理后台使用侧栏、浅色主题并关闭水印。

测试还在两个入口加载前分别写入对方应用命名空间的主题、水印和布局“干扰值”。最终客户中心仍保持`lina-web-customer-*`偏好、顶栏、深色和唯一客户水印，管理后台仍保持`lina-web-antd-*`偏好、侧栏、浅色且无水印；双方 URL 分别固定在`6032/account/*`与`6031/admin/dashboard/*`。这证明端口、路由、布局、主题、水印和本地偏好均由各自应用独立拥有，不会因共用母版而串用。

验证命令与结果：

```text
pnpm exec playwright test --config apps/web-customer/tests/playwright.shell-isolation.config.ts
=> 1 passed
```

普通客户后台全量配置显式排除该双服务用例，避免每个客户用例重复启动管理后台；`TC028`只通过它自己的双服务配置执行。基于既有官网/客户深层刷新、后台不被吞、接口及静态资源保留路径证据和本次`TC028`，任务`15.11`及`TC-4d`已关闭。官网 SDK 的中英文说明也已同步为当前真实能力：访客购物车可以创建和加购，所有权凭据只存在于`HttpOnly + SameSite` Cookie，不由 SDK 接收或返回。

## 单安装包与旧门户退役验证

`linactl build`的完整宿主构建现会依次构建管理后台、客户后台、独立官网和 Go 宿主，并把目标平台所需内容装配进同一个发布目录。Windows/amd64真实构建使用项目当前 Go 1.27 工具链完成，产物包含`lina.exe`、`web-customer/index.html`、`website/default/server/index.mjs`、`runtime/node.exe`和`delivery.json`；清单固定记录接口`6030`、管理后台`6031`、客户后台`6032`、官网`6680`，并为客户后台、官网和运行时记录 SHA-256 摘要。

本机包锁定的官网服务端运行时为 Node `v24.20.0`。验证时没有调用系统 Node，而是直接使用发布包中的`runtime/node.exe`启动`website/default/server/index.mjs`，随后请求`GET /__site/health`得到`200 {"status":"ok"}`。这证明商家安装后不需要另行安装 Node.js。跨平台构建不会误用宿主机运行时：非本机目标必须显式提供`LINAPRO_SITE_NODE_RUNTIME_<OS>_<ARCH>`和对应版本变量，否则构建会直接失败并说明缺少的目标运行时，不会生成表面成功但无法运行的不完整包。

验证命令与结果：

```text
.\make.cmd build plugins=0 platforms=windows/amd64 output-dir=temp/phase15-delivery verbose=0
=> 构建成功，生成完整 Windows/amd64 单安装包

temp/phase15-delivery/runtime/node.exe temp/phase15-delivery/website/default/server/index.mjs
GET http://127.0.0.1:16880/__site/health
=> 200 {"status":"ok"}

go test . -run 'Test(AssembleWebDelivery|ResolveSiteNodeRuntime|TargetExecutableName|RunBuildRunsPluginConfigCommandsBeforeFrontendAndBackend|RunBuildWithoutDirBuildsAllPlugins|RunBuildDirBuildsHostFrontendOnly|RunBuildDirBuildsHostBackendWithPreparedAssets)' -count=1
=> ok

（在 apps/lina-websites 执行）pnpm run build
=> 默认官网、契约、SDK、打包器和 Nuxt 起步模板全部成功

（在 web-customer 执行）pnpm run check:alignment
（在 web-customer 执行）pnpm run i18n:check
（在 web-customer 执行）pnpm run typecheck
=> 全部通过
```

迁移唯一性检查确认`apps/lina-plugins/idc-portal/portal`不存在。公开商品目录与配置器只在`apps/lina-websites/sites/default`维护；购物车、订单、财务、个人实名与企业认证只在 Vben 结构的`web-customer`维护，相关真实浏览器回归已经通过。`idc-portal/frontend/dist/mount.js`仍然保留，因为它是当前插件自有的品牌管理后台入口，并由插件嵌入测试覆盖，不属于已退役的旧客户门户静态壳。基于上述证据，任务`15.7`、`15.12`、`15.12.a`和`15.12.b`已关闭。

本轮发布工具与健康接口没有新增面向终端用户的业务文案，`i18n`无新增翻译影响；中英文 SDK 文档已保持镜像。完整`linactl`工具包测试现已通过；必须创建符号链接的用例会先探测当前 Windows 是否具备开发者模式或管理员权限，不具备时只跳过真正依赖该系统能力的用例，其他工具测试继续执行。

上述独立官网、Vben 客户后台、全部客户页面迁移、路由/运行壳隔离、单安装包、后台品牌发布接线和真实浏览器门禁共同满足`FB-10`与`FB-12`的关闭条件。`apps/lina-plugins/idc-portal/portal`已不存在，Nuxt 只承担独立公开官网，登录后客户业务只由`apps/lina-vben/apps/web-customer/`承担；两个反馈现已关闭。
