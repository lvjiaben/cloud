# idc-notify

`idc-notify`是 IDC 通知的内置源码插件。

[English](README.md) | 简体中文

## 已实现能力

- 基于 PostgreSQL 的事件目录、双语不可变模板、事件策略、投递任务、发送尝试、服务商结果查询、安全重试、人工核对、人工重发和可恢复的渠道秘密变更。
- 插件自己拥有后台、账户安全和公开售前接口，全部使用`idc-notify`插件路由前缀。
- 插件自己构建渠道、模板、事件策略和投递记录管理页面，不会把 IDC 业务页面放进`web-antd`。
- 已有强类型的 SMTP、阿里云短信、短信宝和腾讯云短信适配器。平台对外通知仅支持邮件和短信。
- 已接入共享服务提供的后台站内信和服务端人机验证能力。
- 匿名售前支持邮件或短信验证，包含一次性人机证明、6 位验证码、PostgreSQL 原子频控和费用上限、连续错 5 次锁定、10 分钟有效期、60 秒重发间隔，以及崩溃后不会重复发送的供应商调用证据。
- 匿名售前工单创建和访问续签调用工单插件的权威能力。访问令牌只返回当前浏览器，本插件不保存。第二份访问链接只使用部署时明确配置的`LINAPRO_IDC_NOTIFY_PUBLIC_CUSTOMER_ORIGIN`。

主要实现位于[数据库迁移](manifest/sql/)、[后端](backend/)、[插件前端](frontend/)和[通知中心 E2E 用例](hack/tests/e2e/TC001-notification-center.ts)。

## 安全边界

- 通知后台操作必须拥有插件自己的专用权限。秘密变更、渠道停启与清除、真实测试发送、人工结案和人工重发还会检查最近完整登录、版本、明确确认和当前数据查看权。
- 秘密统一由`secretcap`保存，不会完整回显。验证码 HMAC 密钥只在秘密边界内生成一次，并且只能通过`notify.verify`用途的回调读取。
- 调用外部服务商前会先持久化准备状态。超时、断线或准备后崩溃统一记为“结果未知”，绝不会自动再发一次。
- 公开验证只保存用途隔离后的摘要、脱敏显示、计数和状态，不保存邮箱、手机号、验证码、客户 IP、浏览器随机值、人机证明或匿名工单访问令牌原文。
- SMTP 只允许部署侧批准的地址，并强制使用证书有效的`465`隐式 TLS 或`587 STARTTLS`。

## 部署配置

- 邮件或短信验证码要求对应渠道已经启用，并且所选事件、渠道、语言和版本有启用中的本地模板。缺少配置时直接关闭，不会猜配置或临时降级。
- `LINAPRO_IDC_NOTIFY_SMS_VERIFICATION_DAILY_LIMIT`可以不配置，不配置时全站短信验证码每天最多`1000`条。只要明确配置了但内容为空、不是数字或不在`1..100000`范围内，服务就会启动失败。
- `LINAPRO_IDC_NOTIFY_PUBLIC_CUSTOMER_ORIGIN`可以不配置。生产环境只接受干净的公网 HTTPS 来源，本地开发可以明确填写回环 HTTP 来源。缺失或非法时，创建工单和续签仍会把当前浏览器令牌返回给当前页面，但访问链接显示“不可用”且不会发送。

## 依赖和交付

本插件只使用`idc-core`和`shared-services`公开的能力契约，不导入它们的`internal`实现。`v0.1.0`以`source + builtin`交付，治理信息为`platform_only`、`supports_multi_tenant: false`和`global`。

## 尚未完成

这份 README 不代表第 11 阶段全部完成：

- 外部服务商真实账号、回调公网可达性、SMTP 白名单、短信费用、和真实浏览器验收，仍要在目标部署环境完成。
- 模拟服务商测试通过或配置保存成功，只能证明接线路径可用，不能证明真实外部消息已经送达。

任何必需能力或服务商配置不可用时，受影响的操作都会继续关闭。

## 客户联系方式验证

已登录客户通过`POST /x/idc-notify/api/v1/account/security/contact-codes`获取验证码，通过`POST /x/idc-notify/api/v1/account/security/contact-codes/{verificationNo}/verification`完成验证。服务端从带本人认证的公开资料接口读取原邮箱或手机号。首次绑定验证另一项已有联系方式。原、新方式均验证通过后，用户中心通过现有资料路由自动保存；无法验证的客户使用现有工单。

此功能需要配套的`idc-core >=0.1.162`守卫和`010-login-contact-change.sql`。账户串行化、验证凭证消费与原资料写入处于同一事务。后台须配置可用邮件／短信渠道及`account.login_contact_verification_code`活动模板。默认短信模板使用短信宝；阿里云、腾讯云须配置对应审核通过的供应商模板及变量映射。验证码10分钟有效，错误尝试最多5次，发送冷却和额度持久化。此功能不修改独立用户名，尚未提供手机号／邮箱密码登录别名。

## 附属资源续费通知

`v0.1.10`新增`012-accessory-renewal-notifications.sql`中的三个账务事件。迁移仅补充缺失的事件目录、策略和模板，重复执行保留管理员已有修改，包括已禁用的模板。

| 事件 | 收件人 | 默认开启渠道 |
| --- | --- | --- |
| `billing.accessory_renewal_due` | `customer_contact` | `email` |
| `billing.accessory_renewal_failed` | `customer_contact` | `email` |
| `billing.accessory_renewal_attention` | `staff_user` | `in_app` |

三个事件的业务对象均为`billing`。续费提醒必须包含`invoice_no`、`amount`、`currency`、`tax_amount`、`period_unit`和`period_count`。业务拥有方提供冻结的含税总金额和税额，金额均为按`ISO`币种主单位表示的字符串；`period_unit`为`day`、`month`或`year`，`period_count`为十进制整数。双语模板展示完整周期并明确账单待付款，不宣称已扣款成功。中文周期单位只在渲染副本中本地化，例如`12个月`；冻结事件和投递变量仍保留`month`及原始金额。续费失败通知必须包含`invoice_no`，说明数据保留、使用暂停、付款恢复及客户仍可删除对象。

后台待办提醒必须包含`pending_count`和最近账单号`invoice_no`；可选`locale_code`选择`zh-CN`或`en-US`，默认`en-US`，引导有权限人员到续费待办处理。员工物化仅接受来自`idc-core`的该事件并使用可信员工收件人生成`in_app`任务；开启外部渠道策略不会授权员工外部投递。此种子不授予员工权限，也不解析员工联系方式。其他受支持策略渠道默认关闭，不恢复员工企业微信；邮件仍要求渠道已启用并完成配置。迁移、渲染和事件物化测试只在隔离环境创建投递记录，不外发邮件。


### 邮件与短信通知

`v0.1.11`新增迁移`013-email-sms-notifications.sql`，移除客户废弃渠道和对应策略。历史投递证据保留，已调用供应商且结果未知的记录转为人工核查，不伪造发送成功。保留后台员工续费待办的内部站内同步；客户 `/account/notifications` 接口和通知偏好页已删除。
