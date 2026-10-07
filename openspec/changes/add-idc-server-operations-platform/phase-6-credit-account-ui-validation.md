# 第六阶段信用账户前后台闭环验证记录

## 完成范围

`6.14.a`至`6.14.c`已经完成。客户在`@lina/web-customer`的“财务中心 → 信用账户”内可以提交申请、上传证明材料、查看申请/账户/账单、使用同币种余额还款，并能在待付款账单详情确认本次信用支付金额和支付后剩余额度。

后台授信页面完全属于`idc-core/frontend`，通过插件自己的`frontend/dist/mount.js`交付。页面支持查看申请材料、连续独立审批、信用账户、信用账单以及逾期和冻结状态。本任务没有向`apps/lina-vben/apps/web-antd`写入 IDC 页面、组件、接口、路由、状态、多语言或业务依赖。

## 材料与权限边界

- 客户申请材料只允许`PDF`、`JPG/JPEG`和`PNG`；单次申请最多`10`个文件，单文件不超过`20 MiB`，合计不超过`200 MiB`。
- 上传时同时检查扩展名、实际文件头、声明媒体类型、声明大小和实际大小，伪装文件失败关闭。
- 文件进入私有存储，只在数据库保存私有对象引用和摘要，不生成公共下载地址。
- 预留、上传、提交三个步骤都重新确认当前客户和上传人；重复提交只允许复用已经绑定到同一申请的材料，不能把材料改绑到另一份申请。
- 后台每次列出或打开材料都重新执行申请详情权限和数据范围检查，返回的是受控文件流，不回显对象存储路径。
- 高额度申请沿用两名不同审核人的顺序审批；前端只把审核意见和依据的摘要提交给证明与执行接口，证明和执行使用同一请求编号。

## 前端归属与交付

- 客户源码：`apps/lina-vben/apps/web-customer/src/views/account/credit/index.vue`、`src/api/idc/customer-credit.ts`和客户路由/双语资源。
- 后台源码：`apps/lina-plugins/idc-core/frontend/src/finance/FinanceWorkspace.vue`及同目录接口和双语资源。
- 插件入口按`/finance`装配授信工作区；生产包不含`web-antd`、`@vben/`或浏览器不可用的`process.env`引用。
- `web-antd`的业务源码和依赖差异为零。

## 浏览器验收

| 用例 | 覆盖结果 |
| --- | --- |
| 插件后台`TC011a` | 第二名独立审核人完成双人审批；证明与执行请求一致；明文审核依据不进入业务请求；中文关键截图通过 |
| 插件后台`TC011b` | 英文后台显示冻结账户、冻结原因和逾期账单；英文关键截图通过 |
| 客户端`TC014a` | 客户上传私有`PDF`并提交申请；请求不接受也不携带客户编号；中文关键截图通过 |
| 客户端`TC014b` | 余额还款证明和执行使用同一请求与金额，账单变为已还清 |
| 客户端`TC014c` | 信用支付二次确认同时显示本次金额和支付后剩余额度 |
| 客户端`TC014d` | 英文手机端显示冻结账户和逾期账单，无横向溢出；移动端关键截图通过 |

实际 Chromium 结果为插件后台`2/2`、客户后台`4/4`通过。

## 验证结果

| 门禁 | 结果 |
| --- | --- |
| 信用材料真实 PostgreSQL 测试`TestCreditEvidence*` | 通过，包含私有存储、伪装文件拒绝、同申请重放和跨申请改绑拒绝 |
| 信用迁移与中文接口文档定向测试 | 通过 |
| `GOWORK=off go test ./backend/... -p 1 -count=1` | 通过 |
| `make.cmd lint.go plugins=1 dir=apps/lina-plugins/idc-core/backend` | 通过，`golangci-lint`与`staticcheck U1000`均为`0`问题 |
| 插件前端`pnpm typecheck`与`pnpm build` | 通过，`mount.js`为`394.11 kB`，gzip为`87.68 kB` |
| 客户前端`pnpm typecheck`、`pnpm i18n:check`与生产构建 | 通过 |
| `make.cmd i18n.check plugins=1` | 通过，运行时文案、消息覆盖和前端词条均为`0`项违规 |
| 插件后台 Chromium | 通过，`2/2` |
| 客户后台 Chromium | 通过，`4/4` |
| `openspec validate add-idc-server-operations-platform --strict` | 通过 |

完整真实 PostgreSQL 全包测试曾继续运行到`contenthub`时，被该包的测试库保护规则拒绝使用`public`模式；这不是信用账户失败。信用材料真实 PostgreSQL、整个`billing`包真实 PostgreSQL以及迁移测试均已通过，全部后端包的无外部数据库回归也已通过。

## 审查结论

按反馈闭环和 OpenSpec 审查门禁复核后，未发现阻断项。材料没有公共地址，客户所有权和后台精确权限都在服务端执行；客户页面、插件后台页面和官方宿主边界正确；双语、桌面/移动端、真实数据库、构建产物和浏览器回归证据完整。
