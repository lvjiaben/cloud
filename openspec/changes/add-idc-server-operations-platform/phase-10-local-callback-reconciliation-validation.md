# 第十阶段本地回调收敛验证记录

## 1. 任务结论

任务`10.6.d.3`已在当前安装环境的本地合规网关中完成回调原文边界、供应商专用验签后的中立收敛、加密证据、按供应商隔离的外部事件去重、路由/证据/投递复合约束和终态单调推进。

本任务没有实现腾讯或诺诺生产验签器，也没有注册公开回调`HTTP`路由，运行时使用空验证器集合并失败关闭，不自创通用签名，不回退到软件提供方网关。`2026-08-14`后腾讯验证器转由`10.9`按当时官方精确通知与验签契约实现，诺诺仍受`10.4.c`外部证据门禁；历史中央回调设计与验证只作为算法迁移基线，不能替代本次当前安装所有权实现。

## 2. 根因与本地化结果

迁移`005-callback-reconciliation.sql`曾建立依赖远程实例和中央投递的回调模型，迁移`008-retire-remote-identity.sql`已经按独立交付纠偏删除对应中央路由、证据与投递表。迁移`009-local-supplier-runtime.sql`只留下本地业务请求和简化供应商操作骨架，没有完成以下能力：

- 每个本地供应商写尝试的唯一路由；
- 当前安装、业务请求、写尝试、供应商和操作范围的复合归属；
- 原始回调上限、供应商验证器边界与调用后缓冲清零；
- 独立证据加密与按供应商隔离的外部事件查重；
- 重复、乱序、结果未知和相反终态的单调收敛；
- 证据、路由投影和本地版本化投递的同事务提交。

新增迁移`015-local-callback-reconciliation.sql`和`internal/suppliercallback`只使用当前安装标识与本地业务请求，不恢复平台销售注册、远程实例身份、跨客户路由、中央结算或软件提供方业务服务。

## 3. 冻结的安全实现

### 3.1 回调与验证器边界

- 原始正文最多 256 KiB，签名最多 16 KiB；参与验签的允许列表请求头最多 32 项、合计最多 16 KiB，单值最多 4 KiB，并拒绝换行和空字节。
- 供应商专用验证器必须先验证应用身份、签名、供应商时间、事件类型和精确原始字节，再产生一次性已验证事件。
- 中立收敛方法保持包内不可导出，外部调用只能经过验证器边界；正文、签名、请求头、外部事件编号和证据帧在成功或失败返回后清零。
- 当前生产运行图构造空验证器集合，`internal/app/handler.go`不注册回调处理器；腾讯与诺诺契约确认前返回失败关闭，不存在中央兜底路径。

### 3.2 加密证据与外部事件去重

- 新增独立私有版本化回调证据密钥环，不与进程认证或供应商主体密钥环复用。
- 证据使用`AES-256-GCM`，认证附加数据绑定证据号、路由号、供应商和证据来源；数据库只保存密文、随机数、明文`SHA-256`、格式版本和密钥版本。
- 外部事件编号使用独立`HMAC-SHA-256`密钥族并绑定供应商；活动与保留版本由一次有界数据库查询完成查重，不保存事件编号原文。
- 同一供应商事件编号、路由、规范结果和载荷摘要完全相同时恢复原证据；改变任一项返回冲突并保留首份证据。两个不同路由并发抢占同一外部事件时，唯一约束只允许一份证据，另一方稳定返回冲突。
- `10.6.d.4`复核进一步修正精确重放：完全相同的历史回调现在恢复首次提交时的原证据号、原投递号、原路由版本和原投影，不再用路由后续状态替代历史结果；对应 PostgreSQL 回归已纳入[本地主动查询与人工接管验证记录](phase-10-local-active-query-takeover-validation.md)。

### 3.3 路由、投递与单调状态

- 首次供应商写租约在同一事务创建一个唯一路由；同一业务请求恢复原写尝试和原路由，不会再次调用供应商。
- `idc_gateway_local_supplier_route`永久绑定写尝试、业务请求、安装标识、供应商和操作范围。
- `idc_gateway_local_callback_evidence`与`idc_gateway_local_route_delivery`通过包含归属、路由版本和证据摘要的复合外键闭合；数据库拒绝跨安装证据和跨路由投递。
- `running`只能进入`result_unknown`、`succeeded`或`failed`；`result_unknown`只能进入成功或失败；迟到的`running`不能倒退终态。
- 成功与失败发生冲突时保留先前终态，路由版本单调递增并进入`reconciliation_required`；不采用最后到达结果。
- 每次真实投影变化才产生一个本地版本化投递；证据、路由投影、业务请求投影和投递在同一 PostgreSQL 事务提交。证据不可更新或删除，投递只允许`pending`单向确认。

## 4. 交付与运行配置

运行配置新增`LINAPRO_COMPLIANCE_GATEWAY_CALLBACK_EVIDENCE_KEYRING_FILE`。数据库、安装标识、进程认证密钥环、供应商主体密钥环和回调证据密钥环必须使用五个存在、绝对且互不相同的私有文件路径。

跨平台命令`idc-compliance-local-gateway-evidence-keyring`只创建新文件、拒绝覆盖、要求父目录已存在，并在 Unix 使用`0600`权限。发布工具已把该命令加入 Linux 和 Windows 组件包；配置模板、中英文安装说明及中英文数据生命周期说明同步覆盖生成、就绪、备份、恢复和克隆边界。

## 5. 自动化验证

```text
GOWORK=off go test ./... -count=1
PASS

LINAPRO_IDC_COMPLIANCE_GATEWAY_TEST_DSN=<disposable-dsn> \
  GOWORK=off go test ./... -count=1
PASS

LINAPRO_IDC_COMPLIANCE_GATEWAY_TEST_DSN=<disposable-dsn> \
  GOWORK=off go test ./internal/suppliercallback -count=10 -shuffle=on
PASS

GOWORK=off go vet ./...
GOWORK=off staticcheck ./...
make.cmd lint.go dir=apps/lina-plugins/idc-compliance/central-gateway
PASS（golangci-lint 0 issues；staticcheck 通过）

make.cmd plugins.check
PASS（扫描 1632 个文件，0 findings）

go run ./hack/tools/release ... -goos linux -goarch amd64
go run ./hack/tools/release ... -goos windows -goarch amd64
PASS（两个清单各 24 个文件，均包含主程序、证据密钥命令、迁移 015、配置和双语说明）
```

真实 PostgreSQL 用例覆盖 256 KiB 正文与全部头部边界、密文最大帧、调用后缓冲清零、缺验证器失败关闭、重复恢复、改变载荷冲突、八个并发调用方只有一个证据写入、不同路由并发抢占同一外部事件只允许一个成功、乱序处理中事件不倒退终态、`result_unknown`进入成功、成功与失败冲突进入`reconciliation_required`、业务投影保持既有终态、证据不可变、跨安装证据拒绝和跨路由投递拒绝。迁移用例同时验证 15 个迁移首次执行和账本幂等恢复、最终表存在性、旧中央身份与结算对象缺失、约束拒绝和 PostgreSQL 标识长度。

当前环境`CGO_ENABLED=0`，`go test -race ./internal/suppliercallback`被 Go 工具链以`-race requires cgo`拒绝；没有把该未执行门禁记录为通过。并发正确性由 PostgreSQL 行锁、唯一约束和十轮乱序真实数据库测试验证。

仓库级`make.cmd i18n.check plugins=1`的运行时硬编码和消息覆盖通过；完整前端键检查仍只被禁止在本插件任务中修改的宿主`apps/lina-vben/apps/web-antd/src/views/system/message/notice-preview-modal.vue`既有 6 个中英文缺键阻断。本任务没有新增用户可见页面、接口文案或语言资源，不修改宿主，也不宣称全局`i18n`门禁通过。

## 6. 任务级审查

`lina-review`复核了当前安装所有权、供应商专用验证器不可绕过、生产路由关闭、原始字节边界与清零、密钥用途隔离、认证附加数据、按供应商事件去重、锁顺序、并发恢复、复合外键、同事务提交、终态单调性、发布包和双语运维说明。复核期间修正了三项问题：完整允许列表请求头未计入密文上限、并发状态推进可能被误判为路由归属冲突，以及保留密钥版本查重原本需要最多八次数据库查询。最终实现分别改为完整固定上限、只比较不可变归属和一次有界查重查询，并补充真实 PostgreSQL 回归。

最终没有遗留阻断`10.6.d.3`的`P0`、`P1`或`P2`问题。后续`10.6.d.4`的只读主动查询、退避和当前商家员工人工接管实现与迁移`016-local-active-query-takeover.sql`见[本地主动查询与人工接管验证记录](phase-10-local-active-query-takeover-validation.md)；父任务`10.6.d`仍按精确子任务原则保持未完成。
