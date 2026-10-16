# 项目源码定位清单

本文件是`2026-10-10`工作区的静态结构快照，配合[项目结构与业务链路地图](project-code-map.md)使用。统计包含当前未提交改动，不代表功能已验收；接口路径来自`g.Meta`字面量，需结合路由组前缀和运行时注册确认。机器可检索的明细见[源码索引](project-source-inventory.json)。

## 插件及规模

| 插件 | 类型／分发 | 源码文件 | 测试文件 | 服务目录 |
| --- | --- | ---: | ---: | ---: |
| `content-notice` | `source/builtin` | 59 | 26 | 1 |
| `idc-ai` | `source/builtin` | 23 | 6 | 1 |
| `idc-baota` | `dynamic/managed` | 7 | 2 | 0 |
| `idc-channel` | `source/builtin` | 83 | 19 | 1 |
| `idc-compliance` | `source/builtin` | 805 | 119 | 21 |
| `idc-core` | `source/builtin` | 2756 | 508 | 56 |
| `idc-dcim` | `dynamic/managed` | 12 | 4 | 0 |
| `idc-notify` | `source/builtin` | 319 | 41 | 2 |
| `idc-payment` | `source/builtin` | 128 | 24 | 5 |
| `idc-payment-alipay` | `dynamic/managed` | 2 | 0 | 0 |
| `idc-payment-epay` | `dynamic/managed` | 2 | 0 | 0 |
| `idc-payment-unionpay` | `dynamic/managed` | 2 | 0 | 0 |
| `idc-payment-wechatpay` | `dynamic/managed` | 2 | 0 | 0 |
| `idc-portal` | `source/builtin` | 35 | 6 | 2 |
| `idc-resource` | `source/builtin` | 467 | 129 | 27 |
| `idc-v10` | `dynamic/managed` | 13 | 4 | 0 |
| `idc-zjmf-finance` | `dynamic/managed` | 18 | 8 | 0 |
| `idc-zjmfcloud` | `dynamic/managed` | 63 | 27 | 0 |
| `idc-zjmfmigrate` | `dynamic/managed` | 34 | 14 | 1 |
| `org-core` | `source/builtin` | 106 | 29 | 2 |
| `shared-services` | `source/builtin` | 42 | 5 | 5 |
| `staff-access` | `source/builtin` | 28 | 4 | 2 |
| `tenant-core` | `source/builtin` | 166 | 78 | 9 |
| `vben-adapter` | `source/builtin` | 22 | 5 | 0 |
| `yunsu-ssl` | `dynamic/managed` | 28 | 15 | 0 |

统计为文件数，测试包含在源码文件总数内；生成的`DAO/DO/Entity`计入总数，因此不能用总数直接衡量复杂度。

## 插件服务与公开能力

### content-notice

- 清单：[`plugin.yaml`](../apps/lina-plugins/content-notice/plugin.yaml)。
- 服务目录：`notice`
- 公开能力目录：无`backend/cap`子目录。

### idc-ai

- 清单：[`plugin.yaml`](../apps/lina-plugins/idc-ai/plugin.yaml)。
- 服务目录：`copilot`
- 公开能力目录：无`backend/cap`子目录。

### idc-baota

- 清单：[`plugin.yaml`](../apps/lina-plugins/idc-baota/plugin.yaml)。
- 服务目录：无该层目录；需查看插件`backend`入口和动态处理器。
- 公开能力目录：无`backend/cap`子目录。

### idc-channel

- 清单：[`plugin.yaml`](../apps/lina-plugins/idc-channel/plugin.yaml)。
- 服务目录：`channel`
- 公开能力目录：无`backend/cap`子目录。

### idc-compliance

- 清单：[`plugin.yaml`](../apps/lina-plugins/idc-compliance/plugin.yaml)。
- 服务目录：`closureprecheck`、`contractarchive`、`contractautosign`、`contractfailure`、`contractfulfillmentgate`、`contractlifecycle`、`contractsigner`、`cron`、`customer360summary`、`electroniccontractonboarding`、`electroniccontractpreparation`、`electroniccontracttemplate`、`enterpriseverification`、`numberedpage`、`personalverification`、`privacygovernance`、`providerconfig`、`supplierconfig`、`taxinvoicemanagement`、`taxinvoiceredletter`、`taxinvoicerequest`
- 公开能力目录：无`backend/cap`子目录。

### idc-core

- 清单：[`plugin.yaml`](../apps/lina-plugins/idc-core/plugin.yaml)。
- 服务目录：`asynctask`、`audit`、`billing`、`catalog`、`catalogdraft`、`closureprecheck`、`commerce`、`commission`、`configurationcontract`、`contenthub`、`contractcompensation`、`contractpolicy`、`contractsnapshot`、`customer`、`customer360summary`、`customeractivation`、`customeraddress`、`customerassistance`、`customerclosure`、`customercontact`、`customerexport`、`customermanagement`、`customerprojection`、`financeops`、`fulfillment`、`identityprojection`、`identityworkflow`、`invoiceprofile`、`lifecycle`、`managementruntime`、`marketing`、`merchantcontext`、`notificationpolicy`、`onboarding`、`operations`、`outbox`、`principal`、`productautomation`、`productautomationtoken`、`productdeliverydirectory`、`productdirectory`、`productfulfillment`、`purchaseconfigurationfile`、`reauth`、`renewal`、`resourceinterfaceprojection`、`saleschannel`、`secret`、`serveroperation`、`serviceaction`、`serviceguard`、`sshkey`、`supportdesk`、`supportupstream`、`taxinvoicesource`、`versioncache`
- 公开能力目录：`billingcap`、`channelcommercecap`、`contractcap`、`contractfulfillmentcap`、`contracttemplatecap`、`customer360cap`、`customerauthcap`、`customerclosurecap`、`entitlementcreatedcap`、`idccap`、`identitycap`、`lifecycleresourcecap`、`managementcap`、`notificationcap`、`onboardingcap`、`poolsourcecap`、`productdeliverycap`、`productfulfillmentcap`、`provisionresourcecap`、`renewalownercap`、`resourceinterfaceprojectioncap`、`secretcap`、`serveroperationresourcecap`、`supplierinterfacecap`、`supportcap`、`taxinvoicecap`

### idc-dcim

- 清单：[`plugin.yaml`](../apps/lina-plugins/idc-dcim/plugin.yaml)。
- 服务目录：无该层目录；需查看插件`backend`入口和动态处理器。
- 公开能力目录：无`backend/cap`子目录。

### idc-notify

- 清单：[`plugin.yaml`](../apps/lina-plugins/idc-notify/plugin.yaml)。
- 服务目录：`customer360summary`、`notification`
- 公开能力目录：`channelcap`、`securitycodecap`

### idc-payment

- 清单：[`plugin.yaml`](../apps/lina-plugins/idc-payment/plugin.yaml)。
- 服务目录：`channelconfig`、`exceptionfund`、`paymentattempt`、`paymentflow`、`reconciliation`
- 公开能力目录：`paymentcap`

### idc-payment-alipay

- 清单：[`plugin.yaml`](../apps/lina-plugins/idc-payment-alipay/plugin.yaml)。
- 服务目录：无该层目录；需查看插件`backend`入口和动态处理器。
- 公开能力目录：无`backend/cap`子目录。

### idc-payment-epay

- 清单：[`plugin.yaml`](../apps/lina-plugins/idc-payment-epay/plugin.yaml)。
- 服务目录：无该层目录；需查看插件`backend`入口和动态处理器。
- 公开能力目录：无`backend/cap`子目录。

### idc-payment-unionpay

- 清单：[`plugin.yaml`](../apps/lina-plugins/idc-payment-unionpay/plugin.yaml)。
- 服务目录：无该层目录；需查看插件`backend`入口和动态处理器。
- 公开能力目录：无`backend/cap`子目录。

### idc-payment-wechatpay

- 清单：[`plugin.yaml`](../apps/lina-plugins/idc-payment-wechatpay/plugin.yaml)。
- 服务目录：无该层目录；需查看插件`backend`入口和动态处理器。
- 公开能力目录：无`backend/cap`子目录。

### idc-portal

- 清单：[`plugin.yaml`](../apps/lina-plugins/idc-portal/plugin.yaml)。
- 服务目录：`brand`、`externalmenu`
- 公开能力目录：无`backend/cap`子目录。

### idc-resource

- 清单：[`plugin.yaml`](../apps/lina-plugins/idc-resource/plugin.yaml)。
- 服务目录：`accessorydirectory`、`batchrenewal`、`channelconfig`、`channelinvocation`、`channelsecret`、`customer360summary`、`interfacecandidate`、`interfaceprojection`、`interfacerevision`、`lifecyclesource`、`managedparameters`、`poolcandidate`、`principalprovision`、`providerauthsession`、`providercapacity`、`providerconfiguration`、`providerhealth`、`providerverification`、`provisionsource`、`renewalsource`、`resourceoverview`、`resourceprojection`、`resourcetask`、`serverdirectory`、`serveroperationsource`、`serverownership`、`serverprojection`
- 公开能力目录：`resourcecap`

### idc-v10

- 清单：[`plugin.yaml`](../apps/lina-plugins/idc-v10/plugin.yaml)。
- 服务目录：无该层目录；需查看插件`backend`入口和动态处理器。
- 公开能力目录：无`backend/cap`子目录。

### idc-zjmf-finance

- 清单：[`plugin.yaml`](../apps/lina-plugins/idc-zjmf-finance/plugin.yaml)。
- 服务目录：无该层目录；需查看插件`backend`入口和动态处理器。
- 公开能力目录：无`backend/cap`子目录。

### idc-zjmfcloud

- 清单：[`plugin.yaml`](../apps/lina-plugins/idc-zjmfcloud/plugin.yaml)。
- 服务目录：无该层目录；需查看插件`backend`入口和动态处理器。
- 公开能力目录：无`backend/cap`子目录。

### idc-zjmfmigrate

- 清单：[`plugin.yaml`](../apps/lina-plugins/idc-zjmfmigrate/plugin.yaml)。
- 服务目录：`migrate`
- 公开能力目录：无`backend/cap`子目录。

### org-core

- 清单：[`plugin.yaml`](../apps/lina-plugins/org-core/plugin.yaml)。
- 服务目录：`dept`、`post`
- 公开能力目录：无`backend/cap`子目录。

### shared-services

- 清单：[`plugin.yaml`](../apps/lina-plugins/shared-services/plugin.yaml)。
- 服务目录：`challenge`、`inbox`、`sharedcode`、`sharedsource`、`sharedutil`
- 公开能力目录：`challengecap`、`inboxcap`

### staff-access

- 清单：[`plugin.yaml`](../apps/lina-plugins/staff-access/plugin.yaml)。
- 服务目录：`sourceauth`、`staffaccess`
- 公开能力目录：`staffaccesscap`

### tenant-core

- 清单：[`plugin.yaml`](../apps/lina-plugins/tenant-core/plugin.yaml)。
- 服务目录：`impersonate`、`lifecycleprecondition`、`membership`、`provider`、`resolver`、`resolverconfig`、`shared`、`tenant`、`tenantplugin`
- 公开能力目录：无`backend/cap`子目录。

### vben-adapter

- 清单：[`plugin.yaml`](../apps/lina-plugins/vben-adapter/plugin.yaml)。
- 服务目录：无该层目录；需查看插件`backend`入口和动态处理器。
- 公开能力目录：无`backend/cap`子目录。

### yunsu-ssl

- 清单：[`plugin.yaml`](../apps/lina-plugins/yunsu-ssl/plugin.yaml)。
- 服务目录：无该层目录；需查看插件`backend`入口和动态处理器。
- 公开能力目录：无`backend/cap`子目录。

## 源码插件页面入口

| 声明路由 | 页面源码 |
| --- | --- |
| `/system/notice` | [`notice-management.vue`](../apps/lina-plugins/content-notice/frontend/pages/notice-management.vue) |
| `idc/ai-copilot` | [`ai-copilot.vue`](../apps/lina-plugins/idc-ai/frontend/pages/ai-copilot.vue) |
| `idc/channel` | [`channel-operations.vue`](../apps/lina-plugins/idc-channel/frontend/pages/channel-operations.vue) |
| `channel-tokens` | [`channel-tokens.vue`](../apps/lina-plugins/idc-channel/frontend/pages/customer/channel-tokens.vue) |
| `idc/compliance/electronic-contract` | [`electronic-contract.vue`](../apps/lina-plugins/idc-compliance/frontend/pages/electronic-contract.vue) |
| `idc/compliance/enterprise-verifications` | [`enterprise-verifications.vue`](../apps/lina-plugins/idc-compliance/frontend/pages/enterprise-verifications.vue) |
| `idc/compliance/privacy` | [`privacy-governance.vue`](../apps/lina-plugins/idc-compliance/frontend/pages/privacy-governance.vue) |
| `idc/compliance/providers/aliyun-face` | [`provider-configuration.vue`](../apps/lina-plugins/idc-compliance/frontend/pages/provider-configuration.vue) |
| `idc/compliance/red-letters` | [`red-letter.vue`](../apps/lina-plugins/idc-compliance/frontend/pages/red-letter.vue) |
| `idc/compliance/suppliers` | [`supplier-configurations.vue`](../apps/lina-plugins/idc-compliance/frontend/pages/supplier-configurations.vue) |
| `idc/compliance/tax-invoices` | [`tax-invoice.vue`](../apps/lina-plugins/idc-compliance/frontend/pages/tax-invoice.vue) |
| `idc/assisted-quotes` | [`assisted-quotes.vue`](../apps/lina-plugins/idc-core/frontend/pages/assisted-quotes.vue) |
| `idc/catalog/automation-interfaces` | [`catalog-automation-interfaces.vue`](../apps/lina-plugins/idc-core/frontend/pages/catalog-automation-interfaces.vue) |
| `idc/catalog/categories` | [`catalog-categories.vue`](../apps/lina-plugins/idc-core/frontend/pages/catalog-categories.vue) |
| `idc/catalog` | [`catalog-management.vue`](../apps/lina-plugins/idc-core/frontend/pages/catalog-management.vue) |
| `idc/orders` | [`commerce-orders.vue`](../apps/lina-plugins/idc-core/frontend/pages/commerce-orders.vue) |
| `idc/content` | [`content-management.vue`](../apps/lina-plugins/idc-core/frontend/pages/content-management.vue) |
| `idc/customers` | [`customer-management.vue`](../apps/lina-plugins/idc-core/frontend/pages/customer-management.vue) |
| `idc/finance` | [`financial-management.vue`](../apps/lina-plugins/idc-core/frontend/pages/financial-management.vue) |
| `idc/resource/ip-change-policy` | [`ip-change-policy.vue`](../apps/lina-plugins/idc-core/frontend/pages/ip-change-policy.vue) |
| `idc/marketing` | [`marketing-management.vue`](../apps/lina-plugins/idc-core/frontend/pages/marketing-management.vue) |
| `idc/operations/workbench` | [`operations-workbench.vue`](../apps/lina-plugins/idc-core/frontend/pages/operations-workbench.vue) |
| `idc/catalog/product-directories` | [`product-directories.vue`](../apps/lina-plugins/idc-core/frontend/pages/product-directories.vue) |
| `idc/product-fulfillment/tasks` | [`product-fulfillment-tasks.vue`](../apps/lina-plugins/idc-core/frontend/pages/product-fulfillment-tasks.vue) |
| `idc/sales-channel` | [`sales-channel.vue`](../apps/lina-plugins/idc-core/frontend/pages/sales-channel.vue) |
| `idc/resource/server-operation-security` | [`server-operation-security.vue`](../apps/lina-plugins/idc-core/frontend/pages/server-operation-security.vue) |
| `idc/support` | [`support-desk.vue`](../apps/lina-plugins/idc-core/frontend/pages/support-desk.vue) |
| `idc/support/settings` | [`support-settings.vue`](../apps/lina-plugins/idc-core/frontend/pages/support-settings.vue) |
| `system/configuration-center` | [`system-configuration.vue`](../apps/lina-plugins/idc-core/frontend/pages/system-configuration.vue) |
| `idc/notifications/channels` | [`notification-channels.vue`](../apps/lina-plugins/idc-notify/frontend/pages/notification-channels.vue) |
| `idc/notifications/deliveries` | [`notification-deliveries.vue`](../apps/lina-plugins/idc-notify/frontend/pages/notification-deliveries.vue) |
| `idc/notifications/templates` | [`notification-templates.vue`](../apps/lina-plugins/idc-notify/frontend/pages/notification-templates.vue) |
| `idc/notifications/wechat-work-binding` | [`wechat-work-binding.vue`](../apps/lina-plugins/idc-notify/frontend/pages/wechat-work-binding.vue) |
| `idc/payment-channels` | [`payment-channels.vue`](../apps/lina-plugins/idc-payment/frontend/pages/payment-channels.vue) |
| `idc/payment-exception-funds` | [`payment-exception-funds.vue`](../apps/lina-plugins/idc-payment/frontend/pages/payment-exception-funds.vue) |
| `idc/payment-reconciliation` | [`payment-reconciliation.vue`](../apps/lina-plugins/idc-payment/frontend/pages/payment-reconciliation.vue) |
| `idc/customer-external-menus` | [`customer-external-menus.vue`](../apps/lina-plugins/idc-portal/frontend/pages/customer-external-menus.vue) |
| `idc/portal-brand` | [`portal-brand.vue`](../apps/lina-plugins/idc-portal/frontend/pages/portal-brand.vue) |
| `idc/resource/provider-capacity` | [`provider-capacity-observations.vue`](../apps/lina-plugins/idc-resource/frontend/pages/provider-capacity-observations.vue) |
| `idc/resource/provider-configuration` | [`provider-configuration.vue`](../apps/lina-plugins/idc-resource/frontend/pages/provider-configuration.vue) |
| `idc/resource/health` | [`provider-health.vue`](../apps/lina-plugins/idc-resource/frontend/pages/provider-health.vue) |
| `idc/resource/tasks` | [`provider-tasks.vue`](../apps/lina-plugins/idc-resource/frontend/pages/provider-tasks.vue) |
| `idc/resource/overview` | [`resource-overview.vue`](../apps/lina-plugins/idc-resource/frontend/pages/resource-overview.vue) |
| `idc/resource/server-accessories` | [`server-accessories.vue`](../apps/lina-plugins/idc-resource/frontend/pages/server-accessories.vue) |
| `idc/resource/server-renewals` | [`server-batch-renewal.vue`](../apps/lina-plugins/idc-resource/frontend/pages/server-batch-renewal.vue) |
| `/system/dept` | [`dept-management.vue`](../apps/lina-plugins/org-core/frontend/pages/dept-management.vue) |
| `/system/post` | [`post-management.vue`](../apps/lina-plugins/org-core/frontend/pages/post-management.vue) |
| `system/staff-access` | [`staff-access.vue`](../apps/lina-plugins/staff-access/frontend/pages/staff-access.vue) |
| `platform/tenants` | [`tenant-management.vue`](../apps/lina-plugins/tenant-core/frontend/pages/tenant-management.vue) |
| `/tenant/plugins` | [`tenant-plugin-management.vue`](../apps/lina-plugins/tenant-core/frontend/pages/tenant-plugin-management.vue) |

这些是静态页面注册声明，不等于当前用户菜单可见或具有接口操作权限；动态插件隔离页面另查清单和运行时菜单。

## 优先阅读的大文件

行数只是阅读拆分与影响分析信号，不是缺陷结论；排除了测试、`SQL`和常见生成目录。

| 文件 | 行数 |
| --- | ---: |
| [`apps/lina-vben/packages/effects/layouts/src/authentication/icons/slogan.vue`](../apps/lina-vben/packages/effects/layouts/src/authentication/icons/slogan.vue) | 4569 |
| [`apps/lina-plugins/idc-core/frontend/pages/marketing-management.vue`](../apps/lina-plugins/idc-core/frontend/pages/marketing-management.vue) | 2833 |
| [`apps/lina-plugins/idc-core/backend/plugin.go`](../apps/lina-plugins/idc-core/backend/plugin.go) | 2573 |
| [`apps/lina-plugins/idc-notify/frontend/src/App.vue`](../apps/lina-plugins/idc-notify/frontend/src/App.vue) | 1946 |
| [`apps/lina-plugins/idc-core/frontend/pages/financial-management.vue`](../apps/lina-plugins/idc-core/frontend/pages/financial-management.vue) | 1666 |
| [`apps/lina-vben/apps/web-customer/src/views/account/servers/index.vue`](../apps/lina-vben/apps/web-customer/src/views/account/servers/index.vue) | 1652 |
| [`apps/lina-plugins/idc-core/frontend/pages/components/customer-overview-drawer.vue`](../apps/lina-plugins/idc-core/frontend/pages/components/customer-overview-drawer.vue) | 1573 |
| [`apps/lina-plugins/idc-zjmfcloud/backend/internal/provider/zjmfcloud/operations.go`](../apps/lina-plugins/idc-zjmfcloud/backend/internal/provider/zjmfcloud/operations.go) | 1544 |
| [`apps/lina-plugins/idc-core/backend/channel_priced_lifecycle.go`](../apps/lina-plugins/idc-core/backend/channel_priced_lifecycle.go) | 1512 |
| [`hack/tools/linactl/internal/wasmbuilder/wasmbuilder_backend.go`](../hack/tools/linactl/internal/wasmbuilder/wasmbuilder_backend.go) | 1509 |
| [`apps/lina-plugins/idc-compliance/frontend/pages/components/electronic-contract-manager.vue`](../apps/lina-plugins/idc-compliance/frontend/pages/components/electronic-contract-manager.vue) | 1501 |
| [`apps/lina-core/internal/service/plugin/internal/testutil/testutil_services.go`](../apps/lina-core/internal/service/plugin/internal/testutil/testutil_services.go) | 1388 |
| [`apps/lina-vben/apps/web-antd/src/views/system/plugin/index.vue`](../apps/lina-vben/apps/web-antd/src/views/system/plugin/index.vue) | 1349 |
| [`apps/lina-plugins/idc-compliance/backend/internal/service/electroniccontractpreparation/electroniccontractpreparation_operations.go`](../apps/lina-plugins/idc-compliance/backend/internal/service/electroniccontractpreparation/electroniccontractpreparation_operations.go) | 1333 |
| [`apps/lina-plugins/idc-compliance/frontend/pages/components/electronic-contract-template-editor.vue`](../apps/lina-plugins/idc-compliance/frontend/pages/components/electronic-contract-template-editor.vue) | 1321 |
| [`hack/tools/linactl/internal/plugins/plugins_management.go`](../hack/tools/linactl/internal/plugins/plugins_management.go) | 1320 |
| [`apps/lina-plugins/idc-resource/backend/internal/service/providerconfiguration/providerconfiguration.go`](../apps/lina-plugins/idc-resource/backend/internal/service/providerconfiguration/providerconfiguration.go) | 1281 |
| [`apps/lina-plugins/idc-compliance/central-gateway/internal/suppliercallback/reconciliation.go`](../apps/lina-plugins/idc-compliance/central-gateway/internal/suppliercallback/reconciliation.go) | 1260 |
| [`apps/lina-vben/apps/web-customer/src/views/account/finance/index.vue`](../apps/lina-vben/apps/web-customer/src/views/account/finance/index.vue) | 1239 |
| [`apps/lina-plugins/idc-compliance/frontend/pages/tax-invoice.vue`](../apps/lina-plugins/idc-compliance/frontend/pages/tax-invoice.vue) | 1222 |
| [`hack/tools/linactl/internal/wasmbuilder/wasmbuilder_dispatcher.go`](../hack/tools/linactl/internal/wasmbuilder/wasmbuilder_dispatcher.go) | 1211 |
| [`apps/lina-plugins/idc-core/frontend/pages/customer-management.vue`](../apps/lina-plugins/idc-core/frontend/pages/customer-management.vue) | 1209 |
| [`apps/lina-plugins/idc-core/backend/internal/service/commerce/commerce_checkout.go`](../apps/lina-plugins/idc-core/backend/internal/service/commerce/commerce_checkout.go) | 1207 |
| [`apps/lina-plugins/idc-resource/frontend/pages/provider-configuration.vue`](../apps/lina-plugins/idc-resource/frontend/pages/provider-configuration.vue) | 1158 |
| [`apps/lina-vben/packages/effects/common-ui/src/components/resize/resize.vue`](../apps/lina-vben/packages/effects/common-ui/src/components/resize/resize.vue) | 1121 |
