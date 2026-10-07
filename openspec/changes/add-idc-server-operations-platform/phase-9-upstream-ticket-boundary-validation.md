# 上游工单协议边界验证记录

## 结论

智简魔方公有云管理员`v1`冻结契约没有工单端点。`idc-zjmfcloud`因此把`server.upstream-ticket`固定声明为`unsupported`，执行该能力直接返回不支持且不发起登录或业务网络请求。

旧魔方财务系统虽然提供上游工单转发，但它使用另一套财务/资源渠道协议。该能力不能作为魔方云管理员`v1`生命周期端点实现，也不能复用后者的认证、客户编号、实例编号或任务语义。实际转发继续由工单域任务`11.4`拥有。

## 源码证据

用户提供的旧魔方财务完整源码给出了以下精确调用：

- 后台读取上游部门时从`zjmf_finance_api`取得独立地址和账号，经`POST /zjmf_api_login`换取 Bearer JWT，再调用`GET /ticket/department`。
- 首次转发按渠道类型选择`POST /resource_login`并携带`type=agent`，或选择`POST /zjmf_api_login`；附件独立上传到`POST /upload_image`。
- 工单创建调用`POST /ticket/create`，字段为`dptid/hostid/title/content/attachment/priority/is_api`；其中`hostid`来自本地服务保存的上游`dcimid`，成功后只把响应`data.tid`保存为`upstream_tid`。
- 回复调用`POST /ticket/reply`，字段为`tid/content/attachment/is_api`。

这些调用位于旧财务系统的`app/admin/controller/CommonController.php`上游部门逻辑，以及`app/common.php`的`ticketDeliver`和`ticketReplyDeliver`逻辑。智简魔方云旧资源连接器`app/common/logic/DcimCloud.php`没有提供对应管理员`v1`工单映射。

## 实现门禁

- 能力发现必须给出稳定且可操作的不支持原因，不能以“以后可能有端点”为由保持未知。
- 对`server.upstream-ticket`调用`Execute`必须在认证前返回`errUnsupported`，可控假服务断言网络调用次数为零。
- 魔方云适配器插件版本提升为`v0.1.7`，适配器版本提升为`zjmfcloud-v1.7.0`，确保能力快照能识别边界变化。
- 工单域未来接入独立协议时必须使用独立渠道配置和秘密句柄，并实现字段选择、附件脱敏、部门映射、编号映射、回复同步、内部备注隔离、幂等、退避和审计；在这些条件闭环前不得宣称任务`11.4`完成。

## 规则域与影响

- 架构与插件：变更仅收紧`idc-zjmfcloud`能力声明和执行边界，没有修改宿主目录，也没有把工单所有权下沉到资源适配器。
- 数据权限：没有新增对象编号入口或数据读取；未来工单转发必须由管理员显式选择内容并校验目标工单和服务数据范围。
- 性能：不支持路径为本地常量查找和失败关闭，远程调用次数固定为零。
- `i18n`：没有新增运行时页面、菜单、按钮、接口文档或翻译键；中英文插件技术说明同步更新，因此运行时`i18n`无影响。
- API、数据库、缓存、前端、`E2E`和开发工具：本切片没有对应变更；可观察后端行为由单元测试覆盖。

## 验证证据

| 命令或检查 | 结果 |
|---|---|
| `GOWORK=off go test ./backend/... -count=30 -shuffle=on`（`idc-zjmfcloud`） | 通过；能力矩阵精确原因、执行零网络调用及非规范流量响应失败关闭均纳入重复乱序回归 |
| `.\make.cmd lint.go dir=apps/lina-plugins/idc-zjmfcloud plugins=1` | 通过；`golangci-lint`与`staticcheck U1000`均为零问题 |
| `openspec validate add-idc-server-operations-platform --strict` | 通过 |
| 范围化`git diff --check` | 通过 |

## 任务状态

任务`9.4.h`的管理员`v1`协议边界、显式能力声明、零网络执行保护和离线证据已经闭环。实际上游工单转发仍属于任务`11.4`；附属对象目录、配额、影响预览、拥有方编排和公开动作入口也尚未完成，因此父任务`9.4`保持未勾选。

## Lina 任务级审查

- 范围来自`git status --short`、`git ls-files --others --exclude-standard`、插件未跟踪目录展开和任务`9.4.h`上下文，共审查本任务实际修改或新增的 16 个文件；工作区其他宿主及插件变更不属于本任务，未据此修改或下结论。
- 已重新读取`AGENTS.md`、`.agents/rules/openspec.md`、`.agents/rules/documentation.md`、`.agents/rules/architecture.md`、`.agents/rules/data-permission.md`、`.agents/rules/plugin.md`、`.agents/rules/backend-go.md`、`.agents/rules/testing.md`和`.agents/rules/i18n.md`；插件根目录不存在局部`AGENTS.md`。
- 架构与插件边界通过：管理员`v1`资源适配器只声明自身可验证能力，财务/资源渠道工单协议保留在工单域，没有新增重复抽象、跨插件内部依赖或宿主目录改动。
- 数据权限无运行时影响：本任务没有增加数据查询、对象编号入口、写接口或管理入口；未来任务`11.4`仍必须对目标工单、服务与所选转发内容做权限及数据范围校验。
- 后端与测试通过：不支持路径在认证前结束且固定零网络调用；新增测试自包含、可并行、顺序无关，并覆盖能力原因与执行失败关闭。流量字段补充纯十进制校验及带符号失败用例。
- 文档与 OpenSpec 通过：中英文插件说明表达同一能力边界，增量规范、设计、任务、契约矩阵和验证记录一致，严格校验通过。
- 缓存、API、数据库、前端、`E2E`与开发工具均未命中：没有运行期缓存、路由/DTO、迁移、页面、端到端流程或脚本变化。由于没有用户可观察页面和工作流变化，未触发 E2E 质量审查或截图验证。
- `i18n`无运行时影响：`plugin.yaml`只提升版本，没有改动语言配置、用户文案、接口文档元数据或翻译资源；双语技术说明已同步，因此不触发`make i18n.check`。
- `DI`无影响：没有新增运行期依赖、构造函数、启动装配或共享服务。
- 审查结论为严重问题 0 个、警告 0 个，未发现阻塞任务`9.4.h`完成的问题。剩余风险仅是实际财务/资源渠道工单协议尚未由任务`11.4`实现，且生产写验证仍受可信`HTTPS`与隔离对象条件约束。
