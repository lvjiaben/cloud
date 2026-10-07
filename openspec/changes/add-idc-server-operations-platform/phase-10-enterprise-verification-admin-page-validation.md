# 第 10 阶段企业认证后台页面验证记录

## 记录范围

- 日期：2026-08-18
- 对应任务：`10.12.e`
- 本记录验证合规插件自己的企业认证审核队列、脱敏详情、单次原件下载、审核批准和封闭原因驳回页面。
- 页面、接口客户端、双语资源和插件专属测试均位于`apps/lina-plugins/idc-compliance/`；`apps/lina-vben/apps/web-antd/`状态为空。
- 本任务不证明合同管理页面`10.12.f`、发票与红字页面`10.12.g`或父任务`10.12`完成。

## 当前实现

| 能力 | 当前实现与边界 |
| --- | --- |
| 审核队列 | 使用最多`25`条的服务端游标页，可按精确客户编号和稳定状态筛选，不读取精确总数 |
| 脱敏详情 | 只显示服务端返回的脱敏企业名称、登记标识、代表人和证件投影；列表和详情均不显示原始法定资料 |
| 原件访问 | 页面先签发短期单次访问上下文，再立即消费并下载 Blob；浏览器不取得对象地址或永久链接，并在触发下载后立即撤销 Blob 地址 |
| 逐次审计 | 每次真正放行原件前由后端在同一事务追加不可变审计；页面不能绕过访问上下文直接读取内容 |
| 审核动作 | 开始审核、批准和驳回分别使用独立精确权限、服务端版本和随机幂等键；驳回只允许封闭原因，不接受自由文本 |
| 插件装配 | 插件前端独立构建为插件资源，由现有动态页面装配显示在管理后台；没有把源码、依赖或语言包写入`web-antd` |

任务直接复核文件包括：

- `frontend/pages/enterprise-verifications.vue`
- `frontend/pages/enterprise-verification-client.ts`
- `frontend/pages/components/enterprise-verification-detail-drawer.vue`
- `manifest/i18n/zh-CN/enterprise.json`
- `manifest/i18n/en-US/enterprise.json`
- `hack/tests/e2e/TC003-enterprise-kyc.ts`
- `hack/tests/pages/EnterpriseVerificationPage.ts`
- `hack/tests/support/enterprise-verification-mock.ts`

## 真实装配与浏览器验证

当前默认开发数据库里已有验证码秘密由一次已丢失的临时随机主密钥加密，因此插件完整版后端按设计拒绝启动。没有删除旧记录或重建当前数据库。本轮新建隔离数据库`cloud_phase10_e2e_20260818`，使用单独的测试配置和稳定测试密钥完成基础初始化、内置插件依赖安装、插件迁移、测试数据加载和真实宿主启动；原`cloud`数据库保持不变。

插件完整版在`http://127.0.0.1:9999/`正常启动，管理前端在`http://127.0.0.1:6670/`通过现有插件装配加载企业认证页面。标准插件测试入口最终结果：

```powershell
pnpm test:module -- plugin:idc-compliance --grep "TC-3" --timeout=120000
```

结果为`3/3`通过：

- `TC-3a`验证中文脱敏详情、短期单次原件访问、一次访问上下文只消费一次、开始审核、批准和服务端状态收敛。
- `TC-3b`验证英文队列显示真实英文文案，不出现内部翻译键或原始法定资料。
- `TC-3c`验证驳回必须提交稳定原因码，并显示客户可处理的驳回状态。

页面对象补充了路由加载提示、抽屉滑入和确认框关闭动画的稳定等待，避免截图在动画中途形成不可信证据。当前人工查看以下截图，未发现原始翻译键、敏感明文、遮挡、重叠或异常横向溢出：

- `temp/20260817/162023-enterprise-review-masked-detail-zh-cn.png`
- `temp/20260817/162024-enterprise-review-approved-zh-cn.png`
- `temp/20260817/162033-enterprise-review-queue-en-us.png`
- `temp/20260817/162043-enterprise-review-rejected-zh-cn.png`

## 其他验证

| 验证 | 结果 |
| --- | --- |
| `pnpm install --frozen-lockfile`（插件`frontend/`） | 通过；插件依赖可独立安装 |
| `pnpm run build`与`pnpm run build:check`（插件`frontend/`） | 通过；没有读取或修改`web-antd`依赖 |
| `pnpm exec tsc --noEmit -p hack/tests/tsconfig.json` | 通过 |
| `pnpm test:validate` | 通过；217 个 E2E 文件、17 个范围、编号与清单一致 |
| 企业认证服务与后台控制器真实 PostgreSQL 测试 | 通过；覆盖加密重放、审核、单次原件访问、重复主体、并发预约、内容一致性和独立权限 |
| `go vet ./internal/service/enterpriseverification ./internal/controller/admin` | 通过 |
| `make.cmd i18n.check` | 通过；运行时、消息和前端键覆盖均为零违规 |
| `make.cmd plugins.check` | 通过；2479 个插件文件扫描零发现 |
| `openspec validate add-idc-server-operations-platform --strict` | 通过 |
| `git status --short -- apps/lina-vben/apps/web-antd` | 空；没有 IDC 业务改动 |

仓库没有注册`make lint`命令，执行时明确返回未知命令。本任务直接变更范围不含 Go 生产或测试文件，因此没有把该命令误记为通过；相关后端只做当前工作区回归，并已由真实 PostgreSQL 测试和`go vet`覆盖。

## 影响分析

- 前端：有影响。范围只在合规插件后台页面和插件专属页面对象；没有修改`web-antd`。
- 接口与后端：无新增契约或运行时依赖；页面复用`10.3.c`已经完成的七个管理接口和后端逐次审计。
- 数据权限：有验证影响。列表、详情、原件、开始审核、批准和驳回继续由独立精确权限及后端直接员工、租户和数据范围门禁控制。
- `i18n`：有影响。中文、英文资源和浏览器实际文案均通过验证。
- 缓存一致性：无影响，没有新增或修改缓存。
- 开发工具与跨平台：无生产工具或脚本改动；共享登录页面对象只修复正常路径和 Hash 路由两种登录完成判断，测试 TypeScript 和清单校验通过。
- 文档：有影响。新增本验证记录并更新`tasks.md`精确子任务状态。
- 中央依赖：无影响。原件、审核和审计均在购买方当前安装内完成，不依赖软件提供方业务服务。

## Lina 审查报告

**变更：** `add-idc-server-operations-platform`

**范围：** 任务级`10.12.e`

**审查文件数：** 8 个直接实现与测试文件，另含本验证记录和`tasks.md`

**范围来源：** `git status --short`、`git ls-files --others --exclude-standard`、未跟踪合规插件目录展开结果和任务上下文

**插件本地规范：** `idc-compliance`根目录不存在额外`AGENTS.md`，适用项目顶级规范。

**已读取规则文件：** `AGENTS.md`、`.agents/rules/user-preferences.md`、`.agents/rules/openspec.md`、`.agents/rules/documentation.md`、`.agents/rules/architecture.md`、`.agents/rules/plugin.md`、`.agents/rules/frontend-ui.md`、`.agents/rules/testing.md`、`.agents/rules/i18n.md`和`.agents/rules/dev-tooling.md`。

### 发现的问题

- 已修复：共享登录页面对象只检查普通路径，遇到当前后台 Hash 路由时会把已经成功登录误判为仍在登录页；现同时接受普通路径与 Hash 路由，测试类型通过。
- 已修复：抽屉和确认框截图发生在 CSS 动画中途，不能稳定证明最终页面；页面对象现等待短视觉动画结束，最终`TC003`重新执行`3/3`通过并人工复核截图。
- 未发现阻塞问题或遗留警告。

### 规则域结论

- OpenSpec、插件边界、前端 UI、数据权限和测试：通过。
- `i18n`：通过，中英文页面和仓库键覆盖均已验证。
- E2E 质量审查：已触发；成功、英文、驳回、单次原件和服务端权威状态均有有效断言与截图。
- 缓存一致性：无影响。
- 接口、数据库和后端生产代码：本任务无新增或修改；当前权威服务回归通过。
- 开发工具：无生产工具改动；测试页面对象修改不改变平台运行逻辑。

### 摘要

- 严重：0
- 警告：0
- 结论：未发现阻塞问题，允许只勾选`10.12.e`。
