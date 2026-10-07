# 第 10 阶段发票后台审核与安全取消验证记录

## 记录范围

- 日期：2026-08-17
- 对应任务：`10.11.c`
- 本记录证明后台发票详情、审核、驳回、本地提交排队、安全取消、四项独立权限、供应商能力门禁、精确重放和原子审计已经完成。
- 本记录不证明退款红字、真实诺诺授权与开票、后台页面、客户开票页面或整个`10.11`已经完成；它们仍分别属于`10.11.d`、外部证据完成后的`10.11.e`、`10.12`和`10.11.f`。

## 已实现内容

### 四项独立权限和后台接口

- 查看：`idc:admin:compliance:tax-invoice:query`，对应`GET /admin/compliance/tax-invoices/{taxInvoiceNo}`。
- 审核：`idc:admin:compliance:tax-invoice:review`，分别创建`approvals`或`rejections`动作资源。
- 提交：`idc:admin:compliance:tax-invoice:submit`，创建独立的`submissions`动作资源，只把申请加入本地队列。
- 取消：`idc:admin:compliance:tax-invoice:cancel`，创建独立的`cancellations`动作资源。
- 四项权限已同时进入核心插件权限目录、合规插件清单、接口权限标签和服务层授权检查，不能用查看权限执行审核，也不能用审核权限代替提交或取消。

### 审核、提交和供应商门禁

- 客户申请仍从`pending_review`开始，票种仍为`pending_confirmation`。
- 审核人员只能从当前供应商门禁明确返回的支持票种中选择；审核成功后状态固定变为`approved_pending_submission`，后台文案为“待提交开票”。
- 提交是第二个独立动作，提交前重新检查门禁和已确认票种；通过后只进入本地`queued`，本任务没有调用真实供应商。
- 生产装配使用明确关闭的诺诺门禁，返回`external_confirmation_required`，不构造诺诺客户端、不发网络请求、不伪造支持票种或成功结果。
- 门禁关闭时，审核保持`pending_review`，已审核申请的提交保持`approved_pending_submission`；接口返回明确原因和当前状态。状态或版本本身不合法时仍返回冲突，不用“门禁关闭”掩盖错误状态。

### 驳回、取消和额度释放

- 驳回与取消只接受七个冻结原因：抬头或税务信息有误、金额或账单需要调整、重复申请、当前业务无法开具、客户要求取消、订单或退款情况变化、其他原因。
- 内部备注可不填，最长`2000`个字符，只进入后台投影和只追加动作记录；业务审计不包含该备注，后续客户接口不得返回。
- `pending_review`和`approved_pending_submission`可以取消；`failed`只有存在供应商权威的`provider_not_issued_confirmed_at`正向证据时才可以取消。不能只因没有供应商编号就猜测“未开票”。
- `queued`、`submitting`、`result_unknown`、`issued`、`delivered`、`red_letter_pending`和`red_lettered`取消都会被拒绝。
- 驳回或安全取消在同一事务中更新状态、追加动作记录和追加业务审计；状态变成`rejected`或`canceled`后，既有占用查询不再计算本申请，逐账单和逐明细额度一起释放。

### 数据库、并发和精确重放

- `019-tax-invoice-request.sql`增加待提交状态、审核字段、权威未开票确认时间、工作队列索引、三套双语字典和只追加发票动作表。
- 动作表保存操作人、前后状态、前后版本、票种、供应商、固定原因、内部备注和请求指纹；触发器禁止修改或删除历史。
- 同一申请、同一动作和同一幂等键使用事务级数据库锁串行处理。八路同时提交完全相同审核时固定只有一次首次写入，其余七次精确重放同一结果；同一键改变票种或其他内容明确冲突。
- 状态更新、动作记录和业务审计处于同一事务；强制审计失败的真实 PostgreSQL 用例证明状态和动作记录都会回滚。
- DAO、DO 和 Entity 使用专用代码生成数据库和仓库`linactl dao`入口重新生成，没有手工修改生成字段。

## 自动化验证

以下验证均针对本记录形成时的共享工作区执行：

```powershell
$env:GOWORK='off'
$env:LINAPRO_IDC_COMPLIANCE_TEST_DSN='<专用 PostgreSQL 测试连接>'
go -C apps/lina-plugins/idc-compliance test ./backend/... -count=1
go -C apps/lina-plugins/idc-core test ./backend/... -count=1
go -C apps/lina-plugins/idc-compliance vet ./backend/...
go -C apps/lina-plugins/idc-core vet ./backend/...
go -C hack/tools/linactl run . lint.go dir="E:/golang/cloud/apps/lina-plugins/idc-compliance"
go -C hack/tools/linactl run . lint.go dir="E:/golang/cloud/apps/lina-plugins/idc-core"
go -C hack/tools/linactl run . i18n.check
go -C hack/tools/linactl run . plugins.check
openspec validate add-idc-server-operations-platform --strict
```

结果：两个插件全部后端测试通过，两个插件`go vet`通过，两个插件正式`golangci-lint`均为`0 issues`且`staticcheck U1000`通过；运行时文案、消息、前端键和插件治理均无问题；OpenSpec 严格校验通过。

真实 PostgreSQL 覆盖：

- 迁移原文执行、重复执行、对象名长度、索引、约束和只追加触发器；
- 门禁关闭保持原状态并返回`external_confirmation_required`；
- 审核进入待提交开票，提交独立进入本地排队；
- 相同操作精确重放、修改内容冲突和八路并发重放；
- 七类原因校验、后台内部备注保存和业务审计不泄漏备注；
- 待审核、待提交、失败且确认未开票允许取消；其余提交中、结果不明、已开具、已交付和红字状态全部拒绝；
- 驳回和取消释放占用，失败和结果不明继续占用；
- 审计失败时状态与动作记录整笔回滚。

## 影响判断

- 前端：本任务没有实现页面，没有修改`web-antd`、`web-customer`、插件`frontend/`或官网。后台和客户页面仍由`10.12`直接在各自正确宿主完成。
- HTTP API：有影响。新增一个详情读取和四个动作资源，方法、路径、权限标签、字段说明、示例和 Unix 毫秒时间字段符合接口规则；详情是单次有界查询，没有列表逐行补查或`N+1`。
- 数据权限：有影响。宿主权限中间件和服务边界都校验精确权限，目标使用申请编号，固定当前安装实例范围；不存在时与无权访问折叠为不可访问。
- 多语言：有影响。状态、票种、七类原因、错误信息、菜单权限名和中文接口文档已经覆盖，`i18n.check`通过。
- 缓存：无影响。状态、版本、门禁结果和额度释放都以当前数据库事务为准，没有新增缓存。
- E2E：本任务没有页面，使用接口、服务、真实数据库和启动绑定测试覆盖；浏览器页面与 E2E 仍属于`10.12`，不能用本记录提前勾选。
- 开发工具：没有修改工具源码、Makefile、CI 或脚本；只使用跨平台`linactl`执行既有`ctrl`、`dao`、lint、双语和插件治理。生成器追加的重复空控制器骨架已精确删除，保留原有有效实现。
- 共享工作区：没有删除、回滚或覆盖`web-customer`及其他既有未提交成果，没有提交或推送 Git。

## Lina 审查报告

**变更：** `add-idc-server-operations-platform`  
**范围：** 任务级`10.11.c`  
**审查文件数：** `40`个人工维护文件，另含`105`个本轮重新生成或更新的接口与 DAO/DO/Entity 文件  
**范围来源：** `git status --short`、未跟踪插件目录展开、OpenSpec 任务范围和本轮代码生成输出  
**已读取规则文件：** `AGENTS.md`、`.agents/rules/user-preferences.md`、`openspec.md`、`documentation.md`、`architecture.md`、`data-permission.md`、`plugin.md`、`api-contract.md`、`backend-go.md`、`database.md`、`testing.md`、`i18n.md`和`dev-tooling.md`

### 发现的问题

- 审查中发现相同幂等操作并发时可能在首笔提交前错过重放记录，已增加动作级事务锁和八路并发回归。
- 审查中发现“失败且确认未开票”不能由供应商编号是否为空推断，已改为要求供应商权威的正向确认时间，并覆盖有外部编号但确认未开票的允许路径。
- 审查中发现四项权限只有服务引用、缺少受保护后台 API，会违反权限治理，已补齐五个接口、启动装配和中文接口文档。
- 修复并重新执行全部门禁后，未发现阻塞问题，也没有遗留警告。

### 规则域结论

- OpenSpec、架构和插件边界：通过；能力闭环在 IDC 核心与合规插件内，没有修改宿主核心或引入中央业务服务。
- 后端、接口和数据权限：通过；权限、状态机、事务、幂等、接口标签和单次详情查询均有实现与测试。
- 数据库：通过；迁移幂等、对象名、正向证据、只追加历史、索引和代码生成均通过真实 PostgreSQL 验证。
- 测试与多语言：通过；当前代码测试、正式 lint、双语与插件治理均通过。
- 前端与缓存：无影响；本任务没有页面、路由、前端依赖或缓存变更。
- 开发工具跨平台：无源码影响；只调用现有跨平台 Go 工具入口。

### 摘要

- 严重：`0`
- 警告：`0`

## 后续边界

- 下一项仍是`10.11.d`退款分配和红字任务，不是第十五阶段。
- `10.11.e`继续受`10.4.c`与`10.10`外部证据阻断，当前不得开发或启用真实诺诺客户端。
- `10.11.f`、`10.11`父任务、`10.12`页面和整个第十阶段仍未完成，本记录不得提前代替这些任务的验收证据。
