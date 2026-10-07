# 第八阶段资源只读能力桥接验证记录

> **`FB-52`范围纠正：**资源只读桥仍可作为服务器插件专属能力保留，但旧`serviceNo + channelNo + poolNo`由通用核心拼接路由的证据只用于迁移对账。现行调用必须从长期权益冻结的服务器业务拥有方和不透明资源引用进入服务器插件，不得要求其他商品具有渠道、资源池或服务器服务编号。

## 范围

本记录只证明`8.4.b.2`完成，不代表`8.4.b`、`8.4`或第八阶段整体完成。

## 实现结论

- `idc-resource`发布`resource-read/v1`能力，拥有方固定为`idc-resource`，当前只提供`server-capabilities.list`只读方法。
- 源码契约与动态 bridge 共用同一请求和结果校验。单次最多查询 64 条明确的`serviceNo + channelNo + poolNo`路由，结果顺序与请求一致。
- bridge 只返回规范化能力代码和本地快照观察时间，不返回 Provider 实例、秘密句柄、明文密钥、上游地址、原始响应、数据库表或写方法。
- owner invoker 在插件 HTTP 启动装配完成后只绑定一次；调用方、拥有方、服务、版本、方法、空资源参数、空表参数和载荷大小任一不符都失败关闭。
- `pluginhost.HTTP`与`pluginhost.Jobs`生产回调各自读取同一份只写后冻结的 Provider 声明快照。重复读取不会重新开放注册，也不会创建渠道客户端或发起上游请求。

## 主要文件

- `apps/lina-plugins/idc-resource/backend/cap/resourcecap/read_contract.go`
- `apps/lina-plugins/idc-resource/backend/cap/resourcecap/descriptor.go`
- `apps/lina-plugins/idc-resource/backend/cap/resourcecap/bridge/client.go`
- `apps/lina-plugins/idc-resource/backend/internal/service/resourceprojection/capability.go`
- `apps/lina-plugins/idc-resource/backend/plugin.go`

## 验证结果

- `GOWORK=off go test ./backend/... -count=1`：通过。
- `GOWORK=off golangci-lint run --config E:/golang/cloud/.golangci.yml ./...`：`0 issues`。
- `staticcheck -checks=U1000 -tests=false ...`覆盖资源插件全部生产包：通过。
- 新增契约、descriptor、bridge 与 invoker 测试，覆盖精确 owner 信封、载荷与响应复验、错误表逃逸、凭据形态输入、重复绑定和确定性排序。

## 其他门禁

- 本切片没有新增 HTTP API、数据库表、前端页面或用户可见文案，因此没有 API、SQL、数据权限、前端和 i18n 变更。
- 未调用真实 Provider，未写入既有魔方用户或实例。
- 资源池兼容、多渠道路由和跨 owner 幂等结果回报随后已在[资源 Provider 能力与路由收口记录](phase-8-resource-provider-capability-closure.md)完成汇总复核；本记录自身仍只证明`8.4.b.2`。
