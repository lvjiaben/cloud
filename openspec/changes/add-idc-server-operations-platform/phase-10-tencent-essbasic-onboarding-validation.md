# 第十阶段腾讯电子签开通向导验证记录

## 验证范围与结论

本记录对应任务`10.8.b`，只覆盖购买方安装内的腾讯电子签企业开通向导，不实现模板管理、合同签署业务、自动签本地高风险授权、客户合同页面、回调归档或第十五阶段能力。

当前已完成`10.8.b.1`、`10.8.b.2`和`10.8.b.3`，并据此关闭`10.8.b`。真实 PostgreSQL 迁移、DAO 再生成、可启动宿主、真实 Chrome 双语/暗色/新窗口回归和截图审查均已完成。`10.8`、`10.17`和`FB-11`仍保持未勾选，不将开通向导验收扩大为模板、签署、归档或整阶段完成。

## 冻结的用户流程

1. 第一步只读展示当前安装已有的企业法定名称、地区和本地商家编号；LinaPro 不收集统一社会信用代码、法定代表人身份证、手机号或其他腾讯认证敏感资料。
2. 财务、法务或其他具备精确权限的管理员点击“前往腾讯办理”。服务端通过购买方自己的腾讯应用生成一次性短期官方链接，浏览器在新窗口打开，原向导保持不变。
3. 腾讯要求的法定代表人或超级管理员在腾讯页面完成企业和经办人认证；发起人回到原向导后显式点击“重新检查”，浏览器返回本身不能推进状态。
4. 第四步分别显示企业印章管理、合同下载和企业自动签能力。企业自动签仍默认关闭，不能由本向导自动启用。
5. 腾讯账号、应用、凭据、主体、套餐和费用全部归当前购买方安装；软件提供方没有中央业务网关、中央结算、业务回调或平台操作者依赖。诺诺继续受`10.4.c`和`10.10`外部证据门禁关闭。

腾讯接口实现依据为[生成子客登录链接](https://cloud.tencent.com/document/api/1420/61524)和[查询企业扩展服务授权信息](https://cloud.tencent.com/document/api/1420/85666)。前者明确返回`ConsoleUrl`、`IsActivated`和`ProxyOperatorIsVerified`，且`PC`链接有效期为五分钟、单次有效；本地实现按官方“企业名称最多 200 个字符”校验 Unicode 字符数，不按 UTF-8 字节数误拒中文名称。

## 实现证据

| 能力 | 实现与边界 |
|---|---|
| 核心商家资料 | `idc-core/backend/cap/idccap/merchant.go`增加法定名称、展示名称和地区的安全投影；`merchantcontext`以一次`LIMIT 1`查询只读取默认商家所需列，不产生`N+1`查询。 |
| 显式依赖注入 | 核心插件通过`idccap/spi`声明唯一工厂；合规插件启动装配冻结工厂并把同一请求图中的商家服务、供应商配置服务和本地网关客户端注入开通服务。不存在控制器临时构造关键服务。 |
| 管理 API | 新增只读状态、创建办理链接和显式重新检查三个`/admin/compliance/electronic-contract/onboarding`端点；分别使用查看和主体绑定精确权限，写操作继续要求供应商服务校验最近五分钟完整登录。时间字段统一为 Unix 毫秒。 |
| 本机签名协议 | 插件只向当前安装的本地网关发送企业法定名称和服务端幂等键；读取、链接和重新检查使用三个独立签名范围。相同业务键恢复时返回冲突并停止，不能再次调用腾讯。 |
| 腾讯官方链接 | 本地网关固定调用官方`CreateConsoleLoginUrl`，只传购买方`AppId`、安装派生的企业/经办人`OpenId`和已有法定名称；信用代码、姓名、手机号和身份证字段保持空值。响应 URL 必须为 HTTPS，插件再次限制为腾讯官方域名后才交给浏览器。 |
| 主体隔离 | 腾讯确认企业激活且经办人认证后，本地网关才把服务端一次性主体投影交给供应商主体服务规范化、加密和绑定；管理 API 只返回本地`lsb_`不透明编号。`RecordManagedBinding`固定只接受腾讯，诺诺无法绕过自己的外部证据门禁。 |
| 管理页面 | `frontend/pages/electronic-contract.vue`实现四步同页向导、只读资料、新窗口、显式重新检查、失败关闭原因和逐项能力状态；页面使用 Vben 主题变量并提供中英文可见文案与无障碍进度名称。 |
| 路由与权限 | `plugin.yaml`注册“合规中心 → 电子合同”动态页面和独立开通按钮权限；中英文`menu.json`与页面资源完整对应。 |
| 测试源码 | `TC004-electronic-contract.ts`当前只实现`TC-4a`的只读资料、新窗口、请求载荷、显式重新检查、能力状态和双语不可用说明；中文截图显式固定浅色主题，英文截图显式固定暗色主题。没有把`TC-4a`通过冒充为`10.17`全部 E2E 完成。 |
| 数据库门禁 | `008-tencent-essbasic-onboarding.sql`以可重复执行的约束替换允许腾讯在官方公开契约、活动本地主体同时成立时启用适配器，并继续数据库级禁止诺诺启用。迁移已在本地 PostgreSQL 18 独立 schema 中完成连续与幂等验证，随后通过`make.cmd dao dir=apps/lina-plugins/idc-compliance/backend`重新生成 72 个 DAO/DO/Entity 文件；迁移不新增自增主键、软删除或高频查询。 |

## `lina-review`修正

本轮完成反馈闭环后的独立审查，并修正以下阻断问题：

- 腾讯专用服务端托管绑定入口原先复用了“腾讯或诺诺均合法”的通用供应商校验，理论上可把诺诺主体状态直接写为活动；现已固定只接受`tencent_essbasic`并增加拒绝诺诺的单元测试。
- 办理链接和重新检查已经写入业务幂等围栏，但原处理器忽略“已恢复”结果，网络重试可能再次调用腾讯；现已在供应商调用前返回`process_auth_replay`，测试证明恢复请求不会越过腾讯调用边界。
- 企业名称原按 200 字节校验，会误拒合法的较长中文名称；现按腾讯官方 200 字符上限校验有效 UTF-8，并覆盖 200/201 个中文字符边界。
- 页面卡片原直接使用 Vben 的 HSL 片段变量，暗色主题可能得到无效背景值；现统一使用`hsl(var(--...))`并修正前景、弱化文字和无障碍名称。
- 新增服务按仓库约定使用`serviceImpl`，主文件只保留契约和构造，流程实现拆到职责文件；新增接口、控制器和测试文件补齐用途及公开契约注释。
- 真实 Chrome 首轮暴露页面仅依赖宿主全局注册的组件接线缺口：告警框、状态徽标和四步进度条未渲染。现将`Alert`、`Badge`、`Button`、`Steps`和`Tag`全部显式导入，并在 E2E 中新增状态文案和`.ant-steps`可见性断言。
- 真实 PostgreSQL 回归暴露供应商配置测试 fixture 漏载`008-tencent-essbasic-onboarding.sql`；现已纳入完整迁移链，确保测试的数据库状态与实际安装一致。

## 已通过验证

| 验证 | 结果 |
|---|---|
| `GOWORK=off go test ./...`：`idc-core` | 通过。 |
| `GOWORK=off go test ./...`：`idc-compliance` | 通过。 |
| `GOWORK=off go test ./...`：`central-gateway` | 通过。 |
| 三个模块分别执行`make.cmd lint.go dir=<module> plugins=1` | 通过；`golangci-lint`均为 0 个问题，`staticcheck U1000`均通过。 |
| `pnpm -F @lina/web-antd run typecheck` | 通过。 |
| `pnpm -F @lina/web-antd run i18n:check` | 通过；2 个语言、4 个前端资源文件键一致。 |
| `pnpm -F @lina/web-antd run build` | 通过并生成生产`dist.zip`；只有既有依赖的动态/静态导入和 Browserslist 数据提示。 |
| `make.cmd plugins.check format=json` | 通过；扫描 11 个清单和 1640 个 Go 文件，0 个问题。 |
| `openspec validate add-idc-server-operations-platform --strict` | 通过。 |
| Windows/Linux `amd64`本地网关组件包 | 通过；最终包各 28 个清单文件，全部 SHA-256 校验通过，`supplierClientsIncluded=true`、`businessRoutesEnabled=false`、`localInstallationOnly=true`。 |
| 真实 PostgreSQL | `TestComplianceFoundationMigrationPostgreSQL`与`TestManagedTencentOnboardingBindingEnablesOnlyReviewedAdapterPostgreSQL`在 PostgreSQL 18 独立 schema 中通过，测试结束完成清理。 |
| DAO 再生成 | `make.cmd dao dir=apps/lina-plugins/idc-compliance/backend`通过；当前 72 个生成文件的路径+内容聚合 SHA-256 为`6c6a8dfb4e34d803ac24f1809ca9ad3eb664c49cb34a164bb54679cd51c65e8d`，再生成后全量 Go 测试、`go vet`和 lint 通过。 |
| 宿主启动 | 在不覆盖三个共享宿主文件的前提下，使用仅用于验证的 Go overlay 与安装内一次性私密运行配置启动；`make.cmd status`确认 Lina Core `9999`和 Lina Vben `6670`均为`running`。 |
| 真实 Chrome `TC-4a` | 使用 Playwright `channel=chrome`串行执行 3 个子用例，3/3 通过；覆盖中文浅色四步向导、服务端短期链接新窗口、返回后显式重新检查、英文暗色不可用原因和原始翻译键排除。 |
| 截图审查 | `temp/20260814/103930-electronic-contract-onboarding-zh-cn.png`与`temp/20260814/103938-electronic-contract-unavailable-en-us-dark.png`已人工审查；状态徽标、四步进度、警告、中英文、暗色对比和页面布局均正常。 |

最终验证包位于：

- `temp/codex-10-8-b-final-v2-windows-amd64`
- `temp/codex-10-8-b-final-v2-linux-amd64`

这些目录只是未提交验证产物，不是源码交付目录。

## 仓库级既有阻断与剩余风险

- 三个共享宿主文件仍包含既有重复控制器声明：`apps/lina-core/internal/controller/role/role_new.go`、`plugin/plugin_new.go`和`joblog/joblog_new.go`。本轮未修改、删除或覆盖这些共享改动；浏览器验收使用仅替换这三个编译输入的临时 Go overlay，所有实际页面、插件、API、数据库和运行路由当前工作区加载。
- 根级`make.cmd i18n.check plugins=1`仍只报告`notice-preview-modal.vue`引用的 6 个既有宿主内容提示键；本插件运行时扫描为 0 违规，`pnpm -F @lina/web-antd run i18n:check`已验证 2 种语言和 4 个本插件资源文件的键一致。本轮未新增或修改用户可见文案。
- 全仓 E2E 治理检查在 Node 24/Windows 下需要一次性`pnpm.exe`兼容入口才能越过旧`spawnSync('pnpm')`调用；原检查继续执行后只报告 9 个其他插件的`scaffold.json`和 2 个宿主用例隔离配置。`TC004-electronic-contract.ts`本身未被治理检查报错，且其 3 个子用例已在真实 Chrome 独立通过。
- 诺诺适配器仍受`10.4.c`和`10.10`外部证据门禁关闭，本任务没有生成、伪造或替代外部确认。

## 影响分析

- 数据权限：本轮只修复前端组件接线、测试 fixture 和生成代码，没有扩大已审查 API 的权限或商家范围。
- 缓存一致性：本轮不新增、修改或绕过缓存与权威数据路径。
- `i18n`：本轮没有新增可见文案，只让已有中英文键正确渲染；插件专项检查和真实浏览器双语验收通过。
- 开发工具与跨平台：本轮使用现有`make.cmd dao`、`dev`、`lint.go`、`plugins.check`和`i18n.check`，没有修改工具源码；既有 Windows/Linux `amd64`组件包验证仍有效。
- 插件边界：所有业务修复均位于`idc-compliance`内；没有修改`web-customer`、没有引入中央网关或软件提供方业务服务。

## 明确保持不变

- 未修改`apps/lina-vben/apps/web-customer`，未进入`10.8.f`、`10.12`或第十五阶段。
- 未删除`idc-portal/portal`、旧`dist`、锁文件、测试副本、`go:embed`或旧 SPA 路由。
- 未新增软件提供方中央业务服务、销售注册、跨安装路由、中央结算、业务回调或平台操作者依赖。
- 未实现真实诺诺客户端、规范化器、业务路由、查询器或采集器；`10.4.c`外部证据阻断继续保留。
