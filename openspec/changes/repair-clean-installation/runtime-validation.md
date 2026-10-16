# 首次安装运行验证

## 隔离环境

本轮使用独立的`PostgreSQL schema`：`fresh_install_final_1014`。通过`GF_GCFG_FILE`加载独立配置，并在`database.default.namespace`指定隔离空间；没有使用真实运行实例的数据。宿主、后台前端和根目录工具未因本轮验证而修改。

使用当前源码构建的`official_plugins`完整程序：`temp/20261014/lina-plugins-final`。仅通过程序的`init --confirm=init`初始化宿主数据库，没有提前执行插件迁移。随后通过插件侧安装入口启动，动态目录中预先放置当前的十四个正式插件包，排除历史测试插件。

## 验证结果

| 验证路径 | 结果 | 证据 |
| --- | --- | --- |
| 空库宿主初始化 | 成功 | `temp/20261014/164500-final-init.log` |
| 十三个内建源码插件自动安装、启用并启动 | 成功 | `temp/20261014/164500-final-run.log` |
| 初始管理员通过公开登录接口登录 | 成功 | 自动化脚本用于后续认证；令牌仅保存在权限为`0600`的临时文件中 |
| 十四个正式动态插件逐一安装、启用 | 全部返回成功 | `temp/20261014/164500-final-install-results.json` |
| 同一实例重启 | 成功 | `temp/20261014/165000-final-restart.log` |
| 重启后十四个动态插件的权威安装、启用状态 | 全部为`1` | `temp/20261014/165000-final-managed-status.json` |
| 官网配置查询 | 成功返回两个已安装官网候选 | `temp/20261014/165000-final-site-configuration.json` |

正式动态插件包括`idc-baota`、`idc-dcim`、四个支付插件、`idc-peer`、`idc-v10`、`idc-zjmf-finance`、`idc-zjmfcloud`、`idc-zjmfmigrate`、`yunsu-ssl`以及两个官网插件。没有配置或调用第三方供应商；本轮证明安装、启用、首次启动和重启行为，不宣称第三方真实业务履约已验证。

## 已修复的官网安装问题

官网最初未声明方法。宿主现有声明标准化与快照克隆对空方法列表分别产生空数组和空值，导致授权时的声明比较不一致。修复严格位于插件内：官网拥有者与提供者声明真实的只读`capabilities.describe`能力，提供者实现对应协议，拥有者验证方法边界。最新包在全新隔离环境下安装、启用和重启全部通过。

官网页面保存、模板切换和公共内容联动由官网页面验收记录补充。本记录不替代该界面验收。

## 公开完整构建验证

通过首次安装指南的公开入口执行完整构建，没有用手工工作区编译代替此项验收：

```text
go run apps/lina-plugins/idc-compliance/hack/tools/installation/main.go -state temp/20261014/installation build plugins=1 output-dir=temp/20261014/clean-delivery
```

命令返回成功。日志为`temp/20261014/173000-clean-delivery-build.log`。独立输出目录不覆盖隔离实例的动态插件存储，交付内容包含：

- `lina`：包含源码业务插件和嵌入后台资源的完整后端。
- 十四个正式动态插件的`Wasm`包，包括两个官网插件。
- `web-customer/index.html`及用户中心静态资源。
- `website/default/server/index.mjs`及官网服务端、公共资源。
- `runtime/node`：交付配置指定的`Node.js v24.21.0`运行时。
- `delivery.json`：确认后台为嵌入式，记录用户中心、官网和运行时交付路径及摘要。

本次完整构建没有新增宿主受跟踪文件的变更。官网主题颜色帮助文案随后通过相同公开`linactl wasm`入口重新打包到该独立交付目录；未覆盖正在使用的已安装版本快照。

## 未完成：稳定生命周期存在宿主边界阻塞

十三个内建插件、十四个动态插件安装启用和重启均已通过，但**不能据此将完整生命周期或整个变更标记完成**。在完全新建的`fresh_install_final_1014`环境中，仍能通过公开接口复现官网授权快照丢失；这不是旧实验包或旧数据库残留。

公开调用复现顺序：

1. `PUT /api/v1/plugins/website-minimal/new-business/stop`成功，满足停用前置条件。
2. `PUT /api/v1/plugins/website-minimal/enable`成功，刷新该插件授权。
3. `GET /api/v1/plugins/website-minimal/business-impact`成功，证明拥有者处理器已注册且授权此时有效。
4. `PUT /api/v1/plugins/website-default/enable`成功；该公开状态动作同步全部发现清单。
5. 再次查询`website-minimal/business-impact`失败，返回业务影响检查拒绝；后台内层原因是拥有者`unknown`。
6. `PUT /api/v1/plugins/website-minimal/disable`同样失败。
7. 为保留其他页面验收，最后通过公开接口重新启用并恢复新业务，均成功。此恢复仅用于测试环境状态恢复，不代表根因修复。

完整响应证据：`temp/20261014/181500-authorization-loss-repro.json`。临时重新启用或升级只能短暂刷新授权，不能解决下一次其他插件动作清空授权的问题，因此不作为修复验收。

精确源码原因：

- `apps/lina-core/internal/service/plugin/internal/lifecycle/lifecycle_status.go:89`的`syncDiscoveredManifestsForStatus`在状态操作时对所有清单调用`SyncManifest`。
- `apps/lina-core/internal/service/plugin/internal/store/store_release.go:163`的`syncReleaseMetadata`重新构建并写入版本快照；同文件`buildManifestSnapshotModel`在第`374`行把`AuthorizedBusinessExtensions`初始化为空。
- `apps/lina-core/internal/service/plugin/internal/store/store_authorization.go:169`的`applyExistingHostServiceAuthorization`仅保留宿主服务授权，没有保留业务扩展授权。
- `apps/lina-core/internal/service/plugin/internal/lifecycle/lifecycle_business.go:182`对“声明非空但授权为空”的活跃快照拒绝检查，拥有者为`unknown`；这是正确的安全拒绝，不能放宽检查规避。

上述根因位于用户明确禁止修改的宿主范围。本轮没有修改这些源码，也没有直接改写数据库授权快照。官网插件侧已经通过公开生命周期接口注册业务影响处理器；其独立授权有效时的检查成功，仍需符合宿主边界的可行方案才能完成稳定停用与恢复闭环。


## 标准入口干净安装验收（2026-10-16）

本次从当前可收录源码创建独立副本，不携带`node_modules`、内嵌生成资源、旧输出和私密配置。先执行根`make env.setup`，填写独立数据库连接，随后执行`make db.init confirm=init`及`make dev`。副本的宿主受跟踪源码与当前原项目逐文件摘要一致；仅允许未修改的官方`pack.assets`生成内嵌资源。

| 验收 | 结果与证据 |
| --- | --- |
| 完整构建及六进程启动 | 成功，`temp/20261016/clean-make-dev-final.log` |
| 十六个动态插件安装启用 | 全部成功，`temp/20261016/clean-runtime-plugins.json` |
| 官网、用户中心模板选择 | 公开管理接口保存成功，独立客户静态资源实际由插件提供 |
| 后台深链、用户中心登录、官网商品和帮助页 | 全部`HTTP 200`，`temp/20261016/clean-runtime-http.json` |
| 三套官网切换 | 默认、简约、纯静态模板均成功，证据同上 |
| 真实浏览器 | 官网、客户登录页、后台登录及插件商品管理页面正常；三类入口未发现控制台错误 |
| 中断清理 | 向本次安装工具发送`SIGINT`后，`16040`、`16041`、`16042`、`17680`、`17681`、`17682`全部释放，`clean-runtime-cleanup.json`记录结果 |
| 原实例 | `6030`及`6031`仍监听；此次未停止、重启原实例，也未修改原数据库 |

首次页面检查误用了不存在的`/help`，按插件实际声明改用`/pages/help`后通过；未为了测试添加虚假路由。官网`SSR`运行依赖曾因复制时跳过`node_modules`而缺失，已修复为保留`Nitro`跟踪依赖，两个独立渲染器启动及新增回归测试均通过。

工具验证复用当前状态证据：`installation-standard-tests.log`、`installation-standard-lint.log`、`installation-standard-i18n.log`，以及`Windows`、`macOS`交叉编译产物。后台网关单测通过。最终按`lina-review`定向审查，未发现本次安装交付范围内的阻塞问题。

本次完成标准安装启动和模板实际运行验收，不把前述宿主授权快照丢失问题视为已修复，也不宣称所有第三方业务和插件停用流程已经通过。
