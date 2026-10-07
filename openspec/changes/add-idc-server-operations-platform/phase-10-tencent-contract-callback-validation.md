# 第十阶段腾讯合同状态回调验证记录

## 1. 任务结论

任务`10.9.a`已完成。每套安装的本地合规网关现在可以按腾讯电子签官方契约独立接收`FlowStatusChange`回调，并且只有在购买方自己的腾讯配置明确启用`callback.receive`、同时提供完整验签令牌和 32 字节回调解密密钥时，才注册固定公开地址`POST /callbacks/v1/tencent-essbasic`。

本切片只完成回调入口、官方验签解密、本地流程反查、加密证据、去重和中立状态单调收敛。它没有把拒签、过期、撤销、解除等状态压入最终合同业务状态，也没有实现重新发起、解除或最终文件归档；这些仍分别属于`10.9.b`至`10.9.d`，因此`10.9`父任务保持未完成。

## 2. 官方契约依据

实现按腾讯电子签官方[第三方应用回调数据说明](https://qian.tencent.com/developers/partner/callback_data_types)和[合同流程回调说明](https://qian.tencent.com/developers/partner/callback_types_contracts_sign)冻结：

- 对收到的原始 HTTP 正文计算`HMAC-SHA-256`，使用购买方配置的回调校验令牌验证唯一`Content-Signature`，验签前不改写正文；
- 回调正文只接受`{"encrypt":"..."}`包装，使用购买方配置的`CallbackUrlKey`执行`AES-256-CBC`解密，并严格校验`PKCS#7`填充；
- 明文只接受`MsgVersion=ThirdPartyApp`、`MsgType=FlowStatusChange`，核对消息号、应用、当前安装腾讯主体、流程号、供应商时间和官方状态；
- 接受官方状态`INIT`、`PART`、`WILLEXPIRE`、`ALL`、`REJECT`、`DEADLINE`、`CANCEL`、`INVALID`、`EXCEPTION`和`RELIEVED`，未知状态拒绝；解除状态还必须带解除流程号；
- 官方明确提示回调可能乱序，因此本地事务复用既有单调收敛、精确重放和相反终态对账保护，不按到达先后盲目覆盖结果。

## 3. 根因与实现结果

原有本地网关已经具备供应商中立的回调收敛基础，但生产运行时故意使用空验证器集合，也没有腾讯公开回调地址；外部腾讯流程号虽然已经用本地密钥保护绑定到本地路由，却缺少回调侧的安全反查入口。因此旧测试只能证明中立算法可用，不能证明腾讯官方回调已经接入。

本次完成以下实现：

- `internal/tencentess/config.go`新增封闭能力`callback.receive`。能力、验签令牌和解密密钥必须同时完整存在，否则启动失败关闭；未启用时即使误放秘密也拒绝配置。
- `internal/tencentess/callback.go`保留原始验签字节，拒绝重复 JSON 字段、尾随数据、错误签名、错误密文、错误填充、错误应用、错误主体、未来时间、未知状态和未绑定流程。
- 腾讯流程号只在当前安装、当前供应商和保留密钥版本范围内计算受保护摘要并反查唯一现有路由；不保存、记录或向业务接口返回流程号明文，也没有跨安装或软件提供方兜底查询。
- 原始已签正文、签名和已验证明文被封装成一次性证据帧，再由本地证据密钥环用`AES-256-GCM`加密保存。调用结束后清零正文、签名、明文、流程号和证据缓冲。
- `internal/app/tencent_callback_handler.go`固定方法、路径、精确 JSON 内容类型、单个签名头和 256 KiB 正文上限。只有证据、去重、路由投影和本地投递事务提交成功才返回`200`；冲突返回`409`，其余验证失败返回非`200`。
- 运行时只有在能力启用且完整验证器构造成功时注册回调地址；关闭能力时地址为`404`。诺诺验证器继续关闭，不存在中央回调、中央路由或中央结算回退。

## 4. 审查中修正的问题

新证据需要同时保留“收到的加密原文”和“验签解密后的明文”，既有数据库约束仍按旧单帧上限`299024`字节限制密文，导致最大边界真实 PostgreSQL 测试失败。新增不可变迁移`019-tencent-callback-evidence.sql`把密文上限调整为`561168`字节，同时完整保留以下安全约束：

- 随机数必须正好 12 字节；
- 密文格式版本必须为 1；
- 加密密钥版本必须大于 0。

迁移测试现在直接读取 PostgreSQL 实际约束定义，防止以后只扩大容量却再次丢失安全条件。发布清单测试也要求 Windows 和 Linux 组件包必须包含迁移`019`。

## 5. 自动化验证

```text
LINAPRO_IDC_COMPLIANCE_GATEWAY_TEST_DSN=<本机测试库> GOWORK=off \
  go test -count=1 -v ./internal/suppliercallback
PASS（18 个用例全部通过）

LINAPRO_IDC_COMPLIANCE_GATEWAY_TEST_DSN=<本机测试库> GOWORK=off \
  go test -count=1 ./...
PASS

GOWORK=off go vet ./...
GOWORK=off staticcheck ./...
PASS

make.cmd lint.go dir=apps/lina-plugins/idc-compliance/central-gateway
PASS（golangci-lint 0 issues；staticcheck 通过）

make.cmd plugins.check
PASS（扫描 2360 个文件，0 findings）

openspec validate add-idc-server-operations-platform --strict
PASS

git diff --check
PASS
```

真实 PostgreSQL 覆盖本地流程绑定反查、跨路由冲突、重复恢复、乱序单调收敛、相反终态、并发去重、正文与密文最大边界、19 个迁移首次执行、迁移账本幂等恢复以及最终数据库约束。腾讯单元测试覆盖正确验签解密、篡改在路由查询前拒绝、当前安装主体错配拒绝、全部官方状态映射、完整回调配置接受和不完整配置拒绝。HTTP 回归明确证明能力关闭时公开地址不存在。

当前 Windows 环境`CGO_ENABLED=0`，Go 工具链不支持执行`-race`，因此没有把竞态检测伪记为通过；并发正确性由真实 PostgreSQL 行锁、唯一约束和并发用例验证。

## 6. 双平台交付证据

发布工具重新构建了 Windows amd64 和 Linux amd64 本地网关组件包。两个清单各包含 29 个受哈希保护的文件，均包含迁移`019-tencent-callback-evidence.sql`，该迁移文件的清单哈希均为`6715928b6d80c59bb5236d041daef8efaa836c211b1a8a336d809ec563a25493`。

- Windows amd64 `manifest.json` SHA-256：`2CEA9BAA2C2F7542AAE3AF624F4415F351E7C253EB3B0E9CA1007F80135D0120`
- Linux amd64 `manifest.json` SHA-256：`6B927BB6E15ED1CED7885D897F812834B7EFA219AA78A3681770A126362685E1`

双语安装说明和腾讯配置示例已同步回调地址、能力、令牌与密钥要求。该组件继续随每套系统安装并由购买方自行监管，不依赖软件提供方业务网络服务。

## 7. 任务级审查

`lina-review`复核了能力关闭、配置完整性、原始正文验签、固定算法、严格解密、应用与本地主体核对、受保护流程反查、安装隔离、秘密与正文日志排除、缓冲清零、精确重放、事务后返回、迁移约束、双平台发布和中央依赖排除。最终没有遗留阻断`10.9.a`的`P0`、`P1`或`P2`问题。

本任务没有新增页面、用户可见文案或前端接口，因此没有新的`i18n`资源；`apps/lina-vben/apps/web-antd`差异为空。`web-customer`将在`10.12`实现客户合同页面，本后端回调切片不代替该任务。
