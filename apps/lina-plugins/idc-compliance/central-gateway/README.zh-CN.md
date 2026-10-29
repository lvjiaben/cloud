# LinaPro IDC 本地合规网关

本目录提供随每套 LinaPro 完整安装交付的本地合规网关独立进程。

[English](README.md) | 简体中文

## 纠偏状态

现有销售记录注册、远程实例身份、跨客户路由、平台操作者权威以及软件提供方与购买方之间的结算代码基于已废止架构产生，不是可上线能力，也不得继续扩展。OpenSpec 任务`10.4`至`10.7`负责盘点、删除并转换为本地所有权。

经过审查后可以复用的通用安全成果包括有界输入、幂等、防重放、持久限流、供应商写熔断、回调验签、单调收敛、主动查询、结果未知保护、加密证据和证据约束的人工接管。本地进程认证必须在安装时生成，不得依赖销售记录、远程注册令牌或商业授权码。

## 构建

在本目录运行跨平台 Go 发布工具，并提供一个尚不存在的组件包目录：

```text
go run ./hack/tools/release -out ./dist/idc-compliance-local-gateway -version dev -commit unknown -built-at unknown
```

构建 Windows 目标时传入`-goos windows`，工具会自动使用`.exe`文件名。工具固定使用`CGO_ENABLED=0`，关闭工作区和`VCS`注入，拒绝覆盖既有组件包目录，并生成包含`bin/`、`config/`、`migrations/`、`docs/`和`manifest.json`的完整组件包。清单记录每个文件的`SHA-256`，并明确本组件必须进入产品安装包、只在安装方本地运行，且软件提供方自营实例与购买方实例使用相同程序。

## 运行

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

默认监听地址为`127.0.0.1:9088`。支持以下进程设置：

同机运行的 LinaPro 源码插件使用`LINAPRO_COMPLIANCE_GATEWAY_INTERNAL_BASE_URL`，受审值固定为`http://127.0.0.1:9088`。三条签名写路由分别是`/internal/v1/supplier-subject-bindings`、`/internal/v1/supplier-subject-revocations`和`/internal/v1/supplier-subject-reopenings`。它们不接受浏览器访问，只返回不透明的本地操作编号和绑定编号。

只读对账每批最多领取 50 条路由，使用 PostgreSQL 权威时间、`SKIP LOCKED`、60 秒 256 位租约和最高 30 分钟退避；只有已经保护外部流程号的路由可领取，路由版本或租约变化后的迟到结果不能覆盖回调。本切片不新增公开人工接管路由：后续管理适配器必须提供精确权限、当前商家员工和数据范围证明、最近五分钟完整登录、最后读取版本、五类固定原因之一、同一路由一至十六条不可变证据和 256 位操作键。人工接管只停止自动化，不能强制成功、强制失败、替换外部流程号或重试供应商写操作。

## 完整交付

商家源码插件在`manifest/delivery/local-compliance-gateway.yaml`声明本地网关为每套产品安装包的必需组件；源码插件本身不嵌入某一操作系统的生成二进制。发布装配必须为目标平台执行本目录的发布工具，并把生成的完整组件包与宿主、业务插件和`web-customer`一同交付。安装、监管、备份、恢复和卸载边界见组件包内双语运行说明。

旧 Portal 当前仍是迁移基线，不进入本地网关组件包；它的页面、测试和最终删除只由`FB-10`及`15.12.a`至`15.12.b`处理，本任务不提前删除或把旧 Portal 当作最终客户前端。


电子签适配器、公开回调与合同路由已经退役；诺诺供应商能力在独立接口证据验证前继续关闭。旧业务证据保留，不恢复旧电子签处理。
