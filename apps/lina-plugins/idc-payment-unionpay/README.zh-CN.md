# 银联支付

`idc-payment-unionpay`以`dynamic + managed`交付，提供支付方式配置声明，由`idc-payment`拥有支付、回调验签、查单、退款及账务事实。安装插件不代表渠道已配置或付款已成功。

## 开发契约

遵守[插件开发与安装规范](../README.zh-CN.md)。本插件声明`payment-method@v1`、`owner=idc-payment`、`role=provider`、`methods=[capabilities.describe]`及多实例接口配置。拥有方校验器为[公开支付声明契约](../idc-payment/backend/cap/paymentcap/dynamic_provider_contract.go)。缺少拥有方、依赖、合法字段或元数据时拒绝安装；不要把新增渠道名称当成已实现的支付协议。

- 提供方：`unionpay`。
- 已声明模式：`unionpay_web`、`unionpay_wap`。
- 币种：`CNY`。
- 配置字段：`endpointOrigin`、`merchantId`、`certificateSerialNo`、`privateKey`、`publicKey`。秘密仅在拥有方机密边界保存和使用，不进入公开模板或日志。

## 构建与使用

从仓库根运行`go run ./hack/tools/linactl build dir=apps/lina-plugins/idc-payment-unionpay`，产物使用标准动态插件上传、安装和启用流程。所有依赖以本插件`plugin.yaml`为准，更新先提高版本并预览后确认；不得直接修改数据库状态。

支付方式在支付拥有方的接口配置页维护，并须满足各模式的真实商户资质、凭据与回调配置。真实支付测试必须使用明确授权的测试对象；契约测试不能证明真实收付款成功。
