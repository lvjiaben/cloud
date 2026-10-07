# idc-ai

`idc-ai`是 IDC 只读 AI 辅助的内建源码插件。

[English](README.md) | 简体中文

## 能力范围

本插件负责商家 AI 配置、只读工具、洞察、检索、诊断、预测和草稿，永久不提供危险操作工具。

## 依赖

本插件依赖`idc-core`和`idc-resource`公开的`idccap`与`resourcecap`只读契约。

## 交付

`v0.1.0`以`source + builtin`交付，并使用`platform_only`、`supports_multi_tenant: false`和`global`治理。

## 阶段边界

插件提供商家级加密模型配置、有限的只读运营问答、客户本人数据助手、安全审计和反馈记录。服务器、资金、合规和配置写操作会在模型调用前被拒绝；模型停用或故障时会回到普通业务页面。
