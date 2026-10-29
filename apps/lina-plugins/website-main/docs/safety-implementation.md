# 安全解决方案实现与验收

## 范围与入口

本次仅修改本插件。未提交、推送、修改宿主或覆盖其他任务改动。沿用已有公共导航、页脚、品牌请求、客服适配和游戏页公共壳。

源码预览：http://127.0.0.1:16680/plan/safety.html 。实际浏览器与直接HTTP均返回200。用户随后明确授权直接发布到现运行官网，http://127.0.0.1:6680/plan/safety.html现返回200；仅差量同步安全页入口、脚本、样式及本地展示资源，未停止或重启服务。

附件SHA256：`b87d4852f9693313cf87c4e7197518dd660f7dc9fbf54736a734ddeffa78e4f8`。正文原始基准保存在`hack/tests/support/safety-copy-reference.json`，未从实现反向生成。公共壳另外148条来源记录保存在`safety-shell-reference.json`，生产配置导出`safetyShellCopy`。

## 附件与在线正文

在线页面52条正文与附件比较：20处仅为CRLF与LF差异，没有发现字符内容差异，详见`safety-online-comparison.json`。始终以附件原始CRLF和有效空格保存生产配置；浏览器HTML解析依法规范化换行，浏览器比对仅归一化该换行，未trim正文或忽略有效空格。在线公共菜单和页脚尚未逐条比较，不据此声明整个在线页面无差异。

## 结构及逐项结果

顺序为顶部导航、首屏、六项锚点、我们的优势、面临的问题、解决方案架构、相关产品、合作伙伴、更多方案、页脚，与附件一致。正文配置每项保存`section/order/parent/tag/headingLevel/text/interaction`。

| 栏目 | 条数 | 缺失 | 增加 | 改写 | 顺序变化 | 层级变化 | 隐藏状态变化 |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| 首屏 | 2 | 0 | 0 | 0 | 0 | 0 | 0 |
| 锚点导航 | 6 | 0 | 0 | 0 | 0 | 0 | 0 |
| 我们的优势 | 8 | 0 | 0 | 0 | 0 | 0 | 0 |
| 面临的问题 | 15 | 0 | 0 | 0 | 0 | 0 | 0 |
| 解决方案架构 | 11 | 0 | 0 | 0 | 0 | 0 | 0 |
| 相关产品 | 2 | 0 | 0 | 0 | 0 | 0 | 0 |
| 合作伙伴 | 2 | 0 | 0 | 0 | 0 | 0 | 0 |
| 更多方案 | 6 | 0 | 0 | 0 | 0 | 0 | 0 |

保留附件中的架构优势h1、问题h3、说明h5等真实标题等级；正文父子路径、文字节点顺序、有效尾空格均逐项深比较。相关产品原附件为空，继续空容器；12个伙伴图位及架构、横幅、优势、更多方案图片均为无图占位，没有下载竞品资产。

公共壳单独说明：导航原文分组、描述、推广文案及隐藏标签面板保留，页脚保障词和静态分组链接标签与附件逐项一致。公共产品菜单依赖本方商品接口，附件中的“热销推荐”“四川成都弹性云”未作为本方商品编造；本方无数据时走公共组件空态。菜单按钮采用现有可访问组件，其DOM父子路径、按钮标签与原站模板不完全相同；新增关闭、手机展开、语言切换、搜索空态和错误重试属于公共组件辅助内容。因此仅声明52条正文全字段比对通过，不声明全页面所有文案和DOM完全一致。来源记录并不等于所有公共壳节点都已完成浏览器逐项对照。

三个服务咨询位置复用现有客服入口。更多方案链接映射本方电商、网站、游戏、安全页面；移动方案暂无本方页面，沿用本方404入口。页脚尚未接入的本方服务继续公共组件禁用状态，不跳转来源网站。附件未提供安全页正文标签页或独立弹窗，不额外编造；隐藏导航面板保持菜单及标签交互，未平铺。

## 品牌映射

真实契约依据为相邻`idc-portal`插件的公开品牌API及`brand.Config`，本次只读查阅，未修改该插件。请求复用`GET /x/idc-portal/api/v1/public/brand`及现有封装，响应`{code:0,data:{brand}}`。

| 页面位置 | 真实字段 | 适配 |
| --- | --- | --- |
| 顶部Logo及字样 | `headerLogoUrl/siteName` | `header-logo.js`，空图或图失败使用本方站名 |
| 页脚Logo、公司、版权 | `footerLogoUrl/footer/copyright` | `footer.js` |
| 电话、邮箱、QQ、地址 | `contactPhone/contactEmail/businessQq/companyAddress` | `footer.js`，缺项隐藏 |
| 二维码 | `enterpriseWechatQrCodeUrl/wechatOfficialQrCodeUrl` | `footer.js`，缺项不造图片 |
| ICP、公安备案 | `icpNumber/icpUrl/publicSecurityNumber/publicSecurityUrl` | `footer.js`，缺项隐藏 |
| 三个咨询按钮 | 公开品牌契约没有客服跳转URL字段 | `safety.js`跟随现有客服栏，本方`/contact.html` |

原站Logo、公司、版权、联系方式、备案值、二维码和客服URL均未用于本页。正文52条及方案导航未发现“小白云”品牌词，无需等待正文品牌词替换确认。品牌请求失败保留正文及已有错误重试，不回退竞品信息。

真实品牌接口初期超时，最终复查已恢复200及code=0；直接HTTP浏览器用例核对本方站名、实际存在的电话和邮箱与真实响应一致。受控503与重试成功是独立测试，不冒充真实故障。二维码和Logo沿用已存在公共适配，本次未对每个真实后台可选字段做修改配置后的端到端验收。

## 验证与审查

在插件根运行：

~~~powershell
node --test hack/tests/safety-copy.test.mjs
node --test hack/tests/navigation-content.test.mjs hack/tests/navigation-cache.test.mjs hack/tests/game-copy.test.mjs
$env:PLAYWRIGHT_EXECUTABLE_PATH='C:\Program Files\Google\Chrome\Application\chrome.exe'
pnpm exec playwright test hack/tests/e2e/TC127-safety-solution.ts --config=hack/tests/playwright.config.ts --output=temp/20261023/181000-safety
~~~

结果：正文及公共壳单测4项通过，导航和游戏回归12项通过，TC127三项E2E通过。相关五个JS模块语法检查通过。TC127覆盖1440桌面、390手机、正文全字段、六锚点及高亮、桌面手机菜单与分组、三个咨询位置、重复初始化同实例、单入口脚本、无横向溢出、无竞品请求、品牌失败及重试、真实预览HTTP与真实品牌接口。

截图保存在`temp/20261023/203500-safety-live/`；已实际检查桌面、手机、菜单、架构图位与品牌失败截图，没有正文截断及页面水平溢出。用户随后明确授权恢复附件所列展示图片：3张优势图标、1张架构图、12张伙伴图、5张更多方案图及2项背景图均保存于`/lina/safety-assets/`。顶部与页脚Logo、二维码、客服、备案和联系方式仍由本方品牌适配渲染，未从参考站复制。

使用`lina-review`进行本次范围审查，触发E2E质量审查：来源为用户需求、页面代码、公共接口契约和上述独立测试。测试各自建页与路由，真实预览用例独立上下文并finally关闭，不依赖受控用例状态。按插件边界、前端、测试、文档和语言规则检查。插件未启用框架i18n，本页正文按用户要求锁定附件中文；未新增语言资源。没有数据库、后端、权限契约变更。未发现已实现正文路径的阻塞问题，但以下整体验收缺口仍保留。

## 未完成与需确认

- 正式6680未部署，此次不修改安装目录、不重启现有服务。
- 公开品牌接口缺少客服跳转URL字段。若要求后台独立配置客服目的地，需要用户提供已存在的公开契约，或另行授权相应插件扩展；本页不猜字段。
- 原站图片、二维码、Logo不复制。待提供本方图片后才能填充占位，当前不是视觉资产完全复刻。
- 移动解决方案和部分页脚本方功能尚无入口，维持404或既有禁用状态。
- 公共商品菜单和辅助组件与附件存在上述有意差异，公共壳148条全部DOM层级与在线外壳尚未完成逐项浏览器比对，不能宣称全页原文完全一致。

## 文件清单

新增`frontend/public/plan/safety.html`、`frontend/public/lina/safety-content.js`、`safety.js`、`safety.css`；增量修改`site.js`、`navigation.js`、`footer.js`。新增`hack/tests/safety-copy.test.mjs`、`pages/SafetySolutionPage.ts`、`e2e/TC127-safety-solution.ts`、`support/safety-copy-reference.json`、`support/safety-shell-reference.json`及本报告、在线正文差异文件；追加`hack/tests/tasks.md`本任务记录。
