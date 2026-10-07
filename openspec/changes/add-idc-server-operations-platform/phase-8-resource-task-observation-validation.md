# 资源任务观察收敛验证记录

## 完成范围

本轮完成任务 8.5.b，只实现已经具备上游任务绑定的只读查询收敛持久层；任务 8.5 顶级项继续保持未完成，不把 Provider 写执行器、无任务号资源查询、资源映射、秘密输出、证据回报、具体能力参数编排或补偿伪装为完成。

- `accepted`、`running`和具有上游任务绑定的`result_unknown`按`next_observe_at`进入观察队列。
- 每批最多领取 32 条，使用 PostgreSQL`CURRENT_TIMESTAMP`与`FOR UPDATE SKIP LOCKED`；任务号、当前状态、随机租约令牌、版本和数据库时间下有效的租约共同形成回报栅栏。
- 观察领取不增加 Provider 写入尝试次数。只读观察租约过期后允许重新领取，不会造成外部写入重放。
- 每批固定执行一次任务集合查询和一次最新上游绑定集合查询；最新步骤使用 PostgreSQL`DISTINCT ON`投影，每个本地任务最多返回一条绑定，不随任务数产生`N+1`查询。
- 观察候选必须已经具有合法上游绑定；任务查询返回结果必须通过完整`OperationResult`校验，并且外部任务号与绑定号精确一致。已知外部资源号不一致时同样拒绝。
- 普通状态矩阵仍拒绝同状态覆盖；只有持有精确观察租约且通过上游编号核对的`ObserveTask`允许刷新`accepted/accepted`或`running/running`证据。
- 上游返回`accepted/running/succeeded/failed/terminated/canceled`时映射为本地确定状态；已进入`running`的任务再次观察到`accepted`时保持`running`，避免上游最终一致性读造成进度倒退；返回`unknown`时同样保持最后已知本地状态并继续调度，不猜测成新的结果。
- 查询调用暂时失败时只保存固定、脱敏的 owner 摘要，释放观察租约并延后下一次观察；不得把`accepted/running`退回可提交状态，也不得重复 Provider 写操作。
- 上游绑定状态、进度和最后观察时间与本地任务状态在同一事务更新。终态同时写入完成时间并清除观察计划与租约；迟到、重复或版本不匹配的 worker 全部拒绝。
- 为观察领取新增部分组合索引，固定覆盖单租户、观察状态、到期时间、优先级与主键排序；迁移继续使用`CREATE INDEX IF NOT EXISTS`保持幂等。
- 本轮顺带把资源任务组件已有数据库写入改为 GoFrame 生成的`DO`对象；只有清除 nullable 时间列时使用项目既有的固定列`Data(column, nil)`窄例外，不手工维护自动时间字段。

## 验证结果

- 单元测试覆盖观察领取参数边界、外部任务/资源编号不匹配、非终态调度、终态禁止调度、带证据同状态刷新、普通同状态转换拒绝，以及`unknown`保持最后已知状态。
- 真实 PostgreSQL 隔离 schema 测试验证迁移可重复执行、观察索引存在、查询失败重排不改变`accepted`状态、旧租约无法重复回报、编号不匹配不落库、`accepted → running → succeeded`与上游绑定原子收敛。
- 四个并发观察 worker 各领取四条任务，16 条到期观察得到 16 个唯一任务，没有重复领取。
- 资源插件后端测试和真实 PostgreSQL 隔离 schema 测试均连续运行 20 轮通过；定向 Go 静态检查、插件治理检查、`git diff --check`和 OpenSpec 严格校验通过。

## 影响判断

- `API`：没有新增或修改 HTTP API、DTO、路由或公开响应。
- 数据权限：观察 worker 是`resource` owner 的平台内部任务，固定`tenant_id = 0`并只处理本插件任务与绑定，不新增用户查询或绕过对象可见性的入口。
- SQL：只在当前资源基础迁移中新增一条幂等部分组合索引，没有新增表、字段、Seed 或 Mock 数据；13 组 DAO/DO/Entity 无需重新生成。
- 缓存：没有新增缓存、快照失效或跨节点内存状态；并发权威状态全部在 PostgreSQL。
- `i18n`：没有新增用户可见文案、API 文档源文本、菜单、字典或语言资源。
- 开发工具与跨平台：没有修改构建、生成、CI 或脚本；验证继续使用仓库现有 PowerShell/Go/OpenSpec 入口。
- 测试与 E2E：这是插件内部后端状态行为，没有页面或用户可观察前端变化，因此未触发 E2E 与截图审查；使用单元测试和真实 PostgreSQL 集成测试覆盖。
- `DI`：本轮没有新增运行期接口依赖、构造函数或启动装配；`Store`继续只复用启动期显式注入的同一`gdb.DB`。Provider runtime 和 worker 装配仍在 8.5.c 完成。

## 剩余门禁

- 实现写任务 worker，把`Claim`投影通过启动期共享`ProviderRuntime`调用`Execute`，并把调用错误先规范化再进入提交结果转换。
- 实现观察 worker，通过同一共享 runtime 调用`QueryTask`；本轮只完成该 worker 所依赖的可靠持久层契约。
- 对没有上游任务号的`result_unknown`使用权威资源映射与`QueryResource`查询，无法核对时进入人工接管，不得伪造绑定或重复写入。
- 原子维护资源映射、秘密输出绑定、脱敏证据、跨 owner 回报和补偿 Saga，并完成具体服务器能力参数编排。
