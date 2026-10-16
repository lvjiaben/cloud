# 客户业务与开放 API 闭环

## Why

现有协助报价依赖多余审批，代理页面缺少管理操作且代理价未进入正式交易，渠道扣款失败可能遗留订单。本次按用户确认打通可直接操作的客户业务、双用途代理销售与同系统开放接口。

## What Changes

- **BREAKING**：协助下单移入客户管理，删除审批、驳回和独立下单步骤，直接生成客户订单。
- 代理销售同时支持拿货价格、客户归属、实收佣金及线下结算登记；网页与接口共用客户价格。
- **BREAKING**：开放接口使用绑定客户的`AppID/AppSecret`与接口授权，新购和续费只用同币种余额，原子拒绝余额不足请求。
- 简化菜单与页面，补全同系统连接、接口文档、错误恢复及真实交易验证。

## Capabilities

### New Capabilities

以下能力尚位于首版活跃变更，本次沿用原路径更新相关要求，不创建近义能力；首版冲突条款同步替换。

- `idc-checkout-order`：无需审批的后台协助下单。
- `idc-sales-channel`：客户代理价、销售归属、实收佣金和线下结算。
- `idc-downstream-channel`：客户绑定凭据、原子余额交易与同系统连接。

### Modified Capabilities

无已归档基线能力变更。

## Impact

修改`idc-core`、`idc-channel`及必要的`idc-resource`公开适配；不修改宿主。涉及插件菜单、精确权限、数据库升级、事务审计、显式依赖装配、中文及英文运行文案和接口文档。新规则权威来源为`.agents/memory/preferences/32-assisted-orders.md`及`76-customer-business-openapi.md`。
