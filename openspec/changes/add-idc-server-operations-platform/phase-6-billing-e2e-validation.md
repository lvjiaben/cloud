# 阶段 6.9 客户财务中心 E2E 验证

## 场景归属与编号治理

任务原文字指定 core 插件`TC004-billing-ledger.ts`，但 core 的`TC004-commerce-orders.ts`和 Portal 的`TC004-route-isolation.ts`均已存在。账单、余额和自动续费是独立客户 Portal 的真实用户入口，不应为了沿用过期编号把客户流程复制到 LinaPro 管理端。

本轮按照“测试跟随功能 owner、TC 编号在模块内唯一”的规则，在 Portal 分配下一个可用编号`TC008-financial-center.ts`。用例复用独立 Portal 的公开 Playwright 配置、页面对象和截图路径，没有修改宿主测试 runner 或制造重复编号。

## 四项浏览器场景

| 场景 | 浏览器证据 |
| --- | --- |
| `TC-8a`充值边界 | 在 390px 移动端展示超过 JavaScript 安全整数的精确余额，支付 owner 缺失时显示明确提示并禁用充值按钮，全页无横向溢出且没有任何充值写请求 |
| `TC-8b`余额支付 | 从账单详情确认余额支付，请求体只有随机`portal-UUID`幂等键，不接受客户、金额、账本或状态字段；服务端权威详情刷新后显示支付成功、已付清和零欠款 |
| `TC-8c`账单合并 | 以相反点击顺序选择两张账单，页面提交排序后的来源编号和稳定幂等键，不提交客户或目标状态；服务端返回后列表刷新为派生账单并显示成功提示 |
| `TC-8d`续费失败提醒 | 自动续费余额不足时显示可行动的双语提醒和下次尝试时间，不把`balance.insufficient`等内部失败码暴露给客户 |

测试 fixture 保持金额为最小货币单位字符串，并把写请求完整记录后断言字段白名单。它只隔离浏览器网络、验证页面契约，不替代阶段 6.8 已通过的真实 PostgreSQL 账本、账单、续费和调度状态机测试。

## 页面增强

- Portal 现在把`insufficient_balance`、`retryable_failure`和`exhausted`三类自动续费结果投影为明确的中文和英文客户提示。
- 有`nextAttemptAt`时显示本地化下次尝试时间；原始`latestAttemptFailureCode`不进入可见 DOM。
- 余额不足与可重试失败使用警告层级，重试耗尽使用错误层级。提醒位于对应服务卡片内，不会与全局续费说明混淆。
- `idc-portal`版本由`v0.1.5`提升到`v0.1.6`，Portal 包版本同步为`0.1.6`。生产构建生成新的独立`FinancialCenter`代码块。
- 视觉证据为[自动续费余额不足提醒](../../../temp/20260811/085356-idc-financial-renewal-failure.png)。桌面截图中提醒与服务、策略入口和下次尝试时间保持清晰层级；移动端由浏览器断言确认无页面横向溢出。

## 已执行验证

| 验证项 | 结果 |
| --- | --- |
| Portal `TC008-financial-center.ts`真实 Chromium，单 worker | 通过；4/4 场景，总耗时约 3.6 秒 |
| Playwright 用例发现与 TypeScript 编译 | 通过；4 个测试，1 个文件 |
| Portal `vue-tsc`和 Vite 生产构建 | 通过 |
| Portal 后端全量 Go 测试 | 通过 |
| Portal 后端 Windows/Linux amd64 交叉构建 | 通过 |
| Portal Go 静态检查 | 通过；`golangci-lint`和`staticcheck U1000`均为 0 问题 |
| 插件治理扫描 | 通过；11 个清单、1196 个文件、1174 个 Go 文件、0 项发现 |
| 运行时硬编码和宿主/插件消息覆盖 | 通过；0 个生产硬编码问题，新增双语文案无缺失 |
| 仓库级前端多语言键覆盖 | 仍仅被宿主通知预览页中英文各 3 个既有缺键阻断，共 6 项 |
| 仓库 E2E 静态治理脚本 | 被宿主`validate-e2e.mjs`读取未定义`spawnSync.stdout`的既有`TypeError`阻断；新增文件仍已由 Playwright 实际编译和运行 |

## 真实充值边界

当前系统尚未交付阶段 7 的支付 owner。余额充值必须由支付 owner 创建充值意图，并在支付宝、微信或易支付的可信确认后调用账本确认入口；core 或 Portal 不能自行把“点击充值”解释为到账。因此`TC-8a`当前验证的是支付能力缺失时严格失败关闭，而不是伪造一次成功充值。

余额支付、账单合并和自动续费失败提醒已经由真实 Chromium 完成；真实充值意图、支付确认和余额入账整链路保留为`6.9.b`，待阶段 7 支付 owner 完成后扩展同一 Portal 权威用例。顶级`6.9`因此保持未勾选。

## 影响审查

- 架构与插件：客户财务 E2E 和页面继续归属独立 Portal，账务写入继续由 core 权威服务拥有，支付确认继续等待支付 owner；没有修改宿主或跨插件数据库。
- 数据权限与 API：没有新增或修改接口。浏览器用例断言余额支付与合并请求不携带客户编号和危险状态，客户所有权仍由服务端活动主体派生。
- 前端与 i18n：新增三类续费失败的中英文可见投影和下次尝试时间，不显示内部失败码；390px 与桌面布局均验证通过。
- 数据库、缓存与生产后端：无影响。没有迁移、DAO、查询、缓存或 Go 生产代码变更。
- 开发工具：无影响。没有修改根`hack`、Playwright runner、CI、`Makefile`或`make.cmd`；宿主静态治理脚本异常只记录、不越界修复。

**已读取规则：**`AGENTS.md`、`.agents/rules/openspec.md`、`.agents/rules/documentation.md`、`.agents/instructions/markdown-format.instructions.md`、`.agents/rules/architecture.md`、`.agents/rules/data-permission.md`、`.agents/rules/plugin.md`、`.agents/rules/api-contract.md`、`.agents/rules/backend-go.md`、`.agents/rules/database.md`、`.agents/rules/frontend-ui.md`、`.agents/rules/testing.md`、`.agents/rules/dev-tooling.md`、`.agents/rules/i18n.md`。
