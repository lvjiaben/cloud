# LinaPro IDC 本地合规网关

本目录提供随每套 LinaPro 完整安装交付的本地合规网关独立进程。

[English](README.md) | 简体中文

## 纠偏状态

有效目标是随完整程序安装并在每个购买方自身环境运行的独立进程。每个安装本系统的`IDC`商家提供自己的腾讯电子签、诺诺或替代供应商账号、应用凭据、法定主体、回调地址、数据库、监控、备份和结算关系。软件提供方不代理购买方合规业务流量，也不结算购买方的供应商用量。

现有销售记录注册、远程实例身份、跨客户路由、平台操作者权威以及软件提供方与购买方之间的结算代码基于已废止架构产生，不是可上线能力，也不得继续扩展。OpenSpec 任务`10.4`至`10.7`负责盘点、删除并转换为本地所有权。

经过审查后可以复用的通用安全成果包括有界输入、幂等、防重放、持久限流、供应商写熔断、回调验签、单调收敛、主动查询、结果未知保护、加密证据和证据约束的人工接管。本地进程认证必须在安装时生成，不得依赖销售记录、远程注册令牌或商业授权码。

腾讯生产客户端固定使用官方`2021-05-26` Go SDK 契约，并且只能从当前购买方安装的私有应用文件启用。腾讯主体规范化器只接受服务端得到的一次性授权投影，不向浏览器提供企业或操作人编号输入框。诺诺继续受独立外部证据门禁约束并保持关闭。本地主体绑定、撤销和重新开通的签名路由继续在对应识别器不可用时失败关闭。

本地用量采集服务始终完成装配，并为两个固定供应商记录明确状态。当前腾讯客户端可以执行显式只读能力检查，且只在选择`flow.query`时注册主动查询工作线程，但本切片的生产用量采集器映射仍为空；诺诺也继续保持`disabled`。就绪状态不依赖供应商可用性。后续经审查启用的采集器只在当前安装内运行：启动后立即执行一次，之后每 15 分钟调度；每次最多领取两个供应商，使用 PostgreSQL 权威时间、60 秒租约、45 秒调用时限，以及从 1 分钟至 30 分钟的有界退避。一次原子提交最多接收一个套餐、200 条用量、八条原币种对账、一个`SLA`观测和 256 KiB 加密证据。预算告警只使用本地显式启用的策略；网关不会猜测预算、转换币种或回退调用软件提供方服务。

## 构建

在本目录运行跨平台 Go 发布工具，并提供一个尚不存在的组件包目录：

```text
go run ./hack/tools/release -out ./dist/idc-compliance-local-gateway -version dev -commit unknown -built-at unknown
```

构建 Windows 目标时传入`-goos windows`，工具会自动使用`.exe`文件名。工具固定使用`CGO_ENABLED=0`，关闭工作区和`VCS`注入，拒绝覆盖既有组件包目录，并生成包含`bin/`、`config/`、`migrations/`、`docs/`和`manifest.json`的完整组件包。清单记录每个文件的`SHA-256`，并明确本组件必须进入产品安装包、只在安装方本地运行，且软件提供方自营实例与购买方实例使用相同程序。

## 运行

默认监听地址为`127.0.0.1:9088`。支持以下进程设置：

| 环境变量 | 默认值 |
|---|---|
| `LINAPRO_COMPLIANCE_GATEWAY_LISTEN_ADDR` | `127.0.0.1:9088` |
| `LINAPRO_COMPLIANCE_GATEWAY_READ_HEADER_TIMEOUT` | `5s` |
| `LINAPRO_COMPLIANCE_GATEWAY_READ_TIMEOUT` | `10s` |
| `LINAPRO_COMPLIANCE_GATEWAY_WRITE_TIMEOUT` | `10s` |
| `LINAPRO_COMPLIANCE_GATEWAY_IDLE_TIMEOUT` | `30s` |
| `LINAPRO_COMPLIANCE_GATEWAY_SHUTDOWN_TIMEOUT` | `10s` |
| `LINAPRO_COMPLIANCE_GATEWAY_DATABASE_DSN_FILE` | 无 |
| `LINAPRO_COMPLIANCE_GATEWAY_INSTALLATION_ID_FILE` | 无 |
| `LINAPRO_COMPLIANCE_GATEWAY_PROCESS_AUTH_KEYRING_FILE` | 无 |
| `LINAPRO_COMPLIANCE_GATEWAY_SUPPLIER_IDENTITY_KEYRING_FILE` | 无 |
| `LINAPRO_COMPLIANCE_GATEWAY_CALLBACK_EVIDENCE_KEYRING_FILE` | 无 |
| `LINAPRO_COMPLIANCE_GATEWAY_PROCESS_AUTH_WINDOW` | 无 |
| `LINAPRO_COMPLIANCE_GATEWAY_TENCENT_ESSBASIC_CONFIG_FILE` | 无 |

同机运行的 LinaPro 源码插件使用`LINAPRO_COMPLIANCE_GATEWAY_INTERNAL_BASE_URL`，受审值固定为`http://127.0.0.1:9088`。三条签名写路由分别是`/internal/v1/supplier-subject-bindings`、`/internal/v1/supplier-subject-revocations`和`/internal/v1/supplier-subject-reopenings`。它们不接受浏览器访问，只返回不透明的本地操作编号和绑定编号。

进程超时必须在一秒到两分钟之间，数据库、安装标识、进程密钥环、供应商主体密钥环和回调对账密钥环五个运行文件路径必须存在、为绝对路径且互不相同。首次安装使用`idc-compliance-local-gateway-keyring -out <密钥环路径> -installation-id-out <安装标识路径>`生成安装标识和进程密钥环，使用`idc-compliance-local-gateway-supplier-keyring -out <主体密钥环路径>`生成三套独立版本化主体密钥，再使用`idc-compliance-local-gateway-evidence-keyring -out <证据密钥环路径>`生成证据加密、外部事件摘要、外部流程号加密、外部流程号查询摘要和人工接管操作摘要五套相互独立的版本化密钥。恢复同一实例时保留对应标识和全部密钥环，克隆为新商家实例时必须重新生成。`LINAPRO_COMPLIANCE_GATEWAY_PROCESS_AUTH_WINDOW`必须显式设置在`1s`至`60s`硬上限之间，签发时间最多可领先 PostgreSQL 权威时间`30s`。在 Unix 上，五个必需文件均不得向组或其他用户授予权限；可选腾讯应用文件也必须使用绝对路径、私有权限并与所有必需文件互不相同，且不含可配置端点、代理或主体字段。进程启动时校验本地安装标识和三类密钥环、验证数据库、应用迁移，并只在可选文件存在时装配腾讯客户端。腾讯公开回调只在同时配置`callback.receive`、验签令牌和精确`32`字节`CallbackUrlKey`时注册，且只在`/callbacks/v1/tencent-essbasic`接收加密`FlowStatusChange`。诺诺识别器、查询器、采集器和公开回调继续关闭。

只读对账每批最多领取 50 条路由，使用 PostgreSQL 权威时间、`SKIP LOCKED`、60 秒 256 位租约和最高 30 分钟退避；只有已经保护外部流程号的路由可领取，路由版本或租约变化后的迟到结果不能覆盖回调。本切片不新增公开人工接管路由：后续管理适配器必须提供精确权限、当前商家员工和数据范围证明、最近五分钟完整登录、最后读取版本、五类固定原因之一、同一路由一至十六条不可变证据和 256 位操作键。人工接管只停止自动化，不能强制成功、强制失败、替换外部流程号或重试供应商写操作。

## 完整交付

商家源码插件在`manifest/delivery/local-compliance-gateway.yaml`声明本地网关为每套产品安装包的必需组件；源码插件本身不嵌入某一操作系统的生成二进制。发布装配必须为目标平台执行本目录的发布工具，并把生成的完整组件包与宿主、业务插件和`web-customer`一同交付。安装、监管、备份、恢复和卸载边界见组件包内双语运行说明。

旧 Portal 当前仍是迁移基线，不进入本地网关组件包；它的页面、测试和最终删除只由`FB-10`及`15.12.a`至`15.12.b`处理，本任务不提前删除或把旧 Portal 当作最终客户前端。
