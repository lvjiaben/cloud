# 第七阶段支付关单与异常款处置验证记录

## 完成范围

- 支付插件通过版本化`billingcap`领取核心待关单请求，逐个关闭同一订单的未完成支付尝试，再回报`closed`、`no_attempts`、`confirmed_success`或`unknown`。
- 渠道关单和异常款自动退款在跨越外部调用前写入持久化防重记录。进程退出、超时或结果不明时停止自动重放，保留人工核对状态。
- 支付插件后台新增待归属资金列表、详情摘要、分配、原路退款和人工退款凭证确认入口；源码、接口、多语言和依赖均留在`idc-payment`插件内，没有写入`web-antd`。
- 核心只允许把迟到成功款分配给同一客户已经创建且仍可付款的新订单。每次分配重新核对客户绑定、活动状态、实名、风险、库存、付款期限、币种和当前未付金额；不自动建单，也不复活旧订单。
- 支持渠道自动原路退款；不支持自动退款的渠道创建人工任务。后台直接选择凭证文件，插件使用宿主文件能力保存文件引用并自动生成摘要，确认前资金持续待处理，且不会自动转入余额。
- 相同异常款、目标订单、金额和证据的重复请求返回同一分配与结清结果；改客户、改订单、改币种或改金额均失败关闭。

## 关键实现

- `idc-core/backend/cap/billingcap`新增异常款分配、关单领取和关单回报契约、描述符、桥接客户端与声明期调用分派。
- `idc-core/backend/internal/service/billing/billing_exception_fund.go`拥有目标订单的当前事实重检和统一财务结清。
- `idc-core/backend/internal/composition/payment_closure.go`把支付 owner 的回报接入既有订单取消状态机。
- `idc-payment/manifest/sql/003-exception-fund-operations.sql`新增异常款动作和渠道关单防重记录。
- `idc-payment/backend/internal/service/paymentattempt/closure.go`实现多支付尝试关单、持久化防重和结果未知保护。
- `idc-payment/backend/internal/service/exceptionfund/service.go`实现分配、自动原路退款、人工退款任务、凭证确认和安全重放。
- `idc-payment/frontend/src/App.vue`提供双语待归属资金页面，并使用字符串和`BigInt`格式化金额，避免大金额精度损失。

## 验证结果

- `idc-payment`真实 PostgreSQL 全量测试通过：`go test -p 1 ./... -count=1`。
- `idc-core`真实 PostgreSQL 全量测试通过：`go test -p 1 ./... -count=1`。覆盖 8 路并发分配、相同请求重放、跨客户隔离、金额与币种不符、旧订单不复活和统一财务结清。
- 支付插件前端类型检查、正式构建和产物校验通过：`pnpm run build`。
- 核心与支付插件`golangci-lint`、`staticcheck`通过。
- 插件治理、仓库运行时多语言和前端多语言键检查通过。
- `openspec validate add-idc-server-operations-platform --strict`通过。

核心全量测试曾暴露客户概览 PostgreSQL 测试直接依赖共享库表权限的问题。该测试现已改用随机隔离 schema 和最小测试表，单包与核心全量测试均通过；生产查询和业务行为没有因此改变。

## 尚未包含

支付宝电脑/手机网站、微信`Native/H5`、易支付人工退款、取消后迟到成功和争议资金处置的真实浏览器扩展仍由任务`7.15`单独验收，本记录不把正式构建或后端集成测试冒充为`E2E`完成。
