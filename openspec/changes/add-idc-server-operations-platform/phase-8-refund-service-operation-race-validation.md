# 第八阶段退款与服务操作竞态验证记录

## 范围

本记录只证明`8.2.b.4`完成，不代表`8.2.b`、`8.2`或第八阶段整体完成。

## 根因与修复

退款提交已经创建每台受影响服务器的活动冲突锁，数据库也会拦截之后新插入的普通业务操作和生命周期任务。但两个时间差仍未完全覆盖：

1. 续费操作可以先创建为待结算，退款随后锁住服务器；旧代码付款时没有再次检查退款锁。
2. 到期暂停、恢复或终止任务可以先排队，退款随后锁住服务器；旧代码领取既有任务时没有再次检查退款锁。

现在普通服务器操作、续费最终扣款和到期任务领取都在锁定同一服务器行后检查活动退款锁。这样退款先完成锁定时：

- 普通操作直接返回稳定业务冲突，不创建新操作；
- 待结算续费在扣余额前拒绝，账单和资金不变化；
- 已排队到期任务进入`manual_review`，记录`lifecycle.refund_in_progress`，不暂停、恢复或终止服务器。

数据库触发器继续作为最后一道并发防线，应用检查负责给业务流程稳定结果；两者没有移除或放宽。

## 主要文件

- `apps/lina-plugins/idc-core/backend/internal/service/serviceguard/refund.go`
- `apps/lina-plugins/idc-core/backend/internal/service/serveroperation/serveroperation_command.go`
- `apps/lina-plugins/idc-core/backend/internal/service/renewal/renewal_payment.go`
- `apps/lina-plugins/idc-core/backend/internal/service/lifecycle/lifecycle_processor_provider.go`
- `apps/lina-plugins/idc-core/backend/internal/service/renewal/renewal_postgres_integration_test.go`
- `apps/lina-plugins/idc-core/backend/internal/service/renewal/renewal_lifecycle_postgres_test.go`

## 验证结果

- 本机隔离 PostgreSQL 下，续费退款锁、续费与终止双向锁顺序、到期任务退款冻结三组测试随机顺序执行 10 轮：通过，用时约 23 秒。每次只创建并清理测试自己的随机 schema。
- `GOWORK=off go test ./backend/... -run '^$' -count=1`：全部后端包编译通过。
- `GOWORK=off golangci-lint run --config E:/golang/cloud/.golangci.yml ./...`：`0 issues`。
- 受影响生产包`staticcheck -checks=U1000 -tests=false`：通过。

## 其他门禁

- 本切片没有新增 API、SQL 迁移、前端页面或用户可见文案，现有退款锁表和状态含义未改变，因此无新的 API、数据库、前端和 i18n 资源。
- 没有调用真实支付或 Provider，也没有写入既有魔方用户或实例。
- 付费变配、收费换`IP`及退款成功后的资源回收闭环分别属于`8.8`、`8.9`和退款后续编排，父任务继续保持未勾选。
