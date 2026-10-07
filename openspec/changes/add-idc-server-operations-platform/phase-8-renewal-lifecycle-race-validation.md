# 第八阶段续费与生命周期并发验证记录

## 范围

本记录只证明任务`8.2.b.3`已经完成，不代表父任务`8.2.b`、`8.2`或第八阶段已经完成。付费变配、收费换`IP`、退款后资源残留及后续附属能力组合仍按任务表继续实施和验收。

## 根因与修复

原续费流程在余额扣款前读取一次服务器状态，扣款和续费业务收敛随后分成不同事务。到期暂停或终止任务可能在两者之间改变服务版本，造成“余额和账单已经结清，但续费操作只返回冲突”的半完成状态。

修复后，续费最终资格重验、服务/操作/账单锁定、余额扣款、账单结清、业务操作推进和本地权益到期时间更新处于同一 PostgreSQL 事务。双方都以同一服务行和版本为栅栏：

- 续费先取得锁时，扣款和权益推进原子提交，旧暂停或终止动作不能用过期服务版本覆盖结果；
- 终止先取得锁时，续费等待后读取最新`terminating`状态，明确拒绝且不产生余额扣款、支付分配或账单状态变化；
- 任一后续收敛步骤失败时，外层事务整体回滚，不留下已经扣款但续费未生效的局部事实。

## 变更文件

- `apps/lina-plugins/idc-core/backend/internal/service/renewal/renewal_payment.go`
- `apps/lina-plugins/idc-core/backend/internal/service/renewal/renewal_postgres_integration_test.go`

## 验证

- 设置`LINAPRO_IDC_CORE_TEST_DSN`为本机隔离测试库后，执行`GOWORK=off go test ./backend/internal/service/renewal -run '^TestRenewalPaymentPostgreSQLSerializesExpiryTermination$' -count=10 -shuffle=on -timeout=60s`：通过，用时`11.931s`。每轮只创建并清理测试自己的随机 schema，同时覆盖“续费先取得锁”和“终止先取得锁”两种顺序。
- 执行`GOWORK=off go test ./backend/internal/service/renewal -count=1 -shuffle=on`：通过，用时`13.146s`，包含真实 PostgreSQL 续费回归。
- 执行`GOWORK=off go test ./backend/internal/service/lifecycle -count=1`：通过。
- 执行`GOWORK=off go test ./backend/... -run '^$' -count=1`：完整后端包编译通过。
- 仓库级`make.cmd lint.go`运行到`idc-core`时为`0 issues`；后续被共享工作区中`tenant-core/backend/internal/service/resolver/`三份既有未格式化文件阻断，本任务未修改这些文件，也未替其他工作回退或格式化。

## 边界

- 没有调用真实 Provider 写接口，没有修改真实服务器、客户、余额或订单。
- 测试只使用本机专用测试数据库中的随机隔离 schema。
- 没有修改`web-antd`，没有触碰旧 Portal 删除步骤。
