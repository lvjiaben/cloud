# 后端候选范围与运行门槛

## 可确认事实

本轮复用官方后端构建子步骤，在独立暂存工作区完成编译。未执行完整构建、代码生成、资源生成、安装、程序替换或服务启停；没有本轮新增后端源码。候选对应当前工作区，不是仅含搜索一行修复的补丁。

| 对象 | 证据 |
| --- | --- |
| 暂存根 | `D:/project1/temp/20261018/1792299479378-backend-release-yoj6f8` |
| 候选 | `lina-content-candidate.exe`，`116239360`字节 |
| 候选摘要 | `869c2cb78ebd8048d7531955763c35aadd129caf5ab2ced8c79315d37a9e010b` |
| 回退程序 | `rollback/lina-before.exe`，与当前`temp/output/lina.exe`同摘要 |
| 旧程序摘要 | `50e35d57ad5936a205b7de6d3d9c5beeb79177b53002842ba4cb316b8b782e39` |
| 构建输入 | `599`个本地包、`5803`项输入，最终编译前后全部未变化 |
| 完整清单 | 暂存根的`release-manifest.json`与`build-inputs.before.json` |

独立窗口重新核对全部输入及产物摘要、两次构建退出码和宿主三范围差异，确认构建证据一致。编译成功不等于运行、数据库或业务验收通过。

当前`6030`监听进程为`PID 16312`，关联上述旧程序。只读进程检查没有发现显式配置参数或`server`子命令，源码默认命令为`http`。**没有取得该进程的工作目录、完整有效环境和配置来源，也没有可靠的旧程序逐文件源码清单。** 因此以下是相对当前`Git HEAD`的工作区范围，不能冒充相对运行程序的精确差异；程序回退副本已匹配，配置及启动环境回退尚未闭环。

## 22项工作区编译输入

以下路径均相对`apps/lina-plugins/idc-core/`。全部归既有`idc-core`所有，均为本轮之前已有的改动；本轮仅读取并编入候选，未重写或回滚。`M`表示已跟踪修改，`U`表示未跟踪新增，不是运行基线状态。

| 路径 | 状态与具体影响 |
| --- | --- |
| `backend/api/public/v1/commerce_cart.go` | `M`；移除`values`必填标签，允许无配置字段商品提交空选择；服务端配置及报价校验仍负责业务有效性 |
| `backend/internal/controller/account/account_new.go` | `M`；客户控制器强制注入交接服务；缺依赖时初始化失败 |
| `backend/internal/controller/public/controller.go` | `M`；公开控制器强制注入同一交接服务；缺依赖时初始化失败 |
| `backend/internal/service/contenthub/contenthub_audience.go` | `M`；客户及客户组候选搜索的`ESCAPE`改为单字符，保留参数化前缀匹配及原权限范围 |
| `backend/internal/service/contenthub/contenthub_read.go` | `M`；内容标题前缀搜索的`ESCAPE`改为单字符，编号精确匹配不变 |
| `backend/internal/service/contenthub/contenthub_upload.go` | `M`；本地分片改为对象旁的`.parts/`目录；到期时间显式传值；候选代次改用单值读取；清理时间参数化；修正候选文件插入占位符及代次转换 |
| `backend/plugin.go` | `M`；启动装配交接服务、注入两个控制器，并注册主节点清理任务；不是仅请求时才产生影响 |
| `backend/api/account/v1/purchase_configuration_handoff.go` | `U`；新增登录客户一次性领取接口的契约及响应 |
| `backend/api/public/v1/purchase_configuration_handoff.go` | `U`；新增匿名保存契约；仅选择意图，不接受价格或客户身份 |
| `backend/internal/composition/purchase_handoff_cleanup_job.go` | `U`；主节点每分钟最多扫描`100`个私有对象；非主节点不注册 |
| `backend/internal/controller/account/account_v1_purchase_configuration_handoff_claim.go` | `U`；读取浏览器绑定并委托客户鉴权、重评估及一次性领取 |
| `backend/internal/controller/handoffcookie/handoff_cookie.go` | `U`；保存时创建或复用主机限定、`HttpOnly`、`SameSite=Lax`绑定；读取不创建，非`TLS`不设置`Secure` |
| `backend/internal/controller/public/public_v1_purchase_configuration_handoff_save.go` | `U`；映射既有配置字段，保存并返回引用与到期时间；不创建车或订单 |
| `backend/internal/service/purchasehandoff/purchase_handoff.go` | `U`；交接服务入口，复用客户访问保护、商品评估和插件私有存储 |
| `backend/internal/service/purchasehandoff/purchase_handoff_cleanup.go` | `U`；按对象更新时间清理至少`16`分钟前的合规草稿对象；不删除数据库领取围栏 |
| `backend/internal/service/purchasehandoff/purchase_handoff_code.go` | `U`；集中定义无效、不可访问和不可用错误，不泄露保存内容 |
| `backend/internal/service/purchasehandoff/purchase_handoff_flow.go` | `U`；保存和领取均重评估配置版本；限制大小、数量、周期和字段数，拒绝文件令牌；领取要求有效客户 |
| `backend/internal/service/purchasehandoff/purchase_handoff_persistence.go` | `U`；复用既有幂等记录并加行锁，私有对象保存`15`分钟；一次性领取绑定客户摘要，无库表变更 |
| `manifest/i18n/en-US/apidoc/purchase-handoff.json` | `U`；英文接口说明资源 |
| `manifest/i18n/en-US/purchase-handoff.json` | `U`；英文交接错误资源 |
| `manifest/i18n/zh-CN/apidoc/purchase-handoff.json` | `U`；中文接口说明资源 |
| `manifest/i18n/zh-CN/purchase-handoff.json` | `U`；中文交接错误资源 |

用户此前明确允许补“临时保存选择、登录后恢复”；随后暂缓恢复专项。现有成果按保留原则编入候选，**此前开发授权不能替代当前完整程序发布及后台任务运行授权**。用户允许本地模拟数据验收，不等于本轮已获准创建账号、重新装载数据或将测试结果当正式经营验收。上传及搜索属于既有插件缺陷修复；本轮没有扩大其实现范围。所有收费、库存、客户归属、车、订单与支付继续由既有业务负责。

## 与原页面登录交互的对照

原小白云配置页的可见行为是：配置表单仍在原页提交报价，登录链接使用`target="_blank"`另开`login3.html`，登录后不会把原页选择交给原站后台；原脚本使用`pid`、`currencyid`、`qty`和`billingcycle`，并通过原站`/getLinkAgeList`和`ordersummary`请求联动与报价。这只能证明原页面交互，不能证明原站采用本方的`15`分钟草稿、浏览器绑定或每分钟清理机制。

当前本方官网的最低目标已简化为“进入用户中心后可继续配置”。因此交接草稿不是商品报价或订单业务的替代品，只是当前实现为保留已选字段而增加的适配层：官网保存不含价格、客户身份或文件令牌的意图，用户中心登录后一次性重新读取并由本方配置服务重评估，之后仍由用户中心自行报价和加车。若产品决定接受“登录后回到同商品、客户重新选择配置”，最小方案可以不依赖草稿保存和清理；但现有官网脚本和用户中心已实现并测试了`handoff`参数路径，本轮不擅自移除或缩小后端候选。是否改为最小方案需要明确的产品决定，不能依据原站字段名推断。

## 数据、启动与回退影响

- 没有新增或修改数据库结构、管理后台或用户中心包。候选仍包含当前所有官方源码业务插件；不等于每个插件及其嵌入资源已与旧程序逐项相同。
- 新公开保存路由为`POST /x/idc-core/api/v1/public/purchase-configuration-handoffs`；客户领取路由为`POST /x/idc-core/api/v1/account/purchase-configuration-handoffs/{reference}/claims`。调用会写既有幂等记录及插件私有对象，不是纯读取。
- 即使暂不调用交接接口，主节点启动也会注册草稿清理任务。必须核对存储支持及现存草稿，不得声称“不会写任何数据”。
- 上传修复对新分片路径及候选提升有影响。已有未完成上传、扫描及清理任务须在发布前盘点；不批量迁移、删除或重试旧分片。回退旧程序不能自动保证新旧上传会话兼容。
- 旧程序副本只解决程序字节恢复，不能撤销已经完成的草稿清理、领取或上传写入。批准前还缺有效配置、启动环境及相关存储恢复依据；不输出凭据，不把配置原文纳入公开包。

## 四项独立执行门槛

1. **官网升级**：核对现有`website-main v0.1.9`配置和旧包；经原插件管理流程升级`v0.1.10`，不重启主服务。检查实际入口加载版本化资源及`138`项摘要，再做桌面逐页回归。此项未执行。
2. **完整后端发布**：先由当前运行拥有方确认实际工作目录、环境、配置、恢复方式及上述完整影响，记录未完成上传／草稿与后台任务状态，再批准具体维护窗口。目标是替换`temp/output/lina.exe`关联的`6030`后端；将影响依赖此后端的管理端、用户中心及官网请求。现有不停启约束未解除，不执行停止、替换或启动，也不凭默认值编造启动命令。
3. **当前客户验收**：使用当前`6030/6032`的既有有效客户测试账号，从已升级官网进入同商品配置，正常登录后重新选择、取得原后台报价。平台账号、隔离账号或源码拦截页面均不能替代当前入口；购物车和订单写验收另列，不能仅凭报价通过宣布完成。
4. **旧身份及目录退役**：旧身份官方业务影响门禁仍未解除。保全文件与清理授权分开；不强制卸载、不删业务数据、不停止旧预览依赖。

## 历史候选保全结果

`B1`已按原精确计划只复制`15`份独有成果，并由副窗口独立核对全部源、目标及摘要。证据为`temp/20261018/20261018-semantic-merge-B1-result.json`。

`B2`已另存当前差异及原示例：`temp/20261018/1792300020084-website-legacy-B2/manifest.json`记录`94`份工作区文件及`24`份`HEAD`原示例基线。全部预检来源摘要，目标采用独占创建，复制后核对源和目标；原示例两份缺失文件仅从`HEAD`存入审计区，没有恢复到工作区。此批不覆盖`B1`，也不复制回官网主线。

副窗口已独立重读全部`118`份副本、`94`份当前来源和`24`份`HEAD`基线，核对原计划集合、目标唯一性及原工作区两份文件仍缺失，未发现保全问题。对本文`22`项范围、路由、清理周期和回退限制的独立复核亦未发现阻塞性表述问题；这不是完整后端实现审查或运行验收。本轮文档检查确认`22/22`输入均列明、相关本地链接存在，宿主三范围`git diff`为空；没有产品行为修改，不重复已通过的页面测试或编译。

`B1/B2`不是独立可运行网站、不是新的发布指令，不表示目录可以删除。`B3`归属决定、依赖解除、精确清理及正式退役均未执行。
