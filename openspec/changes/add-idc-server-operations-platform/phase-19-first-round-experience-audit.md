# 第十九阶段第一轮全系统体验审查记录

日期：2026-09-01  
范围：`FB-57`、`FB-58`、`FB-59` 的第一轮静态扫描与客户中心/官网现场复核。

## 审查口径

本轮从管理后台、客户中心、默认官网、插件前端构建产物、接口装配和数据库查询路径重新检查菜单分组、图标语义、普通用户文案、双语、视觉令牌、内容可见性、表格高度、操作列、空态、加载失败、分页、有界候选、批量装配、索引和 `N+1` 风险。已读取项目顶层规则、用户偏好、架构、数据权限、接口、后端、前端、测试、数据库、插件、缓存、文档和 OpenSpec 规则；本轮没有改变业务权限、费用、状态含义或第三方外部流程。

## 发现与处理

| 类别 | 发现 | 处理与证据 |
| --- | --- | --- |
| 客户端字体 | 客户中心业务样式曾在根作用域声明独立 `Inter/PingFang` 字体栈，会覆盖 Vben 主题字体。 | 已改为 `font-family: inherit`，由 Vben 主题统一提供字体；见 `apps/lina-vben/apps/web-customer/src/styles/customer-console.css:25`。 |
| 客户端列表滚动 | 菜单选择器的 VxeGrid 使用 `height: auto`，且数据监听中直接调用 `grid.loadData`，在条件渲染或数据延迟时可能只显示少量行或加载时机不稳。 | 已改为有界 `height: 520`，监听器通过 `setGridOptions({ data })` 更新；见 `apps/lina-vben/apps/web-customer/src/components/tree/src/menu-select-table.vue:69`、`:174`。 |
| 客户端操作列 | 服务器操作历史表的操作列没有固定在最右侧，横向滚动时操作可能离开视口。 | 已补齐 `fixed: 'right'` 和明确宽度；见 `apps/lina-vben/apps/web-customer/src/views/account/servers/index.vue:252`。客户中心其余操作列及可装配后台 VxeGrid 操作列均已扫描确认固定在右侧（独立插件表格使用 `fixedRightActionColumn` 统一契约）。 |
| 官网双语 | 默认官网首页的“服务商信息 / Learn more / From the operator / Terms”存在内联语言分支，容易遗漏其他语言。 | 已统一收敛到首页本地化内容对象；见 `apps/lina-websites/sites/default/app/components/public/PublicHomePage.vue:31-54,96-101`。 |
| 页面结构 | 客户中心未发现业务原生 `table/form/input/button`、列表/树形 `height: auto`、直接 `grid.loadData`、渐变大横幅或独立字体栈；少数响应式图片和快捷入口保留 `height: auto`，不承担数据列表滚动。官网未发现对应视觉禁用项。 | `web-customer` 对齐治理和双语检查均通过；动态源码插件的独立工作区保留在自身根节点内，未修改宿主页面或宿主依赖。 |
| 数据和接口 | 已复核客户、订单、服务、工单、运营和渠道列表的分页/游标、投影和集合化装配；已有阶段证据覆盖索引、查询预算、无 `N+1` 和结算 benchmark。 | 本轮未发现需要改变后端业务契约的本地问题；性能与索引基线沿用阶段 5、6、8、13、18 证据，并在第二轮继续用真实浏览器复核。 |

## 已执行检查

- `git diff --check`：通过（仅保留仓库现有的 CRLF 提示）。
- `openspec validate add-idc-server-operations-platform --strict`：通过。
- `make.cmd i18n.check plugins=1`：运行时 i18n 违规 0，前端/消息覆盖通过。
- `make.cmd plugins.check plugins=1`：3387 个文件，0 项治理问题。
- `pnpm --dir apps/lina-vben --filter @lina/web-customer run typecheck`：通过。
- `pnpm --dir apps/lina-vben --filter @lina/web-customer run check:alignment`：通过。
- `pnpm --dir apps/lina-vben --filter @lina/web-customer run i18n:check`：29 个双语资源文件通过。
- `pnpm --dir apps/lina-websites --filter @lina/site-default run typecheck`：通过。
- `pnpm --dir apps/lina-websites --filter @lina/site-default run i18n:check`：通过。
- `pnpm --dir apps/lina-websites --filter @lina/site-default run seo:check`：通过。
- 默认官网 Playwright：`27 passed`，构建与 SSR/SEO 页面检查随套件通过。
- 客户中心 Playwright：`106 passed, 1 skipped`（107 个用例，约 8.3 分钟）；跳过项是既定外部/真实供应商条件，不是本地失败。

## 现场截图

本轮关键截图保存在 `temp/20260901/`，包括：

- `013051-customer-console-overview-zh.png`：中文桌面客户概览、分组导航、常用入口和办理流程。
- `013054-customer-console-overview-en-mobile.png`：英文移动客户概览、无横向溢出和统一冷灰/白色直角面板。
- 同目录下的客户账户、订单、财务、合规、工单、插件页面截图用于逐页抽查。

人工抽查确认：客户中心标题、入口、面板边界和移动窄屏内容可见；登录成功提示属于短暂通知，不影响页面操作；英文页面未出现中文或未翻译键。

## 外部条件

腾讯电子签、诺诺电子发票、真实供应商证书/主机/回调/SSRF 等仍没有账号、正式契约或联调条件，按 `HANDOFF.md` 保持暂停和关闭，不用本地假数据冒充外部验收。
