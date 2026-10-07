# 第 10 阶段供应商中立发票整组回归验证记录

## 任务结论

任务`10.11.f`已完成。当前代码对发票来源、客户申请、后台审核与安全取消、退款红字四段供应商中立流程完成了同一工作区回归，并重新构建 Windows`amd64`和 Linux`amd64`本地合规网关组件包。

本记录只证明真实诺诺能力继续关闭时，本地供应商中立流程和交付包不会退化。`10.11.e`仍受`10.4.c`与`10.10`外部证据阻断，因此`10.11`父任务继续保持未完成。

## 整组验证

### Go 与真实 PostgreSQL

- 合规插件`go test ./backend/... -count=1`通过，覆盖接口、控制器、启动装配、迁移、发票申请、后台管理和退款红字。
- 核心插件`taxinvoicecap`、`taxinvoicesource`和启动装配包通过，退款来源测试使用真实 PostgreSQL 独立 schema。
- 红字真实 PostgreSQL 用例覆盖唯一与歧义分配、部分与全部红冲、同发票顺序、八路并发精确重放、审计回滚、客户越权、权威复查和持久游标。
- 红字并发、完整流程和持久游标三组用例连续运行`10`轮通过。
- 新增`TestClosedGatewayRejectsSupplierActions`，直接证明诺诺关闭状态不能返回客户操作地址，也不能返回权威确认或完成结果。

### 正式静态门禁

- `make.cmd lint.go dir=apps/lina-plugins/idc-core plugins=1`通过，`golangci-lint`为`0 issues`，`staticcheck U1000`通过。
- `make.cmd lint.go dir=apps/lina-plugins/idc-compliance plugins=1`通过，`golangci-lint`为`0 issues`，`staticcheck U1000`通过。
- `make.cmd lint.go dir=apps/lina-plugins/idc-compliance/central-gateway plugins=1`通过，`golangci-lint`为`0 issues`，`staticcheck U1000`通过。
- `make.cmd i18n.check`、`make.cmd plugins.check`和`openspec validate add-idc-server-operations-platform --strict`通过。

### 双平台本地组件包

使用本地网关正式发布工具和相同版本、提交标识、构建时间重新生成两个不可覆盖的临时验收包：

| 目标 | 清单文件 | 哈希错误 | 禁止资产 | `manifest.json`的`SHA-256` |
|---|---:|---:|---:|---|
| Windows`amd64` | 31 | 0 | 0 | `78ca029e9ac63ce8b3c94530c018fae2c7fc005744d4ba5794b72d14aae376ef` |
| Linux`amd64` | 31 | 0 | 0 | `685cca3f446e8b1f13228a9bd9f22bebcdcb65c285cc5596855dcfa2a79a0a8b` |

两份清单都标记`localInstallationOnly=true`、`sameProgramForAllInstallations=true`和`productPackageRequired=true`。逐文件重新计算`SHA-256`均与清单一致；组件包没有旧 Portal、`dist`、前端锁文件、`node_modules`或测试资产。

临时验收目录为`C:\Users\Administrator\AppData\Local\Temp\linapro-idc-10-11f-b8da96847ef34ea2a98ac991b8288d8a`，不属于仓库正式产物，也没有覆盖旧构建目录。

## Lina 审查报告

**变更：**`add-idc-server-operations-platform`  
**范围：**任务级`10.11.f`  
**范围来源：**`git status --short`、未跟踪插件文件展开、任务`10.11.a`至`10.11.d`证据、关闭门禁测试和双平台发布输出  
**已读取规则文件：**`AGENTS.md`、`.agents/rules/user-preferences.md`、`openspec.md`、`documentation.md`、`architecture.md`、`data-permission.md`、`plugin.md`、`api-contract.md`、`backend-go.md`、`database.md`、`testing.md`和`i18n.md`

### 发现的问题

- 严重：生产装配已经使用`ClosedGateway`，但原测试没有直接证明关闭实现绝不返回客户入口或权威成功。已补充独立自动化测试并通过。
- 修复并重跑全部本任务门禁后，未发现剩余阻断问题或警告。

### 规则域结论

- OpenSpec、架构和插件边界：通过；没有修改`web-antd`、`web-customer`、官网或中央业务服务。
- 后端、数据库、接口和数据权限：通过；整组供应商中立流程使用当前真实 PostgreSQL 与启动装配证据验证。
- 测试：通过；成功、失败、关闭门禁、并发、幂等、越权、回滚和双平台构建均有当前证据。
- 多语言：通过；本任务没有新增运行时文案，既有双语资源在当前工作区复检通过。
- 缓存：无影响；没有新增缓存或改变权威数据源。
- 开发工具跨平台：无源码影响；使用既有发布工具真实生成 Windows/Linux 包。

### 摘要

- 严重：`0`
- 警告：`0`

## 剩余边界

- `10.11.e`继续保持未完成，解除条件是取得并归档诺诺对购买方自有应用、自有纳税主体、票种、税收分类、源文件、红字、外币、用量、结算和数据责任的正式证据。
- 在解除前不得实现、装配或调用真实诺诺客户端，不得用当前关闭测试和组件包冒充真实开票验收。
- `10.11`父任务、`10.12`页面、`10.14`补充测试、`10.18`发票 E2E 和整个第 10 阶段仍未完成。
