# Provider 安全出站验证记录

## 完成范围

本轮完成任务 8.4.a 的 resource owner 出站网络底座：

- 每个客户端绑定一个渠道配置的精确`HTTPS hostname + port`，只允许路径和查询变化。
- 拒绝 HTTP、跨主机或跨端口请求、URL userinfo、fragment 和请求级`Host`覆盖。
- `http.Transport`不使用环境代理，禁止重定向，强制 TLS 1.2 以上并保留标准证书链与主机名校验。
- 每次拨号重新解析 DNS，去重后先校验完整结果；任意地址不安全时整个结果失败关闭。
- 只在全部地址通过后直接拨号已校验 IP，不再把主机名交给底层拨号器，封闭 DNS 校验与连接之间的重绑定窗口。
- 拒绝私网、回环、链路本地、共享地址、基准测试、文档、组播、保留和未指定地址；支持经过同等校验的公网 IPv4 与 IPv6。

## 测试与审查

- 单元测试断言精确 origin 成功，协议、主机、端口、userinfo、fragment 和`Host`覆盖均在到达 transport 前失败。
- DNS 测试断言公网地址被固定为 IP 拨号，公私混合结果在任何网络拨号前拒绝。
- 地址矩阵覆盖公网 IPv4/IPv6、RFC1918、回环、CGNAT、链路本地、TEST-NET、基准测试、组播、IPv6 ULA 与文档前缀。
- TLS/redirect 测试断言无代理、存在受控拨号器、`InsecureSkipVerify=false`、最低 TLS 1.2 且重定向返回`http.ErrUseLastResponse`。
- resource 插件 20 轮乱序全量 Go 测试和 Go lint 通过。

## 未完成门禁

安全客户端尚未通过生产`pluginhost.HTTP/Jobs`回调注入，因为 8.4.b 的安全日志指标环境、资源池兼容、多渠道路由和唯一 owner 启动闭环尚未完成。当前不提前调用 Provider registrar 的全局`Freeze`，避免部分装配把单向声明冻结在不可恢复状态。

本轮没有新增 HTTP API、前端页面、用户可见运行时文案、数据库变更、缓存或字典，API、前端、SQL、缓存和 i18n 无影响。

## 后续生产装配复核

上文“尚未通过生产回调注入”是本切片完成时的历史状态。当前实现已经在`idc-resource/backend/plugin.go`的唯一`pluginhost.Jobs`回调中冻结 Provider 声明，通过`NewProductionProviderRuntime`为每个启用渠道惰性构造安全出站环境，并把`ConstructSession`、集合化资源映射解析器和健康保护显式注入资源任务执行器；主节点托管写入、任务号查询和资源查询三类作业。渠道停用时读取门禁在环境、工厂和秘密读取之前拒绝构造，相关十轮乱序与启动治理证据见[Provider 启动装配治理验证记录](phase-8-provider-startup-governance-validation.md)。动态 owner-aware descriptor/bridge SDK 和其余能力跨 owner 回报继续保留在`8.4.b`父项。
