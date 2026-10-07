# 第十阶段中央网关注册传输验证记录

## 1. 任务结论

任务`10.6.b.3`已完成中央实例注册的受限网络入口、TLS 运行时、商家端安全注册器和显式服务组合边界。`10.6.b.1`至`10.6.b.3`全部完成，因此父任务`10.6.b`完成；密钥轮换撤销、业务请求认证、供应商回调与主动查询仍分别由`10.6.c`至`10.6.e`承接，父任务`10.6`保持未完成。

本切片没有开放平台销售令牌签发 HTTP 接口，没有装配腾讯或诺诺客户端，也没有新增商家后台、Portal 或宿主页面。供应商业务门禁尚未完成，中央就绪探针继续返回`503 supplier_gate_closed`。

## 2. 已实现边界

### 2.1 中央入口与运行时

- 唯一非健康入口固定为`POST /v1/instance-registrations`，只有显式注入注册专用服务时才注册；未配置时路由不存在。
- 请求必须为`application/json`，最大`2048`字节，拒绝未知字段、尾随值和错误协议版本。字节字段采用 JSON Base64，进入耐久服务前验证一次性令牌与`Ed25519`持有证明。
- 中央运行时只有在`LINAPRO_COMPLIANCE_GATEWAY_REGISTRATION_ENABLED`精确为`true`且四个绝对、互不相同的文件路径全部存在时才启用注册。
- 数据库 DSN、用途隔离 HMAC 密钥和 TLS 私钥只从有界普通文件读取；Unix 秘密文件不得向组或其他用户授权。HMAC 文件必须是规范无填充 Base64URL 编码的`256`位密钥，秘密原文不放入环境变量。
- 启动顺序固定为加载当前有效且密钥匹配的 TLS 身份、验证 PostgreSQL、执行幂等迁移、构造无销售签发能力的注册服务，再开始监听。注册启用时所有路由统一使用 TLS 1.3。
- HTTP 路由图中不存在销售令牌签发路径；运行时注册服务不注入销售权威能力，即使进程内误调用签发方法也返回`sale_not_authorized`。

### 2.2 商家端安全传输

- 商家注册器只接受无用户信息、路径、查询和片段的精确 HTTPS 源，固定拼接版本化注册路径。
- 客户端使用平台注册地址专用 CA 池和 URL 主机名校验，最低 TLS 1.3；不使用系统代理，并拒绝所有重定向。
- 拨号、TLS 握手、响应头和总请求均有固定超时；请求最大`2048`字节，响应最大`4096`字节，压缩关闭。
- 令牌、公开证明和签名只在`secretcap`生产者同步回调内完成验证、序列化和网络交换；请求及响应缓冲在返回前清零，错误不包含响应正文或秘密。
- 成功响应必须是严格 JSON。`201`只能表示首次注册且`recovered=false`，`200`只能表示精确恢复且`recovered=true`；非受信证书、重定向、非 JSON、未知字段、超限内容或状态语义冲突统一失败关闭。
- 中央明确拒绝映射为本地`ErrRejected`；连接中断、超时、非规范响应和未知状态映射为`ErrUnavailable`。本地状态保持`ready`，后续仍复用同一令牌、密钥、随机数和证明恢复。

### 2.3 组合与交付隔离

- 商家插件提供显式`BuildGatewayEnrollmentService`组合入口，把两个隔离`secretcap`作用域、本地 PostgreSQL 存储和固定 HTTPS 注册器组成单一服务图；没有新增公共 HTTP、后台按钮或运行时全局查找。
- 中央服务仍位于独立嵌套 Go 模块，商家插件嵌入清单不包含中央源码、二进制、数据库配置、HMAC 密钥或 TLS 私钥。
- 插件版本推进为`v0.1.8`，中英文说明同步记录注册网络边界；中央中英文运行文档同步记录文件配置和 TLS 要求。

## 3. 验证结果

中央模块全量单测、静态检查和真实 PostgreSQL 隔离验证通过：

```text
GOWORK=off go test ./... -count=20 -shuffle=on
GOWORK=off go vet ./...
golangci-lint run --config E:\golang\cloud\.golangci.yml ./...
staticcheck ./...

LINAPRO_IDC_COMPLIANCE_GATEWAY_TEST_DSN=<disposable-dsn> \
go test ./internal/identity ./internal/migration ./internal/app \
  -run 'TestRegistrationPostgreSQLLifecycle|TestExpiredRegistrationPostgreSQLFailsClosed|TestConcurrentExactIssueAndRegistrationPostgreSQL|TestIdentityMigrationPostgreSQL|TestRegistrationRuntimePostgreSQLTLSBoundary' \
  -count=10 -shuffle=on

PASS
```

商家插件全量单测、静态检查、插件治理和本地注册真实库验证通过：

```text
GOWORK=off go test ./backend/... -count=20 -shuffle=on
GOWORK=off go vet ./backend/...
make.cmd lint.go plugins=1 dir=apps/lina-plugins/idc-compliance
make.cmd plugins.check

LINAPRO_IDC_COMPLIANCE_TEST_DSN=<disposable-dsn> \
go test ./backend/internal/service/gatewayenrollment \
  -run 'TestGatewayEnrollmentPostgreSQLLifecycleAndSecretExclusion|TestGatewayEnrollmentPostgreSQLConcurrentExactRequest' \
  -count=20 -shuffle=on

PASS
```

受控 TLS 测试覆盖精确方法、路径、内容类型和持有证明，首次与恢复状态、中央明确拒绝、非规范错误、重定向、非受信证书、不安全源、错误 CA、严格响应和组合失败关闭。中央运行时真实库测试为每轮创建并删除随机`linapro_gateway_runtime_*`schema，同时证明数据库迁移先于 TLS 注册入口、注册服务不能签发令牌且猜测的签发路由返回`404`。

独立发布工具重新构建 Windows 与 Linux `amd64`产物及完整性清单并通过：

```text
windows-amd64 sha256 720802a42fcb64c26e6ee90d696c7b7d40bd5c540155b31d3b4dde3b0e6779a4
linux-amd64   sha256 ad8dcc20ececebc8b2c597627849a1f3b42c2f54eabf2d1eb1bd092d4501e8d9
```

当前 Windows Go 工具链报告`CGO_ENABLED=0`且系统没有`gcc`，所以`go test -race`无法执行；该环境限制与`10.6.b.1`记录一致，不伪造通过。具备受支持 C 编译器的发布流水线仍须执行中央`internal/app`、`internal/identity`、`pkg/identityprotocol`以及商家`gatewayenrollment`的竞态测试。

## 4. 规则域评估

- 架构与插件：实现闭环在`idc-compliance`及其独立中央子模块，只消费`idc-core`公开`secretcap`；未修改宿主源码或宿主页面。
- API：新增的是中央独立服务的版本化注册协议与唯一路由，不进入 LinaPro 宿主 API；方法、路径、媒体类型、体积、状态码和稳定错误均有双端契约测试。
- 数据权限：没有商家客户或员工 HTTP 入口，也没有列表、详情、下载、导出或管理动作。销售签发只允许平台内部权威能力，公共网络无法访问。
- 数据库：没有新增本切片迁移；复用已验证的中央身份迁移与商家三阶段注册表。运行时启动在监听前执行幂等迁移，无缓存或双重权威源。
- 性能：每次注册固定一次 HTTPS 请求和单行中央注册事务；请求、响应、连接池和超时全部有界，不存在列表或`N+1`装配。
- 安全：TLS 1.3、专用 CA、主机名校验、无代理、无重定向、秘密文件、严格协议和精确恢复共同失败关闭；没有危险操作、供应商调用或线上数据变更。
- `i18n`与前端：没有运行时用户文案、菜单、页面或用户可观察工作流变化。中英文技术 README 已同步。
- 开发工具：未修改根构建脚本或 CI；只使用中央模块既有发布工具验证独立产物。

## 5. LinaReview 结论

审查覆盖路由最小化、销售权威隔离、TLS 与证书信任、秘密来源、响应丢失、协议限长、错误折叠、状态语义、数据库启动顺序、关闭路径、测试隔离、文档镜像和宿主边界。审查期间补充了全文件路径互斥、TLS 文件限长、叶证书当前有效性校验、共享错误码、HSTS 和传输测试并发安全。最终未发现阻断`10.6.b.3`或父任务`10.6.b`的`P0`、`P1`或`P2`问题。
