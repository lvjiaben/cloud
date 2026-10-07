# 服务器生命周期结果不明验证记录

> **`FB-52`范围纠正：**结果不明时禁止重放和伪造成功的原则继续有效；本文依赖旧`service_instance`运行投影的证据只用于服务器迁移对账。服务器运行状态与 Provider 任务投影迁入服务器插件后，必须从`service_entitlement`冻结的业务拥有方精确路由并重新验证，不能把服务器三维状态当作所有长期权益的通用状态。

## 完成范围

本轮关闭任务`8.2.b.2`。到期暂停、恢复或终止的资源任务进入`result_unknown`或`manual_review`时，`core`现在同步更新服务上的 Provider 任务投影，但不改写权益和运行状态。终止结果不明时继续保持`terminating`，业务操作和生命周期任务进入人工处理，禁止误报仍在普通执行、恢复原运行态或自动重放上游写请求。

父任务`8.2.b`和`8.2`继续保持未完成。人工暂停/恢复、付费变配、换 IP、退款联动及其他能力组合竞态仍按后续子任务推进，本记录不把它们冒充完成。

## 根因与修复

原收敛逻辑已经把`business_operation`和生命周期任务转为人工处理，但遗漏了`service_instance.provider_task_status_code`，导致服务第三套状态仍停在`executing`。修复在同一数据库事务中按服务版本栅栏更新为`result_unknown`或`manual_review`；更新失败时整次收敛回滚，不留下互相矛盾的投影。

## 验证结果

- `gofmt`通过。
- `GOWORK=off go test ./backend/internal/service/lifecycle ./backend/internal/service/renewal -run 'TestServiceLifecycle|TestProviderLifecycle' -count=10 -shuffle=on`通过。
- 设置`LINAPRO_IDC_CORE_TEST_DSN`为本地`cloud_codex_e2e`隔离库后，`go test ./backend/internal/service/renewal -run 'TestServiceLifecycle' -count=10 -shuffle=on`通过，用时 77.113 秒。测试每轮创建并清理自己的随机临时 schema，没有读写正式业务库。
- 两条结果不明定向 PostgreSQL 用例随后再执行 10 轮乱序回归，全部通过，用时 24.315 秒。
- 新增真实 PostgreSQL 用例断言：终止写入提交阶段发生来源不明错误，或提交后权威查询返回结果不明时，权益/运行状态都保持`expired/terminating`，Provider 状态为`result_unknown`，生命周期任务进入`manual_review`；已建立的业务操作也进入`manual_review`。

## 影响判断

- 数据权限、API、前端和`i18n`：无新增影响；本轮只修正内部状态收敛和后端测试。
- 性能：单次事务增加一条带`tenant_id + service_no + version`栅栏的精确更新，不增加列表查询或循环数据库访问。
- 缓存与开发工具：无影响；权威状态继续只在 PostgreSQL，未修改构建或测试脚本。
