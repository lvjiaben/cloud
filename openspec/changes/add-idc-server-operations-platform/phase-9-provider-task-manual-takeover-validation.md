# Provider 任务管理与人工接管验证记录

## 交付结果

资源插件版本提升到`v0.1.8`，核心插件最低依赖提升到`v0.1.54`。后台新增 Provider 任务核对页和三项独立权限：列表只返回本地有界非秘密投影，详情只返回最新标准化上游绑定及不可变脱敏证据；人工接管只关闭自动收敛并记录责任人，不调用 Provider、不重试写入，也不裁决任务成功或失败。

## 安全与性能边界

- 列表按任务编号使用键集分页，每页默认 25、最大 100，一次联表读取任务、渠道和不可变接管记录。详情固定使用三次有界本地查询，分别读取任务/渠道/接管联表、最新上游绑定和最多 50 条证据，不产生按任务或证据增长的数据库`N+1`。
- 列表、详情和接管分别使用`idc:admin:resource:task:list`、`idc:admin:resource:task:query`和`idc:admin:resource:task:manual-takeover`；授权先于参数、状态和目标存在性校验。单商家数据范围固定为默认商家，详情与接管目标固定为精确任务编号。
- 只有`accepted`、`running`和`result_unknown`允许接管。请求必须包含最后读取的任务版本、一个稳定受控原因以及 1—16 条属于该任务的既有证据；不接受自由文本、任意操作人、结果状态、重试开关或上游操作参数。
- HTTP 传输边界再次使用最大 8 KiB 的严格 JSON 解码拒绝未知字段、尾随 JSON 和绑定结果不一致。严格解码只形成内部违规标志，业务服务仍先授权再拒绝，因此不会通过错误差异泄露任务存在性。
- 接管事务锁定精确任务，集合核对证据归属，追加唯一不可变接管记录，撤销保留的输入秘密绑定，把任务置为封闭`manual_takeover`，清空写租约、下次观测与观测领取，并追加业务审计。任一步失败整体回滚；相同人员、版本、原因和证据集合可以按摘要安全重放。
- 接管记录与任务证据由 PostgreSQL 触发器限制为只追加。API、页面和审计不返回规范化写参数、秘密绑定、凭据或上游原始载荷。

## 验证结果

| 门禁 | 结果 |
| --- | --- |
| `make.cmd dao dir=apps/lina-plugins/idc-resource/backend` | 通过；只对明确命名的本地`linapro_idc_codegen`专用库重放迁移并生成新增接管 DAO/DO/Entity |
| `make.cmd ctrl dir=apps/lina-plugins/idc-resource/backend` | 通过；生成三项 API 接口与 Controller 后补回显式依赖注入和安全实现 |
| `GOWORK=off go test ./backend/... -count=5 -shuffle=on` | 通过 |
| `TestResourceFoundationMigrationPostgreSQL -count=5 -shuffle=on` | 通过；每轮使用随机隔离 schema，覆盖迁移幂等、权限顺序、跨任务证据拒绝、版本与内容冲突、安全重放、观测关闭、证据/接管不可变和审计失败回滚 |
| 核心`idccap`与`managementruntime`测试五轮随机回归 | 通过；新增三项权限仍经宿主精确权限与数据范围能力发布 |
| 资源与核心插件`make.cmd lint.go` | 通过；`golangci-lint`和`staticcheck U1000`均为 0 项 |
| `pnpm -F @lina/web-antd typecheck` | 通过 |
| `pnpm -F @lina/web-antd i18n:check` | 通过；四个宿主前端语言文件双语键一致 |
| `pnpm -F @lina/web-antd build` | 通过；仅有既有依赖动态/静态导入和 Browserslist 数据提示 |
| `make.cmd plugins.check` | 通过；11 个插件清单、配置和生产 Go 文件无治理问题 |
| `playwright test .../TC003-provider-task-takeover.ts --list` | 通过；发现 1 个文件中的 5 个 Chromium 场景，覆盖中英文、最小安全请求、无危险操作、无自由文本和只读权限 |
| `openspec validate add-idc-server-operations-platform --strict` | 通过 |
| `make.cmd i18n.check` | 未通过；本插件扫描、消息与 API 文档覆盖通过，但被宿主通知预览页 6 个既有前端缺键阻断 |

## 当前外部阻断

真实 Chromium 执行依赖宿主开发服务。`make.cmd dev`仍在未归属本切片的`apps/lina-core/internal/controller/{role,plugin,joblog}/*_new.go`因重复`ControllerV1/NewV1`声明而构建失败；根目录不存在`.contributing`，业务插件任务不得修改宿主文件。因此当前只有 Playwright 发现清单，没有把未运行的浏览器场景和截图冒充为通过。

仓库级`make.cmd i18n.check`仍被`apps/lina-vben/apps/web-antd/src/views/system/message/notice-preview-modal.vue`引用、但宿主中英文资源都缺少的`plugin.content-notice.preview.title`、`fields.type`和`fields.createdBy`阻断，共 6 项。插件自己的中英文页面、菜单、错误和 API 文档资源均通过覆盖扫描。

Windows 上的`pnpm test:validate`还在宿主`hack/tests/scripts/validate-e2e.mjs`调用`spawnSync('pnpm', ...)`后直接访问空`stdout`而抛出`TypeError`，没有进入测试清单治理检查；该工具位于禁止修改的宿主目录。模块清单仍已识别本插件 3 个`TC`文件，单独 Playwright 清单可正确发现新增 5 个场景。

根据`lina-review`门禁，仓库级国际化、E2E 治理脚本和真实浏览器证据缺失均按严重外部阻断记录，9.5.e 暂不作为已完成任务验收，也不进入归档。

## 影响判断

- 数据权限：三项接口均使用独立精确权限；授权先于业务校验，详情与接管绑定精确任务目标，列表只查询`tenant_id = 0`的单商家数据。
- `i18n`：新增中英文菜单、页面、错误和中文 API 文档；英文 API 文档沿用英文源码。插件自身覆盖通过。
- 缓存：无影响。任务、证据、接管和审计都以 PostgreSQL 为唯一权威，不新增缓存或失效路径。
- 依赖注入：任务管理服务与库存、健康服务一样由`pluginhost.HTTP`启动回调构造，显式接收资源存储和`managementcap.Runtime`；不新增运行期全局服务定位。
- 开发工具：无源码影响。本切片未修改`Makefile`、`make.cmd`、根`hack/`、CI 或脚本；Windows 的既有 E2E 校验脚本缺陷只记录为外部阻断。
- 宿主：未修改`apps/lina-core`、`apps/lina-vben`或根`hack`源码。核心插件只新增公开权限常量目录，资源业务逻辑全部闭环在资源插件。

## Lina-review 结论

本轮实际读取`AGENTS.md`、`.agents/rules/{openspec,documentation,architecture,data-permission,plugin,api-contract,backend-go,database,frontend-ui,testing,i18n,dev-tooling}.md`和`.agents/instructions/markdown-format.instructions.md`；两个命中的业务插件根目录均不存在本地`AGENTS.md`。缓存规则未命中，因为没有新增或修改缓存。

- 严重：仓库级`i18n`门禁被禁止修改的宿主 6 个既有缺键阻断。
- 严重：真实浏览器执行被禁止修改的宿主 Controller 重复声明阻断。
- 严重：Windows E2E 治理脚本在启动前因禁止修改的宿主`spawnSync`空输出处理缺陷中止。
- 已修复：初次实现依赖默认 JSON 绑定，不能证明夹带`status/result/retry`等未知字段会被拒绝。现已增加有界严格解码、服务授权后拒绝及回归测试。
- 本切片代码未发现剩余的安全、数据权限、事务、性能、API、SQL、缓存或国际化阻塞问题；真实 PostgreSQL、Go 测试与静态检查、前端类型检查/构建、插件治理和 OpenSpec 严格校验均通过。

## 2026-08-20 最终复核

此前记录的宿主启动、国际化和 Windows 测试治理阻断已经由共享工作区其他有效改动解除，本轮没有为绕过门禁修改`web-antd`。修正测试基线中查询参数匹配、抽屉就绪点、虚拟下拉项和确认框定位后，在当前宿主、插件路由和 Chromium 中重新执行`TC003-provider-task-takeover.ts`，安全请求、危险操作缺失、只读权限和英文页面共 5 项全部通过；与`TC002`合并执行共 10/10 通过，用时 43.3 秒。因此上文“没有真实浏览器结果”的历史状态不再代表当前结论，任务`9.5.e`完成，并与`9.5.a`至`9.5.d`共同关闭父任务`9.5`。
