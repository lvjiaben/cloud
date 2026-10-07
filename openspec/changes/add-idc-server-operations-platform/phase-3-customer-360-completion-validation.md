# 第三阶段客户 360 补全验证记录

## 范围

本记录只覆盖任务 3.4.b、3.4.b.2 和 3.5.b。标准后台 `TC001` 的任务状态仍由 3.4.c.3 与 3.8.b 单独控制，不因本记录提前完成。

## 客户 360 与运营动作

- 订单、账单、交易、服务器、合同、发票、工单、通知和审计摘要由各自插件通过冻结后的能力注册表集合化提供；缺项、重复项、未知项、负数或贡献者失败都会整体失败关闭。
- 分组和标签明细按客户、类型、语言与页大小绑定不透明游标，翻页前重新核对权威边界。
- 备注采用追加更正并保留被更正历史；跟进只允许从计划中进入完成或取消；黑名单先提交为待审批，再单独批准、驳回或撤销。
- 所有写操作要求精确权限、客户可见范围、同事务审计和重复提交键。同一键同一内容返回原结果，不重复写业务记录或审计；同一键改内容返回冲突。
- 后台源码只位于 `apps/lina-plugins/idc-core/frontend/`，没有向 `web-antd` 写入 IDC 页面、路由、接口或文案。

## 验证结果

- `go test ./backend/internal/service/customermanagement -run TestCustomerActivityMutationsPreserveHistoryAndRequireBlacklistApprovalPostgreSQL -count=1 -v`：通过，使用隔离 PostgreSQL，覆盖备注历史、跟进状态、黑名单审批、重放、语义冲突和审计计数。
- `go test ./backend/...`（`idc-core`）：通过。
- `go test ./backend/...`（`idc-compliance`、`idc-notify`）：通过，跨域摘要贡献者使用真实 PostgreSQL 空集与双客户集合查询验证。
- `pnpm run build`（`idc-core/frontend`）：通过，包含 `vue-tsc`、Vite 构建和产物校验。
- `go run ./hack/tools/linactl i18n.check`：通过，运行时文案与插件前端键覆盖均为零违规。

## 客户通知偏好

- `pnpm exec playwright test --config tests/playwright.config.ts tests/e2e/TC001-support-notifications.ts`：8/8 通过，覆盖普通消息、安全消息、营销同意、免打扰、版本冲突、读取失败和缺少政策证据等分支。
- 修正客户地址语言包多包了一层命名空间的问题；`pnpm run typecheck`、`pnpm run i18n:check`、`pnpm run check:alignment` 均通过。
- 复跑 `TC001b` 通过，并审查 `temp/20260818/084734-customer-notification-preferences.png`：地址菜单已显示为“我的地址”，通知偏好、营销授权和免打扰布局正常，没有裸露语言键、接口报错提示或遮挡。

## 客户域后端测试门禁

- 使用隔离数据库执行 `go test ./backend/internal/service/customer ./backend/internal/service/customermanagement ./backend/internal/service/customerclosure -count=1`：三包全部通过。
- 覆盖日期和高精度数值字段投影、法定资料变更不覆盖已核验事实、直接客户受众和所有权门禁、稳定游标与固定查询预算、最终注销重检、匿名化、幂等和审计失败回滚。

## 尚未关闭的门禁

后台 `TC001` 已在正确启用源码插件扫描的开发运行方式下达到 6/6 通过，但截图发现当前旧后端进程的运行时语言包仍是启动时的旧快照，新增加的客户运营历史标题尚未进入该进程。必须使用包含当前插件资源的新后端构建重新执行并审图后，才能勾选 3.4.c.3 与 3.8.b。
