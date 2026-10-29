# Plugin development and installation guide

This is the development entry for all current plugins. Each linked plugin document describes actual capabilities, constraints, and verification limits. Business tabs are extensible presentation configuration, separate from runtime type, extension contracts, and installation permission.

## Contracts and installation gates

- Manifest identifiers must match directory names. Versions, runtime type, distribution, dependencies, public resources, and surfaces must be valid. Built-in plugins are assembled automatically and cannot be uninstalled; managed dynamic plugins use the governed lifecycle.
- Providers declare `point`, contract `version`, `owner`, `role=provider`, and a versioned owner dependency. Methods, schemas, and metadata must pass the owner's public validator. Missing validators or rejected declarations must block installation and updates. A category, README, or successful compilation never replaces contract validation.
- Providers own their configuration schemas; business owners retain secrets. Actual plugins define product fields, purchase fields, and customer resource lists. Server-specific fields must not be imposed on SSL, domains, or future products. Menus and pages use published surfaces; backend identity and ownership checks remain mandatory.
- Stopping new business blocks new sales and bindings while retaining existing services. Disabling and uninstalling require dependency and business-impact checks. Never edit state directly, delete artifacts, skip authorization, or fabricate success.

| Business | Public contract and validator | Installation requirement |
| --- | --- | --- |
| Fulfillment and cloud products | [`productfulfillmentcap`](idc-core/backend/cap/productfulfillmentcap/declaration.go) | `product-fulfillment@v1` and typed metadata; declare actual fields and operations |
| Server connectors | [`resourcecap`](idc-resource/backend/cap/resourcecap/dynamic_provider_contract.go) | `resource-provider@v1`, confidential outbound and verified protocols; commonly paired with fulfillment |
| Payments | [`paymentcap`](idc-payment/backend/cap/paymentcap/dynamic_provider_contract.go) | `payment-method@v1`; configuration descriptors do not become independent financial executors |
| SMS and notifications | [`channelcap`](idc-notify/backend/cap/channelcap/channel.go) | Currently a source-adapter SPI; dynamic `sms-channel` validation is unpublished and installation remains rejected |
| Websites | [Guide](idc-portal/docs/website-plugin-development.zh-CN.md), [`websitecap`](idc-portal/backend/cap/websitecap/contract.go) | `website-template@v1`, actual packaged pages, public assets, and settings |
| Customer centers | [Guide](idc-portal/docs/customer-plugin-development.zh-CN.md), [`customertemplatecap`](idc-portal/backend/cap/customertemplatecap/contract.go) | `customer-template@v1`, independent build and customer identity boundaries |
| Migration and system features | [Migration plugin](idc-zjmfmigrate/README.md), [public host plugin interface](../lina-core/pkg/plugin/README.md) | Declare actual routes, permissions, and governed host services; do not invent unnecessary business extensions |

Contract identifiers such as `product-fulfillment` and `resource-provider` are independent of management tabs. Future businesses can publish an owner contract and validator before adding their presentation category.

## Development, verification, and delivery

1. Read repository instructions, local plugin rules, and the plugin's bilingual documentation. Consume other plugins through public contracts rather than private implementations.
2. Use each plugin's actual lockfiles and build entry. Independent websites and customer centers retain standalone builds. Installable artifacts are `.wasm` packages, not source directories or plain Go WASM binaries without packaged metadata.
3. From the repository root run `go run ./.agents/tools/plugincheck/main.go` to audit every manifest, bilingual documentation, and public provider contract, including rejection of invalid metadata. The tool creates its own temporary workspace and log without changing shared workspaces or running instances.
4. Run `make plugins.check`, relevant Go tests, and `make lint dir=apps/lina-plugins/<id>`; cover frontend behavior with E2E tests. This audit does not replace real API, money, authorization, artifact, or failure-recovery validation.
5. Installation and updates still use public lifecycle APIs and server validators. Upload success is not installation or update completion. Real third-party writes require authorization for the specific test objects.

## Plugin index

| Plugin documentation | Runtime | Distribution | Declared extension points |
| --- | --- | --- | --- |
| [`content-notice`](content-notice/README.md) | `source` | `builtin` | `—` |
| [`customer-default`](customer-default/README.md) | `dynamic` | `managed` | `customer-template` |
| [`customer-tdesign`](customer-tdesign/README.md) | `dynamic` | `managed` | `customer-template` |
| [`idc-ai`](idc-ai/README.md) | `source` | `builtin` | `—` |
| [`idc-baota`](idc-baota/README.md) | `dynamic` | `managed` | `product-fulfillment, resource-provider` |
| [`idc-channel`](idc-channel/README.md) | `source` | `builtin` | `—` |
| [`idc-compliance`](idc-compliance/README.md) | `source` | `builtin` | `—` |
| [`idc-core`](idc-core/README.md) | `source` | `builtin` | `product-fulfillment` |
| [`idc-dcim`](idc-dcim/README.md) | `dynamic` | `managed` | `product-fulfillment, resource-provider` |
| [`idc-notify`](idc-notify/README.md) | `source` | `builtin` | `sms-channel` |
| [`idc-payment`](idc-payment/README.md) | `source` | `builtin` | `payment-method` |
| [`idc-payment-alipay`](idc-payment-alipay/README.md) | `dynamic` | `managed` | `payment-method` |
| [`idc-payment-epay`](idc-payment-epay/README.md) | `dynamic` | `managed` | `payment-method` |
| [`idc-payment-unionpay`](idc-payment-unionpay/README.md) | `dynamic` | `managed` | `payment-method` |
| [`idc-payment-wechatpay`](idc-payment-wechatpay/README.md) | `dynamic` | `managed` | `payment-method` |
| [`idc-peer`](idc-peer/README.md) | `dynamic` | `managed` | `product-fulfillment, resource-provider` |
| [`idc-portal`](idc-portal/README.md) | `source` | `builtin` | `customer-template, website-template` |
| [`idc-resource`](idc-resource/README.md) | `source` | `builtin` | `resource-provider` |
| [`idc-v10`](idc-v10/README.md) | `dynamic` | `managed` | `product-fulfillment, resource-provider` |
| [`idc-zjmf-finance`](idc-zjmf-finance/README.md) | `dynamic` | `managed` | `product-fulfillment, resource-provider` |
| [`idc-zjmfcloud`](idc-zjmfcloud/README.md) | `dynamic` | `managed` | `product-fulfillment, resource-provider` |
| [`idc-zjmfmigrate`](idc-zjmfmigrate/README.md) | `dynamic` | `managed` | `—` |
| [`org-core`](org-core/README.md) | `source` | `builtin` | `—` |
| [`plugin-manager`](plugin-manager/README.md) | `source` | `builtin` | `—` |
| [`shared-services`](shared-services/README.md) | `source` | `builtin` | `—` |
| [`tenant-core`](tenant-core/README.md) | `source` | `builtin` | `—` |
| [`vben-adapter`](vben-adapter/README.md) | `source` | `builtin` | `—` |
| [`website-default`](website-default/README.md) | `dynamic` | `managed` | `website-template` |
| [`website-html`](website-html/README.md) | `dynamic` | `managed` | `website-template` |
| [`website-minimal`](website-minimal/README.md) | `dynamic` | `managed` | `website-template` |
| [`website-qycn`](website-qycn/README.md) | `dynamic` | `managed` | `website-template` |
| [`yunsu-ssl`](yunsu-ssl/README.md) | `dynamic` | `managed` | `product-fulfillment` |
