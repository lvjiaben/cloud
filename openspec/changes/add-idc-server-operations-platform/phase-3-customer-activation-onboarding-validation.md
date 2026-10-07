# 第三阶段客户开户与激活收尾验证记录

## 验证范围

本记录收尾任务 3.4.c.3、3.8.b、3.8.c、3.9、3.10 和 3.11。范围只包括客户账户、后台开户与激活邀请、客户自行开户意图和对应浏览器回归，不代表第四阶段及以后任务完成。

## 已实现内容

- 后台只创建个人或企业客户资料，并发送 24 小时有效、一次使用的设密邀请；后台表单和接口都不接收客户明文密码。
- 同一待激活客户只保留一条当前有效邀请，重新发送会让旧邀请立即失效；已有身份绑定只接受完全匹配且没有员工、管理员或其他客户受众冲突的身份。
- 客户与员工受众转换必须由有权限的后台人员明确操作并写入审计，普通创建流程默认要求独立客户身份。
- 客户自行开户先完成人机校验、联系方式验证码、浏览器随机数和短期开户意图，再调用宿主原生注册或登录，最后通过一次性领取接口原子绑定客户受众。
- 开户状态查询和领取是认证后的窄例外；其他客户接口仍要求当前身份已经存在活动客户绑定。员工、超级管理员、篡改、过期、重复和并发领取都会失败关闭。
- 客户激活页清理地址栏令牌时只使用浏览器历史替换，不再触发组件二次挂载，避免成功状态被随后一次“无令牌”加载覆盖。
- 客户账户`TC012`验证地址栏伪造其他客户编号不会进入资料接口，页面只显示当前登录客户。

## 主要实现位置

- `apps/lina-plugins/idc-core/manifest/sql/047-customer-activation-invitation.sql`
- `apps/lina-plugins/idc-core/manifest/sql/048-customer-onboarding-intent-creation.sql`
- `apps/lina-plugins/idc-core/backend/internal/service/customeractivation/`
- `apps/lina-plugins/idc-core/backend/internal/service/onboarding/`
- `apps/lina-plugins/idc-core/backend/internal/service/principal/`
- `apps/lina-plugins/idc-core/frontend/pages/customer-management.vue`
- `apps/lina-plugins/idc-notify/backend/internal/service/notification/onboarding_verification.go`
- `apps/lina-vben/apps/web-customer/src/views/_core/authentication/customer-activation.vue`
- `apps/lina-vben/apps/web-customer/src/views/_core/authentication/customer-onboarding-complete.vue`
- `apps/lina-vben/apps/web-customer/tests/e2e/TC011-customer-onboarding.ts`
- `apps/lina-vben/apps/web-customer/tests/e2e/TC012-customer-account.ts`

`web-antd`没有新增或修改 IDC 页面、组件、接口、路由、状态或业务多语言资源；后台页面继续由`idc-core/frontend/`通过插件机制装配。

## 自动化验证

| 门禁 | 结果 |
| --- | --- |
| `GOWORK=off go test`：`idc-core`运行配置、激活、开户、身份投影、注销和插件启动包 | 6 个包通过 |
| `GOWORK=off go test`：`idc-notify`通知服务与插件启动包 | 2 个包通过 |
| `make.cmd lint.go dir=apps/lina-plugins/idc-core plugins=1` | 通过，0 个问题 |
| 全仓`make.cmd lint.go plugins=1`中的本次相关模块 | `idc-core`、`idc-notify`和此前模块全部通过；随后被无关、未跟踪的`tenant-core`三个`gofmt`问题阻断，未修改该目录 |
| 插件前端`vue-tsc --noEmit` | 通过 |
| `make.cmd i18n.check` | 通过，运行时硬编码违规 0，宿主与插件消息覆盖通过 |
| 标准 runner `TC001` | 6/6 通过 |
| 标准 runner `TC009` | 4/4 通过 |
| `web-customer`完整 Playwright 回归 | 46/46 通过，其中`TC011`3/3、`TC012`1/1 |

## 截图审查

- `customer-management-first-page`：客户筛选、表格、真实状态和实名标签完整可见，没有法定身份明文。
- `customer-management-overview`：客户 360 抽屉、合同规则和运营动作布局正常；客户编号、状态和协助查看标识清楚。
- `customer-account-ownership-zh-cn`：客户中心只显示当前登录客户的资料，菜单和表单没有横向溢出。
- `customer-management-filtered`在测试点击远程下拉后出现横向截图偏移，但数据行、筛选条件和操作仍可见，未发现业务数据丢失或按钮失效。该现象记为测试截图工具风险，不用来冒充业务完成证据。

## 审查结论

- 数据权限：客户资料接口不接受前端传入客户编号作为权威归属，后台权限与客户活动绑定分离；越权和不存在保持统一不可见。
- 性能：后台客户列表和 360 继续使用既有集合化装配与固定查询预算，本轮激活和开户写路径没有新增按返回行数循环查询。
- 双语：新增页面和错误提示均通过运行时硬编码及键覆盖检查。
- 缓存：本轮开户和激活权威状态保存在本地数据库，不新增缓存一致性路径。
- 外部证据：第三阶段收尾不依赖腾讯、诺诺或其他外部厂商确认。
