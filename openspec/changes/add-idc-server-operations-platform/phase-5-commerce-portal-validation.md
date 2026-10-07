# 第五阶段客户交易门户与后台订单目录验证记录

> **`FB-52`部分失效声明：**客户所有权、金额格式、游标、幂等与有界聚合可以保留；独立旧 Portal、逐台服务器履约详情、资源池库存和服务器专用配置已经失效，不能作为当前`web-customer`通用交易页面完成证据。现行页面必须展示通用交付记录与履约意图，长期权益仅在冻结契约声明持续管理时出现。

## 完成范围

本轮完成任务`5.7`的客户交易接口、订单读模型、独立 Portal 页面和 LinaPro 后台订单工作台：

- 在`/account/commerce`装配购物车打开/读取、配置加入/移除、最终报价、原子结账、付款准备/恢复、取消、订单列表和详情接口。客户身份只从当前活动主体取得，请求不接受`customer_no`。
- 新增客户与后台分离的订单目录服务。后台列表和详情分别使用`idc:admin:commerce:order:list`与`idc:admin:commerce:order:query`；客户列表响应清除员工专用客户编号。
- 列表使用部署密钥用途隔离的 AES-GCM 键集游标，不返回总数；游标绑定入口、客户、筛选和页大小。详情在一次只读可重复读事务内，以固定有界查询聚合订单、账单、项目、逐台履约、库存、取消、审核和付款准备。
- 独立 Portal 实现响应式购物车、重新核价、报价差异、结账、实名后恢复、订单列表、详情与安全取消。金额全程是十进制最小货币单位字符串并由`BigInt`格式化；结账和取消使用会话内持久的随机 UUID 幂等键。
- LinaPro 后台新增订单队列和聚合详情抽屉。待风控订单只展示事实；在详情未携带完整证据摘要时批准/驳回按钮保持禁用，不伪造摘要或绕过原有精确权限。

Portal 插件版本由`v0.1.3`提升到`v0.1.4`，核心插件版本为`v0.1.39`。本轮没有数据库迁移或缓存语义变化。

## 权限、隐私与性能边界

客户接口不允许提交客户编号；账户订单服务先解析活动客户主体，再把所有查询限制到该客户。订单不存在与非本人订单统一返回不可访问，避免通过业务编号探测存在性。后台服务只接受默认商家的直接活动员工，拒绝客户主体、冒充、代操作、跨商家和不可表达数据范围。

客户列表默认 20、最多 50 条，后台默认 25、最多 100 条；列表只投影当前页，不执行总数查询。详情最多返回 100 个订单项目、1000 个逐台履约单元、1000 条库存事实和 100 条审核事实，响应不包含数据库主键、内部摘要、资源池绑定、Provider 信息、凭据或原始载荷。前端分别只调用一个列表接口和一个聚合详情接口，不按订单或服务器逐行补查。

金额接口统一返回十进制字符串，浏览器以`BigInt`处理，已验证超过 JavaScript 安全整数范围的金额仍不丢精度。时间点统一为 Unix 毫秒。结账成功后页面锁定提交按钮；刷新恢复后对同一报价重试仍复用原随机幂等键并由服务端返回首单。

## 自动化与浏览器验证

1. `GOWORK=off go test ./backend/... -count=20 -shuffle=on`通过，包含核心插件全部后端包的随机顺序 20 轮回归。
2. 配置隔离 PostgreSQL 后，`go test ./backend/internal/service/commerce -run '^TestOrderDirectoryAggregatePostgreSQL$' -count=10`通过；每轮创建并删除随机 schema，验证客户所有权、客户编号隐藏、聚合边界和后台投影。
3. `go test ./backend/internal/controller/account ./backend/internal/controller/admin -count=1`通过；额外验证`int64`最大金额保持十进制字符串、时间保持 Unix 毫秒且客户响应省略员工专用客户编号。
4. 标准`linactl ctrl`生成流程已执行。生成器对已有手工聚合 Public Controller 仍会产生重复空壳，本轮删除这些重复产物并保留原有单聚合实现；最终`go test ./backend/... -run '^$'`编译门禁通过。
5. `linactl lint.go dir=apps/lina-plugins/idc-core`通过，`golangci-lint`和`staticcheck U1000`均为 0 问题。路由装配按目录、营销和交易职责拆分，插件包复杂度门禁通过。
6. `linactl plugins.check`扫描 11 份插件清单、1015 个文件和 993 个 Go 文件，治理问题为零。
7. Portal `pnpm run build`通过，产物使用`v0.1.4`版本化静态资源路径；LinaPro 工作台源码插件类型检查与正式生产构建均通过。
8. Portal `TC007-commerce-checkout.ts`在真实 Chromium 中 5/5 通过，覆盖商品配置进入购物车、二次核价与差异提示、刷新后稳定幂等重试、实名恢复、安全取消、未登录隔离、完整英文文案和移动端无横向溢出。
9. 核心插件后台`TC004-commerce-orders.ts`在真实登录工作台中 2/2 通过，覆盖加密游标翻页、超过 JavaScript 安全整数的精确金额、单次聚合详情、逐台履约、库存、审核事实和危险风控入口禁用。
10. 截图审查通过：[客户结账](../../../temp/20260810/171446-idc-commerce-checkout-zh-cn.png)、[移动端订单](../../../temp/20260810/171447-idc-commerce-orders-mobile.png)和[后台订单详情](../../../temp/20260810/171504-idc-admin-commerce-order-detail.png)。审查后进一步锁定已创建订单的提交按钮，并强制后台金额单行显示；修改后 7 个浏览器场景再次全部通过。
11. `openspec validate add-idc-server-operations-platform --strict`通过。

## 已知仓库级阻断

`linactl i18n.check`确认本轮插件运行时硬编码、消息资源和前端插件键均无新增问题；完整仓库仍只被禁止修改的宿主`notice-preview-modal.vue`缺少中英文各 3 个既有键阻断，共 6 项。因此`5.7`及其子项保持未勾选，但实现和任务内验证已经完成。

仓库 E2E 总体验证脚本在 Windows/Node 24 下调用`spawnSync('pnpm')`时返回未定义的`stdout`并触发`TypeError`；该文件位于禁止修改的宿主`hack/tests`。本轮两个新增测试文件均能被 Playwright 正确枚举，且分别真实执行 5/5 与 2/2 通过。全工作区 Go lint 另被宿主`hack/tools/linactl/internal/process/process_windows.go`既有未检查`CloseHandle`错误阻断；核心插件定向 lint 已通过。

## 规则影响记录

- 架构与插件：业务后端、Portal、管理页面、测试和资源均闭环在`idc-core`与`idc-portal`，没有修改核心宿主源码。
- 数据权限：客户所有权只来自活动主体；后台列表和详情权限独立，直接员工和数据范围在访问目标前验证。
- API：新增版本化客户与后台接口，金额为字符串、时间为 Unix 毫秒、列表为有界加密游标、详情为单聚合；标准 Controller 生成和编译门禁已执行。
- 数据库与缓存：没有 SQL、DAO 或缓存变更；订单目录只读现有本地 PostgreSQL，未增加外部调用。
- 前端与`i18n`：Portal 与 LinaPro 页面均提供完整中英文文案和响应式布局；用户可观察流程由 6 个真实 Chromium 场景及截图验证。
- 外部系统：结账页面不伪造支付成功，订单详情不调用 Provider；真实支付渠道仍由阶段`7`接入。
- Git：未执行暂存、提交、推送或创建 PR。

**已读取规则：**`AGENTS.md`、`.agents/rules/openspec.md`、`.agents/rules/documentation.md`、`.agents/instructions/markdown-format.instructions.md`、`.agents/rules/architecture.md`、`.agents/rules/data-permission.md`、`.agents/rules/plugin.md`、`.agents/rules/api-contract.md`、`.agents/rules/backend-go.md`、`.agents/rules/frontend-ui.md`、`.agents/rules/testing.md`、`.agents/rules/i18n.md`、`.agents/rules/dev-tooling.md`
