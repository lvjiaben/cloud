# 资质荣誉页面交付记录

实际访问：<http://127.0.0.1:6680/certificate.html>。导航：了解我们 → 荣誉资质。首屏标题保留“资质荣誉”。验收日期：2026年10月25日。

## A．参考原文核对

唯一正文来源是用户附件`d73aa243-90af-4cf3-9600-8b091db8c490/已粘贴的文本.txt`，快照保存在`hack/tests/support/certificate-source.html`，独立基准保存在`hack/tests/support/certificate-copy-reference.json`。本次在线HTML快照为`temp/20261025/103100-certificate-online.html`。

附件与在线页的两项首屏原文及层级一致，二者`main.et-main`均为空。附件片段没有首屏背景定义，图片按用户单独授权取在线背景，不据此改换正文来源。来源对比记录见`docs/certificate-source-comparison.json`。

| 原文项 | 标签 | 缺失／增加／改写 | 顺序／父子关系／标签等级变化 |
| --- | --- | --- | --- |
| 资质荣誉 | `h1` | 均为0 | 均为0 |
| 我们是一家拥有丰富资质荣誉的云服务商，为您提供卓越的技术支持和可靠的服务，助您在竞争激烈的市场中脱颖而出。 | `p` | 均为0 | 均为0 |

单元测试同时比较完整首屏元素树，运行页比较标签、原始文字及顺序。参考部分两项已完成逐项核对。导航、页脚及品牌资料按本方适配处理，不计入正文一致性。

## B．授权新增内容

独立配置位于`frontend/public/lina/certificate-content.js`：`certificateSource`记录附件原文、栏目、顺序、父路径、标签、标题等级与可见状态；`certificateAdditions`标记为`user-authorized-additions`。模板从配置取文案，不额外维护另一套同义正文。

| 新增栏目 | 标题层级 | 内容及数量 |
| --- | --- | --- |
| 以规范为基石，以信任见长 | `h2` | 标题及授权说明，共2项 |
| 资质展示 | `h2` | 标题及授权说明，共2项 |
| 值得信赖，始于每一处认真 | `h2` | 栏目标题，三个`h3`卡片标题及三个`p`说明，共7项 |
| 进一步了解我们，开启合作沟通 | `h2` | 标题、`p`说明、联系咨询链接，共3项 |

新增14项展示文案，均按用户提供内容实施，源文换行仅作为排版处理。“01／02／03”是卡片顺序装饰，标记`aria-hidden`，不表示证书数量或经营数据。咨询箭头为装饰。

参考页原本为空的`main`已按授权加入四个栏目；这属于明确新增，不能称整页与参考站原文完全一致。

## C．真实资料缺口与预留格式

当前没有提供本方证书，检查的公开品牌契约也没有证书列表字段。`certificateRecords`为空数组，没有猜测证书接口，也没有生成证书、编号、印章、奖项或示例认证。

资质栏目保留标题、说明与24像素空内容留白；未用大块固定高度制造空洞。隐藏的`template`保留卡片、证书名称、颁发机构、有效期及预览按钮结构；真实字段为空时不显示对应行。图片保持`object-fit:contain`，大图采用原生`dialog`，关闭后恢复按钮焦点。

卡片与预览仅用浏览器请求拦截注入临时“图片预览测试”数据验证，所用图片是本页通用背景。测试数据未写入源码配置、运行产物、数据库或公开接口，当前线上仍为空证书状态。未新增上传、审核及后台管理功能。

## D．品牌、图片和链接

核对了现有`idc-portal`品牌`Config`及`GET /x/idc-portal/api/v1/public/brand`的返回结构，复用`site.js`、`footer.js`、`header-logo.js`、`support-bar.js`及现有错误重试机制。本页只增加品牌标题和咨询链接重绘，不新增请求封装或身份接口。Logo、公司、联系方式、版权、二维码及备案来自本方；字段为空按公共适配隐藏，不回退对方身份资料。

首屏通用背景`remoteL17069813671571.webp`已下载到`frontend/public/lina/certificate-assets/`，原始URL、字节数和SHA256见`docs/certificate-images-source.json`。使用`background-size:cover`保持比例，手机允许背景裁切；不是拉伸图片。公共页脚沿用现有本地图标和本方身份资料。

| 入口 | 本方映射 |
| --- | --- |
| 首页了解我们 → 荣誉资质 | `/certificate.html` |
| 已有方案页原文菜单“资质荣誉” | `/certificate.html`，保留其原有标签 |
| 公共页脚“荣誉资质／资质荣誉”及英文对应项 | `/certificate.html` |
| 联系咨询 | 复用本方客服适配，当前为`/contact.html` |

旧404菜单及旧标签已归并；没有跳转参考站业务页。页脚本地入口在品牌请求之前初始化，并在品牌重绘后重新绑定，避免接口失败时留下禁用入口。

## E．待确认宣传表述

保留的首屏原文“拥有丰富资质荣誉”“卓越的技术支持和可靠的服务”“助您在竞争激烈的市场中脱颖而出”属于参考宣传表述，尚未由本方证明资料确认。尤其“拥有丰富资质荣誉”不能作为本方已取得资质的证据。本次按用户明确要求保留，未擅自扩展为认证、排名或结果保证。

新增文案只采用用户授权的品牌表达；证书数量、颁发机构、有效期、真实证书图片仍待提供。不声明已获得任何具体认证或已完成资质审核。

## 验证证据与审查

- 附件原文及层级、空证书、旧标签映射单元测试2项通过；连同既有导航回归共10项通过。
- 实际`6680`的`TC137`覆盖6项：桌面、手机、品牌失败重试、首页入口、临时预览、金融页原文导航。
- 最终实现的桌面、手机、失败恢复、预览及金融菜单5项通过；同轮首页曾因真实品牌加载超时失败，增加测试对现有重试入口的处理后单独复测通过，耗时19.7秒。没有把该次失败记为通过，也没有重启服务或改写品牌接口。
- 桌面手机、原文、四个栏目、图片解码和显示尺寸、背景比例、空证书、本方咨询及页脚链接、无水平溢出、正文无截断、幂等初始化和页面错误均有断言。品牌失败时原文及页脚入口仍可用，重试恢复本方公司和页面标题。
- 截图已视审首屏、全部新增栏目、资质留位、手机卡片及页脚、失败和恢复、首页展开菜单及测试预览。界面保持蓝白色系，未发现阻塞布局问题。
- 最终桌面手机与失败恢复截图位于`temp/20261025/1792903006215-website-test-results/`；同轮预览及金融菜单证据位于`1792903083936-website-test-results/`；首页最终复测证据位于`1792903163698-website-test-results/`。
- 本次JavaScript语法检查及`openspec validate migrate-xiaobai-website --strict`通过。

使用`lina-review`审查本任务新文件、共享脚本差量、测试及规范任务；因新增页面、入口和品牌异常路径触发E2E质量审查。预期原文来自独立附件基准，新增文案独立断言；覆盖真实HTTP成功和失败恢复，不依赖跨用例残留。曾发现品牌失败时页脚入口未绑定，已修复并补失败断言。未发现剩余阻塞问题。

插件未开启框架`i18n`，正文按授权保持中文，不新增框架语言资源；公共导航既有中英文标签映射继续复用。没有后端、数据库、权限、缓存或运行期依赖装配变化；没有新增目录README，本文无双语镜像要求。

## 修改文件与运行状态

本次代码均在`apps/lina-plugins/website-main/`：

- 页面：`frontend/public/certificate.html`。
- 原文与新增配置：`frontend/public/lina/certificate-content.js`。
- 初始化和预览：`frontend/public/lina/certificate.js`。
- 布局：`frontend/public/lina/certificate.css`。
- 本地背景：`frontend/public/lina/certificate-assets/remoteL17069813671571.webp`。
- 公共差量：`frontend/public/lina/site.js`、`navigation-content.js`、`footer.js`。
- 测试：`hack/tests/certificate-copy.test.mjs`、`e2e/TC137-certificate.ts`、`pages/CertificatePage.ts`、`support/certificate-copy.mjs`、`certificate-copy-reference.json`、`certificate-source.html`及`tasks.md`。
- 记录：本文、来源对比JSON、图片来源JSON，以及现有`migrate-xiaobai-website`变更的相关规范和任务。

静态资源已同步到当前运行目录`temp/output/websites/website-default/public/`。共享脚本仅补本次差量，保留其他窗口修改。初始8项HTTP回执及覆盖前备份位于`temp/20261025/1792902494366-certificate-before/`；最终状态回执位于该目录的`final-receipt.json`。

未修改宿主、重启服务、提交、推送或创建PR。未重打冻结安装包，重新安装旧包可能回退当前运行静态资源。证书资料及上述宣传表述确认仍待用户提供；本次页面及空资料状态交付完成。
