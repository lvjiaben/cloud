# 第十九阶段第二轮真实体验与回归记录

日期：2026-09-01  
对应任务：`FB-59`、`FB-60`

## 回归范围

第二轮把第一轮发现的本地问题逐项对照修复，并从普通客户和后台员工的实际操作路径复核中文、英文、桌面、移动、空数据、慢加载、异常返回、固定操作列和插件运行时隔离。外部供应商仍不具备账号、正式契约或费用授权，因此没有把外部项目伪装成本地通过。

## 第一轮问题收口

| 问题 | 修复 | 验证 |
| --- | --- | --- |
| Windows 文件对象键使用反斜杠，和公开 URL 规范化结果不一致 | 文件服务使用 URL 语义的正斜杠拼接，并增加跨平台回归测试 | `apps/lina-core/internal/service/file/file_upload.go:247`、`file_upload_test.go:32`；动态文件管理删除/下载流程通过 |
| 动态文件管理删除测试只按响应体猜接口，Windows/插件路由下容易误判 | 测试按请求 URL 和 HTTP 方法识别真实删除请求，并保留 `200` 断言 | `hack/tests/e2e/settings/file/TC001-file-management.ts:350` |
| 动态插件运行时存储目录固定写死，宿主模式与隔离模式可能互相污染 | E2E runner 根据 host-only 模式选择 `temp/hostonly-output` 或 `temp/output`，八个动态用例统一读取该路径 | `hack/tests/scripts/run-suite.mjs:55-64`；动态目标回归 `8 passed`，生命周期边界 `3 passed` |
| 通知插件翻译结构测试绑定单引号格式，代码改用双引号时误报 | 翻译调用校验改为只关心键是否被调用，不绑定无意义的引号格式 | `apps/lina-plugins/idc-notify/frontend_delivery_test.go:90`；全量 Go 测试通过 |
| 资源渠道非秘密配置字段缺少中文 API 说明 | 为创建/更新 DTO 补齐中文 API 字段说明 | `apps/lina-plugins/idc-resource/manifest/i18n/zh-CN/apidoc/generated-coverage.json:35,38`；API i18n 检查通过 |
| E2E 默认地址仍指向已废弃的 `9120` | 测试默认浏览器入口改为管理后台 `6031`，接口默认入口改为 `6030`，仍允许环境变量覆盖 | `hack/tests/playwright.config.ts:27`、`hack/tests/fixtures/config.ts:17-18`；不设置环境变量的 `plugin:idc-ai` 回归 `10 passed` |

## 真实浏览器结果

- `web-customer` 全量 Playwright：`106 passed, 1 skipped`，共 107 个用例。覆盖登录、概览、产品与服务、账户、安全、财务、合规、工单、服务器、插件页面、中文/英文、桌面/移动和权限失败路径。唯一跳过是既定真实供应商条件，不是本地失败。
- 默认官网 Playwright：`27 passed`。首页、商品目录/详情、新闻、公告、知识库、下载、帮助、页头页脚、中英文和移动布局均通过。
- `plugin:idc-ai` 在当前规范端口环境（后台 `6031`、接口 `6030`）重新执行：`10 passed`，包括只读助手、客户上下文、危险操作拒绝、证据时效和插件页面主题圆角。
- 动态插件 host-only 隔离回归：运行时 Hook、热升级、低优先级宿主服务共 `8 passed`；生命周期边界共 `3 passed`，包含低版本替换拒绝和高版本保留。

## 体验与性能复核

- 中文和英文页面均未出现原始翻译键、中文残留、研发占位文案或接口原始错误；客户移动端没有横向溢出，官网移动网格能够自然换行。
- 客户中心所有业务表格使用 Vben/VxeGrid，有界高度、内部滚动和固定最右操作列；未发现 `watch` 中直接 `grid.loadData`。保留的 `height: auto` 只用于响应式图片和快捷入口，不承载列表数据。
- 客户中心业务源码没有原生 `table/form/input/button/select/textarea`；默认官网源码没有 `backdrop-filter`、渐变或独立 `font-family` 覆盖。
- 客户、订单、服务、工单、运营和渠道列表继续使用分页/游标、投影、批量装配和已有索引；阶段 5 的结算 benchmark（`p95≤2s`、`p99≤5s`）以及阶段 2、6、8、13、18 的索引与无 `N+1` 证据继续作为性能基线。第二轮没有新增无法量化的“优化完成”宣称。

## 结论

第一轮发现的本地可实现问题已关闭，第二轮真实浏览器和异常路径回归没有发现新的本地阻断项；`FB-59`、`FB-60`具备勾选依据。腾讯电子签、诺诺和真实供应商证书/主机/回调/费用授权继续保持暂停。
