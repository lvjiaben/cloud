# 第五阶段购物车服务验证记录

## 实现范围

本轮完成任务`5.2`的服务层闭环，不提前建设`5.3`报价快照、`5.4`结算事务或`5.7`门户页面：

- 商品域新增内部`PurchaseConfigurationResolver`，从公开商品短标识、签名配置修订、签名计费周期、币种和选择项反查当前商品版本、计费规则与双语规范化配置。交易域不接受浏览器提交的`product_no`、`version_no`、`billing_rule_no`、内部名称或规范化值。
- 加入购物车前固定次数读取配置图、价格/税务事实和选择项安全投影；完成后再次检查当前发布版本。批量失效检查最多接受 100 个购物车项、合计 5000 个选项，通过商品、分类、版本、计费规则和选项五组集合查询完成，不在项目或选项循环中访问数据库。
- 游客令牌由 256 位随机数生成，原文仅在创建结果返回一次；数据库只保存小写`SHA-256`摘要。客户所有权只由`principal.Reader.Current`解析的活动客户受众提供，服务命令没有客户端`customer_no`字段。
- 活动购物车只允许一个币种。不同币种加入和登录合并返回稳定冲突且不写部分结果；显式换币只允许空购物车，并通过关闭旧车、创建新车保留历史，避免原地修改未来可能被不可变报价引用的复合外键。
- 登录合并用客户级 PostgreSQL 事务咨询锁串行化所有者命令，再锁定来源和目标。客户没有活动购物车时仍创建独立客户目标；游客项目使用一次集合读取和一次批量插入复制到目标，原项目批量标记为已合并移除，来源车保留`merged_into_cart_no`。重复请求读取同一来源凭据并返回同一客户目标。
- 购物车读取固定装配购物车头、最多 100 个项目、最多 5000 个配置项和商品域批量状态，不写数据库。商品下架、版本替换、计费规则停用和配置变化分别返回稳定问题代码及`remove`、`replace_version`、`requote`、`reconfigure`修复动作。
- 新增中英文购物车错误资源，插件版本由`v0.1.32`提升为`v0.1.33`。

## 审查结论

- 独立审查发现“直接修改游客购物车项的`cart_no`”会在后续已有不可变报价项目时破坏`(cart_no, cart_item_no)`复合外键，已改为批量复制目标购买意图并保留来源项目历史。
- 独立审查发现客户空购物车并发创建可能使其中一个正常请求落入唯一键错误，已让开户、换币与登录合并共享客户级事务咨询锁；真实 PostgreSQL 两工作线程验证只产生一个活动购物车且两个请求返回同一业务编号。
- 独立审查发现初版批量复核没有限制合计选项数且会读取目标版本的全部计费规则，已增加 100 项/5000 选项总界限、逐项目重复选项拒绝，并把计费查询收窄到请求中的业务编号集合。
- 接口影响：本轮新增 Go 内部服务契约，没有新增或修改 HTTP API、路由、请求 DTO、响应 DTO 或 OpenAPI 元数据；HTTP 与门户接入归任务`5.7`。
- SQL 影响：本轮复用`022-commerce-foundation.sql`，没有新增迁移或修改表结构。所有命令使用数据库时钟、事务、唯一约束、行锁或咨询锁；没有物理删除。
- 数据权限影响：游客只凭高熵令牌摘要精确命中；客户只读取当前活动客户受众，不接受客户编号，不允许员工受众退化为客户，不暴露目标存在性。
- 缓存影响：无。购物车、商品失效和所有权均读取 PostgreSQL 权威事实，没有引入缓存或失效路径。
- `i18n`影响：新增的 7 个错误码均有`zh-CN`和`en-US`资源；插件运行时和消息覆盖通过。仓库总门禁仅剩宿主通知预览页已有的 6 个前端键缺失，本轮因根目录不存在`.contributing`不得修改宿主。
- 前端与 Chrome：本轮无页面、路由或用户可见前端变更，不适用 Chrome 页面验证；门户购物车将在任务`5.7`按真实页面门禁验收。
- Git：未暂存、未提交、未推送。

## 验证证据

以下命令在`2026-08-10`执行：

1. `go test ./backend/internal/service/catalog ./backend/internal/service/commerce -count=20 -shuffle=on`：通过。
2. 设置隔离 PostgreSQL 测试 DSN 后执行`go test ./backend/internal/service/commerce -run TestCommerceCartServicePostgreSQL -count=20 -shuffle=on`：通过，覆盖令牌摘要、加入项目、非空换币拒绝、同来源并发合并与重放、来源/目标证据、版本替换提示、移除修复、并发开户、币种冲突回滚、显式空车换币和换币后合并。
3. 清除测试 DSN 后执行`go test ./backend/... -count=20 -shuffle=on`：通过。
4. `make.cmd lint.go plugins=1 dir=apps/lina-plugins/idc-core/backend`：`golangci-lint`零问题，`staticcheck U1000`通过。
5. `make.cmd plugins.check`：扫描 914 个文件，零治理发现。
6. `openspec validate add-idc-server-operations-platform --strict`：通过。
7. `make.cmd build plugins=1 platforms=windows/amd64,linux/amd64 verbose=1`：后台、门户、11 个插件以及 Windows/Linux 后端正式构建成功；构建日志仍显示宿主已有`TS4058`和大分块警告，但命令成功退出。
8. `make.cmd i18n.check plugins=1`：插件运行时扫描、消息覆盖通过；仓库级检查失败项仍仅为`apps/lina-vben/apps/web-antd/src/views/system/message/notice-preview-modal.vue`在中英文各缺 3 个`content-notice`键，共 6 项，与本轮变更无关且属于禁止修改的宿主范围。

## 未完成边界

`5.2`实现和任务内验证已经完成，但因上述仓库级`i18n`既有阻断，父项及子项保持未勾选。下一步进入`5.3`集合化确定报价：它将消费本轮购物车与商品购买投影，创建新的不可变报价快照；不得把本轮读取态失效投影伪装为报价或金额承诺。
