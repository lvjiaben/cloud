# 第十阶段商家安装实例受保护注册验证记录

> 状态：本文件只保留历史技术验证证据。平台销售令牌和远程中央注册前提已被[完整独立交付边界纠偏记录](phase-10-independent-delivery-correction.md)废止；后续改为安装环境本地生成的进程间身份。

## 1. 任务结论

任务`10.6.b.2`已完成商家安装实例的受保护本机密钥、一次性注册令牌、耐久恢复状态和中央响应丢失精确恢复。该切片没有开放 HTTP API、后台页面或供应商业务调用；注册传输和中央运行时装配仍由`10.6.b.3`承接，因此父任务`10.6.b`保持未完成。

## 2. 实现边界

### 2.1 秘密隔离

- 本机实例主密钥使用`compliance.gateway-enrollment-key`作用域，由`secretcap.GeneratedManager`按`url_safe_256`策略在秘密边界内部生成。
- 一次性注册令牌使用独立的`compliance.gateway-registration-token`作用域，通过同步单次生产者写入；两个作用域不能互换访问。
- 主密钥的`256`位随机值只在回调中解码为`Ed25519`种子。私钥、注册令牌、临时解码值、签名消息和签名缓冲均在同步回调返回前清零。
- 注册协议新增字节切片入口，商家端无需把回调中的令牌强制转换为不可清除字符串；内部规范化和错误路径缓冲同样清零。

### 2.2 耐久恢复状态机

本地 PostgreSQL 状态固定为：

```text
preparing -> ready -> registered
```

- `preparing`只保存销售记录、中央注册编号、到期时间、随机数以及两组服务端生成且不对外暴露的`secretcap`幂等操作键。
- 密钥与令牌成功进入各自秘密作用域后，单事务写入两个受保护句柄、公开密钥、公开指纹和`enrollment_ready`事件，再推进`ready`。
- 中央注册成功或精确恢复后，单事务写入中央实例编号、中央密钥编号和`enrollment_registered`事件，再推进`registered`。
- 中央结果不明确时保持`ready`。下一次调用复用同一令牌、密钥、随机数和证明；改变令牌会在本地`secretcap`幂等围栏处失败，不会发起第二次中央注册。
- 每套单商家安装实例最多有一个当前注册聚合；注册和事件禁止物理删除，事件禁止更新。

### 2.3 数据最小化与 ORM 边界

- 普通数据库没有注册令牌、令牌摘要、私钥、私钥密文或供应商凭据列，只保存`secretcap`路由句柄、公开密码材料、中央公开编号和证据摘要。
- 随机数、公开密钥和公开指纹固定为`32`字节，并以数据库正则约束的`64`位小写十六进制保存。
- 初始真实库随机测试发现 GoFrame 扫描同一行多个 PostgreSQL `BYTEA`时可能复用底层切片，造成公开密钥与指纹别名污染；改为规范十六进制文本后消除该不稳定映射，并连续通过并发测试。
- 所有迁移对象名不超过 PostgreSQL 的`63`字节限制；两张新表已通过仓库标准`make.cmd dao`入口生成 DAO、DO 和 Entity。

## 3. 验证结果

以下门禁通过：

```text
GOWORK=off go test ./backend/... -count=20 -shuffle=on
GOWORK=off go vet ./backend/...
make.cmd lint.go plugins=1 dir=apps/lina-plugins/idc-compliance
make.cmd plugins.check
```

真实 PostgreSQL 只使用显式可丢弃的`linapro_idc_codegen`数据库，并为每个用例创建随机`linapro_gateway_enrollment_*`或`linapro_compliance_test_*`隔离 schema。验证结果：

```text
go test ./backend/internal/service/gatewayenrollment \
  -run 'TestGatewayEnrollmentPostgreSQLLifecycleAndSecretExclusion|TestGatewayEnrollmentPostgreSQLConcurrentExactRequest' \
  -count=20 -shuffle=on

go test ./backend \
  -run 'TestComplianceMigrationIdentifiersFitPostgreSQL|TestComplianceFoundationMigrationPostgreSQL' \
  -count=10 -shuffle=on

go test ./backend -run TestComplianceFoundationCodegenPostgreSQL -count=1

PASS
```

测试覆盖输入令牌清零、两个秘密作用域、秘密创建精确重放、中央响应丢失恢复、变更令牌零中央调用、普通行令牌排除、三条事件、第二个当前注册拒绝、非法状态跳转拒绝、事件不可变、注册不可删除和并发精确请求收敛。

## 4. 影响评估

- 架构与插件：商家端服务、SQL、模型、组合入口和测试全部闭环在`idc-compliance`；只消费`idc-core`公开`secretcap`契约，没有修改宿主源码或页面。
- API 与数据权限：没有新增 HTTP 路由、DTO、权限标签、列表、详情或导出；受限注册入口的传输认证和数据存在性保护留在`10.6.b.3`。
- 数据库：新增两张安装实例本地表、三个有界索引、删除保护和只追加事件触发器；无软删除、无 Seed/Mock、无跨插件外键。
- 性能：单实例固定至多一行注册聚合和三条事件；每次恢复使用固定次数单行事务、两个秘密访问和一次中央调用，没有列表或`N+1`路径。
- 缓存：无影响，PostgreSQL 和`secretcap`为唯一权威状态。
- `i18n`与前端：无运行时用户文案、菜单、页面或用户可观察工作流变化。
- 开发工具：只更新插件自己的 DAO 表白名单，未修改根代码生成器、脚本或 CI；Windows 已使用标准`make.cmd dao`入口生成。

## 5. LinaReview 结论

审查覆盖插件边界、秘密驻留、两阶段外部调用恢复、事务状态机、数据库约束、ORM 映射、并发、错误折叠、数据权限、性能、`i18n`和测试隔离。审查期间修复了公开二进制字段的 ORM 切片别名以及协议临时缓冲未主动清零两项问题。最终未发现阻断`10.6.b.2`的`P0`、`P1`或`P2`问题。
