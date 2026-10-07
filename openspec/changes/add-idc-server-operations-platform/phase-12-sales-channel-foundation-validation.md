# 第12阶段销售渠道基础验证记录

## 本次完成内容

`apps/lina-plugins/idc-core/manifest/sql/072-sales-channel-foundation.sql`已经补齐第12.1项要求的销售渠道基础数据模型：

- 代理等级、代理申请、资格事件、信用限制、商品范围和价格规则；
- `AFF`推荐链接、访问触点和归因记录；
- 销售组、组成员、客户归属、协作人、批量转移和转移明细；
- 佣金规则、阶梯、独立逐币种佣金账户、账户流水、成熟记录；
- 收款资料、结算申请、结算审核、打款批次、打款明细、打款凭证和失败恢复记录；
- 与原有佣金快照表的来源字段、唯一约束和关联约束。

所有插件表都使用完整的`plugin_idc_core_`前缀，配置文件已同步纳入代码生成表清单。追加写表使用数据库触发器拒绝修改和删除，账户流水通过行锁维护顺序和余额守恒；金额按币种和受益人独立核算，不与客户消费余额混用。

## 验证结果

在本机隔离`PostgreSQL`连接上执行：

```text
$env:LINAPRO_IDC_CORE_TEST_DSN='pgsql:postgres:123456@tcp(127.0.0.1:5432)/cloud_idc_phase10_e2e_v2?sslmode=disable'
GOWORK=off go test ./backend/internal/service/commission -run '^TestCommissionFoundationMigrationPostgreSQL$' -count=1 -v
```

测试通过，覆盖随机隔离`schema`中的：

1. 迁移文件首次执行和重复执行；
2. 29张销售渠道表、外键、唯一约束、索引和字典数据；
3. 追加写事件、归因、佣金流水、成熟、结算明细、审核、凭证和恢复记录的更新/删除拒绝；
4. 代理申请状态约束、代理资格唯一生效记录、`AFF`触点归因关联和佣金来源互斥；
5. 独立佣金账户的逐币种隔离、流水顺序和金额守恒；
6. 测试结束只删除本次随机`schema`，默认业务库没有残留临时表。

同时通过：

```text
openspec validate add-idc-server-operations-platform --strict
.\make.cmd plugins.check
git diff --check
```

## 影响说明

本次只完成第12.1项数据库基础，不改变客户页面、后台页面、HTTP接口或权限交互，因此本项没有新增`i18n`和`E2E`影响；12.2至12.9仍需在后续任务中实现并验证。
