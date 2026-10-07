# 第十阶段腾讯电子签官方客户端与运行时门禁验证记录

## 验证范围

本记录只关闭任务`10.8.a`。本轮实现每套购买方安装自行持有和运行的腾讯电子签服务端客户端、私有应用配置、服务端主体接线、逐能力运行时检查、安全错误映射、受保护合同发起和主动查询接线。未实现后台开通向导、模板本地版本、合同业务模型、自动签本地策略、客户页面、腾讯回调、文件归档或腾讯用量采集器，因此不勾选`10.8.b`至`10.8.g`、`10.8`父任务、`10.9`或任何第十五阶段任务。

## 实现证据

| 能力 | 实现与边界 |
|---|---|
| 官方 SDK | `central-gateway/go.mod`直接固定腾讯官方`common`和`essbasic`模块`v1.3.159`；生产包只使用`v20210526`客户端。 |
| 固定网络出口 | `internal/tencentess/client.go`固定`essbasic.tencentcloudapi.com`、`HTTPS`、`POST`、`TC3-HMAC-SHA256`和 30 秒超时，并关闭 SDK 自动网络重试、限流重试、区域切换、代理、自定义端点和调试输出。配置结构不存在端点、代理或区域字段。 |
| 购买方私有应用 | `internal/tencentess/config.go`只从`LINAPRO_COMPLIANCE_GATEWAY_TENCENT_ESSBASIC_CONFIG_FILE`指向的绝对私有文件读取当前安装自己的`SecretId`、`SecretKey`、`AppId`和封闭能力清单；未知字段、重复能力、错误版本、宽松权限和超限文件均失败关闭。 |
| 主体安全 | 私有应用文件不含腾讯企业或操作人主体。`internal/tencentess/subject.go`只规范化服务端一次性授权投影，`supplieridentity.Service`加密保存后仅在同步 SDK 调用期间解密并清理；浏览器、普通业务请求和部署文件都不能提交或切换主体原文。 |
| 逐能力门禁 | 能力清单固定为模板读取、合同发起、签署链接、主动查询、下载、企业自动签、用量和账号套餐八项。模板、自动签、下载授权、用量和账号套餐使用有界只读探测；扩展服务探测必须读取腾讯返回的`ENABLE`状态，必须依赖真实合同或执行写入的能力明确返回`operation_check_required`并在每次操作中复核，不用一次成功冒充全部可用。 |
| 写入保护 | 单合同模板发起先取得现有 PostgreSQL 权威写租约，继承业务幂等、每安装与每操作限流、持久熔断和半开探针；恢复请求不重复调用腾讯，并按既有结果返回成功、明确失败或`result_unknown`，不得把未知结果冒充成功。成功后先密封外部`FlowId`再记录成功，网络中断、超时、响应缺失或本地结果提交失败均保持`result_unknown`。 |
| 读取与短期链接 | 主动查询最多一次一百个`FlowId`，校验腾讯响应的`ApplicationId`和`ProxyOrganizationOpenId`与当前活动主体一致，再映射到本地运行中/成功/失败状态并密封权威响应。客户签署链接只按需生成，固定 30 分钟、`HTTPS`返回地址且不缓存为长期证据。 |
| 发布装配 | `deploy/tencent-essbasic.json.example`、环境变量说明和双语运行文档进入组件包；发布清单固定`supplierClientsIncluded=true`且`businessRoutesEnabled=false`，准确表示客户端已交付但管理与合同业务路由尚未开放。 |

## 自动化验证

在`apps/lina-plugins/idc-compliance/central-gateway`下使用`GOWORK=off`执行：

| 命令 | 结果 |
|---|---|
| `go test ./...` | 通过；覆盖本地网关全部 Go 包，腾讯包使用假官方客户端且没有生产触网。 |
| `go vet ./...` | 通过。 |
| `make.cmd lint.go dir=apps/lina-plugins/idc-compliance/central-gateway plugins=1` | 通过；`golangci-lint`为 0 个问题，`staticcheck U1000`通过。 |
| `go test -cover ./internal/tencentess` | 通过，语句覆盖率`68.8%`。 |
| 发布工具构建`windows/amd64`组件包 | 通过；清单 29 个文件，包含`config/tencent-essbasic.json.example`，`supplierClientsIncluded=true`，`businessRoutesEnabled=false`。 |
| 发布工具构建`linux/amd64`组件包 | 通过；清单字段与配置示例同样正确。 |
| `openspec validate add-idc-server-operations-platform --strict` | 通过。 |

曾尝试执行`go test -race ./internal/tencentess ./internal/app`，但当前 Windows 环境未安装`gcc`，Go race 构建无法启用`cgo`。本轮未把该未执行项写成通过；普通全模块测试、`go vet`和双目标纯 Go 构建均已通过，完整真实 PostgreSQL、前端与回归门禁仍由`10.8.g`统一完成。

## 保持未完成

- `10.8.b`后台“合规中心 → 电子合同”开通向导和可见错误说明未实现。
- `10.8.c`模板本地不可变版本、预览、复制、草稿、启停和删除保护未实现。
- `10.8.d`签署方、模板变量、订单/服务/金额快照以及可选/强制合同门禁未实现。
- `10.8.e`企业自动签本地授权、近期登录、二次确认、范围和风险上限未实现。
- `10.8.f`客户合同后端契约未实现，且未进入`@lina/web-customer`或第十五阶段。
- `10.8.g`真实 PostgreSQL、管理端类型/构建、双语和完整回归证据未完成。
- 腾讯回调验签与公开回调路由仍由`10.9`实现；当前发布清单继续声明`businessRoutesEnabled=false`。
- 腾讯用量只读接口已经进入官方客户端和能力检查，但生产用量采集器尚未接入`localusage`，因此不会启动腾讯用量采集工作线程。
- 诺诺客户端、识别器、查询器、采集器和回调继续受`10.4.c`与`10.10`外部证据门禁关闭，没有中央替代路径。
