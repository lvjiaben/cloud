# 第八阶段服务器附属资源安全目录验证记录

## 范围

本记录只证明`8.7.a`完成，不代表`8.7`、`8.7.b`、`8.7.c`或第八阶段整体完成。

## 实现结论

- `idc-resource`拥有并发布服务器附属资源安全目录。目录只保存本地服务编号、资源类型、本地对象编号和允许客户看到的白名单投影，不向客户或`core`泄露 Provider 标识、上游地址、原始响应或凭据。
- 目录覆盖当前`IP`、磁盘、快照、备份、安全组、`NAT`和虚拟介质。完整刷新采用整批快照语义；查询必须同时满足服务归属、目录完整和新鲜度要求，过期或不完整时失败关闭。
- `idc-core`的客户服务器详情只读取资源插件提供的安全投影，并在返回动作前同时检查本地服务权益、运行状态、资源渠道能力和目录新鲜度。
- 快照、备份、安全组、`NAT`和虚拟介质写操作先用“当前服务编号 + 本地对象编号”在安全目录中核对归属，再由资源插件内部翻译为私有 Provider 引用。恢复操作还必须输入并精确匹配当前唯一服务器编号；删除操作不得夹带该确认字段。
- 服务器处于普通暂停状态时不能购买流量。只有存在明确的“因流量耗尽而暂停”本地证明时，才允许追加流量，避免把欠费、风控或人工暂停误当成流量暂停。
- 本子项没有实现收费、每台服务器免费份数、对象独立到期与自动续费，也没有开放客户或后台附属能力页面；这些内容继续留在`8.7.b`和`8.7.c`。

## 主要文件

- `apps/lina-plugins/idc-resource/manifest/sql/016-resource-accessory-directory.sql`
- `apps/lina-plugins/idc-resource/backend/internal/service/accessorydirectory/`
- `apps/lina-plugins/idc-resource/backend/internal/service/serveroperationsource/serveroperationsource_accessory.go`
- `apps/lina-plugins/idc-core/manifest/sql/057-server-accessory-operation.sql`
- `apps/lina-plugins/idc-core/backend/cap/serveroperationresourcecap/contract.go`
- `apps/lina-plugins/idc-core/backend/internal/service/serveroperation/`
- `apps/lina-plugins/idc-core/backend/api/account/v1/server.go`
- `apps/lina-plugins/idc-core/backend/api/account/v1/server_operation.go`

## 验证结果

- `go test ./backend/internal/service/serveroperation ./backend/api/account/v1 ./backend/cap/serveroperationresourcecap`：通过。
- `go test ./backend/internal/service/serveroperationsource`：通过。
- `go test ./backend/... -run '^$'`分别在`idc-core`、`idc-resource`和`idc-zjmfcloud`执行：通过。
- `TestServiceRenewalFoundationMigrationPostgreSQL|TestServerOperationCreateReplayAndConvergePostgreSQL`隔离 PostgreSQL 连续 10 轮：通过。
- `TestResourceFoundationMigrationPostgreSQL`隔离 PostgreSQL 连续 10 轮：通过。
- 资源能力、目录投影和 MagicCube 适配器相关测试连续 10 轮：通过。
- `make.cmd i18n.check`：通过，运行时硬编码、宿主与插件消息覆盖、前端语言键覆盖均无违规。
- `openspec validate add-idc-server-operations-platform --strict`：通过。
- `make.cmd lint.go`已确认本子项涉及的`idc-core`、`idc-resource`、`idc-zjmfcloud`等模块均为`0 issues`；全仓命令最终被`tenant-core`中 3 个不属于本子项的既有`gofmt`问题阻断，本次未修改这些共享文件。

## 边界复核

- 未调用真实 Provider 写接口，未操作真实魔方服务器。
- 未向`apps/lina-vben/apps/web-antd`写入 IDC 页面、组件、接口、路由、状态、多语言或业务依赖。
- 当前没有新增前端页面；`web-customer`客户页面与资源插件`frontend/`后台页面仍由`8.7.c`负责。
- 旧 Portal 未删除，发布迁移任务未提前执行。
