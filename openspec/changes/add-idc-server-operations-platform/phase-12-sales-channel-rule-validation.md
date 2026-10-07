# 第12阶段销售渠道规则实现记录

## 已完成内容

在`apps/lina-plugins/idc-core/backend/internal/service/saleschannel/`新增了可复用的销售渠道规则服务：

- `Evaluate`检查代理资格、商品范围、发布状态、生效/失效时间、最低毛利和逐币种信用额度，并输出带规则编号、版本和金额的价格快照；
- `EvaluateStored`按固定的四次有界查询批量读取资格、商品规则、价格规则和信用额度，避免按订单项目逐条查询；
- `Attribute`拒绝自荐、同一已认证主体、失效链接和失效触点；生产归因由服务端读取权威实名认证投影，不接受调用方传入同实名结论；
- `CalculateCommission`支持固定金额、比例、阶梯、封顶和成熟日期，并输出规则版本和阶梯编号；
- `Assign`检查销售归属保护期，要求转移方追加新归属，不修改历史归属记录。

本记录对应第12.2至12.9。后台和客户中心接口、权限接入、归因落库、账本写入、成熟任务、结算审核、打款流程、页面和浏览器验证均已接通。

## 同实名自荐补充

`PersistStoredAttribution`现已在同一事务中加载权威推荐链接、触点和实名认证投影。相同客户直接拒绝；不同客户仅在个人或企业主体类型一致、双方状态均为已认证且用途隔离摘要相同时，以`rejected_self_referral/same_identity`拒绝本次推荐奖励。后台请求 DTO 已删除`relatedIdentity`，调用方无法自行声明或绕过同实名结果。

命中自荐时，服务同时追加一条拒绝归因和一条`self_referral`统一营销待复核案件。案件只保存关联账号数量、稳定证据摘要和脱敏结论；不会保存认证原文或认证摘要，也不会自动封号、取消订单、扣余额或追回既有资金和权益。确定性编号、事务锁和唯一约束保证重放不重复落库。

## 验证

```text
$env:GOWORK='off'
$env:LINAPRO_IDC_CORE_TEST_DSN='pgsql:postgres:123456@tcp(127.0.0.1:5432)/cloud_idc_phase10_e2e_v2?sslmode=disable'
go test ./backend/internal/service/saleschannel -count=1
```

通过，包含纯规则单元测试和隔离`schema`的`EvaluateStored`真实`PostgreSQL`查询测试。

```text
.\make.cmd lint.go dir=apps/lina-plugins/idc-core plugins=1
.\make.cmd plugins.check
.\make.cmd i18n.check plugins=1
openspec validate add-idc-server-operations-platform --strict
git diff --check
```

以上门禁均通过。全插件并行回归中另有历史测试因共享测试库内存不足或数据库保护规则主动拒绝公共库，未归因于本次销售渠道规则代码；已用插件全包编译烟测和本包真实测试覆盖当前改动。

### `2026-08-31`当前电脑补充验证

```text
$env:GOWORK='off'
$env:LINAPRO_IDC_CORE_TEST_DSN='pgsql:postgres:123456@tcp(127.0.0.1:5432)/postgres?sslmode=disable'
go test ./internal/service/saleschannel -count=1 -v
```

通过。测试使用随机隔离 schema，覆盖普通归因、重复归因、不同客户同一已认证主体、拒绝归因、统一营销案件内容和重放幂等，结束后自动删除隔离 schema。

Go 1.27 下`go test ./api/admin/... ./internal/controller/admin/... ./internal/service/saleschannel -count=1`通过。`make.cmd plugins.check`和`make.cmd i18n.check plugins=1`通过。Go 1.27 下`make.cmd lint.go dir=apps/lina-plugins/idc-core plugins=1`中的`golangci-lint v2.12.2`为`0`项问题；固定`staticcheck v0.7.0`在导入 Go 1.27 标准库时因导出数据版本不兼容而失败，不是代码告警，但按任务级审查门禁仍阻断完成。插件全包测试的当前唯一失败是既有`idc-channel`客户/公开接口元数据声明管理员权限，与本轮销售渠道文件无关。

## 本批新增的真实接入

- 后台路由新增代理报价、AFF 归因、佣金计算三个受精确权限保护的接口，统一返回价格/规则版本快照；实现文件为 `backend/api/admin/v1/sales_channel.go` 和 `backend/internal/controller/admin/admin_v1_sales_channel.go`。
- 佣金成熟、结算门槛、打款结果未知处理已增加独立安全判断，并覆盖“部分退款、全额退款、风险冻结、收款资料未核验、供应商已受理不得重试”等单元测试。
- 运营工作台新增一次 SQL 聚合读取，返回待办队列和按原币种的账单金额，避免逐行查询；后台路由为 `/api/v1/admin/operations/workbench`。
- `idc-core/frontend` 新增隔离的运营工作台页面，支持加载、刷新、失败提示和中英文文案；插件构建和 bundle 内联样式校验已通过。
- 新增插件专属 E2E 用例 `TC016-operations-workbench.ts`，覆盖中文标题、队列、原币种金额、刷新和截图节点；四个业务端口启动后已完成真实页面回归。

## 客户中心接入补充

- `web-customer` 新增推荐概览、佣金概览、结算列表和结算申请四个接口适配及页面路由，结算申请先冻结可用余额，并用幂等键防止重复申请。
- `idc-core` 客户账户控制器已接入同名四个接口，所有客户编号都从当前登录会话取得，不接受前端传入其他客户编号；定向 Go 测试和客户中心类型检查、构建均通过。
- 客户中心浏览器用例为 `TC024-referral-commission.ts`，已用隔离 mock 验证中英文和移动端展示；客户中心`16677`端口已确认正常响应。

本轮补充了 `apps/lina-plugins/idc-core/hack/tests/e2e/TC017-sales-marketing.ts` 销售渠道契约回归：在浏览器上下文中分别调用代理报价、`AFF` 归因、资格转换校验和佣金计算接口，并检查销售渠道页面的中文标题、原币种展示和内部客户编号不泄露。由于 `TC015` 已由商品履约阶段占用，按模块连续编号使用 `TC017`，避免重复测试编号。客户中心使用未冲突的`TC024-referral-commission.ts`。两项用例均通过 E2E 治理，后台真实页面已在`19120/15666`运行时打开并确认正常加载。

## 最终收口

- `AFF`归因从数据库加载权威链接和触点，在事务内追加归因；重复和并发请求由唯一约束及数据库锁收敛。
- 佣金成熟任务按到期时间有界领取，成熟记录和账户账本在同一事务落库。
- 客户结算申请只冻结同币种真实可用余额；审核拒绝通过账本释放冻结金额，审核通过后才能进入同币种打款批次。
- 打款成功、明确失败和结果未知分开处理；结果未知只进入人工处理，不自动重复打款。
- 后台真实页面`http://127.0.0.1:15666/admin/idc/sales-channel`已完成浏览器检查。
