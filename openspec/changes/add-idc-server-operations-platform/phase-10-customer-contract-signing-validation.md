# 第 10 阶段客户合同签署验证记录

## 记录范围

- 日期：2026-08-17
- 对应任务：`10.12.c`
- 本记录证明`@lina/web-customer`已经接通当前客户“我的合同”列表、腾讯`H5`签署入口、浏览器返回提示、权威重新检查和原合同原流程的安全重试。
- 本记录不证明实名页面整组复核、合规插件后台页面或父任务`10.12`完成；这些仍属于`10.12.d`至`10.12.h`。
- 腾讯由每套安装的购买方使用自己的企业账号和本地合规网关接入，本任务没有引入软件提供方中央网关、中央结算或业务回调。

## 客户流程与安全边界

1. 客户只能读取当前登录身份拥有的合同，浏览器请求不能指定客户编号。
2. 列表只显示“处理中”“已完成”“失败”三个客户状态，以及冻结的合同编号、订单编号、模板名、金额、语言、创建时间和固定七天签署截止时间。
3. 每次点击签署入口时，页面先同步打开空白新窗口，再向服务端为原合同、原腾讯流程和匹配签署人申请新的三十分钟`H5`入口，避免异步请求后被浏览器误判为弹窗。
4. 页面只接受`HTTPS`入口，并在跳转前断开新窗口的`opener`；服务端未返回安全地址时立即关闭空白窗口并显示通俗提示。
5. 从腾讯页面返回只显示“请重新检查”的提醒，不会把网址参数、页面本地时间或浏览器跳转当作签署成功。
6. “重新检查签署结果”只调用服务端权威查询；页面仅用返回的最新安全投影替换对应合同。
7. 处理中且后端明确允许时，可以再次取得原流程的新短期入口；不会创建第二份合同、重新发起腾讯流程或延长固定七天期限。
8. 已拒签、撤销、终态失败或过期的合同由后端投影为失败且不给签署或重新检查动作，客户不能自行重发。

## 页面与文件归属

- 客户页面：`apps/lina-vben/apps/web-customer/src/views/account/contracts/index.vue`
- 客户接口：`apps/lina-vben/apps/web-customer/src/api/idc/customer-contracts.ts`
- 客户路由：`/account/contracts`
- 中英文资源：`apps/lina-vben/apps/web-customer/src/locales/langs/{zh-CN,en-US}/contracts.json`
- 浏览器回归：`apps/lina-vben/apps/web-customer/tests/e2e/TC008-customer-contracts.ts`
- 页面对象与接口替身：`apps/lina-vben/apps/web-customer/tests/pages/ContractsPage.ts`、`apps/lina-vben/apps/web-customer/tests/support/customer-contract-api-mock.ts`
- `web-antd`范围状态为空；本任务没有向官方管理后台宿主写入 IDC 页面、接口、路由、状态、双语资源或依赖。

## 自动化验证

### 后端当前接口与真实 PostgreSQL

以下验证通过：

```powershell
$env:LINAPRO_IDC_COMPLIANCE_TEST_DSN='<local-test-postgresql-dsn>'
$env:GOWORK='off'
go test ./backend/internal/controller/account ./backend/internal/service/electroniccontractpreparation -count=1
go vet ./backend/internal/controller/account ./backend/internal/service/electroniccontractpreparation
```

真实 PostgreSQL 用例覆盖当前客户归属过滤、稳定游标、签署人匹配、原流程短期入口、过期和终态失败关闭、只读权威回查及安全状态收敛。控制器测试确认请求不接受客户编号，并分别调用列表、生成入口和权威重新检查动作。

### 前端与真实 Chrome

以下验证通过：

```powershell
pnpm exec vitest run src/api/idc/customer-contracts.test.ts
pnpm run check:alignment
pnpm run i18n:check
pnpm run typecheck
pnpm exec tsc --noEmit -p apps/web-customer/tests/tsconfig.json
pnpm run build
pnpm exec playwright test --config apps/web-customer/tests/playwright.config.ts apps/web-customer/tests/e2e/TC008-customer-contracts.ts
```

结果：接口客户端`3/3`通过；Vben 母版对齐、双语键、应用类型、测试类型和生产构建通过；真实 Chrome 用例`4/4`通过。

- `TC008a`：中文桌面端确认浏览器返回提示不会改变处理中状态，打开腾讯`H5`入口后，只有点击权威重新检查并收到服务端完成结果才显示“已完成”。
- `TC008b`：同一处理中合同连续两次只取得原合同的新短期入口；终态失败合同没有客户签署或重新检查按钮。
- `TC008c`：非`HTTPS`签署地址会关闭空白窗口并明确提示，不会打开不安全外部页面。
- `TC008d`：英文手机端显示完整英文文案，没有原始翻译键和横向溢出。
- 人工查看截图：`temp/20260817/154136-customer-contract-completed-zh-cn.png`和`temp/20260817/154141-customer-contracts-mobile-en-us.png`，未发现文案未翻译、遮挡、元素重叠或手机横向溢出。

生产构建成功生成`dist`和`dist.zip`，只出现 Vite 已有的静态与动态导入重复提示。全仓`make.cmd i18n.check`为零项违规，`openspec validate add-idc-server-operations-platform --strict`通过。

## 审查修复

1. 页面原先在请求层抛出非标准错误时没有稳定的客户提示，权威重新检查失败也只依赖全局请求提示；现已补充中英文失败提示，旧合同状态保持不变，允许客户稍后主动重试。
2. 合同卡片原先没有按合同编号稳定定位的测试标记；现已增加卡片级标记，浏览器测试不会依赖偶然顺序或重复按钮。
3. 首轮测试替身错误地组合了“终态失败”和“允许重试”两个真实后端不会同时返回的值。审查时已按后端权威投影修正：处理中合同允许取得新入口，终态失败合同不提供动作，并重新执行`TC008`通过。

## 影响判断

- 前端：有影响，只修改`@lina/web-customer`合同页面、双语资源和测试；没有修改`web-antd`。
- HTTP API：无新增或修改；接线使用`10.8.f`已经冻结并验证的当前客户合同接口。
- 数据权限：无新实现；重新验证服务端只从认证上下文取得当前客户，并在查询中固定客户归属。
- 数据库：无迁移；真实 PostgreSQL 回归复用既有合同、流程和签署快照表。
- 多语言：有影响，新增签署入口和重新检查失败提示；应用与全仓双语检查通过。
- 缓存：无影响，没有新增或修改缓存。
- 运行时依赖与依赖注入：无影响，没有新增后端依赖、服务构造或启动装配。
- 开发工具：无影响，没有修改 Makefile、脚本、CI 或代码生成配置。

## `lina-review`结论

- 页面只消费服务端安全状态和动作开关，不在浏览器推导客户权限、合同终态或签署期限。
- 短期签署地址不进入本地存储、页面网址或持久化状态；页面拒绝非`HTTPS`地址并断开新窗口反向引用。
- 返回提示、重新检查和安全重试语义与增量规范一致；真实测试明确证明终态失败不能由客户重发。
- E2E 可单独运行且不依赖其他用例残留，覆盖中文桌面、英文手机、成功路径、重复取入口和不安全地址异常路径。
- 本任务范围没有未解决的严重或一般问题，允许只勾选`10.12.c`；父任务`10.12`和后续子任务保持未勾选。
