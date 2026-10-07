# 第四阶段赠送与返现权益验证记录

## 范围

本记录覆盖任务`4.9.c`：充值赠送和订单返现的规则绑定、活动名额、待入账、财务引用、冻结、释放、消费、过期、冲回与不可提现分层。真实客户余额账本仍属于后续交易与财务域，本切片没有用营销权益表冒充资金分录。

## 实现证据

- `backend/internal/service/marketing/marketing_value_grant.go`提供插件内部`ValueGrantService`。创建请求必须精确绑定活动、权益规则、客户、大写三字母币种和业务来源；服务锁定当前有效活动及规则，要求固定金额与规则完全一致，并保存复用优惠计算规范生成的规则版本摘要。
- 待入账权益创建与活动总名额、逐客户名额预占处于同一 PostgreSQL 事务。财务域创建分层账本记录后携带不可变`financial_posting_reference`入账，名额由预占转为核销；入账前取消把完整待入账金额转为已冲回并释放名额，不产生伪造财务引用。
- 已入账权益以单权益行锁串行执行冻结、释放、消费、过期和冲回。过期后释放的冻结额度直接进入过期桶；冲回按可用、冻结、已消费顺序处理，多个部分退款追加各自幂等流水并共同引用原`post`流水。
- `020-marketing-foundation.sql`对待入账、可用、冻结、已消费、已过期、已冲回六个桶建立非负守恒约束；追加流水显式保存六桶变化且每笔和为零。充值赠送在服务和数据库层均强制`withdrawable = false`，返现可按明确规则保存该属性。
- 生成的 DAO、DO 和 Entity 已包含规则绑定、名额预占证据及完整流水字段；插件版本提升为`v0.1.26`。

## 验证结果

| 门禁 | 结果 |
|---|---|
| `go test ./backend/internal/service/marketing -run 'PostgreSQL' -count=1 -v` | 通过；覆盖固定规则金额、创建/入账重放、变化重放冲突、冻结、释放、消费、多个部分冲回、入账前取消、到期释放、并发冻结、并发名额争抢和同来源并发重放 |
| `go test ./backend/internal/service/marketing -count=20 -shuffle=on`，显式设置隔离 PostgreSQL DSN | 通过；每轮创建随机独立 schema，活动名额为 1 时两个并发权益仅一个成功，取消后名额可复用并在入账后精确转为已核销；同客户同来源并发创建通过客户稳定锁返回同一权益编号 |
| `go test ./backend -run '^TestMarketingFoundationMigrationPostgreSQL$' -count=20 -shuffle=on` | 通过；权益守恒、充值赠送不可提现、财务入账状态和复合归属外键在真实 PostgreSQL 生效 |
| `make.cmd dao dir=apps/lina-plugins/idc-core/backend` | 通过；只刷新名称精确为`linapro_idc_codegen`的本地专用数据库中的两张权益表后执行标准生成入口 |
| `go test ./backend/... -count=20 -shuffle=on` | 通过 |
| `make.cmd lint.go plugins=1 dir=apps/lina-plugins/idc-core/backend` | 通过，`0 issues` |
| `make.cmd plugins.check` | 通过，扫描`800`个插件文件且`0 findings` |
| `openspec validate add-idc-server-operations-platform --strict` | 通过 |
| `make.cmd i18n.check plugins=1` | 插件运行时扫描和消息覆盖通过；仓库级结果仍只有宿主通知预览既有的 6 个前端缺失键，本切片未修改宿主且未新增缺失键 |
| `make.cmd build plugins=1 platforms=windows/amd64,linux/amd64 verbose=1` | 通过；Windows、Linux、Portal 和 11 个插件完整构建，核心插件版本为`v0.1.26` |

## 规则域影响

- 架构与后端：权益仍位于`core`营销子域，通过财务入账引用与未来账本协作，不调用外部渠道、不写真实余额；每个状态变更使用固定查询数，没有按返回行数查询。
- 数据权限：服务不发布匿名 HTTP 接口，也不推断调用身份；未来财务编排调用前必须证明精确客户和业务来源权限。
- 数据库：迁移是新项目幂等 DDL，不包含 Mock、浮点金额或手写自增主键；金额统一使用整数最小货币单位。
- API、前端与缓存：无影响；本切片没有新增 HTTP、页面或缓存。
- `i18n`：新增 3 个稳定错误码并提供中英文消息；复用已有活动名额错误码，没有新增用户页面文案。

## Lina 审查结论

- 已按本轮实际影响重新读取开发流程、文档、架构、数据权限、插件、后端、数据库、测试和`i18n`规则；API、前端、缓存没有实现影响，开发工具只复用标准 DAO、检查和构建入口。
- 首轮审查识别并修复了规则金额可漂移、活动名额未占用和同来源并发创建不能稳定复用三个问题；修复后规则行与活动行锁、客户稳定锁、权益行锁形成固定有界路径，真实 PostgreSQL 20 轮均通过。
- 未发现本切片剩余阻断项或警告项。财务账本创建、消费顺序编排和 HTTP/页面入口明确留给后续交易与管理切片，没有用营销状态或测试夹具冒充这些能力。
