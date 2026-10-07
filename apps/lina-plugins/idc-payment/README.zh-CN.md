# idc-payment

`idc-payment`是 IDC 支付集成的内建源码插件。

[English](README.md) | 简体中文

## 能力范围

本插件负责支付渠道、支付与退款尝试、回调证据、主动查询和对账。

## 依赖

本插件只依赖`idc-core`公开的`idccap`契约。

## 交付

`v0.1.0`以`source + builtin`交付，并使用`platform_only`、`supports_multi_tenant: false`和`global`治理。

## 阶段边界

第一阶段只提供注册、依赖、国际化、测试和能力开关骨架，暂不实现支付流程或持久化业务数据。
