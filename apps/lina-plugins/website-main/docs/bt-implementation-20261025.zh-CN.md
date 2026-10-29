# 宝塔面板实施验收记录

实际入口：<http://127.0.0.1:6680/bt.html>。首页“支持与服务”中的“宝塔面板”由`/404.html`接到`/bt.html`，已检查首页点击、直接访问和刷新。

## 文件与发布

- 页面：`frontend/public/bt.html`。
- 正文、行为和样式：`frontend/public/lina/bt-content.js`、`bt.js`、`bt.css`，复用现有`frontend/public/static/css/bt.css`。
- 图片：`frontend/public/lina/bt-assets/`的九项资源。
- 公共接入：`frontend/public/lina/site.js`、`navigation.js`、`navigation-content.js`。
- 基准：`docs/bt-reference/`、`docs/bt-images.json`。
- 测试：`hack/tests/bt-content.test.mjs`、`hack/tests/e2e/TC132-bt.ts`、`hack/tests/pages/BtPage.ts`、`hack/tests/support/bt-copy.mjs`和`bt-copy-reference.json`。
- 回归稳定性：`TC106-six-navigation-panels.ts`采用无界面浏览器执行，保持原有悬停断言，避免可见浏览器指针干扰。

运行静态目录为`D:/project1/temp/output/websites/website-default/public`。已差量发布十六项文件，逐项核对实际HTTP返回及资源哈希，清单见[发布回执](bt-deployment.json)。替换前仅备份将覆盖的文件到`D:/project1/temp/20261025/1792898000792-bt-before/`。运行公共脚本采用定点补丁，保留其他任务已部署的实现。未停止或重启服务，未修改宿主，未提交、推送或创建PR。

## 正文与结构

唯一基准为附件`22c749aa-cc9e-42b1-853e-b46619a95b3a`，SHA256为`aca4b2965d8e84818a38d1d71b5add60050198b4222b2013995016746dc13cd6`。附件与在线正文提取均为六十九项，比对无差异；完整快照和哈希保存在`docs/bt-reference/`。

| 栏目 | 项数 | 缺失、增加、改写、顺序、标签、标题等级、父子路径、交互差异 |
| --- | --- | --- |
| 首屏 | 4 | 均为零 |
| 锚点导航 | 5 | 均为零 |
| 功能优势 | 11 | 均为零 |
| 系统支持 | 14 | 均为零 |
| 安装命令 | 17 | 均为零 |
| 挂载命令 | 8 | 均为零 |
| 注意事项 | 10 | 均为零 |

每项保留`section/order/parent/tag/headingLevel/text/interaction`。命令卡片保留七个`h2`；注意事项保持原始展示，没有改为手风琴。比对仅规范化HTML缩进空白，命令复制载荷另做逐字符严格比对。公共导航、页脚和客服属于本方公共壳及品牌替换项，不计入六十九项业务正文。

## 图片映射

下表源目录前缀均为`/themes/web/www/static/`，本地目录均为`/lina/bt-assets/`；文件名保持一致。完整路径、大小和哈希见[图片映射清单](bt-images.json)。

| 来源相对路径 | 本地文件名 | 位置 |
| --- | --- | --- |
| `banner/remoteL17069717876526.png` | `remoteL17069717876526.png` | 首屏CSS背景 |
| `picture/remoteL17069683484677.webp` | `remoteL17069683484677.webp` | 功能展示 |
| `os/2.svg` | `2.svg` | 系统支持 |
| `os/3.svg` | `3.svg` | 系统支持 |
| `os/4.svg` | `4.svg` | 系统支持 |
| `os/10.svg` | `10.svg` | 系统支持 |
| `os/1.svg` | `1.svg` | 系统支持 |
| `picture/remoteL17075794763815.png` | `remoteL17075794763815.png` | 注意事项背景 |
| `picture/remoteL17075795114472.png` | `remoteL17075795114472.png` | 注意事项背景 |

九项均通过加载、哈希和图片解码检查；首屏截图确认背景实际显示。图片视审未发现小白云标识、联系方式或推广信息。未复制参考站Logo、二维码、客服和备案资源。

## 品牌与命令

品牌继续通过`GET /x/idc-portal/api/v1/public/brand`及现有公共渲染器加载。2026年10月25日页脚反馈修复后，移除宝塔页独立的`mapBrand()`图片过滤，与其他页面统一读取本方权威品牌字段；业务展示图片仍使用本地资源。

当前后台Logo指向外域图片，按用户最新要求恢复本方Logo展示，不再按页面过滤。品牌失败时正文和本地图片保留，已有错误与重试恢复路径通过验证。公共修复及验证限制见[页脚与字号修复记录](bt-shared-footer-fix-20261025.zh-CN.md)。

七条命令分别为Centos、Ubuntu、Debian、万能、国产龙芯安装脚本，以及官方和自定义目录挂载命令。全部`displayText/copyText`与附件逐字符一致，复制成功、拒绝和剪贴板不可用时的反馈及手动选中均通过测试。命令从未执行。

五条安装命令的`59d9cde77`原样保留，其归属和替换规则待用户确认。业务正文没有“小白云”等待替换品牌词。“宝塔”、命令中的下载地址、演示和两项技术资料链接按附件保留。

## 验证与审查

在插件根目录执行：

```powershell
node --test hack/tests/bt-content.test.mjs hack/tests/navigation-content.test.mjs hack/tests/navigation-cache.test.mjs
$env:PLAYWRIGHT_EXECUTABLE_PATH='C:/Program Files/Google/Chrome/Application/chrome.exe'
npx playwright test --config hack/tests/playwright.config.ts TC132-bt.ts
$env:BT_LIVE='1'
npx playwright test --config hack/tests/playwright.config.ts TC132-bt.ts
npx playwright test --config hack/tests/playwright.config.ts TC125-game-exact-live.ts TC127-safety-solution.ts TC106-six-navigation-panels.ts
```

单元测试十三项通过；源码拦截预览五项、真实6680页面五项通过。6680验证页面、菜单和资源实际HTTP交付；品牌失败恢复用受控响应验证，不作为真实后台成功的证据。游戏两项、安全三项通过。原导航用例在可见浏览器悬停阶段失败，改为无界面执行后单独复测通过一项；没有删减断言。

截图目录：`D:/project1/temp/20261025/1792898002634-website-test-results/`，包含桌面首屏、功能展示、安装区、手机首屏和安装区、品牌失败及恢复、首页菜单。已视审桌面、手机、安装和失败截图，无页面横向溢出，长命令在代码区域内滚动。九项资源、五锚点高亮、七复制载荷、单入口和初始化幂等、菜单及Escape、无竞品或外部图片请求均有断言。

按`lina-review`完成本任务范围审查，触发并完成E2E质量审查，未发现阻塞问题。插件根目录不存在`AGENTS.md`，沿用根目录规则。插件未启用框架`i18n`，本次正文按用户要求锁定附件中文；无后端、数据库、权限和运行期DI契约变更。现有其他任务改动未纳入审查或回滚。

剩余待确认项：安装参数`59d9cde77`；后台同源品牌图片资源。除此之外，本页实现和发布验收已完成。
