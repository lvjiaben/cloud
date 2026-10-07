# 第十阶段中央身份签发与注册事务验证记录

## 1. 任务结论

任务`10.6.b.1`已完成中央端销售授权注入、一次性注册令牌签发和耐久注册事务。该切片没有开放令牌签发或注册 HTTP 路由，没有装配腾讯或诺诺客户端，也没有改变中央服务失败关闭的就绪状态。

商家端受保护本机密钥与令牌管理留在`10.6.b.2`，注册传输安全和运行时装配留在`10.6.b.3`，所以父任务`10.6.b`保持未完成。

## 2. 已实现不变量

### 2.1 销售授权与签发恢复

- 注册服务只能通过构造函数注入的平台销售权威能力取得销售记录、商家编号、授权证据摘要和允许的令牌有效期，不存在公共 HTTP 签发入口。
- 平台销售系统必须生成并耐久保存带`lop1_`前缀的`256`位不可见幂等操作键。
- 中央端分别使用独立 HMAC 域计算操作键摘要、签发请求摘要、令牌派生值和令牌数据库摘要，禁止跨用途复用。
- 注册令牌由销售记录与幂等操作键确定性派生，因此签发事务提交后即使响应丢失，完全相同请求仍能恢复同一令牌、注册编号和到期时间。
- 数据库只保存操作键 HMAC、签发请求`SHA-256`和令牌 HMAC，不保存操作键或令牌原文；同一操作键改变销售记录、商家、授权证据或有效期时返回`registration_conflict`。

### 2.2 原子注册与精确重放

- 注册服务先验证固定域、长度前缀帧和`Ed25519`持有证明，再把令牌摘要交给存储层，原始令牌不会进入存储命令。
- PostgreSQL 事务锁定令牌行，并使用数据库权威时间判断到期；令牌消费、安装实例、第一代活动公钥和三条注册身份事件在同一事务提交。
- 相同令牌、公钥和随机数形成的完全相同请求摘要可以恢复原实例与密钥编号；任何变更、过期、撤销或未知令牌统一失败关闭为`registration_rejected`。
- 第一代注册密钥退役后，旧注册证明不能恢复为当前有效凭据，统一返回`registration_rejected`。
- 签发使用事务级 advisory lock 串行化同一操作 HMAC，注册使用令牌行锁串行化同一令牌；并发精确请求只产生一个首次结果，其余返回恢复结果。

### 2.3 错误和数据边界

- 对外稳定错误码只有`invalid_request`、`sale_not_authorized`、`registration_conflict`、`registration_rejected`和`temporary_unavailable`。
- 错误文本只暴露稳定错误码，内部授权或数据库原因只通过错误链保留，不能进入客户端响应。
- 身份事件只保存封闭事件码、操作者类型和摘要证据，不保存令牌、操作键、私钥、签名或销售资料原文。
- 未新增列表、详情、导出或管理写接口，因此本切片无宿主数据权限接入点；后续 HTTP 装配必须独立完成认证、限流和数据存在性保护。

## 3. 验证结果

中央模块门禁通过：

```text
GOWORK=off go test ./... -count=20 -shuffle=on
GOWORK=off go vet ./...
golangci-lint run --config E:\golang\cloud\.golangci.yml ./...
staticcheck ./...
```

真实 PostgreSQL 测试只在显式提供的可丢弃`linapro_idc_codegen`数据库中创建随机`linapro_gateway_identity_*`隔离 schema，测试结束后删除该 schema。生命周期、过期和并发用例各随机执行十轮并通过：

```text
LINAPRO_IDC_COMPLIANCE_GATEWAY_TEST_DSN=<disposable-codegen-dsn> \
go test ./internal/identity \
  -run 'TestRegistrationPostgreSQLLifecycle|TestExpiredRegistrationPostgreSQLFailsClosed|TestConcurrentExactIssueAndRegistrationPostgreSQL' \
  -count=10 -shuffle=on

PASS
```

竞态检查在当前 Windows 环境未执行成功：Go 的`-race`要求 CGO，而当前机器没有可用的`gcc`。这属于明确记录的本机工具链限制，不伪造通过；普通并发真实库测试、20 轮全量单测和全部静态门禁均已通过，正式发布流水线仍应在具备受支持 C 编译器的环境执行`go test -race ./internal/identity ./pkg/identityprotocol`。

## 4. 影响评估

- 数据库：只修改中央网关独立模块的首个身份迁移，新增签发幂等 HMAC 与请求摘要约束；未修改商家插件或宿主数据库。
- `API`：新增中央模块内部服务和公开的纯数据注册协议包，没有新增 HTTP 路由。
- 数据权限：销售授权仅为构造注入能力，商家安装实例尚无网络入口。
- 缓存：无影响，所有状态与时间判断来自 PostgreSQL。
- `i18n`：无运行时界面和用户可见文案影响。
- 宿主：未修改宿主代码、页面或构建入口。

## 5. 审查结论

本切片按 LinaReview 检查了权限边界、秘密最小化、密码学域隔离、响应丢失恢复、数据库权威时间、事务原子性、并发重放、状态泄露和错误映射。首代密钥轮换后的注册重放已由真实库测试证明失败关闭。当前未发现阻断`10.6.b.1`的`P0`、`P1`或`P2`问题。
