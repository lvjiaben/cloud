# 第十阶段阿里云实人认证后端底座验证记录

## 实现范围

本记录覆盖任务`10.2.a`。`idc-compliance`由`v0.1.1`提升到`v0.1.2`，新增阿里云供应商配置、个人认证尝试表、固定协议适配器、安装实例级凭据配置服务和精确后台权限。本轮没有开放`HTTP API`、后台页面、客户页面、回跳路由或恢复调度器，也没有调用真实阿里云账号；运行时尚不可从外部路由触达。

任务`10.2`父任务保持未完成。后台向导与最近重新认证门禁由`10.2.b`交付，个人认证状态机与案件投影由`10.2.c`交付，客户页面、调度器、单独同意和`E2E`由`10.2.d`交付。

## 官方契约冻结

截至`2026-08-12`，实现依据阿里云[金融级实人认证 HTTPS 原生调用文档](https://help.aliyun.com/zh/id-verification/financial-grade-id-verification/native-https-calls-1)、[金融级实人认证方案概述](https://help.aliyun.com/zh/id-verification/financial-grade-id-verification/product-overview/overview)和[常见问题](https://help.aliyun.com/zh/id-verification/financial-grade-id-verification/support/faq)冻结以下边界：

- 请求固定为`POST https://saf.cn-shanghai.aliyuncs.com`，公共参数固定使用`Action=ExecuteRequest`、`Version=2017-03-31`、`Service=fin_face_verify`和供应商规定的`HMAC-SHA1`签名。端点、协议和产品标识不是商家配置项。
- 发起固定使用`method=init`、`bizCode=FACE`、`identityType=CERT_INFO`和`certType=IDENTITY_CARD`；`outerOrderNo`是唯一 32 位字母数字串。返回的`certifyId`和`certifyUrl`有效 30 分钟且只能提交一次。
- 查询固定使用`method=query`、原`sceneId`和`certifyId`。`passed=T`是唯一通过事实；`passed=F`同时包含失败与尚未提交，不能提前写成终态；供应商代码`407/408`表示失效。
- 商业化范围只接受中国内地二代身份证，以及港澳台居民居住证。低于 14 周岁不支持，14 周岁及以上未成年人可以使用。其他证件不自动降级为弱认证。

## 安全与事务边界

- 一组阿里云`AccessKey ID`和`AccessKey Secret`作为一个原子秘密信封，通过回调作用域字节缓冲区进入`secretcap`。调用完成后清零输入、副本、签名请求体和响应缓冲区；服务投影只返回是否配置、掩码尾部、代次、版本和安全时间。
- 数据库禁止`access_key_id`、`access_key_secret`、`credential_ciphertext`、`certify_url`等列，也不保存人脸图片、视频、身份明文或供应商原始报文。`credential_secret_ref`只供插件内部恢复路由使用，不进入接口、日志或审计快照。
- 配置写入使用两阶段失败关闭协议：第一事务锁定单例并写`updating`、幂等摘要和准备审计；随后幂等创建或替换秘密；第二事务以数据库版本和秘密更新时间围栏提交`configured`、代次、版本及完成审计。完成审计失败会整体回滚第二事务，重试同一幂等键可恢复且不会产生第二份秘密。
- 查询与写入分别要求`idc:admin:compliance:provider:query`和`idc:admin:compliance:provider:write`。授权在数据库读取、凭据复制或外部能力调用前完成，目标固定为安装实例级配置；越权请求不泄露配置是否存在。
- 供应商客户端只接受超时不超过 30 秒的显式`HTTP`客户端副本，禁用重定向，限制响应为 256 KiB，并验证返回认证入口为无用户信息、无片段的`HTTPS URL`。初始化传输或供应商`5xx`保持结果未知，禁止自动重复外部写。

## 数据模型与性能

合规插件现有 15 张表，其中本轮新增`provider_configuration`和`personal_verification_attempt`。配置表是默认租户的阿里云单例，保存安全状态、场景、秘密引用、代次、幂等围栏和乐观版本。认证尝试表按案件保存外部订单号、`certifyId`、配置版本、30 分钟期限、非终态观察次数和终态时间，不保存认证入口地址。

配置查询最多一次单行数据库读取和一次本地秘密元数据读取；配置写入是两个有界事务、一次秘密变更和两条审计，不存在列表、循环装配或`N+1`。本轮没有引入缓存，PostgreSQL 单例行和`secretcap`元数据是权威来源。

## 测试与门禁证据

| 命令或检查 | 结果 |
|---|---|
| `go test ./backend/internal/service/providerconfig -count=20 -shuffle=on` | 通过；真实 PostgreSQL 覆盖创建、精确重放、替换、陈旧版本冲突、完成审计失败恢复、越权零写入和输入缓冲区清零 |
| `go test ./backend/internal/provider/aliyunface -count=20 -shuffle=on` | 通过；覆盖固定签名参数、未知初始化、`passed=F`非终态、`407/408`过期、不安全 URL 拒绝和重定向禁用 |
| `TestComplianceMigrationIdentifiersFitPostgreSQL` | 通过；全部 PostgreSQL 标识符不超过 63 字节 |
| `TestComplianceFoundationMigrationPostgreSQL` | 通过；15 张表、11 个关键索引、迁移幂等、危险列缺失及新约束均在随机隔离 schema 验证 |
| `TestComplianceFoundationCodegenPostgreSQL` | 通过；15 张表的 DAO、DO 与 Entity 和迁移一致 |
| `GOWORK=off go test ./backend/... -count=1 -shuffle=on` | 通过；插件后端全包编译和行为测试通过 |
| `make.cmd plugins.check` | 通过；扫描 11 份插件清单，生产治理发现 0 项 |
| `openspec validate add-idc-server-operations-platform --strict` | 通过 |

定向`make.cmd lint.go plugins=1 dir=apps/lina-plugins/idc-compliance`首次发现一处对包装错误使用直接类型断言，已改为`errors.As`并纳入复测。`make.cmd i18n.check`确认运行时硬编码和消息覆盖通过，插件新增的 5 个错误码中英文资源完整；整仓前端键覆盖仍被禁止修改的宿主通知预览页 6 个既有缺键阻断，与本切片无关且未越权修改。

## 规则域影响与审查结论

- 架构、插件和依赖注入：实现闭环在业务插件。`secretcap`与`managementcap`均由`idc-core`公开能力注入，插件没有访问宿主实现包。秘密管理器在插件组合层绑定默认租户和合规 owner；服务构造与路由装配仍是`10.2.b`的显式缺口。
- 数据权限：直接员工、精确查询/写权限和固定安装目标在任何读取或秘密处理前执行。客户认证所有权尚未实现，不存在可误用的客户入口。
- 数据库与软删除：两张表均为新项目迁移，使用完整插件前缀、幂等 DDL、内部外键、检查约束和恢复索引。配置与认证法律过程保留显式状态，不提供物理删除或通用软删除。
- `i18n`：新增 5 个稳定业务错误码及`en-US`、`zh-CN`资源。状态尚未进入 API/UI，用户可见状态字典明确留到`10.2.b/10.2.d`，没有提前维护不可消费字典。
- API、前端、缓存和`E2E`：本轮无公开 API、页面或缓存，因此不触发用户界面与 E2E 质量审查。任何真实认证前仍需完成后台配置入口、客户所有权、单独同意和回跳后权威重查。
- 开发工具：只更新插件自己的 DAO 配置，没有修改根构建脚本或默认开发路径。Windows 使用`make.cmd dao`；其他平台继续使用同一 GoFrame 生成入口，无跨平台脚本变化。

Lina 审查基于当前工作树、两份迁移、生成模型、适配器、服务、权限、双语资源及 OpenSpec 文档完成。审查中修复了错误链类型判断、授权前不必要的凭据复制和文档仍宣称未连接供应商适配器三项问题；未发现本切片内剩余阻断。任务`10.2`的运行时入口、个人状态机、用户同意和浏览器流程仍明确保持未完成。

**已读取规则：**`AGENTS.md`、`.agents/rules/openspec.md`、`.agents/rules/documentation.md`、`.agents/rules/architecture.md`、`.agents/rules/data-permission.md`、`.agents/rules/plugin.md`、`.agents/rules/backend-go.md`、`.agents/rules/database.md`、`.agents/rules/testing.md`、`.agents/rules/i18n.md`、`.agents/instructions/markdown-format.instructions.md`，以及合规插件目录无局部`AGENTS.md`的检查结果。
