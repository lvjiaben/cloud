# 第五阶段订单取消、到期与风控审核验证记录

> **`FB-52`部分失效声明：**订单取消、支付关闭、风控和 Outbox 的状态机可作为迁移对账参考；等待“逐台服务器履约单元”和资源池库存的结论已经失效，不能作为当前通用订单完成证据。现行取消只处理通用交付记录、尚未越过外部调用边界的履约意图和商品版本简单库存，具体服务器资源影响由服务器插件拥有。

## 完成范围

本轮完成任务`5.5`中由`idc-core`拥有的订单生命周期边界：

- 商家付款有效期策略作为单例本地配置，在结账事务中使用 PostgreSQL 权威时间把同一截止时间固化到订单、待付账单和本地库存预留；继续复用`5.4`客户结账摘要幂等，不创建第二套重复点击事实。
- 客户主动取消和系统到期取消共用同一取消引擎。仅允许仍未付款、未进入不可逆履约且资金影响全为零的订单在一个事务中取消订单、账单、等待履约单元和待处理审核，释放或过期库存，冲回营销名额，追加取消记录与 Outbox。
- 到期作业由源码插件`pluginhost.Jobs`声明，只在主节点执行；每轮默认最多处理 32 条、调用上限 100 条，逐条事务使用`FOR UPDATE SKIP LOCKED`领取，事务中没有支付渠道、Provider、通知或其他外部调用。
- `payment_processing`订单只创建唯一`pending_payment_closure`取消意图并发出关单请求事件。支付 owner 只能以`closed/no_attempts/confirmed_success/unknown`明确结果、引用和证据摘要回报；前两者完成本地取消，后两者转人工核对，核心不猜测资金状态。
- 风控审核使用追加式独立证据、操作人和审计。批准与驳回使用不同精确权限和两条`PUT`后台资源；批准只解除风险门禁并保留库存人工确认或人工履约，驳回复用统一取消事务。

本轮没有实现支付渠道中的支付尝试表、真实关单执行、迟到成功款归属或异常款退款；这些继续由`7.1`与`7.14`的`idc-payment`owner负责，当前不得宣称完整支付关单已经闭环。

## 数据库与事务不变量

`024-commerce-order-lifecycle.sql`新增付款策略、订单取消和订单审核权威表，补充营销预占冲回时间与状态、订单所有者和到期扫描索引，以及支付关单结果字典。迁移可重复执行，标准 DAO/DO/Entity 已重新生成；关单结果使用可空指针表达尚未回报，不用空字符串伪装事实。

本地取消先锁订单与账单，再用一次集合投影检查支付分配、活动资金锁定、账本流水、库存、履约和营销状态。只有账单的已付、可退和已退款金额都为零，且不存在支付分配、活动资金锁定与账本流水、库存全部仍为`held`、全部履约单元仍为`pending_payment`时才继续。营销计数、客户计数和优惠资产次数在同一事务冲回；任何 SQL、营销、Outbox 或员工审计失败都会回滚全部状态。

到期领取明确投影订单所需字段而不使用`SELECT *`。取消事务按“转换校验、资金/资源状态检查、营销冲回、资源状态更新、取消记录、Outbox”拆分小函数，但仍由调用方的同一个 PostgreSQL 事务包裹，锁序和提交边界没有变化。

## 接口与权限

新增两条后台接口：

- `PUT /admin/commerce/order-risk-reviews/{reviewNo}/approval`，权限`idc:admin:commerce:risk-review:approve`；
- `PUT /admin/commerce/order-risk-reviews/{reviewNo}/rejection`，权限`idc:admin:commerce:risk-review:reject`。

接口只接受审核编号、稳定幂等请求号和小写 SHA-256 证据摘要，决定由路由固定，客户端不能在同一通用接口中切换批准或驳回。响应只返回有界业务投影，审核时间使用 Unix 毫秒。授权服务拒绝客户主体、非活动员工、冒充、跨商家视图和不可表达数据范围，并在访问目标前检查对应精确权限；宿主通配权限和平台管理员仍按宿主统一治理显式处理。

插件清单已提升到`v0.1.37`并登记两项隐藏按钮权限，中英文菜单、业务错误和中文 API 文档翻译已补齐。标准`make ctrl`生成了 admin 接口聚合与两条控制器骨架；生成器同时对已有手写 public 聚合产生了重复一次性骨架，已只删除本次多余生成文件并保留原 public 实现，未修改生成工具或宿主源码。

## 验证结果

1. `LINAPRO_IDC_CORE_CODEGEN_DSN=... GOWORK=off go test ./backend -run '^TestOrderLifecycleCodegenPostgreSQL$' -count=1 -v`通过；测试先确认数据库名称精确为`linapro_idc_codegen`，再连续执行迁移两次。
2. 配置隔离 PostgreSQL 后，取消、Outbox 回滚、支付关单、到期扫描和风控审核 7 组真实事务测试全部通过：8 路相同取消只生成一份取消事实；不同幂等请求冲突；营销全局/客户名额与优惠资产次数完整冲回；Outbox 或审计失败整体回滚。
3. 风控批准覆盖“风险是唯一门禁”和“仍存在人工履约门禁”两种场景，前者回到`pending_payment`，后者保持`manual_review`；风控驳回在同一事务完成订单取消。
4. 控制器测试证明批准/驳回映射为不同服务决定、证据摘要原样传递且响应时间为 Unix 毫秒；`TestIDCPluginAPIPermissionsUseAdminNamespace`证明清单中的两个权限都有受保护后台 API，且 API 权限都已登记。
5. `GOWORK=off go test ./backend/... -count=1`通过；包含 API DTO、控制器、宿主作业装配、授权器和全部插件后端包。
6. `make.cmd lint.go plugins=1 dir=apps/lina-plugins/idc-core/backend`通过，`golangci-lint`和`staticcheck U1000`均为 0 问题；审查中发现并修复取消函数复杂度超限和到期领取`SELECT *`问题，修复后重新运行全部真实事务测试通过。
7. `make.cmd plugins.check`扫描 11 份插件清单、987 个文件与 965 个 Go 文件，治理问题为零。
8. `openspec validate add-idc-server-operations-platform --strict`通过。
9. `make.cmd build plugins=1 platforms=windows/amd64,linux/amd64 verbose=1`通过；宿主前端、Portal 插件以及 Windows/Linux 后端正式产物均成功生成。构建日志中的既有 TypeScript 声明提示不影响构建成功。
10. `make.cmd i18n.check plugins=1`确认运行时硬编码与宿主/插件消息覆盖均通过；完整仓库前端键覆盖仍仅被禁止修改的宿主`notice-preview-modal.vue`缺少中英文各 3 个既有键阻断，共 6 项。本轮插件新增菜单、错误与 API 文档没有缺键，因此父项和子项继续不勾选直至宿主基线修复。

## 复审结论

本轮按`lina-review`检查了客户所有权、对象存在性、支付与资金前置条件、订单锁竞态、营销守恒、两段式关单、迟到成功款边界、到期扫描上限、主节点作业、风险门禁独立性、精确权限、证据摘要、审计原子性、API 权限登记、时间契约和同步外部调用。审查期间修复了接口权限未绑定 API、生成器重复骨架、取消函数复杂度以及到期查询宽投影；修复后的真实 PostgreSQL、全量测试、lint、插件治理、严格规范与正式构建均通过。当前没有遗留的严重或高优先级问题。

## 规则影响记录

- 架构与插件：生产逻辑闭环在`idc-core`，没有修改`lina-core`、`lina-vben`、根构建工具或其他业务插件；作业只通过宿主统一 Jobs 扩展点声明。
- 数据权限：客户取消只从当前活动客户主体取得所有者；后台风控决定由直接活动员工、默认商家、精确权限和可表达数据范围共同授权，未授权和不存在目标统一失败关闭。
- 数据库：新增幂等迁移、索引、约束、字典与生成模型；金额保持整数最小货币单位，时间使用 PostgreSQL 权威时间，历史记录不使用软删除。
- API：新增两条后台风控决定接口并执行标准 Controller 生成、权限登记、中文 API 文档和映射测试；没有新增客户门户 HTTP 取消接口，该入口仍由`5.7`拥有。
- 前端与 E2E：本轮没有页面、组件或用户可观察前端流程变更，因此没有 Chrome 实测或 E2E 影响；菜单仅登记隐藏权限资源。
- `i18n`：插件新增中英文菜单、错误消息和中文 API 文档翻译均通过覆盖检查；仓库级既有宿主缺键仍是唯一阻断。
- 缓存与外部系统：取消、到期和审核均不使用缓存；同步事务不访问支付、Provider、通知或 AI。支付实际关单继续由后续 payment owner完成。
- Git：未执行暂存、提交、推送或创建 PR。

**已读取规则：**`AGENTS.md`、`.agents/rules/openspec.md`、`.agents/rules/documentation.md`、`.agents/rules/architecture.md`、`.agents/rules/data-permission.md`、`.agents/rules/plugin.md`、`.agents/rules/api-contract.md`、`.agents/rules/backend-go.md`、`.agents/rules/database.md`、`.agents/rules/testing.md`、`.agents/rules/i18n.md`、`.agents/rules/dev-tooling.md`
