# 第九阶段 NAT 端口转发适配器验证记录

## 完成范围

本记录对应子任务`9.4.e`，只闭环`idc-zjmfcloud`当前实例 NAT 端口转发的创建与删除适配器契约。NAT 转发目录持久化、配额、客户自助、后台动作入口和资源拥有方业务编排尚未完成；共享 NAT 网关在已安装版本文档中标注“以后支持”，NAT 建站又是独立能力，二者均不纳入本切片。父任务`9.4`继续保持未完成。

`2026-08-12`复用用户授权的 Chrome 登录真实`3.9.14 beta`后台，只读打开生成式开发文档，确认实例 NAT 列表为`GET /clouds/:id/nat_acl`，支持`list_type=all`并返回`id/name/ext_port/int_port/protocol`；创建为同路径`POST`，接收`name/ext_port/int_port/protocol`，协议`1/2/3`对应`tcp/udp/tcp+udp`，外部端口范围排除`80/443/22`；删除为`DELETE /nat_acl/:id?hostid=:cloud_id`。本次仅登录、导航和阅读文档，没有调用任何线上 NAT API、实例动作或业务写接口，票据和凭据未写入仓库。

## 实现边界

- `server.nat`声明为`conditional`，只公开`action=create/delete`以及条件使用的`nat_ref/name/external_port/internal_port/protocol`。创建和删除使用严格互斥字段组合，未知字段、批量和`force`在认证前拒绝。
- 创建必须指定 1 至 128 字节、无首尾空白和控制字符的名称；外部端口为 1 至 65535 且不得是`22/80/443`，内部端口为 1 至 65535，协议只接受`tcp/udp/tcp_udp`并映射为上游`1/2/3`。外部端口`0`和缺省被拒绝，避免自动选号写入歧义无法唯一对账。
- 每次写前固定一次`GET /clouds/:resource_ref/nat_acl?list_type=all`，解码最多 10,000 条且逐项要求完整合法标量。创建拒绝同外部端口上协议重叠，包括任一侧为`tcp_udp`；删除要求`nat_ref`在当前实例列表中唯一存在。
- 创建只发送一次`POST /clouds/:resource_ref/nat_acl`，表单固定为`name/ext_port/int_port/protocol`；删除只发送一次`DELETE /nat_acl/:nat_ref?hostid=:resource_ref`且无表单参数。
- 写响应携带真实`taskid/task_id`时返回`accepted`。没有任务号的 2xx 普通成功或空响应、`401`及传输歧义都只读回查一次 NAT 列表：创建只有名称、外部端口、内部端口和协议精确匹配且唯一时成功，删除只有目标编号不存在时成功；否则保持不可重试的结果未知。明确`400/403/404/405`保持确定失败。
- 共享 NAT 网关、NAT 建站、批量、自动外部端口和`force`不开放；每个调用最多一次写入，任何歧义都禁止自动重放。

## 测试与规则域

受控 TLS 假服务覆盖创建/删除精确路径、查询与表单、任务号受理、同步回读、输入组合、外部端口保留值和上限、协议枚举、端口协议冲突、对象缺失/重复/畸形、`401`不重放、普通成功无法证明和传输断线后精确回读。测试自包含且可乱序运行，不访问生产 Provider。

- 架构与插件：实现闭环在源码 Provider 适配器，只消费`resourcecap`既有公开契约；没有宿主修改、新抽象或服务定位。
- 性能：每个动作固定一次写前全量读取、一次写入，无任务号时一次回读；远程调用次数不随条目数增长，列表有 10,000 条硬上限且单次扫描，不存在逐项远程`N+1`。
- 数据权限：没有新增公开 HTTP 入口；`ResourceRef`仍必须由资源拥有方按本地资源与渠道映射后传入，客户/后台权限和数据范围入口尚未实现。
- API、数据库、缓存、前端、E2E、开发工具与运行时`i18n`：无影响。没有路由、DTO、SQL、缓存、页面、运行时文案、脚本或工具变化；中英文技术`README`同步维护。
- 安全：没有共享 NAT 网关、建站、批量或强制路径；歧义写绝不重放，只有真实任务号或精确后置状态可收敛。

## 验证证据

| 命令或检查 | 结果 |
|---|---|
| `GOWORK=off go test ./backend/... -count=30 -shuffle=on`（`idc-zjmfcloud`） | 通过；全部后端包编译，NAT 与既有能力契约在 30 轮乱序执行中稳定通过 |
| `.\make.cmd lint.go dir=apps/lina-plugins/idc-zjmfcloud plugins=1` | 通过；`golangci-lint`和`staticcheck U1000`零问题 |
| `openspec validate add-idc-server-operations-platform --strict` | 通过 |
| 任务范围`git diff --check` | 通过；无尾随空白或补丁格式问题 |

任务级`lina-review`核对真实文档证据、能力声明与输入组合、端口协议冲突、删除对象归属、列表全局限界、同步回读证明、`401`不重放、版本与中英文文档一致性。审查最初发现归属核对函数复杂度超过仓库阈值和一处无效局部赋值，已拆分 NAT 核对函数并清理初始化后通过全部门禁；最终严重问题 0 个、警告 0 个。

生产可信 HTTPS 入口仍然缺失，因此本记录只证明离线契约，不证明线上已经安全联调。没有向生产 Provider 发送任何 NAT 写请求，也没有访问或修改既有客户、用户或实例。
