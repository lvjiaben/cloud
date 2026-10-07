# 第一阶段验证记录

## 范围与工作树边界

- 仓库根目录不存在`.contributing`，因此本阶段生产代码只新增`apps/lina-plugins/idc-*/`，未修改`apps/lina-core`、`apps/lina-vben`、根`hack`或无关插件。
- 十个目标插件在创建前均不存在，因此没有插件本地`AGENTS.md`需要继承。
- 执行前后已有的`.gitmodules`、`Makefile`、`make.cmd`、`apps/lina-core/manifest/config/config.template.yaml`和`apps/lina-vben`开发配置改动保持原状；完整构建没有留下新的宿主跟踪文件修改。
- `OpenSpec`设计文件和`.agents/rules/user-preferences.md`属于设计与跨任务约束记录，不属于运行期宿主代码。

## 插件交付矩阵

| 插件 | 分发方式 | 硬依赖 | 第一阶段交付边界 |
| --- | --- | --- | --- |
| `idc-core` | `source + builtin` | 无 | 客户、商品、交易、售后和商家治理的拥有方边界 |
| `idc-payment` | `source + builtin` | `core` | 支付、退款与对账适配边界 |
| `idc-resource` | `source + builtin` | `core` | 资源账户、库存、实例和异步操作拥有方边界 |
| `idc-zjmfcloud` | `source + builtin` | `resource` | 智简魔方公有云`Provider`适配边界 |
| `idc-compliance` | `source + builtin` | `core` | 实名、合同和发票边界 |
| `idc-notify` | `source + builtin` | `core` | 通知事件、模板与投递边界 |
| `idc-channel` | `source + builtin` | `core` | 下游渠道、兼容接口与`Webhook`边界 |
| `idc-ai` | `source + builtin` | `core`、`resource` | 只读洞察、诊断、预测和草稿边界 |
| `idc-portal` | `source + builtin` | 仅`core` | 独立官网与客户中心，其他模块只做运行时发现 |
| `idc-zjmfmigrate` | `source + managed` | `core`、`resource` | 人工启用、无外部副作用的迁移窗口工具 |

清单依赖图无环，源码插件聚合文件包含十个插件。九个插件声明为`builtin`，迁移插件声明为`managed`；宿主通用协调逻辑的定向测试使用合成夹具验证了依赖顺序、安装、启用、升级和降级保护，但十个真实插件的完整生命周期集成验证仍属于任务 1.7 的未完成部分。

## 开关与生命周期契约

- 九个`builtin`插件嵌入了`stop_new`、`drain_active`、`preserve_history`和`resume_after_validation`生命周期声明，并完成嵌入与关键字段存在性测试。声明的统一结构化解析、运行时消费者和真实停新、排空、恢复集成验证尚未完成。
- `Portal`只硬依赖`core`，支付、资源、合规、通知、渠道和`AI`被声明为可选运行时能力。当前阶段只预留发现与降级边界，页面仍是静态交付骨架，尚未实现按运行时能力隐藏入口。
- 迁移插件嵌入并测试`manual_enable: true`、`migration_mode: true`和`no_external_side_effects: true`，其清单保持`distribution: managed`。
- 当前只验证声明、嵌入和编译边界；各业务域在拥有真实任务、资金或资源状态后，仍须在对应后续任务中补充结构化契约、停新、排空、恢复和并发行为测试。

## `Portal`交付验证

- `Portal`使用独立`Vue`、`Vite`、`TypeScript`和`Ant Design Vue`工程，并采用与管理端一致的浅色组件与视觉语言；开发端口为浏览器安全的`6677`，管理端继续使用`6666`。
- 构建产物使用`/x-assets/idc-portal/v0.1.0/`版本化基础路径和内容哈希，并通过`go:embed`进入插件交付物。
- 显式`SPA`范围只包含`/`、`/products/*`、`/pages/*`、`/auth/*`和`/account/*`；测试明确排除`/admin`、`/api`、`/x`和`/x-assets`。
- 使用本仓库新构建的宿主在隔离端口`9998`启动，通过真实`Chrome`顺序执行`Portal`的 4 个文件、9 条阶段用例，结果为`9 passed`。覆盖中文与英文桌面、中文与英文移动端、语言偏好持久化、深层刷新、客户中心入口、管理后台隔离，以及`API`、插件`API`和静态资源路径隔离。
- 代表性视觉证据为[中文桌面](../../../temp/20260806/113057-idc-portal-public-zh-desktop.png)、[英文桌面](../../../temp/20260806/113101-idc-portal-i18n-en-desktop.png)、[中文移动端](../../../temp/20260806/113101-idc-portal-i18n-zh-mobile.png)和[管理后台隔离](../../../temp/20260806/113104-idc-portal-route-admin-isolated.png)。完整商品购买与客户中心业务流程仍按任务 15.8—15.10 的未完成子项实施。
- 宿主公共`globalSetup`会因后台登录成功后仍保留`/admin/auth/login`路径、仅切换`hash`而超时；`Portal`用例不需要后台登录，因此插件内`playwright.public.config.ts`复用宿主配置并跳过无关登录前置。宿主`Windows`模块运行器仍因`spawnSync('pnpm')`返回`ENOENT`无法使用，未修改受保护的根`hack`目录。

## 验证结果

| 验证 | 结果 |
| --- | --- |
| 十个插件分别设置`GOWORK=off`执行`go test -mod=readonly -count=1 ./...` | 通过 |
| 十个插件分别执行`make.cmd lint.go dir=apps/lina-plugins/<plugin-id> plugins=1` | 通过，均为`0 issues` |
| `make.cmd plugins.check format=json` | 通过，`0 findings` |
| 宿主生产代码 panic 白名单定向测试 | 通过 |
| 宿主`TestBootstrapBuiltinPlugins*`通用协调逻辑合成夹具测试 | 通过，不代表十个真实插件生命周期已集成验证 |
| `Portal`执行`pnpm install --frozen-lockfile`、类型检查与生产构建 | 通过 |
| `Portal`定向`TypeScript`测试编译 | 通过 |
| `Portal`隔离宿主真实`Chrome`端到端测试 | 通过，4 个文件、9 条用例，`9 passed` |
| `Windows`插件完整构建 | 通过，生成`temp/output/lina.exe` |
| `openspec validate add-idc-server-operations-platform --strict` | 通过 |

仓库级`i18n.check`仍被已有`notice-preview-modal.vue`缺少六个`content-notice`中英文键阻断；运行时扫描和消息覆盖本身通过。仓库级`lint`仍被已有`Windows`工具代码未检查`CloseHandle`返回值阻断。完整`Go`测试还存在宿主配置路径、任务数据、`HTTP`测试环境、动态插件代次、`Windows`文件替换、`SQL`资产和`README`换行符等既有失败。这些问题位于本阶段禁止修改的宿主目录。

阶段插件范围内的测试、检查、`Windows`构建和`Portal`浏览器验收已通过，但任务 1.7 的真实插件生命周期集成、`Linux`/`macOS`构建烟测及上述仓库级`i18n`强制门禁仍未完成。因此本变更当前不能标记为整体完成或归档。

## 2026-08-18 真实生命周期补充验证

- 当前共享工作区已存在`.contributing`，本次补充验证没有新增 IDC 宿主业务代码；开始构建时发现`joblog`、`plugin`和`role`三个宿主控制器文件各自被重复追加了同名空控制器，导致完整构建失败。删除这三段纯重复内容后，三个文件恢复为原有版本，`go test ./internal/cmd -count=1`通过。
- 使用当前 13 个源码插件构建`windows/amd64`完整宿主，并在独立数据库`linapro_phase1_lifecycle`执行宿主初始化后真实启动。九个 IDC `builtin`插件均自动安装并启用；`core`先于`resource`，`resource`先于`zjmfcloud`，其余依赖也满足后才安装。再次启动时全部收敛为无变化，没有重复迁移。
- `idc-zjmfmigrate`保持`managed + uninstalled`，随后通过真实管理 API 完成安装、启用、停用、保留数据卸载、重新安装和恢复启用，所有动作均返回成功。
- 九个`builtin`插件通过普通插件停用 API 均被稳定拒绝，证明不能绕过启动治理直接关闭内建插件。业务功能、渠道和 Provider 自身的停新、在途排空、历史保留、恢复，以及真实版本升级，仍需在对应业务域实现完成后做最后一轮集成验证，因此 1.7 父任务继续保持未完成。
- 完整`make.cmd i18n.check plugins=1`已重新执行并通过，旧记录中的六个宿主缺键阻断已经消失。

## 2026-08-22 最终生命周期收口

- 十个 IDC 插件的注册、清单、依赖排序、内建安装、升级与生命周期定向测试全部通过；宿主内建插件安装、升级、依赖排序、停用拒绝及恢复测试通过。
- 功能、通知渠道、Provider 的“停止接收新任务、已有任务继续排空、历史数据保留、校验后恢复”定向测试全部通过；迁移插件继续按`managed`生命周期验证，默认不自动启用。
- 十个插件逐一执行`go mod tidy -diff`均无差异；依赖修正后，`notify`、`channel`、`ai`、`portal`和`zjmfmigrate`后端全量测试通过。
- 清理误生成的空控制器后，`idc-core`执行精确权限目录测试与`GOWORK=off go test ./backend/... -count=1`全部通过。此次只修复插件自有依赖和误生成文件，没有把 IDC 业务写入宿主源码。
- 上述证据补齐了此前记录中“真实生命周期尚未收口”的缺口，任务`1.7`现可独立完成；这不代表后续阶段的具体业务能力已经完成。

基于上述真实插件生命周期、开关停新/排空/恢复和迁移插件生命周期证据，任务 `1.7` 已于 2026-08-29 收口。未修改宿主代码，也未提前勾选依赖第 12 阶段代理关系事实的任务。
