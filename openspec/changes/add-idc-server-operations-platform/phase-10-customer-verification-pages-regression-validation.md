# 第 10 阶段客户认证页面整组复核记录

## 记录范围

- 日期：2026-08-17
- 对应任务：`10.12.d`
- 本记录以当前共享工作区重新验证`@lina/web-customer`的个人实名认证、企业认证和企业合同签署经办人页面，不使用旧 Portal、历史构建或此前截图代替当前结果。
- 本任务只复核客户页面及其当前接口，不证明合规插件后台页面或父任务`10.12`完成。

## 当前客户入口

| 功能 | 当前路由 | 主要边界 |
| --- | --- | --- |
| 个人实名认证 | `/account/identity` | 单独同意后前往购买方自己的阿里云实名页；浏览器返回不等于通过，只有权威回查可以推进结果 |
| 企业认证 | `/account/enterprise-verification` | 法定资料和材料只进入当前安装的本地加密合规服务；浏览器只保存随机重试键，审核退回可修正后重提 |
| 合同签署经办人 | `/account/contracts/signers` | 经办人与通知联系人分离；本人实名和手机号都通过且未停用后，才成为新合同候选 |

三个入口均由`apps/lina-vben/apps/web-customer/src/router/routes/modules/account.ts`注册，页面、接口、状态和双语资源都在`web-customer`范围内。`web-antd`范围状态为空。

## 当前接口复核

### 个人实名认证

- 当前状态、单独同意、发起和权威重新检查均走合规插件`account`接口，请求不接受客户编号。
- 页面只在内存中短暂持有姓名和身份证号；真实供应商入口必须为安全地址，打开前断开新窗口反向引用。
- 返回网址中的成功样式参数不改变状态；付款恢复只依赖服务端权威认证结果。

### 企业认证

- 当前状态只返回安全企业投影、活动法定资料变更和当前客户案件的安全材料元数据。
- 浏览器分两步预约材料和上传内容，提交前先取得单独同意；法定明文、文件名、文件字节、材料编号和客户编号不写入浏览器存储。
- 审核退回显示通俗原因并保留可恢复流程；已认证企业变更只先声明变更类别，旧认证投影在新案件批准前继续有效。

### 企业合同签署经办人

- 客户单独添加经办人的本人姓名、证件和手机号，不复用普通通知联系人。
- 经办人本人实名与统一短信通道验证是两个独立步骤，两项都通过且经办人未停用后才显示“可用于新合同”。
- 短信发送结果不明时不自动再次发送，只提示客户主动取得新的人机验证题后重试。
- 停用只阻止经办人用于新合同，不改写已经冻结的合同签署人。

## 当前自动化验证

### 接口客户端、类型、双语与生产构建

以下命令在本轮当前工作区通过：

```powershell
pnpm exec vitest run src/api/idc/personal-verification.test.ts src/api/idc/enterprise-verification.test.ts src/api/idc/contract-signers.test.ts
pnpm exec tsc --noEmit -p apps/web-customer/tests/tsconfig.json
pnpm run check:alignment
pnpm run i18n:check
pnpm run typecheck
pnpm run build
```

结果：三个接口客户端文件共`5/5`通过；测试类型、Vben 母版对齐、双语键、应用类型和生产构建通过。生产构建成功生成`dist`和`dist.zip`，只有 Vite 已有的静态与动态导入重复提示。

全仓`make.cmd i18n.check`为零项违规，`openspec validate add-idc-server-operations-platform --strict`通过。

### 真实 PostgreSQL 与后端静态检查

以下命令在本轮当前工作区通过：

```powershell
$env:LINAPRO_IDC_COMPLIANCE_TEST_DSN='<local-test-postgresql-dsn>'
$env:GOWORK='off'
go test ./backend/internal/service/personalverification ./backend/internal/service/enterpriseverification ./backend/internal/service/electroniccontractpreparation ./backend/internal/controller/account -count=1
go vet ./backend/internal/service/personalverification ./backend/internal/service/enterpriseverification ./backend/internal/service/electroniccontractpreparation ./backend/internal/controller/account
```

真实数据库回归覆盖个人实名发起、权威通过、未通过与本地过期、结果未知不重放、配置改变转人工；企业材料加密、精确重放、人工审核、单次原件访问、重复主体、并发预约和内容一致性；合同签署经办人候选、冻结快照、统一短信和客户归属。

### 真实 Chrome

以下命令在本轮当前工作区通过：

```powershell
pnpm exec playwright test --config apps/web-customer/tests/playwright.config.ts apps/web-customer/tests/e2e/TC003-contract-signers.ts apps/web-customer/tests/e2e/TC004-personal-verification.ts apps/web-customer/tests/e2e/TC005-enterprise-verification.ts
```

结果：`10/10`通过。

- `TC003`覆盖经办人添加、本人实名、统一短信验证、成为候选，以及短信结果不明不自动重发。
- `TC004`覆盖单独同意、权威回查后恢复付款、伪造浏览器返回参数不生效、非安全供应商入口关闭、身份明文不进浏览器存储和英文手机页面。
- `TC005`覆盖单独同意、加密材料上传、本地人工审核、退回修正、认证后变更先建变更申请、旧投影继续有效和英文手机页面。
- 本轮人工查看`temp/20260817/154624-customer-contract-signer-verified.png`、`temp/20260817/154642-customer-personal-verification-mobile-consent-en-us.png`、`temp/20260817/154647-customer-enterprise-verification-rejected-zh-cn.png`和`temp/20260817/154653-customer-enterprise-verification-mobile-en-us.png`，未发现原始翻译键、遮挡、元素重叠、敏感明文泄露或手机横向溢出。

## 影响判断

- 代码：本任务是对当前实现的整组复核，没有新增业务代码、数据库迁移、接口或运行时依赖。
- 前端：有验证影响，范围只包括`@lina/web-customer`三个客户入口；没有修改`web-antd`。
- 数据权限：有验证影响，三套服务均从认证上下文取得当前客户，浏览器不能选择或提交客户编号。
- 多语言：有验证影响，中英文桌面与手机文案、状态、按钮和错误提示均通过当前检查。
- 缓存：无影响，没有新增或修改缓存。
- 开发工具：无影响，没有修改 Makefile、脚本、CI 或代码生成配置。
- 中央依赖：无影响，实名、企业材料、经办人和短信均使用当前购买方安装实例的本地服务与本地配置，不依赖软件提供方业务服务。

## `lina-review`结论

- 三个客户入口的当前接口、双语、生产构建、真实数据库和真实 Chrome 均已重新验证，证据不依赖旧 Portal 或历史结果。
- 页面不会用浏览器返回、按钮状态或本地存储代替服务端权威结果，敏感数据也没有写入浏览器持久化存储。
- E2E 可单独运行，覆盖成功、失败、结果不明、不安全跳转、退回修正和手机布局等关键路径。
- 本任务范围没有未解决的严重或一般问题，允许只勾选`10.12.d`；父任务`10.12`和后续子任务保持未勾选。
