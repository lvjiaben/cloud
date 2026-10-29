# 魔方 V10 供应商接口

这是 **智简魔方业务系统 V10** 的独立托管 WASM 连接器。它不是魔方财务、WHMCS `/v1`，也不是魔方云管理员接口。

## 已实现的接口流程

- 每条接口实例分别保存接口地址、API 用户名和 API 密码，管理员可以添加多条实例。
- 宿主通过 `POST /api/v1/auth` 登录，JWT 只保留在保密出站代理中，业务请求由宿主添加 `Authorization: Bearer ...`。
- 商品发现使用 `GET /api/v1/product`。
- 已选商品经 `GET /api/v1/product/:id/resource` 明确识别模块，通用转售只接受已实现的 `idcsmart_common` 协议，不下载或执行远程模块压缩包。
- 开通按照官方的“先查后建”流程执行：先删除或检查 `/console/v1/cart`，确认没有已有订单后加入商品、结算，再在需要时调用 `/console/v1/pay` 支付。
- 暂停、恢复、终止和资源详情只使用已经从源码确认的 `/console/v1/host/...` 与 `/console/v1/refund` 路径。
- 模块专用且尚未确认的操作保持禁用，不猜接口。

回调令牌、供应商账号、JWT 和供应商原始响应都留在 `idc-resource`。WASM 只能拿到白名单过滤后的业务字段。供应商可能已经受理但响应超时时，结果会进入“待核对”，不会盲目重复提交。

## 依据

实现依据是智简魔方业务系统官方 10.4.6 源码中的 `SupplierModel`、`UpstreamLogic` 和 `ResModuleLogic`。V10 路径与魔方财务 `/zjmf_api_login` 流程保持完全分离。


## 通用模块远程核价转售

`v0.2.4` 根据官方 10.4.6 源码，经 `GET /api/v1/product/:id/resource` 明确识别源模块，只接受已实现的 `idcsmart_common` 协议，不下载或执行远程模块压缩包。配置及核价使用 `reidcsmart_common/.../configoption` 和 `/calculate`，同一包装接口兼容上游原生与再次转售商品。该接口只提供当前选配整件价格，连接器因此返回 `remote_evaluation`，源配置只读，通过 `is_downstream=1` 读取当前账号价格；不起价冒充完整静态价表，不伪造同系统报价令牌。未支持的实际模块禁止导入。

原生参数保留真实 `config_options[cycle]`、单选、多选与数量数组、有限重复数量以及 `self_defined_field` 客户字段，沿既有购物车查询优先与资源拥有方回调关联执行。无法完整表达的源规则拒绝处理。本地 HTTPS 协议测试不代表已在真实上游账号成功购买。
