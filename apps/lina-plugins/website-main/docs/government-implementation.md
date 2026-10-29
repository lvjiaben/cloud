# 政企解决方案实现记录

## 当前来源与入口

本轮以在线参考页`https://www.xiaobaiyun.cn/plan/government.html`保存快照为正文唯一基准；快照保存在`hack/tests/support/government-source.html`。用户新附件`1c8c9deb-0916-43a3-99cb-e8b0c7185781/已粘贴的文本.txt`与在线快照的51项正文、标签、层级及父子路径一致，仅统一HTML解析中的CRLF/LF，没有混用或改写。

实际地址为`http://127.0.0.1:6680/plan/government.html`。按最新指定，首页“解决方案 → 通用解决方案 → 工业解决方案”进入该政企页；卡片说明仍为“了解工业数字化的应用场景。”，页面标题保留“政企解决方案”。原有行业政企入口保留，不复制第二份页面。

## 逐栏目比对

| 栏目 | 文本项 | 缺失 | 增加 | 改写 | 顺序、父子关系、标签及等级变化 |
| --- | --- | --- | --- | --- | --- |
| 首屏 | 2 | 0 | 0 | 0 | 0 |
| 锚点导航 | 6 | 0 | 0 | 0 | 0 |
| 我们的优势 | 8 | 0 | 0 | 0 | 0 |
| 面临的问题 | 16 | 0 | 0 | 0 | 0 |
| 解决方案架构 | 9 | 0 | 0 | 0 | 0 |
| 相关产品 | 2 | 0 | 0 | 0 | 0 |
| 合作伙伴 | 2 | 0 | 0 | 0 | 0 |
| 更多方案 | 6 | 0 | 0 | 0 | 0 |

51项均保持有效空格、标点、大小写、重复词及原始标签，保留面临问题的h3、说明h5和架构优势h1。四个问题标签及四项正文同时展示，无原站未提供的标签切换。正文没有小白云或白猿品牌名，无待确认替换位置。公共导航在对应菜单状态中展示，未平铺到正文。

## 图片、品牌和链接

21张正文展示图片及CSS背景共29项原文件保存至`frontend/public/lina/government-assets/`；原图地址、大小和SHA-256见`docs/government-images-source-20261023.json`。恢复首屏、伙伴、问题区、选中标识、架构优势、更多方案、菜单及推广图片；四项页脚服务图标复用现有公共外壳。图片保持比例，架构和图标使用contain，背景适配裁切。十二张伙伴图仅恢复展示，不新增合作关系文字；相关产品保持空容器，不生成商品。

品牌来源经核实为`GET /x/idc-portal/api/v1/public/brand`，类型位于`idc-portal/backend/internal/service/brand/brand.go`。`siteName/headerLogoUrl/footerLogoUrl/footer/copyright/contactPhone/contactEmail/businessQq/companyAddress/icpNumber/icpUrl/publicSecurityNumber/publicSecurityUrl/enterpriseWechatQrCodeUrl/wechatOfficialQrCodeUrl`由现有公开适配使用。客服沿现有support适配进入本方`/contact.html`，没有猜测新增客服字段。身份接口失败保留正文并支持重试，不回退对方资料。

更多方案电商、网站、游戏、安全分别进入本方对应页面；移动方案仍缺本地目标，沿现有规则进入`/404.html`。

## 本轮修改文件

- `frontend/public/lina/navigation-content.js`：工业卡片目标及后台旧404覆盖。
- `frontend/public/lina/government-content.js`：来源切换至本轮在线快照，集中正文配置。
- `frontend/public/lina/government.css`：补回背景、解除问题说明截断、保持图片比例。
- `frontend/public/lina/government-assets/`：29项本地图像资源。
- `hack/tests/government-copy.test.mjs`、`hack/tests/support/government-copy-reference.json`、`hack/tests/support/government-source.html`：独立来源及四项单测。
- `hack/tests/e2e/TC128-government-solution.ts`、`hack/tests/pages/GovernmentSolutionPage.ts`：实际页面、工业入口、图片尺寸解码与手机正文测试。
- `hack/tests/tasks.md`、本报告及图片来源记录：验收证据。

关联记录仅更新`openspec/changes/migrate-xiaobai-website/`对应规范和任务，以及项目偏好`72-customer-website-visual.md`中的最新入口决定。沿用已有页面、路由、公共导航、页脚和品牌适配，不修改宿主、接口或数据库。

## 验证与审查

当前实际6680的TC128四项通过，17.9秒；覆盖51项原文层级、六锚点、吸顶高亮、菜单、21张正文图片与29项资源解码、伙伴可见宽度至少60px、手机无溢出和正文不截断、品牌失败重试、真实品牌接口、重复初始化、首页工业入口及页面无脚本错误、无参考站请求。单测四项、JS语法及OpenSpec严格校验通过。

关键截图在`temp/20261023/1792754297453-website-test-results/`。已实看桌面首屏、手机问题与伙伴、首页通用工业入口；早期截图见`1792753946567-website-test-results/`。最初入口测试在展开动画结束前操作导致分组跳回，POM改为等待真实动画完成后成功；手机菜单检查后显式关闭外层菜单再截图，最终正文无遮挡。未因此修改公共菜单行为。

按lina-review审查本轮差量并触发E2E质量审查，核心成功、失败恢复、尺寸和入口路径均有有效断言，未发现本轮阻塞问题。插件未启用框架i18n，正文按要求保留单语；公共控件沿用现有语言机制。没有缓存、权限、DI或长期开发工具变更。

运行备份和32项HTTP状态及哈希回执在`temp/20261023/1792753930080-government-before/`。公共导航只差量合并工业入口，保留其他窗口运行代码。没有重启服务、提交、推送、创建PR或写业务数据；未重打冻结安装包，重装旧包可能回退资源。移动方案缺页仍为未完成项，不宣称逐像素一致。
