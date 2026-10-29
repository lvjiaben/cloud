# 金融解决方案实现与验收

## 来源与交付

正文唯一来源为用户附件`ec0b4d61-e6b6-41df-b3d6-34dcb3a01fa5/已粘贴的文本.txt`。在线`https://www.xiaobaiyun.cn/plan/financial.html`仅用于布局、结构与交互核对。附件与在线页面的54项正文、父子路径、标签和标题等级一致，比较仅统一浏览器HTML解析产生的`CRLF/LF`差别；正文有效空格、标点、大小写及重复词未归一化。

实际地址为`http://127.0.0.1:6680/plan/financial.html`。首页顶部“解决方案 → 行业解决方案 → 金融解决方案”已由旧`/404.html`接入本页。首页保留原有导航卡片说明，金融页菜单使用附件原文配置；未将金融页整套菜单文案覆盖其他页面。

## 逐栏目正文比对

独立配置记录栏目、顺序、父子路径、原始HTML标签、标题等级、原文和交互状态。自动化对比当前运行页面与附件基准，结果如下。

| 栏目 | 文本项数 | 缺失 | 增加 | 改写 | 顺序、父子关系及层级变化 |
| --- | --- | --- | --- | --- | --- |
| 首屏 | 2 | 0 | 0 | 0 | 0 |
| 锚点导航 | 6 | 0 | 0 | 0 | 0 |
| 我们的优势 | 8 | 0 | 0 | 0 | 0 |
| 面临的问题 | 18 | 0 | 0 | 0 | 0 |
| 解决方案架构 | 10 | 0 | 0 | 0 | 0 |
| 相关产品 | 2 | 0 | 0 | 0 | 0 |
| 合作伙伴 | 2 | 0 | 0 | 0 | 0 |
| 更多方案 | 6 | 0 | 0 | 0 | 0 |

合计54项。保留“面临的问题”的`h3`、说明的`h5`与“架构优势”的`h1`，不统一标题等级。`Intel Haswell CPU`、`DDR4`、`I/O`、`SSD`、`P2P`、`ECS`、`API`及相关有效空格保持原文。正文未发现“小白云”或“江苏白猿”等品牌名称，无待确认的正文品牌替换位置。

六个问题标签是静态列表，四项问题正文同时显示，未构造六组选项或面板映射。顶部菜单仍在对应分组、标签和展开状态中展示，金融页菜单标题、卡片名称和说明与附件配置比较通过，没有平铺到正文。

## 品牌、图片与链接

| 内容 | 本方来源或处理 |
| --- | --- |
| 网站名称、顶部Logo | 既有品牌适配的`siteName/headerLogoUrl` |
| 公司、版权、页脚Logo | `footer/copyright/footerLogoUrl` |
| 电话、邮箱、QQ、地址 | `contactPhone/contactEmail/businessQq/companyAddress` |
| 备案与公安备案 | `icpNumber/icpUrl/publicSecurityNumber/publicSecurityUrl` |
| 两类二维码 | `enterpriseWechatQrCodeUrl/wechatOfficialQrCodeUrl` |
| 咨询按钮 | 既有客服适配的目标，当前为本方`/contact.html` |

以上字段通过`GET /x/idc-portal/api/v1/public/brand`读取，已核实公开DTO及品牌投影类型。没有独立公开客服跳转字段，不猜测新增接口或字段；品牌失败沿用公共重试状态，静态正文不丢失，也不回退竞品资料。真实接口验证与受控失败、恢复测试分别记录。

用户后续明确要求图片与参考页相同，覆盖初版图片留位要求。现恢复21张正文图片（3项优势、1张架构图、12张伙伴展示图、5项更多方案图标）、4项页脚服务图标及首屏、伙伴、问题区、菜单和推广等CSS背景，共31个唯一图片资源。全部下载到插件本地`frontend/public/lina/financial-assets/`，来源和SHA-256见`financial-images-source-20261023.json`，不依赖参考站外链。伙伴仅恢复原有图片，不新增合作关系文字。相关产品继续保留空容器，不生成商品。

网站Logo、二维码、客服头像、联系方式及备案继续使用本方公开资料。图片使用`object-fit:contain`保持比例，首屏背景使用`cover`适配容器；不再使用灰色图片占位。

更多方案中的电商、网站、游戏、安全分别映射本方`/plan/electricity.html`、`/plan/web.html`、`/plan/game.html`、`/plan/safety.html`。移动方案尚未实现，按项目现有规则进入本地`/404.html`，不跳转参考站。

## 修改文件

全部实现位于`apps/lina-plugins/website-main/`，本轮具体文件如下。

| 文件 | 用途 |
| --- | --- |
| `frontend/public/plan/financial.html` | 金融页面入口 |
| `frontend/public/lina/financial-content.js` | 独立原文、菜单、页面与公共外壳配置 |
| `frontend/public/lina/financial.js` | 幂等初始化、滚动高亮和品牌及咨询接入 |
| `frontend/public/lina/financial.css` | 桌面及手机布局、图片留位与完整正文显示 |
| `frontend/public/lina/site.js` | 初始化、品牌刷新及页面分流 |
| `frontend/public/lina/navigation.js` | 金融页附件菜单配置 |
| `frontend/public/lina/navigation-content.js` | 本地金融入口及旧目标覆盖 |
| `hack/tests/financial-copy.test.mjs` | 附件、资源边界和入口单测 |
| `hack/tests/support/financial-copy-reference.json` | 独立附件基准 |
| `hack/tests/pages/FinancialSolutionPage.ts` | 金融页POM |
| `hack/tests/e2e/TC129-financial-solution.ts` | 正文、交互、手机、失败恢复及入口验收 |
| `frontend/public/lina/financial-assets/` | 31个本地展示图片资源 |
| `hack/tests/support/financial-image-reference.json` | 展示图片与CSS背景验证基准 |
| `docs/financial-images-source-20261023.json` | 原图来源、大小及SHA-256 |
| `hack/tests/tasks.md` | 实施和验收记录 |

关联规范及任务位于`openspec/changes/migrate-xiaobai-website/`。本说明同时记录审查与资源缺口，不新增目录级README。

## 验证与审查

- 源码`TC129`四项通过；修正截图发现的手机问题正文省略后，源码手机定向复验通过。
- 实际`6680`的`TC129`四项和首页`TC123`一项共5项通过；最后CSS解除截断后，实际桌面、手机两项定向复验再次通过，包含正文可见高度断言。
- 金融及导航单测9项通过；相关JavaScript语法及OpenSpec严格校验通过。
- 关键截图已视审：桌面、手机首屏、问题说明、架构、菜单、品牌失败及恢复。最初手机问题说明被公共样式折叠，已在金融页解除行数和高度限制，最终截图不再省略正文。
- 触发`lina-review`和E2E质量审查：用户可观察的新页面及旧404入口涉及交互行为。54项原文和层级、全部六锚点、菜单分组、单次初始化、真实品牌、失败恢复、手机不溢出及无页面脚本错误均有自动化断言；独立运行不依赖其他测试数据。未发现本轮范围阻塞问题。

本次公共字段、请求和错误处理沿用现有组件，不增加缓存、API契约、数据库、数据权限或运行期DI。插件未启用框架`i18n`，正文按用户要求单语保留，公共控件继续原有语言机制。没有修改宿主、后台资料、服务启停或交易数据，没有提交、推送、创建PR或归档。

## 实际资源与剩余事项

运行资源变更前备份在`temp/20261023/1792751611284-financial-before/`，其中`receipt.json`记录实际HTTP结果及SHA-256。四个金融资源按源码交付；共用`site.js/navigation.js/navigation-content.js`仅合并金融差量，保留运行目录其他窗口独立改动，未整文件覆盖。

最终实际桌面和手机截图在`temp/20261023/1792751801051-website-test-results/`；品牌失败、恢复与首页回归证据在`temp/20261023/1792751640124-website-test-results/`。附件在线比较证据在`temp/20261023/201100-financial-comparison.json`。

最新图片补齐变更只同步金融页配置、样式与31个本地图片资源，共33项，实际HTTP状态及SHA-256全部匹配。运行备份及回执位于`temp/20261023/1792752476771-financial-images-before/`。源码`TC129`四项及单测三项通过；桌面首屏与手机架构截图已检查，图片显示且正文无截断。

实际`6680`的`TC129`四项通过，证据位于`temp/20261023/1792752501287-website-test-results/`。截图发现旧外层网格将伙伴图片挤在左侧，已恢复单层六列／手机三列网格，并增加每张伙伴图片可见宽度至少60px的断言。最终桌面、手机两项复验通过，截图位于`temp/20261023/1792752624031-website-test-results/`；已实看两种视口的伙伴图及更多方案、桌面首屏和架构、手机架构。31项本地图片均成功解码，21张正文图片完整加载，无参考站请求，54项原文及层级保持一致。

按`lina-review`完成当前图片、样式、测试与规范差量审查，并触发E2E质量审查；测试覆盖成功加载、尺寸、移动布局、品牌失败重试与正文保留，未发现本次范围阻塞问题。不涉及后端、权限、缓存、DI、运行文案翻译或长期开发工具变更。首次测试因默认浏览器可执行文件缺失而未启动，随后使用已有Chrome完成验证，没有安装浏览器或重启服务。

移动方案页面仍未实现，沿用本地404。本轮没有重打或升级冻结安装包；安装旧包可能恢复旧页面资源。
