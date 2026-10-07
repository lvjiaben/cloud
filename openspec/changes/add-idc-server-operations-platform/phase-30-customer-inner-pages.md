# 第三十阶段客户内页视觉优化

## 分诊与边界

- 日期：2026-09-05。反馈：用户明确要求保留用户端 layout，只优化每个菜单内页，参考企业云控制台风格。
- 分类：`openspec-existing`，归入当前变更的`FB-116`，不另建并行业务变更。
- 参考：[阿里云实例信息组织](https://help.aliyun.com/zh/ecs/user-guide/view-instance-information)、[腾讯 TDesign](https://static.tdesign.tencent.com/)。只参考视觉层级，不引入竞品字段或业务规则。
- 静态盘点：37 个组件路由、29 个内页文件；23 个侧栏入口，另含资料／地址、隐藏详情与表单。动态客户表面包括证书、云服务器，源码客户表面包括渠道令牌。
- 根因：共用样式只管颜色和直角，没有统一内页结构；重复标题区、960～1440px 等不一致外层宽度、Card 正文错误外边距、部分 HSL 变量直接作为颜色导致分隔背景失效。
- 方案：直接使用原 Vben Page 的公开 class 接口和客户专属样式，修正各页局部盒模型与排版。表单保持合理阅读宽度，列表使用全部可用宽度；不新增 UI 框架或第二层页壳。
- 并行评估：独立代理只读盘点；客户插件视觉可独立闭环，主线处理静态客户页面与集成验证。

## 影响评估

- 用户已确认边界同步到用户偏好和本变更规范。字段、状态、按钮业务逻辑、菜单路由不变。
- API、权限、SQL、缓存、跨模块数据装配：无变更，不增加请求和 N+1 查询。
- i18n：保留现有文案及键，中英文样式一起检查。
- 宿主边界：不修改`lina-core`、共享`Vben`包或客户`layout`；动态样式仅在客户页面生效。`web-antd`仅包含用户另行确认的一行升级预览空值修复，后台样式不变。
- 跨平台：生产改动是 Vue／CSS，测试沿用现有 Playwright 工具；不新增平台专属生产脚本。

### 已确认的发布阻碍

- 正常升级 `idc-zjmfcloud` 时，预览接口返回成功但 `riskHints=null`，后台弹窗直接读取 `.length` 造成渲染异常；确认按钮不能操作。
- 用户确认仅补前端空值处理，保留升级流程、权限和后台样式。空风险列表仍展示已有“无额外风险”提示；有风险列表照常展示，不隐藏风险或绕过升级检查。
- 两个客户插件通过补丁版本正常发布：`yunsu-ssl v0.3.5`、`idc-zjmfcloud v0.2.5`。前端样式不写入业务数据，升级预览权限差异为空，无新增`SQL`迁移；通过原“同步插件→升级→确认升级”完成，未卸载、重装或修改权限。

## 实现结果

- 25 个根页面接入`Vben Page`公开的`class`、`header-class`和`content-class`；个人、企业认证嵌套页沿用同一内容作用域。客户概览、交易、财务、服务、认证、合同、资料、安全、支持与通知页面统一标题、间距、分隔线和白色面板。
- 移除同一内页重复标题，统一列表可用宽度，保留表单合理阅读宽度。原`Vben`表格列、搜索、分页和高度契约不变，表头灰色、其他表格区域白色。
- 修复`Ant Card`清除浮动伪元素参与弹性布局造成的操作区空档；修复工单详情桌面分栏未生效；手机服务器详情标题与操作区分行，长名称不再被按钮挤掉。
- 动态证书、云服务器及源码渠道令牌页面只修改各自客户根节点内的样式，没有影响后台、菜单发现、插件隔离或业务动作。
- 正常启动后曾遇到客户开发缓存仍引用旧适配器导出，已将该客户`.vite`缓存移至`temp/fb116-customer-vite-cache-before`保留备份，再正常重启开发服务；没有删除业务数据。

## 验证记录

### 浏览器与运行版本

- 使用真实开发服务和正常后台审计协助入口，检查客户账号的 24 个主要内页，分别使用`1440×1000`与`390×844`视口；内容均处于主内容区内，没有整页横向溢出或页面错误提示。桌面侧栏仍为`204px`，手机默认关闭，原菜单行为未变。
- 实际检查包含商品、服务器、购物车、订单、优惠、财务总览、信用、账单、流水、续费、退款、发票抬头、发票、推荐、资料、地址、密钥、数据权利、隐私、认证、合同、工单、通知和相关详情；自动遍历的 24 个路径与人工概览／详情验证分开计数，不把详情重复计入菜单数。
- 真实动态页面已加载`/x-assets/idc-zjmfcloud/v0.2.5/customer/servers/index.html`和`/x-assets/yunsu-ssl/v0.3.5/customer/certificates/index.html`。桌面、手机外层与隔离页内部均无横向溢出。
- 人工查看最终截图：概览、工单详情桌面分栏、英文手机服务器详情、桌面订单、手机财务、资料、认证，以及云服务器／证书桌面和手机。空数据使用真实空状态；有数据的订单、金额和服务器详情用现有测试替身验证，不伪造真实客户业务数据。
- 两次测试协助会话均已通过原后台入口结束。没有执行付款、退款、服务器控制、重装或其他破坏性业务操作。

### 命令结果

| 验证 | 结果 |
|------|------|
| 客户`TC029`与`TC030`定向回归 | 7 项通过，包含布局不变、桌面／手机、中英文、表格分页和详情 |
| 后台`TC011`新增`TC-7c` | 1 项通过，分别覆盖`null`、空数组、正常风险数组，重复关闭打开与原确认升级流程 |
| 客户和后台`typecheck` | 均通过 |
| 客户生产`build` | 通过；保留原有构建体积提示，不作为运行错误 |
| 证书前端构建及测试 | 构建通过，7 项测试通过 |
| 云服务器前端构建及测试 | 构建通过，3 项测试通过 |
| 渠道令牌`Vue SFC`编译 | 通过 |
| 两个插件`make wasm` | 均通过，正常发布到开发运行实例 |
| `make i18n.check`、客户`i18n:check` | 均通过，前者零违规 |
| 严格`OpenSpec`、本轮差异和证据文件检查 | 通过 |

客户回归命令在`apps/lina-vben`运行：

```powershell
pnpm exec playwright test --config apps/web-customer/tests/playwright.config.ts TC029 TC030 --output=D:/golang/cloud/temp/20260905/201900-inner-results
```

后台新增回归在`hack/tests`运行：

```powershell
$env:E2E_BROWSER_CHANNEL='chrome'
pnpm exec playwright test --config playwright.config.ts e2e/extension/plugin/TC011-plugin-runtime-upgrade.ts --grep 'TC-7c' --output=D:/golang/cloud/temp/20260905/202500-upgrade-results
```

结果文件分别为上述目录内的`.last-run.json`，均为`passed`。最终图像证据在`temp/20260905`：

- `120000-inner-overview-live.png`、`120816-inner-support-detail-desktop.png`。
- `120845-inner-server-detail-mobile.png`、`120527-inner-commerce-desktop-orders.png`、`120502-inner-finance-mobile-billing.png`。
- `201400-cloud-customer-desktop.png`、`201500-ssl-customer-desktop.png`、`201600-cloud-customer-mobile.png`、`201600-ssl-customer-mobile.png`。
- `121319-plugin-upgrade-empty-risks-2-1.png`；截图等待动画稳定，不修改产品动画。

## Lina 审查报告

- 变更：`add-idc-server-operations-platform`，仅`FB-116`与`FB-116a`反馈级审查，不代表整个历史变更已完成。
- 审查文件数：44 个。包含客户 27 个页面文件、2 个样式文件、2 个测试文件；插件 5 个客户入口／样式文件及 2 个清单版本；后台升级组件及其回归共 2 个；用户偏好、任务、增量规范及本记录共 4 个。
- 范围来源：`git status --short`、展开的`git ls-files --others --exclude-standard`、本轮任务清单与`openspec status`。工作区大量既有未提交内容不纳入本次完成结论；未运行代码生成器。
- 已读取规则文件：`user-preferences.md`、`frontend-ui.md`、`testing.md`、`openspec.md`、`documentation.md`、`dev-tooling.md`、`i18n.md`、`architecture.md`、`plugin.md`；并读取其要求的`markdown-format.instructions.md`。三个目标插件根目录均无独立`AGENTS.md`。
- 前端与插件：通过。使用原`Vben`公开接口，客户样式限定作用域；无共享布局覆盖，无任意全局插件样式。升级只作空数组等价处理，正常风险仍全部展示。
- 后端、接口、权限、数据库、业务缓存：无影响。没有修改`Go`、接口契约、鉴权或数据路径，未新增查询。开发预构建缓存备份不属于业务缓存一致性变更。
- 多语言：通过。保留原文案与语言键，覆盖中文桌面和英文窄屏。没有新翻译资源或后台状态含义变化。
- 跨平台与文档：前端及测试沿用既有跨平台工具，临时证据落在日期目录；未修改服务启停、打包工具和平台脚本。无新增目录级说明，严格规范校验通过。

### 警告与剩余覆盖

1. 旧全量客户用例与当前业务页面失配：`TC001-support-notifications.ts:43`用无名称按钮匹配导致歧义；`TC004-personal-verification.ts:19`等仍按旧实名路由／标题断言；`TC005-enterprise-verification.ts:32`使用的旧模拟数据未覆盖先个人认证的入口。全量运行发现这些失败后停止，不能宣称 117 项全部通过。本轮未改变这些业务规则，也未用放宽权限方式让旧测试通过。
2. 后台旧`TC011`的`TC-7a/TC-7b`依赖`hack/tests/pages/PluginPage.ts:432`已失配的运行状态测试标识，在打开弹窗前失败。新增`TC-7c`直接覆盖本次问题，已通过；旧用例仍需另行维护。
3. `sales-channel/index.vue:80`原空状态翻译键产生控制台提示，本轮未改状态语义。渠道令牌页面完成作用域审查和编译，但普通测试客户无该业务入口，未冒充有权限客户作真实数据操作。

结论：本轮范围未发现阻塞问题；上述 3 项为旧测试／数据覆盖警告，不将其写成全量通过。`FB-116`与`FB-116a`可完成，整个业务变更不在本轮归档。
