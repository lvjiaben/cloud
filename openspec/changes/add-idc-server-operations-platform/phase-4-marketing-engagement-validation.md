# 第四阶段营销发送治理验证记录

## 范围

本记录覆盖任务`4.9.d.1`：`core`拥有的营销渠道同意/退订投影、免打扰设置和发送前批量门禁。实际通知任务、渠道绑定、投递、回执、费用频控及原始退订证据仍由`idc-notify`在`4.9.d.2`、`11.6`和`11.9`实现，因此本记录不把本地许可结论冒充已发送消息。

## 实现证据

- `021-marketing-send-governance.sql`新增按客户、联系人、渠道区分的追加式同意事件、失败关闭当前投影和客户免打扰投影。事件保存策略、语言、来源、证据摘要、幂等摘要和请求指纹；原始通知退订证据不复制到核心。
- `backend/internal/service/marketing/marketing_engagement.go`的`SetConsent`先锁联系人，显式同意必须拥有活跃联系人和可用目的地；同意或退订事件、当前投影及既有`marketing`主题联系偏好在同一事务提交。同一幂等请求并发复用，变化重放冲突，退订后重放旧同意返回当前已退订状态。
- `SetQuietHours`验证 IANA 时区、分钟范围和非空启用窗口，通过客户稳定锁和预期版本防止并发覆盖；跨午夜窗口由同一计算路径处理。
- `CheckSendBatch`每批接受 1 至 500 个互不重复目标，先读取一次 PostgreSQL 权威时间，再以一次集合查询装配活动、完整快照、成员、客户、联系人、已验证目的地、营销偏好、渠道同意和免打扰，总查询数固定为 2。每项结果按稳定优先级失败关闭，并生成绑定内容摘要、状态版本和评估时间的 SHA-256 决策摘要。
- 数据库索引覆盖同意幂等、历史、当前门禁和客户免打扰；DAO、DO 和 Entity 均由项目标准生成器维护。插件版本提升为`v0.1.27`。

## 已完成验证

| 门禁 | 结果 |
|---|---|
| `go test ./backend/internal/service/marketing -run 'PostgreSQL' -count=1 -v` | 通过；覆盖默认拒绝、显式邮件同意、相同重放、变化重放冲突、并发短信同意、允许决策摘要、跨午夜免打扰、禁用免打扰、过期版本冲突、非目标成员和退订立即拒绝 |
| `go test ./backend/internal/service/marketing -count=20 -shuffle=on`，显式设置隔离 PostgreSQL DSN | 通过；每轮使用随机独立 schema，结束后仅删除该轮 schema |
| `go test ./backend -run '^TestMarketingFoundationMigrationPostgreSQL$' -count=20 -shuffle=on` | 通过；21 张营销表、幂等迁移、免打扰窗口和归属约束在真实 PostgreSQL 生效 |
| `go test ./backend -run '^TestMarketingFoundationCodegenPostgreSQL$' -count=1`及`make.cmd dao dir=apps/lina-plugins/idc-core/backend` | 通过；测试先确认数据库名称精确为`linapro_idc_codegen`，仅重建该专用库营销表，再生成新增 3 组模型 |
| `go test ./backend/... -count=20 -shuffle=on` | 通过；插件后端全量随机顺序回归 20 轮 |
| `make.cmd lint.go plugins=1 dir=apps/lina-plugins/idc-core/backend` | 通过，`0 issues` |
| `openspec validate add-idc-server-operations-platform --strict` | 通过 |
| `make.cmd plugins.check` | 通过，扫描 813 个插件文件，`0 findings` |
| `make.cmd i18n.check plugins=1` | 插件运行时扫描和消息资源通过；仓库级命令仅报告宿主既有`notice-preview-modal.vue`缺少 6 个中英文键，本任务受宿主边界约束未修改宿主 |
| `make.cmd build plugins=1 platforms=windows/amd64,linux/amd64 verbose=1` | 通过；Windows、Linux、Portal 和 11 个插件完成正式构建，当前插件版本`v0.1.27` |

## 独立复核结论

- 已修复同一活动、快照、客户、联系人和渠道仅变更内容摘要时可在同一批次重复出现的问题；批量门禁现按投递目标去重，内容摘要只参与各自决策摘要，真实 PostgreSQL 20 轮回归包含该拒绝路径。
- 已确认旧同意事件重放只返回当前投影，不会在退订后恢复旧授权；批量结果还会核对数据库返回序号，投影数量或顺序异常均失败关闭。
- 已确认策略、版本、来源和幂等文本拒绝控制字符，防止不可见分隔符污染证据指纹。
- 当前切片没有发现未解决的阻断项。它只完成核心许可治理；真实通知 worker、渠道绑定、发送回执、费用频控和通知侧原始退订证据仍按`4.9.d.2`保持未完成。

## 规则域影响

- 架构：客户联系资料和营销许可由`core`拥有，通知插件只保存投递与原始退订证据；门禁返回决策而不越界发送。
- 数据权限：本地服务不发布匿名 HTTP 接口；调用方必须先证明精确客户、联系人和业务来源范围，门禁拒绝结果不用于对外探测目标存在性。
- 性能：批量门禁最多 500 项且固定两次查询，没有逐联系人、逐同意或逐快照成员查询。
- 数据库：迁移为新项目幂等 DDL，无 Mock、软删除或手写自增主键；同意历史追加保存，当前投影只指向同一客户、联系人和渠道的事件。
- API、前端和缓存：无影响；本切片没有新增 HTTP、页面或缓存。
- `i18n`：新增 3 个稳定错误码并提供中英文消息，没有新增用户页面文案。

## 剩余接线

- `idc-notify`尚未实现真实营销投递 worker，因此`4.9.d`顶级任务保持未完成。后续 worker 必须在每次实际发送前调用本门禁并保存决策摘要，不能复用排队时的旧许可结果。
