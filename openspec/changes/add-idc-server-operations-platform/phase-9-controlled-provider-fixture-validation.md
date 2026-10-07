# 第九阶段可控 Provider 假服务验证记录

## 验证结论

任务`9.6`要求的成功、失败、乱序、重复、超时、未知状态、令牌过期和部分分页失败已经全部由本地可控`HTTPS`假服务、资源拥有方状态机测试及隔离`PostgreSQL`测试覆盖。测试不访问生产魔方云，不创建或修改任何线上用户、实例、磁盘、`IP`或任务。

本任务只补充验证资产和证据，没有修改生产实现、公开`HTTP API`、数据库迁移、运行期依赖、菜单、页面或语言资源。

## 故障矩阵

| 场景 | 可控输入 | 安全结果 | 主要证据 |
|---|---|---|---|
| 成功 | 写接口返回白名单任务号和资源号 | 只发布规范化`accepted`结果，丢弃密码类字段 | `TestCoreLifecycleMappingsUseExactVerifiedRoutes`、各附属能力映射测试 |
| 失败 | 写接口返回已冻结的明确拒绝状态 | 形成无原始响应文本的确定失败，不进入未知重放 | `TestDocumentedRejectionIsFailedWithoutRawErrorBody` |
| 乱序 | 任务依次返回`running → accepted → succeeded` | 适配器保留原始规范化事实；资源拥有方保持`running`，不回退到`accepted` | `TestTaskObservationSequenceKeepsOutOfOrderAndDuplicateFactsExplicit`、`TestAcceptedObservationDoesNotRegressRunningTask`、真实数据库观察测试 |
| 重复 | 重复返回同一终态并重复提交已消费观察租约 | 查询保持幂等；第二次终态回报被租约栅栏拒绝，证据只追加一次 | 同上序列测试、`assertResourceTaskObservationSafety` |
| 超时 | 登录前`408/503`或写入连接中断 | 登录失败不制造未知写；已发送写只形成不可重放的结果未知 | `TestInitialAuthenticationFailureDoesNotProduceAnUnknownWrite`、`TestAmbiguousWriteTransportFailureIsUnknownAndNeverRetried` |
| 未知状态 | 任务返回冻结矩阵外的状态 | 不猜测成功或失败，返回带稳定失败码的`unknown` | `TestTaskStatusMatrixAndResourceProjection` |
| 令牌过期 | 本地令牌到期、只读请求`401`、写请求`401` | 本地到期和只读`401`只刷新一次；已发送写不在同一次调用重放 | `TestExpiredLocalTokenTriggersNewLogin`、`TestUnauthorizedResponseRefreshesTokenExactlyOnce`、`TestUnauthorizedWriteIsUnknownAndNeverRetried` |
| 部分分页失败 | 第一页成功，后续页返回临时错误或双源指纹变化 | 不伪造终页，不替换最后完整快照 | `TestPartialInventoryPaginationFailureDoesNotInventCompletion`、双源快照变化测试 |

## 首版能力映射补强

新增表驱动假服务测试逐项核对以下首版核心生命周期映射，并对每个动作断言只发送一次写请求、方法与路径精确、表单字段无扩张、响应任务号规范化：

- 启动：`POST /clouds/:id/on`。
- 优雅关机与硬关机：`POST /clouds/:id/off`、`POST /clouds/:id/hardoff`。
- 优雅重启与硬重启：`POST /clouds/:id/reboot`、`POST /clouds/:id/hard_reboot`。
- 暂停与恢复：`POST /clouds/:id/suspend`、`POST /clouds/:id/unsuspend`。
- 变配：`PUT /clouds/:id`，只传冻结的正整数资源字段。
- 终止：`DELETE /clouds/:id`。

换`IP`继续遵循冻结契约：`PUT /clouds/:id/ip`只提交规范化`num`和可选`ip_group`，无论同步响应是否夹带任务号，都必须由资源拥有方使用后续实例详情中的`ip_count`精确收敛，不能仅凭写响应结案。

开通、重置密码、重装、控制台、救援、快照、备份、安全组、`NAT`、虚拟介质和临时流量包已有各自的正向映射、输入失败关闭、任务证明、归属核对或后置状态测试，本任务没有复制这些测试。

## 乱序与重复收敛

可控上游序列明确返回`running → accepted → succeeded → succeeded`，证明适配器不会把未知顺序改写成伪造终态。资源拥有方的纯状态机与真实数据库测试进一步证明：

1. 本地任务进入`running`后收到旧的`accepted`事实时仍保持`running`。
2. 乱序事实仍以新的只读观察计划继续查询，不触发第二次写请求。
3. 终态提交后观察租约立即失效，同一租约的重复提交被拒绝。
4. 重复终态不会再次追加同一批 Provider 任务证据。

## 规则域影响

- 架构与性能：仅增加本地测试，没有新增抽象或运行期调用。假服务请求次数有精确断言；库存分页继续受页数、响应体和全局候选上限约束，不引入`N+1`。
- 数据权限：没有新增接口、对象编号入口或数据查询路径；测试只使用固定虚构编号和随机隔离数据库 schema。
- 缓存一致性：没有修改令牌缓存实现；测试只验证现有到期和`401`失效语义。
- `i18n`：没有新增或修改用户可见文案、接口文档、菜单或语言资源，无运行时影响。
- 数据库：没有新增或修改`SQL`；只扩展现有真实数据库测试对观察租约和不可变证据的断言。
- `DI`：没有新增运行期依赖或构造函数；假服务通过现有`ProviderEnv`显式注入。
- 前端与`E2E`：没有用户可观察页面或浏览器工作流变化，不触发页面`E2E`与截图审查。
- 开发工具：没有修改脚本、构建入口或跨平台命令。

## 验证证据

| 命令或检查 | 结果 |
|---|---|
| 定向生命周期、换`IP`、乱序与重复假服务测试，`-count=10 -shuffle=on` | 通过 |
| `go test ./... -count=20 -shuffle=on`（`idc-zjmfcloud/backend`，`GOWORK=off`） | 通过 |
| `go test ./... -count=5 -shuffle=on`（`idc-resource/backend`，`GOWORK=off`） | 通过 |
| `TestResourceFoundationMigrationPostgreSQL -count=3 -shuffle=on` | 通过；使用本机数据库中的随机隔离 schema，乱序不回退且重复终态只落一次证据 |
| `make.cmd lint.go dir=apps/lina-plugins/idc-zjmfcloud plugins=1` | 通过，零问题 |
| `make.cmd lint.go dir=apps/lina-plugins/idc-resource plugins=1` | 通过，零问题 |

## Lina 审查结论

任务级审查从根仓库与插件目录的`git status --short`、`git ls-files --others --exclude-standard`展开，实际范围为两处测试和本记录关联的两份验证文档。两个目标插件根目录均不存在局部`AGENTS.md`。审查重新读取`AGENTS.md`、`openspec.md`、`documentation.md`、`architecture.md`、`data-permission.md`、`plugin.md`、`backend-go.md`、`database.md`、`testing.md`与`i18n.md`。

- 后端测试：自包含、可并行、使用`httptest`本地`TLS`服务和随机隔离数据库 schema；没有依赖执行顺序、生产对象或生产凭据。
- 生命周期契约：断言精确方法、路径、参数和单次写入；没有为了测试扩张适配器能力。
- 状态安全：乱序事实不回退本地状态，终态重复回报受租约和版本栅栏保护，重复失败不会追加第二份证据。
- 架构、数据权限、缓存、`i18n`、开发工具和`DI`：均无运行时变化，原因已在本记录的规则域影响中明确。
- `E2E`质量审查：本任务没有页面、路由、公开接口交互或用户可观察行为变化，不触发浏览器截图审查；后端故障注入由可控假服务和真实数据库测试直接证明。

审查结果为严重问题 0 个、警告 0 个，未发现阻断任务`9.6`完成的问题。

生产隔离写测试属于任务`9.7`，真实后台浏览器流程属于任务`9.8`，均不因本任务完成而自动放行。父任务`9.2`、`9.4`和`9.5`仍按各自剩余边界与既有外部门禁保持未完成。
