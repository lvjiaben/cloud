# 第十阶段合同状态机整组回归验证记录

## 1. 任务结论

任务`10.9.e`已完成。任务`10.9.a`至`10.9.d`实现的腾讯合同回调、状态收敛、重新发起、解除和文件归档已经用同一份当前代码完成整组回归，并重新构建 Windows `amd64`和 Linux `amd64`两套本地合规网关组件包。

全部要求均已通过，因此可以同时收口`10.9`父任务。本记录不代表客户页面或浏览器`E2E`已经完成；客户页面仍由`10.12`直接在`@lina/web-customer`实现，电子合同页面`E2E`仍属于`10.17`。

## 2. 整组测试对应关系

| 验收内容 | 主要自动化证据 | 结果 |
|---|---|---|
| 腾讯回调验签和解密 | `tencentess/callback_test.go`验证正确签名、密文解密、腾讯应用和本地路由解析；篡改正文会在查路由前拒绝 | 通过 |
| 本地主体错配 | `TestCallbackVerifierRejectsWrongInstallationSubject`和腾讯主动查询跨主体拒绝用例 | 通过 |
| 精确重放 | 真实 PostgreSQL回调收敛测试验证相同事件恢复原证据与投递；内容改变的同事件号会冲突；合同时间线重复投递不追加第二条 | 通过 |
| 乱序和终态冲突 | 网关真实 PostgreSQL测试覆盖迟到非终态、相同终态和相反终态；插件状态时间线覆盖腾讯十种精确状态且终态不倒退 | 通过 |
| 主动查询补回丢失回调 | 网关只读查询测试验证直接取得`ALL`终态、重复查询不写供应商；插件直接终态快照测试验证本地状态补齐 | 通过 |
| 重新发起 | 真实 PostgreSQL测试覆盖原合同保留、同内容追加流程、内容变化新建不可变版本、固定新七天期限、预览和提交分离、重复提交不重复调用腾讯 | 通过 |
| 解除 | 真实 PostgreSQL测试覆盖独立权限、解除原因、二次确认、沿用原签署人、独立`termination`流程、腾讯`RELIEVED`权威生效和原历史不覆盖 | 通过 |
| 最终文件和签署报告 | 腾讯客户端单元测试验证官方资源读取、收费报告只创建一次、五分钟短期地址立即消费和不安全下载地址拒绝 | 通过 |
| 私有归档和哈希 | 插件真实 PostgreSQL测试验证最终`PDF`写入私有对象存储、回读核对、容量、`SHA-256`、供应商时间、不可变记录和重复执行不再归档 | 通过 |
| 摘要错误拒绝 | 插件本地网关客户端测试会拒绝长度、`PDF`文件头或`SHA-256`不一致的响应 | 通过 |
| 短期存证地址 | 归档测试证明只保存不含链接的存证元数据，不保存腾讯二维码或查看地址 | 通过 |

上述真实数据库用例使用相互隔离的 PostgreSQL schema，不依赖 SQLite 或内存数据库替代事务、行锁、唯一约束和触发器行为。

## 3. 自动化验证结果

```text
LINAPRO_IDC_COMPLIANCE_GATEWAY_TEST_DSN=<本机测试库> GOWORK=off go test ./... -count=1
PASS（本地合规网关全部包）

LINAPRO_IDC_COMPLIANCE_GATEWAY_TEST_DSN=<本机测试库> GOWORK=off go vet ./...
PASS

LINAPRO_IDC_COMPLIANCE_TEST_DSN=<本机测试库> GOWORK=off go test ./... -count=1
PASS（合规插件后端全部包）

LINAPRO_IDC_COMPLIANCE_TEST_DSN=<本机测试库> GOWORK=off go vet ./...
PASS

make.cmd lint.go dir=apps/lina-plugins/idc-compliance/central-gateway
PASS（golangci-lint 0 issues；staticcheck通过）

make.cmd lint.go dir=apps/lina-plugins/idc-compliance
PASS（golangci-lint 0 issues；staticcheck通过）

make.cmd plugins.check
PASS（扫描2397个文件，0 findings）

make.cmd i18n.check
PASS（违规0；前端键覆盖和运行时消息覆盖通过）

openspec validate add-idc-server-operations-platform --strict
PASS
```

全仓`make.cmd lint.go`此前已经运行。本任务涉及的合规插件和本地网关均通过，命令只在无关的`tenant-core`三个既有格式问题上停止。本轮没有擅自修改其他插件，目标范围使用正式检查入口单独通过。

## 4. 双平台组件包

发布工具使用相同代码、相同版本和相同构建时间重新生成两个不可覆盖的本机验收包。第一次启动发布工具时因为顶层`go.work`不包含本地网关独立模块而在编译前退出，没有生成半成品；随后按该独立组件的正式构建要求设置`GOWORK=off`，两套构建均成功。

| 目标 | 清单文件数 | 逐文件哈希错误 | `manifest.json`的`SHA-256` |
|---|---:|---:|---|
| Linux `amd64` | `31` | `0` | `fede229e8616e8ed49fe61df825496e028b65fab651d6fbe6719af28135001f7` |
| Windows `amd64` | `31` | `0` | `7feb64531b9f19bd230615011ad7435918f47e5b42f79d4693d59dcfc0dc5faf` |

Linux主程序文件头为`ELF`，Windows主程序文件头为`MZ`。两个清单都满足：

- `productPackageRequired=true`；
- `localInstallationOnly=true`；
- `sameProgramForAllInstallations=true`；
- `supplierClientsIncluded=true`；
- `businessRoutesEnabled=true`。

两个包都包含`019-tencent-callback-evidence.sql`、`020-tencent-contract-status.sql`和`021-tencent-contract-artifact.sql`。验收包位于`temp/codex-validation/contract-state-machine-linux-20260817-01`和`temp/codex-validation/contract-state-machine-windows-20260817-01`，仅作为本机验证产物，不是正式发布目录。

## 5. 发布版本纠正

任务级审查发现：第`018`号插件数据库迁移已经新增，但插件版本仍停在`v0.1.12`。这可能让正式安装或升级无法清楚区分旧包和包含合同归档表的新包。

已把`idc-compliance`版本推进到`v0.1.13`，并同步插件注册测试。修复后重新运行插件后端全量测试、`go vet`、正式定向代码检查和插件治理，结果全部通过。

## 6. 边界和影响

- 本地独立运行：通过；每套安装使用自己的腾讯账号、应用、本地主体、本地网关、数据库、私有对象存储和监管方式，没有软件提供方中央业务依赖。
- 公共API：无新增；本组新增的文件能力只通过安装内签名协议使用。
- 数据权限：没有新增用户列表、详情或下载入口；托管作业只按当前安装和受保护合同路由处理，用户页面权限留给`10.12`。
- 数据库：插件第`018`号迁移和网关第`021`号迁移均在真实 PostgreSQL中重复执行通过。
- 缓存：无影响；没有新增或修改缓存。
- `i18n`：无影响；没有新增用户可见文案或语言资源，多语言治理检查通过。
- 前端：没有修改`web-antd`、`web-customer`或官网；`apps/lina-vben/apps/web-antd`状态差异为空。
- E2E：本任务是后端状态机整组回归，没有冒充`10.17`浏览器验收完成。
- Git：没有提交、推送或创建PR。

## 7. 官方契约依据

- 腾讯第三方应用回调数据：<https://qian.tencent.com/developers/partner/callback_data_types>
- 腾讯合同流程回调：<https://qian.tencent.com/developers/partner/callback_types_contracts_sign>
- 腾讯查询合同流程：<https://cloud.tencent.com/document/api/1420/66683>
- 腾讯最终合同文件：<https://qian.tencent.com/developers/partnerApis/files/DescribeResourceUrlsByFlows/>
- 腾讯申请签署报告：<https://qian.tencent.com/developers/partnerApis/certificate/CreateChannelFlowEvidenceReport/>
- 腾讯查询签署报告：<https://qian.tencent.com/developers/partnerApis/certificate/DescribeChannelFlowEvidenceReport/>

## 8. Lina审查结论

本轮按任务`10.9`完整范围复核了回调入口、签名和解密、主体与路由绑定、事件防重放、乱序和冲突收敛、主动查询、合同本地时间线、重新发起、解除、收费报告防重复、文件下载、私有归档、哈希、数据库迁移、启动装配和双平台组件包。

审查使用根`AGENTS.md`以及`.agents/rules/user-preferences.md`、`openspec.md`、`documentation.md`、`architecture.md`、`data-permission.md`、`plugin.md`、`api-contract.md`、`backend-go.md`、`database.md`、`testing.md`、`i18n.md`和`dev-tooling.md`。插件根目录没有额外`AGENTS.md`。

本组审查累计修正了报告编号密钥用途不一致、数据库错误分类、迁移不能直接重跑、保留地址拦截不完整、数据库更新和对象关闭错误被忽略、文件响应安全头缺失，以及插件版本未随新迁移推进等问题。每次修复后都重新运行对应测试和正式检查。

最终没有剩余阻塞`10.9.e`或`10.9`父任务的问题。未完成的客户页面、浏览器合同流程和更大范围订单补偿测试仍按`10.12`、`10.14`和`10.17`继续，不能被本记录提前认定完成。
