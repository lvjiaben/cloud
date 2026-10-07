# 服务器与资源状态所有权验证记录

## 所有权结论

- `idc-core`独占客户服务实例、不可变规格与价格快照、权益状态、运行状态、续费策略版本和统一`business_operation`。服务实例只保存 resource 任务的不透明编号、渠道编号和脱敏状态投影，不保存上游任务或原始 Provider 证据。
- `idc-resource`独占渠道、资源池、上游资源映射、资源任务、上游任务绑定、任务秘密绑定、库存及成本/余额/到期/健康证据。资源表只引用 core 提供的不透明本地资源编号，不建立跨插件数据库外键。
- 静态复核未发现 resource 创建第二套客户服务实例、权益或业务操作表，也未发现 core 创建上游资源映射、上游任务绑定或 Provider 原始证据表。

## 数据结构

core 的`030-service-renewal-foundation.sql`建立服务实例、规格快照、生命周期策略与提醒、续费策略、价格快照、业务操作和不可变操作事件，并用数据库检查约束封闭三维状态、金额、版本和关联关系。后续结清与开通迁移只扩展同一个 owner 的聚合，没有复制 resource 真相。

resource 的`001-resource-foundation.sql`建立 13 张 owner 表。资源映射以“本地资源编号 + 渠道 + 资源池 + 上游资源引用”形成唯一关系；任务、秘密、上游任务与观察证据采用显式生命周期或不可变历史，不通过软删除抹除证据。

## 验证结果

- `GOWORK=off LINAPRO_IDC_CORE_TEST_DSN=... go test ./backend -run '^TestServiceRenewalFoundationMigrationPostgreSQL$' -count=1`：通过。测试在随机隔离 schema 连续执行关键迁移两遍，验证表、字典、外键、不可变触发器、三维状态和业务操作约束，结束只删除自己的 schema。
- `GOWORK=off LINAPRO_IDC_RESOURCE_TEST_DSN=... go test ./backend -run '^TestResourceFoundationMigrationPostgreSQL$' -count=1`：通过。测试在随机隔离 schema 重复迁移并验证 13 张 owner 表、唯一映射、任务参数、软删除边界和秘密句柄读取门禁。
- 两次测试都连接本地 PostgreSQL，只创建和删除随机测试 schema，没有修改正式业务数据。

## 规则影响

本项是既有数据所有权与迁移门禁的收口，没有新增 HTTP 接口、页面、用户文案、缓存或业务流程，因此 API、数据权限、前端、E2E、缓存和 i18n 均无新增影响。后续状态转换、非财务操作和 Provider 执行仍按 8.2 及以后任务推进，不能因本项完成而视为后续能力已完成。
