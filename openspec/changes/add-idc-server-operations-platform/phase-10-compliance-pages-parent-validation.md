# 第 10 阶段合规后台与客户页面整组收口验证记录

## 记录范围

- 日期：2026-08-18。
- 对应任务：`10.12.h`和`10.12`父任务。
- 本记录只收口第 10 阶段的合规后台与客户页面组，不代表第 10 阶段全部完成。
- `10.13`至`10.19`仍按`tasks.md`顺序独立实施；已有同名或相邻 E2E 文件不能提前代替这些任务的精确子场景。

## 子任务完成关系

| 子任务 | 已完成范围 | 独立证据 |
|---|---|---|
| `10.12.a` | 客户发票抬头、安全银行账号和`web-customer`页面 | [客户发票抬头验证记录](phase-10-customer-invoice-profile-validation.md) |
| `10.12.b` | 当前客户可开票选项、申请、历史和`web-customer`页面 | [客户开票申请验证记录](phase-10-customer-tax-invoice-validation.md) |
| `10.12.c` | “我的合同”、腾讯签署入口、权威检查和安全重试 | [客户合同签署验证记录](phase-10-customer-contract-signing-validation.md) |
| `10.12.d` | 个人实名、企业实名和合同签署经办人页面整组复核 | [客户认证页面整组复核记录](phase-10-customer-verification-pages-regression-validation.md) |
| `10.12.e` | 合规待办、企业材料脱敏详情和单次原件查看审计 | [企业认证后台页面验证记录](phase-10-enterprise-verification-admin-page-validation.md) |
| `10.12.f` | 合同列表、详情、发起、提交、重发、解除和文件证据 | [电子合同管理页面验证记录](phase-10-electronic-contract-management-page-validation.md) |
| `10.12.g` | 发票审核、提交、取消、红字分配、确认状态和凭证 | [电子发票与红字后台页面验证记录](phase-10-tax-invoice-management-page-validation.md) |

## 前端边界与装载

- Lina 官方管理宿主`apps/lina-vben/apps/web-antd/`没有 IDC 页面、组件、接口客户端、路由、状态、多语言资源或业务依赖改动。
- 合规后台页面全部由`apps/lina-plugins/idc-compliance/frontend/`拥有，插件独立构建后由现有源码插件前端机制装载。
- 登录后的客户页面全部位于`apps/lina-vben/apps/web-customer/`，没有写入旧 Portal，也没有进入官网目录。
- `web-customer`的依赖清单与`web-antd`当前母版依赖清单完全一致；结构对齐脚本通过，没有恢复 Nuxt 或另造客户基础设施。
- `web-customer`继续使用固定`6677`端口，管理后台本轮真实验证使用`6670`测试服务；两者页面、路由和偏好存储保持隔离。

## 类型、构建、双语与单元测试

以下命令在当前共享工作区执行并通过：

```powershell
pnpm --filter @lina/web-customer run check:alignment
pnpm --filter @lina/web-customer run i18n:check
pnpm --filter @lina/web-customer run typecheck
pnpm --filter @lina/web-customer run build
pnpm exec vitest run --dom apps/web-customer/src

pnpm -C apps/lina-plugins/idc-compliance/frontend run build
pnpm -C apps/lina-plugins/idc-compliance/frontend run build:check
pnpm -C hack/tests exec tsc --noEmit -p ../../apps/lina-plugins/idc-compliance/hack/tests/tsconfig.json

.\make.cmd i18n.check
.\make.cmd plugins.check
node hack/tests/scripts/validate-e2e.mjs
```

结果：

- `web-customer`母版对齐通过。
- `web-customer`两种语言共`12`份页面语言资源检查通过。
- `web-customer`类型检查和生产构建通过，并生成`dist/`与`dist.zip`交付产物。
- `web-customer`单元测试为`24`个文件、`112`条测试全部通过。
- 合规插件独立构建和产物复核通过。
- 合规插件 E2E TypeScript 检查通过。
- 全仓运行时双语检查为`0`项违规；插件治理扫描`2492`个文件并为`0`项问题；E2E 治理检查通过，共检查`218`个文件。
- Vite 构建仅输出已有静态和动态导入落入同一代码块的优化提醒，不影响类型、构建产物或页面运行，也没有要求修改`web-antd`。

## 权限与数据隔离

- 企业材料页面按查看、开始审核、批准、驳回和单次原件查看权限分别控制入口；原件获取仍由服务端逐次授权并审计。
- 合同管理按查看、人工发起、重新发起、解除、模板管理和自动签授权等独立权限控制，页面隐藏不能替代服务端校验。
- 发票管理按查看、审核、提交和取消四项独立权限控制；红字任务按查看和处理两项权限控制。
- 客户实名、企业认证、签署经办人、合同、发票抬头和开票接口都从当前登录身份取得客户主体，不接受浏览器自报客户编号作为权威归属。
- 客户查询在 PostgreSQL 条件中同时固定默认安装实例和当前客户编号；详情、动作和重试再次核对当前客户归属。其他客户、无权访问和不存在的目标不会通过安全投影泄漏业务详情。
- 合规插件全量 Go 测试已使用专用 PostgreSQL 连接执行并通过；覆盖客户隔离、不可变归属、权限、幂等、并发、秘密投影和审计失败回滚。
- 列表使用有上限的游标分页，详情使用固定数量、有上限的关联查询，没有页面逐行请求或数据库`N+1`装配。

## 真实浏览器验证

### 客户页面

使用`apps/lina-vben/apps/web-customer/tests/playwright.config.ts`执行`TC003`至`TC008`：

```powershell
pnpm exec playwright test tests/e2e/TC003-contract-signers.ts tests/e2e/TC004-personal-verification.ts tests/e2e/TC005-enterprise-verification.ts tests/e2e/TC006-invoice-profiles.ts tests/e2e/TC007-tax-invoices.ts tests/e2e/TC008-customer-contracts.ts --config=tests/playwright.config.ts --workers=1
```

结果为`18 passed`，覆盖：

- 企业合同签署经办人本人实名、统一短信验证和发送结果不明时不自动重发。
- 个人实名的单独同意、权威回查、浏览器伪造拒绝、安全跳转关闭和英文手机页面。
- 企业认证的草稿、提交、驳回修正、认证后变更和英文手机恢复。
- 发票抬头新增、修改、停用、银行账号脱敏和英文手机页面。
- 部分与合并开票、待后台确认票种和英文手机页面。
- 我的合同、浏览器返回不冒充成功、权威检查、安全重试、非`HTTPS`关闭和英文手机页面。

### 插件后台页面

使用管理前端`http://127.0.0.1:6670`和后端`http://127.0.0.1:9999`执行合规插件`TC003`至`TC005`：

```powershell
pnpm -C hack/tests exec playwright test apps/lina-plugins/idc-compliance/hack/tests/e2e/TC003-enterprise-kyc.ts apps/lina-plugins/idc-compliance/hack/tests/e2e/TC004-electronic-contract.ts apps/lina-plugins/idc-compliance/hack/tests/e2e/TC005-tax-invoice.ts --config=playwright.config.ts --workers=1
```

结果为`31 passed`，覆盖企业认证后台、合同开通与管理、插件自有合同编辑器、自动签、发票审核、红字处理、权限隔离、中文、英文、暗色和英文手机页面。

新增的`TC-5k`明确证明电子发票与红字页面已经由插件机制在后台装载，英文文案正确，`390 × 844`手机视口没有整页横向溢出。该测试不代替`10.18`要求的`TC-5a`至`TC-5d`。

## 截图人工检查

本轮重新检查了以下关键截图：

- `temp/20260817/181723-tax-invoice-management-mobile-en-us.png`。
- `temp/20260817/181500-customer-contracts-mobile-en-us.png`。
- `temp/20260817/181450-customer-tax-invoices-mobile-en-us.png`。
- `temp/20260817/181445-customer-invoice-profiles-mobile-en-us.png`。
- `temp/20260817/181437-customer-enterprise-verification-mobile-en-us.png`。
- `temp/20260817/181425-customer-personal-verification-mobile-en-us.png`。
- `temp/20260817/181407-customer-contract-signer-verified.png`。

检查结果：中英文界面均未显示原始翻译键；手机页面没有整页横向滚动、遮挡主要操作或无法点击控件；桌面客户导航、状态卡片和成功提示正常；页面保持 Vben 与 Ant Design Vue 的统一视觉。

## Lina 任务级审查

- 审查范围来自`git status --short`、未跟踪目录展开、`10.12.a`至`10.12.h`的全部任务证据、插件与客户页面源码、后端当前客户过滤和本轮自动化结果，不只依赖普通`git diff`。
- 审查确认`web-antd`没有 IDC 业务改动；`git status --short -- apps/lina-vben/apps/web-antd`为空。
- 审查确认没有新增中央业务网关、中央结算、平台销售注册、平台操作者或软件提供方业务服务依赖；每套安装仍只使用本地合规网关和购买方自己的供应商账号。
- `i18n`：有影响，插件后台和客户页面的中英文资源、接口文档映射及真实页面均已验证。
- 数据权限：有影响，客户页面按当前客户主体隔离，后台页面按精确权限和数据范围处理。
- 缓存：无新增影响，合规状态、权限和客户归属继续从权威服务与数据库读取。
- 开发工具跨平台：无源码影响，本轮只使用现有 Go、Node、Playwright 和 Windows 命令入口。
- E2E 质量审查：已触发。测试覆盖成功、权限拒绝、外部能力关闭、结果不明、浏览器伪造、移动端和双语路径，并断言请求载荷、权威状态或数据不泄漏，不只检查元素存在。
- 严重问题：`0`。
- 本任务范围警告：`0`。

整仓`.\make.cmd lint.go`已实际执行。本任务所属`idc-compliance`为`0 issues`，但整仓继续运行时被`tenant-core/backend/api/auth/v1/`下三个不属于本任务的既有未格式化文件阻断。为避免覆盖共享工作区其他未提交成果，本轮没有修改它们；合规插件正式独立 lint 已再次通过。

## 收口结论

- `10.12.a`至`10.12.h`均已实现并具备当前验证证据，可以勾选`10.12.h`和`10.12`父任务。
- 不能据此勾选`10.13`至`10.19`，也不能把既有浏览器回归误作真实腾讯、诺诺外部能力已经开放。
