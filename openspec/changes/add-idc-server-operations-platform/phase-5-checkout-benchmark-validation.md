# 第五阶段结算性能基准验证记录

> **`FB-52`部分失效声明：**基准清单、分段计时和查询预算仍可作为迁移对账参考；覆盖逐台服务器履约单元和资源池库存的原基准不是当前通用结账性能证据。新基准必须覆盖商品版本简单库存、通用交付记录和冻结插件契约，并至少用服务器、`SSL`和手机三类商品证明查询次数不会随商品形态产生`N+1`。

## 实现范围

本轮完成任务`5.10`的版本化结算性能门禁。基准入口直接执行客户最终报价的原子结账，再执行同一订单的付款准备投影；测试结果只有在付款准备返回`ready`时才算成功。因此计时覆盖创建订单、订单项、原始待付账单、账单明细、库存预留、逐台履约单元、购物车关闭、事务`Outbox`和首次付款实名重检，不把单纯数据库探针或普通单元测试耗时冒充进入付款页性能。

固定清单、数据脚本、可执行基准和本次原始报告分别位于：

- `apps/lina-plugins/idc-core/hack/benchmarks/checkout/v1/manifest.json`；
- `apps/lina-plugins/idc-core/hack/benchmarks/checkout/v1/dataset.sql`；
- `apps/lina-plugins/idc-core/backend/internal/service/commerce/commerce_checkout_benchmark_postgres_test.go`；
- `apps/lina-plugins/idc-core/hack/benchmarks/checkout/v1/report-2026-08-11-windows-amd64.json`。

## 固定基准口径

| 维度 | 固定值 |
|---|---|
| 硬件下限 | 4 逻辑核、8 GiB、`local-ssd` |
| 数据库 | PostgreSQL 14 以上，回环连接 |
| 数据规模 | 76 个客户，其中 73 个基准客户；1 个正式商品；10,000 台可售库存 |
| 单次结算 | 8 个购物车项，每项 1 个配置选择、1 台服务器、`CNY` |
| 冷场景 | 第一次结账与付款准备，未强制清空 PostgreSQL 或操作系统页缓存 |
| 热场景 | 8 次完整预热后，64 个样本，并发度 8 |
| 查询门禁 | 单样本最多 50 条；1 项到 8 项最多增长 2 条 |
| 性能门禁 | 错误率 0，`p95≤2s`、`p99≤5s` |
| 外部网络 | 0 次 |

清单要求运行者显式填写实测内存和已核验存储类型；运行器同时校验逻辑核、回环数据库地址、PostgreSQL 主版本、数据行数、商品数和可售库存，避免在不满足清单的机器或漂移夹具上生成伪通过报告。数据脚本不修改不可变库存历史，而是追加一份本基准专用的最新权威库存快照。

## 同步路径与查询证明

基准构造图只注入本地 PostgreSQL、公开目录本地解析器、当前客户主体和事务`Outbox`，没有支付渠道、Provider、通知、合同、发票或`AI`客户端。计时期间把进程默认`HTTP Transport`替换为失败关闭的计数探针；发生任何默认 HTTP 请求都会立即使样本失败。本次 73 个性能样本及 1 个查询形状探针的外部网络调用为 0。

运行器为每个样本写入独立上下文编号，并通过 GoFrame 数据库日志在并发条件下分别统计 SQL。热路径 1 项购物车为 37 条，8 项购物车仍为 37 条，增长为 0；批量订单项、账单项、库存预留和履约写入没有形成按项目增长的`N+1`。冷路径首次使用数据库元数据缓存时为 47 条，仍低于固定 50 条上限。

## 正式报告

本次环境为 Windows `amd64`、8 逻辑核、30.9 GiB、KIOXIA EXCERIA PLUS G3 SSD、Go 1.26.5、PostgreSQL 18.4，数据库通过回环地址访问。夹具准备耗时 5,514 ms，预热阶段耗时 146 ms；这两段均不计入单请求延迟。

| 场景 | 样本 | 并发 | `p50` | `p95` | `p99` | 最大值 | 最大查询 | 错误率 |
|---|---:|---:|---:|---:|---:|---:|---:|---:|
| 应用冷路径 | 1 | 1 | 98.117 ms | 98.117 ms | 98.117 ms | 98.117 ms | 47 | 0 |
| 应用热路径 | 64 | 8 | 108.680 ms | 189.159 ms | 241.093 ms | 241.093 ms | 37 | 0 |

热路径`p95`只占 2 秒门禁约 9.46%，`p99`只占 5 秒门禁约 4.82%。报告保存清单`SHA-256`、环境、夹具、查询形状、冷热分段、错误率和外部网络计数，可与固定清单精确核对。

## 执行与验证证据

1. 设置真实隔离测试数据库、`LINAPRO_IDC_CORE_CHECKOUT_BENCHMARK=1`、实测内存和存储类型后，执行`GOWORK=off go test ./backend/internal/service/commerce -run '^TestCheckoutBenchmarkManifestPostgreSQL$' -count=1 -v`：通过，输出上述`CHECKOUT_BENCHMARK_REPORT`。
2. 第一次校准发现整段 8 项热链路为 47 条 SQL，未直接放宽门禁；补充 1 项/8 项查询形状探针后确认热缓存下两者均为 37 条、增长为 0，冷路径 47 条，最终把固定上限设为 50 条。
3. 第二次校准正确触发原夹具 100 台库存不足；基准改为追加不可变的 10,000 台专用快照，库存争抢继续由`5.9`独立回归负责，不污染纯结算延迟。
4. `make.cmd lint.go plugins=1 dir=apps/lina-plugins/idc-core/backend`：通过，`golangci-lint`和`staticcheck U1000`均为 0 问题。
5. 未设置性能开关时，基准测试明确跳过，不拖慢普通后端测试；固定清单畸形、硬件声明缺失、非回环数据库、数据漂移、SQL 超限、查询增长、外网调用、业务错误或延迟超限都会失败关闭。

## 复审结论与边界

本轮没有修改生产 API、数据库迁移、权限、缓存、前端或用户可见文案；新增内容只位于业务插件的版本化基准、真实 PostgreSQL 测试和 OpenSpec 证据。金额仍由原生产链路以整数最小货币单位处理，测试没有新增客户编号输入或绕过主体所有权。基准只证明本地服务段；公开目录到浏览器付款页的真实 Chromium 整链路仍由`5.11`和后续浏览器性能验收负责，不把服务基准冒充端到端页面性能。

任务`5.10`的实现和任务内验证已完成。仓库级`i18n`门禁仍仅被禁止修改的宿主通知预览页 6 个既有缺键阻断，因此任务继续保留未勾选；该阻断与本轮无用户文案变更无关。

## 规则影响记录

- 架构与插件：基准和夹具全部闭环在`idc-core`，不修改宿主；生产模块边界和跨插件契约无影响。
- 数据权限与 API：只调用既有客户主体服务，未新增路由、DTO、权限或数据暴露面。
- 数据库：没有生产迁移；测试数据只进入随机隔离 schema，脚本显式追加不可变库存快照并由测试自动删除 schema。
- 性能：固定查询上限、查询形状、冷热场景、并发和分段报告；没有用未计时夹具耗时替代请求延迟。
- 测试与工具：新增显式开关的 Go 测试基准和跨平台 SQL 数据脚本；Windows 已正式执行，其他平台使用相同 Go 入口和环境变量，不新增平台专用脚本。
- 前端与`i18n`：无用户可观察行为或文案变化，不新增 E2E；仓库既有 6 个宿主缺键不在插件授权范围内。

**已读取规则：**`AGENTS.md`、`.agents/rules/openspec.md`、`.agents/rules/documentation.md`、`.agents/rules/architecture.md`、`.agents/rules/data-permission.md`、`.agents/rules/plugin.md`、`.agents/rules/api-contract.md`、`.agents/rules/backend-go.md`、`.agents/rules/database.md`、`.agents/rules/frontend-ui.md`、`.agents/rules/testing.md`、`.agents/rules/dev-tooling.md`、`.agents/rules/i18n.md`。
