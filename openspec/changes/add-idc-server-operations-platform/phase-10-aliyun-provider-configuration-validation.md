# 第十阶段阿里云服务商配置验证记录

## 完成范围

任务`10.2.b.1`和`10.2.b.2`已完成安装实例级阿里云`FACE`服务商后台配置纵向切片。插件提供安全查询、首次配置、替换、专用中断恢复和合成只读连通性验证`API`，以及双语`Vben`页面和插件本地`TC001-provider-configuration.ts`自动化资产。仓库标准 runner 已在可启动宿主和隔离数据库上实际执行，`TC001`六个场景全部通过，关键截图已经人工审查，因此`10.2.b`父项可以完成。

## 安全与业务边界

| 边界 | 实现结果 |
|------|----------|
| 权限 | 查询、写入和验证分别要求`idc:admin:compliance:provider:query/write/validate`，目标固定为安装实例级`compliance-provider-aliyun-face` |
| 最近登录 | 新增、替换、恢复和验证要求当前员工在最近五分钟内完成宿主完整登录；查询不额外要求重新登录 |
| 秘密处理 | 页面和`API`只返回场景、状态、版本、秘密代次和掩码尾部；`AccessKey`只以可清除缓冲进入`secretcap`回调，不进入数据库、日志或审计快照 |
| 中断恢复 | 服务端持有随机操作键；浏览器不提交操作键，恢复只能创建、替换或精确重放同一次秘密变更；恢复元数据不进入任何外部投影，后续替换使用新键覆盖 |
| 连通性 | 只向固定阿里云端点发送合成`query`，不调用`init`且不提交身份资料；只保存`reachable/rejected/unavailable`和时间，网络与`5xx`均为`unavailable` |
| 数据权限 | 配置是默认租户内唯一平台单例，没有客户级动态结果集；管理运行时仍在每次操作前检查精确权限和固定目标，不接受请求方提供的租户或目标编号 |

## 依赖来源检查

`idc-core`拥有的管理运行时在插件路由装配期提供授权器与业务审计器，宿主`sessioncap.Service`由`HTTPRegistrar.Services()`提供。合规插件从自己的运行配置绑定一组共享`secretcap.Accessor/Manager`，并在启动期建立复用十秒超时`HTTP`客户端的阿里云连通性检测器。上述依赖逐项进入`providerconfig.New()`，控制器只接收该`Service`；请求路径没有服务定位、独立关键服务图或本地缓存。

## 自动化验证

| 验证 | 结果 |
|------|------|
| `GOWORK=off go test ./backend/... -count=1` | 通过；覆盖适配器、控制器、权限、秘密清理和服务状态机 |
| 设置隔离测试`DSN`后执行`GOWORK=off go test ./backend/... -count=1 -shuffle=on` | 通过；真实`PostgreSQL`覆盖创建、重放、替换、旧验证清除、三类检测结果和两类中断恢复 |
| `make.cmd lint.go plugins=1 dir=apps/lina-plugins/idc-compliance` | 通过，`golangci-lint`与`staticcheck`均为零问题 |
| `LINAPRO_SOURCE_PLUGINS=1 pnpm --dir apps/lina-vben --filter @lina/web-antd run typecheck` | 通过 |
| `make.cmd plugins.check` | 通过，扫描 11 个清单和 1492 个文件，零发现 |
| `openspec validate add-idc-server-operations-platform --strict` | 通过 |
| `make.cmd i18n.check` | 通过；运行时硬编码、宿主与插件消息覆盖、宿主与插件前端键覆盖均为零问题 |
| `pnpm test:validate`（`hack/tests`） | 通过；校验 217 个 E2E 文件、17 个范围、6 个 smoke 文件和 197 个串行文件，CI 分片有效 |
| `pnpm exec tsc --noEmit -p tsconfig.json`（`hack/tests`） | 通过 |
| `pnpm build:check`（合规插件独立前端） | 通过 |
| 标准 runner：`pnpm test:module -- plugin:idc-compliance` | 通过，23/23；其中`TC001-provider-configuration.ts`为 6/6 |

## E2E 质量审查

本切片新增页面、表单、权限动作和接口联动，已触发`E2E`质量审查。`TC001`按插件本地连续编号闭环在合规插件目录，覆盖中文安全投影、首次配置整组提交且不回显秘密、只读验证、中断失败关闭与专用恢复、最近登录过期失败路径和英文译文。标准 runner 实际执行 6/6 通过。

人工审查了中文安全投影、配置弹窗、保存后掩码、连通性可达、恢复弹窗、恢复完成、近期登录过期失败和英文页面八张截图。页面未显示秘密原文、秘密引用或内部翻译键；中英文层级清楚，弹窗、失败状态和操作按钮均可见。截图顶部的“加载菜单中”是宿主动态菜单加载提示，不遮挡任务操作区，也没有被当作页面成功依据。

此前记录的宿主重复控制器、通知预览缺键、E2E 类型错误和 Windows runner 异常已经在共享工作区各自所有者范围内闭环；本切片没有通过修改`web-antd`业务源码或依赖绕过。当前复核中`web-antd`没有 IDC 业务差异。

## 影响分析

- `i18n`：有影响。插件启用中英文资源，运行时页面、菜单、状态字典、错误和中文`apidoc`已同步；英文`apidoc`保持空对象并使用英文源文本。
- 缓存一致性：无影响。当前服务不维护缓存、订阅或节点本地派生状态。
- 数据权限：有影响。使用固定安装实例目标的精确授权，没有批量、列表或客户数据投影；越权统一返回不可访问。
- 开发工具跨平台：有验证影响。仓库 runner 已能在 Windows 正确调用`pnpm`并使用仓库相对测试路径；根 E2E 治理和标准模块执行均通过。
- 性能：单次查询至多读取一个本地配置并读取一次秘密元数据；写入和验证使用固定次数事务与一次远程调用，没有随客户、列表或插件数量增长的循环查询。
