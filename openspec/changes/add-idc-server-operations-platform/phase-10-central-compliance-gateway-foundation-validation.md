# 第十阶段中央合规网关独立交付底座验证记录

> 状态：本文件只保留历史技术验证证据。平台中央部署和不进入购买方安装包的交付前提已被[完整独立交付边界纠偏记录](phase-10-independent-delivery-correction.md)废止，不再证明当前需求符合性。

## 1. 任务结论

任务`10.5`已完成供应商中立的中央合规网关独立进程、独立 Go 模块、跨平台发布工具、完整性清单和商家安装包隔离验证。

本记录形成时的网关切片有意保持失败关闭：只存在`GET /health/live`和`GET /health/ready`两个运维端点，就绪端点固定返回`503 supplier_gate_closed`。不存在实例注册、合同、发票、回调、凭据或供应商业务路由，也没有腾讯、诺诺、宿主或商家插件运行时依赖。`2026-08-14`后腾讯实现依据改由已完成的`10.4.b`官方契约记录提供，真实腾讯客户端转由`10.8`实现；本历史记录不能继续作为腾讯外部阻断证据。

## 2. 交付边界

| 边界 | 实现 |
|---|---|
| 独立源码模块 | `apps/lina-plugins/idc-compliance/central-gateway/go.mod`定义独立模块`lina-platform/idc-compliance-gateway` |
| 独立进程 | `cmd/idc-compliance-gateway`只装配运维服务、受限环境配置和优雅关闭 |
| 独立发布 | `hack/tools/release`使用 Go 标准库生成目标平台二进制和同路径`.manifest.json` |
| 产物完整性 | 发布清单记录文件名、`SHA-256`、目标系统、架构、版本、提交和构建时间 |
| 商家包隔离 | 商家插件继续只嵌入`plugin.yaml`、`frontend`和`manifest`，新增自动测试拒绝中央网关目录或二进制进入嵌入文件 |
| 供应商隔离 | 独立模块依赖图只有 Go 标准库，不包含`lina-core`、`lina-plugin-*`、腾讯、`essbasic`或诺诺依赖 |
| 运行门禁 | 存活端点返回`200`；就绪端点返回`503 supplier_gate_closed`；未知业务路径返回`404` |

商家插件和中央网关使用两个不同的构建边界。中央网关源码位于插件专用目录只表示源码治理归属，不表示其二进制进入商家安装包。

## 3. 运行与安全约束

- 默认只监听`127.0.0.1:9088`，监听地址必须包含`1`至`65535`的数字端口。
- 请求头、读取、写入、空闲和关闭超时有固定安全默认值；可配置值只能在一秒至两分钟之间。
- 最大请求头固定为`64 KiB`，运维响应设置`no-store`和`nosniff`。
- 当前配置模型不存在供应商凭据、应用编号、能力开关或任意外部端点。
- 发布工具固定`CGO_ENABLED=0`、`GOWORK=off`、`-mod=readonly`、`-trimpath`和`-buildvcs=false`，构建子进程有十分钟上限。
- 发布工具要求显式提供输出路径并拒绝覆盖既有二进制或清单，避免误覆盖已发布产物。
- 发布清单明确记录当前产物不含供应商客户端、不开放业务路由且不进入商家安装包。

## 4. 接口与领域影响

- `API`：只新增独立中央进程的无业务数据运维端点；没有接入宿主路由、商家接口或供应商接口。
- 数据权限：运维端点不返回商家、客户、主体、额度或业务数据；本任务没有新增业务数据权限路径。
- 数据库与缓存：没有新增表、迁移、查询、缓存或跨模块状态。
- `i18n`：运行时只返回稳定机器码和构建元数据，没有新增用户界面或可翻译业务文案。
- 宿主：没有修改`apps/lina-core`、`apps/lina-vben`、根构建工具或宿主页面。

## 5. 验证结果

### 5.1 中央模块

以下命令全部通过：

```text
GOWORK=off go test ./... -count=20 -shuffle=on
GOWORK=off go vet ./...
golangci-lint run --config E:\golang\cloud\.golangci.yml ./...
staticcheck ./...
```

测试覆盖安全默认配置、配置边界、存活与失败关闭就绪、业务路径不存在、运维写请求拒绝、构建信息规范化、发布参数校验、既有产物覆盖拒绝，以及依赖图排除宿主和供应商`SDK`。

### 5.2 商家插件隔离

以下验证通过：

```text
GOWORK=off go test ./backend/... -count=20 -shuffle=on
GOWORK=off go vet ./backend/...
.\make.cmd lint.go dir=apps/lina-plugins/idc-compliance plugins=1
.\make.cmd plugins.check
```

商家插件测试直接检查嵌入文件系统，证明不存在`central-gateway`目录、中央网关产物或`.exe`文件。插件治理扫描`1589`个文件，发现数为`0`。

### 5.3 跨平台发布

同一发布工具分别成功生成`windows/amd64`和`linux/amd64`产物及清单：

| 目标 | 验证产物`SHA-256` |
|---|---|
| `windows/amd64` | `c7df7181656cf647ac27fe8791b0d143c5e0c5bd41b9aecda2ba012255bc9a43` |
| `linux/amd64` | `b511afec69ca47cb8f281052a09293239e6603dc61819462213533db483cbe99` |

验证产物生成在操作系统临时目录，没有写入仓库或商家插件资源。

### 5.4 规范

```text
openspec validate add-idc-server-operations-platform --strict
git diff --check
```

两项门禁均通过。

## 6. LinaReview

审查范围限定为任务`10.5`的独立模块、发布工具、运维接口、商家嵌入隔离和对应 OpenSpec 文档。

初次定向 lint 发现并修复四项新模块问题：响应编码错误未显式处理、主函数退出绕过延迟清理、构建子进程缺少上下文，以及监听器缺少可取消上下文。修复后重复测试、`go vet`、`golangci-lint`和`staticcheck`全部通过。

最终未发现阻断该历史`10.5`切片的`P0`、`P1`或`P2`问题。原记录列出的腾讯书面授权、中央身份和平台结算后续项已分别被`2026-08-14`的腾讯官方契约门禁及完整独立交付纠偏废止或替换；当前后续任务以`tasks.md`中的`10.8`、`10.9`及诺诺独立门禁为准，本文件不得用于恢复中央业务服务。
