# yunsu-ssl

`yunsu-ssl`是云速 SSL 证书供应商的托管动态履约插件。

[English](README.md) | 简体中文

## 当前能力

插件通过`idc-core`发布的`product-fulfillment@v1`接入通用商品体系，已经实现：

- 读取云速`POST /product/list`并生成商品配置候选；
- 选中供应商产品时同时冻结 DV/OV/EV 和 DNS/文件验证能力，不让商家手工重复录入；
- 在付款确认或后台审核通过后的持久异步任务中调用`POST /payment/balance`；
- 通过`POST /order/sync`查询签发状态；
- 不调用供应商的纯配置校验；
- 普通域名、多域名、通配符和 IP 证书的严格约束，并校验 DNS/文件验证能力是否匹配；
- 客户必须提供 CSR，插件校验 CSR 签名和域名/IP 是否与购买内容完全一致；
- OV/EV 证书必须提供完整联系人和组织资料；
- 严格、不伪造数据的`POST /order/detail`、`POST /order/update-dcv`、`POST /order/revalidate`和`POST /order/cancel`适配器；
- 只有签发前才能取消申请或切换验证方式，取消不代表自动退款；
- 提交超时返回结果未知，禁止自动重复扣款；
- API Key 和 API Secret 只由宿主注入请求头，不进入 WASM、日志或普通响应。

## 安全边界

供应商接口必须使用 HTTPS。宿主固定请求到管理员配置的同一源站，校验证书，禁止重定向、系统代理、内网地址和 DNS 重绑定。动态插件只提交接口字段名和安全业务数据；接口地址与密钥值由宿主根据当前短期履约上下文读取。供应商响应按字段白名单投影，密钥回显会被拒绝。

## 由拥有方治理的服务页面

插件已经声明后台和客户中心证书页面。页面只能调用它在冻结清单中逐项声明的`idc-core`服务能力。宿主负责确定真正的接口拥有方，并重新检查当前会话、权限、数据范围和客户本人所有权，然后只为一个确定的方法和路径签发短期授权。服务变更通过持久任务执行，先查状态再提交，重试次数有上限；结果不确定时停止自动操作并允许后台人工接管。

证书链下载使用宿主管理的两分钟一次性票据。宿主直接下载供应商压缩包，清理后存入私有临时空间，只允许公开证书和公开证书链。WASM 永远拿不到供应商 ZIP、私钥、供应商密钥或浏览器令牌。本版仍不支持续费。

## 构建与测试

```powershell
$env:GOWORK='off'
go test ./...
$env:GOWORK=$null
Set-Location E:\golang\cloud
go run ./hack/tools/linactl wasm dir=apps/lina-plugins/yunsu-ssl out=temp/output/yunsu-ssl
```

生产验证必须使用合法的云速隔离账号和专用测试证书订单，不能操作真实客户证书。
