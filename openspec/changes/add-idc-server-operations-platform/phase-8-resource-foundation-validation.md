# 资源插件数据库基础验证记录

## 完成范围

本轮完成任务 8.3，并完成 2.7.b.2 中资源 owner 持久化与进程内构造部分：

- 新增渠道、渠道秘密绑定、资源池、权威资源映射、能力快照及明细、资源任务、任务秘密绑定、上游任务绑定、库存快照及明细、成本/余额/到期快照和健康快照共 13 张表。
- 渠道和资源池采用可恢复软删除；秘密绑定、映射、任务与观测历史不使用软删除，以显式状态或不可变行保留证据。
- 渠道秘密表只保存核心信封加密秘密的不透明引用。读取器只在 owner 内部恢复不可序列化 `secretcap.Handle`，不返回、不记录、不审计该引用。
- 渠道读取固定为一次渠道投影和一次活跃绑定集合查询，上限 64 个绑定；停用或软删除渠道在构造前拒绝。
- Provider runtime 只消费冻结后的声明副本，按精确 `pluginID + providerID + contractVersion`选择工厂，并为每次构造重新读取渠道、派生独立秘密 scope 和注入 accessor/sink，不提供全局运行时定位器。

## SQL 与性能设计

- 迁移文件为当前迭代唯一的 `001-resource-foundation.sql`，只包含 DDL，没有 Seed 或 Mock 数据；所有表、索引均使用 `IF NOT EXISTS`，可安全重复执行。
- 所有插件表、索引和约束均使用完整 `plugin_idc_resource_`前缀；DAO 的 `removePrefix` 与该前缀一致。
- 渠道路由使用`tenant_id + status/resource_kind + priority/weight`索引；任务领取使用`tenant_id + status + available_at + priority + id`索引；任务资源历史、租约恢复、上游任务与资源核对、各类最新快照均有对应复合索引。
- 快照明细按一个快照批量写入和集合读取，不要求应用层逐条关联；渠道秘密绑定一次集合读取，避免 N+1。
- 没有显式写入自增主键，没有`ON DUPLICATE KEY UPDATE`，也没有把 Mock 数据混入迁移。

## 验证结果

- 在随机命名的隔离 PostgreSQL schema 中连续执行迁移两遍，表数量保持 13，测试结束后只删除本次随机 schema。
- 真实 PostgreSQL 断言仅渠道和资源池具有`deleted_at`；重复活跃秘密绑定、重复活跃上游资源映射、数组型任务参数均被数据库拒绝。
- 真实 PostgreSQL 验证启用渠道可以恢复一个活跃 handle，停用后同一读取器立即拒绝构造。
- 使用独立本地`linapro_idc_codegen`数据库准备结构后执行项目标准 DAO 生成入口，生成 13 组 DAO/DO/Entity；没有连接或复制任何生产业务数据。
- 资源插件全量 Go 测试和声明顺序打乱测试通过。

## 未完成门禁

- 任务 2.7.b.2 仍缺后台秘密管理 API 的最近重新认证、权限、数据范围、审计、失败补偿与恢复。
- 任务 8.4 已完成受 DNS 重绑定保护的安全出站 HTTP 环境，仍缺生产 HTTP/Jobs 启动回调、owner-aware bridge 与编排服务注入，因此没有把当前 runtime 注册为可执行生产路径。
- 本轮没有新增 HTTP API、前端页面、用户可见运行时文案、缓存或字典，API、前端、缓存和 i18n 无影响。
