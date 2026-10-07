# 第 10 阶段腾讯合同模板对应关系验证记录

## 对应任务

- 任务：`10.8.d.1`
- 日期：`2026-08-16`
- 范围：只完成腾讯同步模板版本的安全填写位置、签署角色投影、管理员明确对应、启用前完整性检查和模板版本变化失效。
- 不包含：真正发起腾讯签署、签署链接、外部流程号和结果未知保护；这些仍属于未完成的 `10.8.d.2`。

## 根因与修正

首次用真实 PostgreSQL 执行迁移时，原始长表名超过 PostgreSQL 的 63 字节标识符上限，`recipient` 和 `recipient_mapping` 两张表被截断成同一个名字。实现已改用仍带完整插件标识、但不超过上限的五个表名，并新增真实 PostgreSQL 表名长度测试，避免只在内存测试中通过。

完整后端测试还发现 `backend/internal/controller/account/account_new.go` 末尾残留了一份空的 `ControllerV1` 和 `NewV1` 生成壳，和文件前面的正式身份核验控制器重复。只删除了这份空壳，正式控制器、依赖和返回地址检查全部保留。

## 已实现内容

### 本地合规网关安全投影

- 只向插件后端返回腾讯模板的安全字段：不透明位置标识、显示名称、位置类型、是否必填、所属签署角色、角色类型、角色显示名、签署顺序、是否发起方和签署位置数量。
- 普通填写位置、签署位置、签署角色均有单模板上限；普通位置与签署位置合计也不得超过 `512`，同时保留整次同步总量上限。
- 拒绝重复位置、重复角色、未知角色引用、不支持的角色类型、缺少签署顺序所需数据和畸形供应商响应。
- 安全投影按不透明标识排序，后端不按腾讯显示名称或返回顺序猜测对应关系。

### 版本保存与失效

- 每次完整同步把腾讯填写位置和签署角色保存为不可变版本投影。
- 版本摘要同时包含模板状态、填写位置和签署角色。腾讯返回内容发生变化时创建新本地版本，不覆盖旧版本。
- 新版本出现后，旧的已启用版本自动停用；对应关系只指向准确的本地模板版本，不能跨版本沿用。

### 管理员明确对应与完整性检查

- 新增管理员接口：`POST /admin/compliance/electronic-contract/templates/{templateNo}/versions/{versionNo}/provider-mappings`。
- 精确权限：`idc:admin:compliance:contract-template:manage`。
- 页面只能从固定 LinaPro 合同内容中选择来源，不能输入或修改订单、客户、金额等权威数据。
- 服务明细无论一项还是多项都只对应一个腾讯动态表格位置；缺少该表格、出现多个服务明细表或位置类型不兼容时拒绝保存。
- 所有腾讯必填位置必须完成对应；必须同时明确客户方和商家方签署角色。
- 腾讯固定个人角色只能用于个人模板，固定企业角色只能用于企业模板；当前腾讯安全投影没有证明一个角色同时支持两类客户，因此同步模板不能设置为“全部客户”。
- 本地签署顺序必须与腾讯权威签署顺序一致；缺失或冲突时拒绝保存。
- 保存后的对应关系不可修改。模板启用前再次检查供应商状态、准确版本的对应关系指针、双方签署位置和必需模板设置。后续 `10.8.d.2` 的提交签署命令必须复用同一准确版本的不可变对应关系，不能另建宽松判断。

### 插件前端与双语

- 管理页面只位于 `apps/lina-plugins/idc-compliance/frontend/`，通过现有源码插件机制加载。
- `apps/lina-vben/apps/web-antd/` 没有新增或修改任何 IDC 页面、组件、接口、路由、状态、多语言或业务依赖。
- 管理员必须逐项选择填写内容来源和客户方/商家方；未完成全部门禁前按钮不可点击。
- 页面明确提示这里只选择可信内容来源，不能修改订单真实数据。
- 中文和英文运行时文案均已补齐；新 API 字段和命令的简体中文接口文档翻译也已补齐，并把新接口加入自动覆盖测试。

## 数据、性能和安全审查

- 数据范围：合同模板配置属于单套安装的全局合规配置，使用固定安装租户 `0`，不读取其他安装或中央平台数据。
- 权限：查询和管理权限分离；保存对应关系只接受精确模板版本和精确管理权限，不会因为能查看模板而获得修改权。
- 写入：对应关系集合、字段对应和角色对应在同一事务中追加保存，带乐观锁、幂等键、命令指纹和业务审计。
- 防篡改：不可变投影和不可变对应表均有数据库更新、删除阻断触发器；不使用软删除伪装法律证据变更。
- 查询次数：编辑器读取对应关系使用固定的两次集合查询，保存使用集合装配和批量写入，没有随着页面行数逐行查库的 `N+1` 问题。
- 缓存：本任务未使用缓存，没有缓存一致性影响。
- 中央依赖：腾讯调用只发生在购买方每套安装自己的本地合规网关，不存在中央业务网关、中央结算、平台销售注册或软件提供方业务回调。

## 验证结果

以下命令均在共享工作区当前代码上执行：

- `openspec validate add-idc-server-operations-platform --strict`：通过。
- `cmd /c make.cmd i18n.check`：通过，运行时文案、消息和前端键覆盖均无问题。
- `cmd /c make.cmd plugins.check`：通过，扫描 `2268` 个文件，发现 `0` 个治理问题。
- 插件根目录执行 `GOWORK=off go test . ./backend/... -count=1`：通过。
- 插件根目录执行 `GOWORK=off go vet . ./backend/...`：通过。
- 对本任务涉及的插件后端包执行 `golangci-lint` 和 `staticcheck -checks=U1000 -tests=false`：`0 issues`。
- 本地合规网关执行 `GOWORK=off go test ./... -count=1` 和 `go vet ./...`：通过。
- 本地合规网关执行 `golangci-lint` 和 `staticcheck -checks=U1000 -tests=false ./...`：`0 issues`。
- 真实 PostgreSQL 执行 `TestComplianceMigrationIdentifiersFitPostgreSQL`、`TestComplianceFoundationMigrationPostgreSQL`：通过。
- 真实 PostgreSQL 执行 `TestElectronicContractTemplatePostgreSQLWorkflow`：通过，覆盖保存对应关系、启用准备状态、相同快照不增版、腾讯快照变化追加新版本和旧对应关系保持不变。
- 插件前端执行 `pnpm run build`、`pnpm run build:check`：通过。
- E2E TypeScript 执行 `pnpm exec tsc --noEmit -p ../../apps/lina-plugins/idc-compliance/hack/tests/tsconfig.json`：通过。
- `LINAPRO_SOURCE_PLUGINS=1` 下从 `apps/lina-vben/apps/web-antd` 执行 `pnpm run build`：通过，共转换 `8891` 个模块，证明宿主能直接装配插件源码；`web-antd` 源码差异仍为空。
- 使用真实 Chrome 执行 `TC-4b`：`9 passed`，覆盖只读权限、中文亮色、英文暗色、本地模板编辑、复制、启用和腾讯对应流程。

全仓 `cmd /c make.cmd lint.go` 已实际执行，但被本任务范围外的宿主文件 `hack/tools/linactl/internal/process/process_windows.go:52` 挡住：`syscall.CloseHandle` 返回值未检查。根据当前宿主边界，本任务没有修改 `hack/` 来绕过该问题；合规插件本次涉及包和本地合规网关的独立静态检查均为 `0 issues`。此外，插件全模块 lint 还报告了既有 `electroniccontractpreparation.prepare` 复杂度 `44` 超过 `35`，该函数不属于模板对应关系，本任务未顺带重写合同准备业务。

## 审查结论

- 本任务范围内未发现阻止交付的高风险或中风险问题。
- API、数据库、权限、双语、插件边界、性能、不可变版本和浏览器回归均有对应验证。
- `10.8.d.1` 可以单独标记完成。
- `10.8.d` 父任务和 `10.8.d.2` 仍未完成，不得因为本记录勾选。
