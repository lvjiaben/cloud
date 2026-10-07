# Provider 健康管理验证记录

## 交付结果

资源插件版本提升到`v0.1.7`，核心插件最低依赖提升到`v0.1.53`。后台新增 Provider 健康页和两项独立权限：健康列表仅返回本地有界当前投影；策略写入使用精确渠道、乐观版本和同事务业务审计。页面没有手工解除保护、强制成功或直接执行上游动作的入口。

## 安全与性能边界

- 列表按渠道编号使用键集分页，每页默认 25、最大 100，仅执行一次渠道与健康状态联表查询，不调用 Provider，不产生按渠道或健康维度增长的数据库查询。
- 当前状态行保存认证、查询、写入和库存四类最新稳定健康码、摘要码、耗时与数据库观测时间；原始上游错误、响应、端点、凭据和秘密引用不进入投影。
- 策略安全范围由 API、服务和 PostgreSQL 约束共同校验。策略版本独立于运行状态版本，避免健康采集与后台编辑互相制造虚假冲突。
- 策略写先授权再校验参数，随后锁定精确渠道和状态行；审计失败使策略更新回滚。收紧阈值命中现有窗口会立即保护，已开启保护不能被宽松策略缩短，缩小窗口不能小于当前样本数。
- 保护只阻止新的自动 Provider 写入，精确只读查询和对账继续可用。人工接管与通知仍留在父任务，未被本切片伪装完成。

## 验证结果

| 门禁 | 结果 |
| --- | --- |
| `make.cmd dao dir=apps/lina-plugins/idc-resource/backend` | 通过；只对明确命名的本地`linapro_idc_codegen`专用库重放迁移并生成模型 |
| `make.cmd ctrl dir=apps/lina-plugins/idc-resource/backend` | 通过；生成后补回显式依赖注入和 Controller 实现 |
| `GOWORK=off go test ./backend/... -count=5 -shuffle=on` | 通过 |
| `TestResourceFoundationMigrationPostgreSQL` | 通过；随机隔离 schema 验证策略阈值、保护延续、版本冲突、权限顺序、四维投影和审计失败回滚 |
| `pnpm -F @lina/web-antd typecheck` | 通过 |
| `pnpm -F @lina/web-antd i18n:check` | 通过，四个宿主前端语言文件双语键一致 |
| `pnpm -F @lina/web-antd build` | 通过；仅有既有依赖动态/静态导入和 Browserslist 数据提示 |
| `make.cmd plugins.check` | 通过；11 个插件清单、配置和生产 Go 文件无治理问题 |
| `playwright test .../TC002-provider-health.ts --list` | 通过；发现 1 个文件中的 5 个 Chromium 场景，覆盖中英文、最小策略请求、无手工解锁、只读权限和非法阈值零写入 |
| `make.cmd i18n.check` | 未通过；运行时扫描和消息覆盖通过，但被宿主通知预览页 6 个既有前端缺键阻断，见下节 |

## 当前外部阻断

本地真实浏览器执行依赖宿主开发服务。`make.cmd dev`当前在未归属本切片的`apps/lina-core/internal/controller/{role,plugin,joblog}/*_new.go`因重复`ControllerV1/NewV1`声明而构建失败；根目录没有`.contributing`，本切片不得修改宿主文件。因此未把 E2E 定义冒充为已实际运行，待宿主基线恢复后执行`pnpm test:module -- plugin:idc-resource`并审查中英文截图。

仓库级`make.cmd i18n.check`还被`apps/lina-vben/apps/web-antd/src/views/system/message/notice-preview-modal.vue`引用、但宿主中英文资源均缺失的`plugin.content-notice.preview.title`、`fields.type`和`fields.createdBy`阻断，共 6 项。插件自己的四份前端语言文件键集合、运行时扫描和消息覆盖均已通过；该宿主文件同样位于当前业务插件禁止修改的宿主范围。根据`lina-review`门禁，完整仓库`i18n`和真实浏览器证据缺失均按严重外部阻断记录，9.5.d 暂不作为已完成任务验收，也不进入归档。

## 影响判断

- 数据权限：列表使用健康列表精确权限与平台单商家数据范围目标；策略写使用独立权限和精确渠道目标，授权先于参数与存在性检查。
- `i18n`：新增菜单、页面、错误和中文 API 文档；英文 API 文档沿用英文源码。
- 缓存：无影响。健康与策略都以 PostgreSQL 为权威，不新增缓存。
- 依赖注入：`managementcap.Runtime`由`core`拥有并在`resource`的`pluginhost.HTTP`路由装配时构造；健康服务只接收显式运行时依赖，DAO 与业务审计共享请求事务上下文，不增加全局运行时定位器。
- 开发工具：无影响。本切片没有修改`Makefile`、`make.cmd`、`hack/`工具或 CI；验证命令覆盖 Windows，Linux 影响仅为相同 Go/前端源码和 SQL，未新增平台脚本。
- 宿主：未修改`apps/lina-core`、`apps/lina-vben`或根`hack`；生产构建产物变化只来自验证命令，不属于源码实现。

## Lina-review 结论

本轮实际读取`AGENTS.md`、`.agents/rules/{openspec,documentation,architecture,data-permission,plugin,api-contract,backend-go,database,frontend-ui,testing,i18n,dev-tooling}.md`和`.agents/instructions/markdown-format.instructions.md`；两个命中的业务插件根目录均不存在本地`AGENTS.md`。缓存规则未命中，因为没有新增或修改缓存；开发工具规则无源码影响，因为没有修改工具入口。

- 严重：仓库级`i18n`门禁被上述禁止修改的宿主 6 个既有缺键阻断。依据：`.agents/rules/i18n.md`和`lina-review`强制门禁。
- 严重：真实浏览器执行被禁止修改的宿主 Controller 重复声明阻断，目前只有 Playwright 发现清单而没有实际 Chromium 结果和截图。依据：`.agents/rules/testing.md`和`lina-review`证据要求。
- 本切片代码未发现新的安全、数据权限、事务、性能、API、SQL、缓存或国际化阻塞问题；真实 PostgreSQL 5 轮随机回归、两插件 Go lint、前端类型检查/构建、插件治理和 OpenSpec 严格校验均通过。

## 2026-08-20 最终复核

此前记录的宿主启动和国际化阻断已经由共享工作区其他有效改动解除，本轮没有为绕过门禁修改`web-antd`。在当前宿主、插件路由和 Chromium 中重新执行`TC002-provider-health.ts`，中文完整权限、最小策略更新、无手工解锁、只读权限和英文页面共 5 项全部通过；与`TC003`合并执行共 10/10 通过，用时 43.3 秒。因此上文“只有发现清单、没有真实浏览器结果”的历史状态不再代表当前结论，任务`9.5.d`完成。
