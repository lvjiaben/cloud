# 附属购买结清断链：局部实现与验证

日期：`2026-09-04`。关联任务：`FB-114`。本记录仅覆盖原有付费附属订单的结清衔接，不是完整流量包上线或验收记录。套餐库存和删除方式仍待用户确认；固定套餐管理、客户购买界面、云页面桥及真实上游执行不在本次通过范围。

## 根因与修复边界

原付费附属结账未生成财务链所需的报价及订单库存快照；同时，正式结清消费者只分发到通用新购履约，无法从`awaiting_financial_settlement`推进原服务器操作。把订单的`manual_confirmation`改成`available`会混淆新购与附属操作，因此不能作为修复。

- 结账事务补齐非库存快照，数量仍为一个附属订单项，库存追踪关闭，库存版本和可售数量均为零。已有不一致快照拒绝，不能覆盖；不占用或扣减商品库存。
- 财务模块提供只读`SettlementEvidenceReader`，复用现有持久化结清投影与来源验证。缺少或不一致的资金证据拒绝，不通过调用`SettleOrder`补造资金事实。
- 结清路由先识别持久化的附属意图。异常附属关联也不得回退到新购消费者；普通新购保留原路径。
- 锁定订单、账单、原服务器和业务操作后，验证客户、币种、金额、报价、版本及唯一事件；同一事务只创建一次原服务器执行、业务事件和交付回执，不创建新服务器。
- 重复结清只返回原回执，不将已成功或结果未知的任务重新置为待执行。流量原因暂停的原服务器保持暂停，不自动开机或解除暂停。

## 权限、依赖与性能

该消费者是内部结清任务，不是新增客户或后台接口。授权来自已通过原客户结账门禁的持久化操作，查询在数据库侧绑定原客户、订单、账单和服务器；任何关联或证据不一致均拒绝。原结账保留真实客户或实际协助员工审计，异步阶段追加系统事件，不伪造员工会话。

`plugin.go`在启动期将同一`g.DB()`后端注入财务只读证据读取器及附属消费者，再与原通用消费者一起注入唯一结清路由和任务执行器。没有在请求或任务循环里创建新服务图；没有增加宿主能力、动态插件协议、跨插件后门或角色权限。

证据采用固定次数的联表与有界投影，不按商品行数发起单项查询。分类仍保留严格的原订单关联，不为走索引而增加可能漏判异常订单的账单联结。新增`109-server-accessory-settlement-index.sql`为该路径建立非唯一局部索引；它只有幂等索引定义，没有新表、初始业务数据或演示数据，不改变收费、删除、唯一性或状态语义。

## 生成代码范围

原服务器执行表及报价、订单库存快照表已有真实迁移，但缺少生成模型。本轮使用官方`make dao`定向读取三张既有表，新增每表的`DAO`、内部`DAO`、`DO`及`Entity`，共十二个文件；不手写生成文件。

生成使用临时限定配置及只读连接，永久插件`hack/config.yaml`只追加三表及必要空值映射，确保以后可重复生成。生成前确认目标文件不存在，生成后对既有模型指纹比对，未覆盖其他生成文件；开发数据库未改表或重置。新增生产写入使用生成`DO`，自动创建和更新时间交给框架。

工具仍走跨平台的官方生成入口；本机验证为`Windows`，没有声称在其他系统执行过。连接信息仅通过子进程内存传入，不保存在本记录或正式配置中。没有改动宿主`hack/`工具。

## 测试边界

测试使用随机隔离的`PostgreSQL`结构，测试自行创建并清理；没有动实际客户余额、支付渠道或上游服务器。完整链测试使用真实迁移、外键和不可变约束，经公开客户结账方法、真实余额付款、权威结清、只读证据和正式分流，最终检查原服务器唯一执行记录。身份和审计边界使用明确测试替身，不能把它描述为浏览器或真实第三方付款验收。

独立消费者用例另覆盖流量、快照及备份的并发重放、十六类证据异常、事件写入失败回滚及重试。索引测试使用`20,001`条操作记录，未禁用顺序扫描、未强制执行计划，两条实际查询均选择新增索引；重复应用迁移两次仍只有一个索引。

## 审查状态与未完成项

`lina-review`按本次明确文件范围执行，不把未跟踪的整个`idc-core/backend/`当成全部审查过。已读取顶层入口与用户偏好、文档、开发流程、架构、数据权限、插件、后端、数据库、接口、测试、国际化、缓存及开发工具规则；`idc-core`根目录不存在本地`AGENTS.md`覆盖规则。

无新增前端、接口契约、运行时翻译键或缓存；引用已有状态、业务错误和审计身份，不更改用户可见规则。新建局部索引、生成模型、依赖来源、数据库侧归属校验、重放和事务失败均已复核；未发现本次局部补丁的阻塞问题。完整功能的剩余风险仍如下，不以局部通过代替整项验收。

### 精确代码与配置范围

以下路径均相对于`apps/lina-plugins/idc-core/`，共`28`个文件；审查只覆盖本轮改动和新增文件，不覆盖这些目录下其他既有工作。

- `backend/plugin.go`。
- `backend/internal/service/billing/`下的`billing.go`、`billing_settlement_reader.go`、`billing_settlement_reader_test.go`。
- `backend/internal/service/fulfillment/`下的`fulfillment.go`、`fulfillment_routed_consumer.go`、`fulfillment_routed_consumer_test.go`。
- `backend/internal/service/serveroperation/`下的`serveroperation_accessory_checkout.go`、`serveroperation_accessory_checkout_postgres_test.go`、`serveroperation_accessory_checkout_chain_postgres_test.go`、`serveroperation_accessory_settlement.go`、`serveroperation_accessory_settlement_evidence.go`、`serveroperation_accessory_settlement_test.go`。
- `backend/internal/service/commerce/product_fulfillment_worker_postgres_test.go`，仅适配结清路由构造及非附属测试替身。
- 生成文件：`backend/internal/dao/`、`backend/internal/dao/internal/`、`backend/internal/model/do/`和`backend/internal/model/entity/`各自的`server_operation_execution.go`、`commerce_quote_item_inventory_snapshot.go`、`commerce_order_item_inventory_snapshot.go`，共十二个。
- `hack/config.yaml`和`manifest/sql/109-server-accessory-settlement-index.sql`。

### 当前版本验证证据

- 官方定向生成：`./make.cmd dao dir=temp/20260904/152309-addon-dao`通过，连接强制只读，三表结构与既有迁移一致，全部既有生成文件指纹不变。
- 库存与完整本地链：`go test ./internal/service/serveroperation -run '^TestAccessoryCheckout' -count=1 -v`，四个顶层测试、七个实际场景通过；最终生成`DO`版本用时`35.791`秒。
- 附属结清、只读财务与事件路由的最终定向测试通过；隔离数据库轮次分别为服务器操作`12.698`秒、财务`0.851`秒和路由`0.550`秒。未付款、重复事件、八路并发、十六类证据损坏、未知及成功任务重放、审计回滚和索引幂等均覆盖。
- 相关服务器操作、财务、履约包及插件启动绑定包的`go test -count=1`通过；未带数据库连接的普通包运行中，其他需要数据库的测试按显式条件跳过，不将跳过报告为实测。
- 实际尝试`./make.cmd lint dir=apps/lina-plugins/idc-core plugins=1`，当前`Windows`包装入口报告不支持`lint`。随后使用`backend-go.md`明确允许的官方`linactl lint.go`路径，即`./make.cmd lint.go dir=apps/lina-plugins/idc-core plugins=1`，覆盖本轮插件模块并通过，零问题；没有裸跑静态工具或修改包装器来绕过门禁。
- `./make.cmd i18n.check plugins=1`通过，零违规；`openspec validate add-idc-server-operations-platform --strict`通过。

最终组合命令在`apps/lina-plugins/idc-core/`执行；隔离数据库连接由测试进程环境提供，不写入记录：

```powershell
go test ./backend/internal/service/serveroperation ./backend/internal/service/billing ./backend/internal/service/fulfillment -run 'TestAccessoryCheckoutToOriginalServerPostgreSQL|TestAccessoryCheckoutNonStock|TestAccessorySettlement|TestSettlementEvidenceReader|TestSettlementRouting' -count=1 -timeout=180s
go test ./backend/internal/service/billing ./backend/internal/service/serveroperation ./backend/internal/service/fulfillment ./backend -count=1 -timeout=180s
go test ./backend/internal/service/commerce -run '^$' -count=1
```

本轮没有重启服务或向开发库部署`109`索引。正式运行前仍需通过插件正常升级迁移路径应用；普通重启不等于执行迁移。完整流量包功能、真实上游写入、信用支付附属订单完整链、页面验收、库存与删除规则仍未完成，`FB-114`保持未勾选。
