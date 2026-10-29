# Product fulfillment connector contract

English | [简体中文](README.zh-CN.md)

New cloud products integrate through the `idc-core`-owned `product-fulfillment@v1` extension. Do not add a product-kind enum or copy server tables into the commerce core. The platform owns commercial options, quotes, orders, payment settlement and frozen purchase facts; a connector owns the actual supplier protocol and explicitly supported operations.

## Declare the product

| Declaration | Responsibility |
| --- | --- |
| `interface_configuration` | Supplier connection settings; credentials use protected secret fields |
| `product_configuration` | Merchant-maintained supplier parameters and their real limits |
| `customer_configuration` | Typed customer input, candidate options and conditions |
| `provider_metadata` | [Provider declaration](declaration.go), fulfillment granularity, management and renewal facts |

The merchant editor supplies commercial names, order, sellable choices and their prices. A local unbound information field remains with the order. Sending it upstream requires an explicit parameter name. Declared mappings retain their typed limits; merchant-defined native names and scalar contents are frozen in `custom_parameters` using [the custom parameter contract](custom_parameters.go). Direct provisioning connectors must translate this envelope into their final outbound form, or explicitly reject it; they must not silently discard it. Credentials, supplier account identity and paid unit quantity remain owner controlled. `productChoiceFields` opts supported product text parameters into pure batch validation; merchant `productChoiceBindings` maps commercial choice IDs to those actual values. `customerProductFields` declares customer replacements for product parameters. The connector owns unit conversion; the core must not infer that a number means memory, bandwidth or a certificate count.

Candidate lists and relations must reflect supplier facts, with bounded requests and field whitelists. The editor, preview and purchase form use these declared relations. A retained directory is not a real-time capacity guarantee. [Server](../../../../idc-zjmfcloud/plugin.yaml) and [certificate](../../../../yunsu-ssl/plugin.yaml) declarations illustrate different schemas; their existence is not proof of supplier production acceptance. The certificate connector currently blocks automatic publication where supplier domain capabilities are unconfirmed.

## Fulfill a paid purchase

Implement distinct provision and query methods from [the request/result contract](contract.go). Use the supplied immutable purchase configuration and idempotency key, rather than rebuilding an old order from today's product. Respect `per_order_item` versus `per_quantity_unit` granularity. Preserve external references for accepted asynchronous work. Report success only after supplier confirmation; a timeout or uncertain acceptance must remain `result_unknown`, never a silent resubmission or a fabricated rejection.

For ongoing services, declare the real entitlement owner and supported management actions in [the service action contract](service_action.go). One-time delivery must not invent a renewable service. Renewal follows [the declared billing/renewal facts](billing.go): provider-confirmed, local entitlement or unavailable. Trial support requires explicit pause/resume and formal renewal support. An installed connector does not imply those capabilities.

[Pure configuration validation](configuration_validation.go) may not access credentials, call suppliers or write state. Declare `valid` explicitly in every result. Unknown or malformed results cannot approve a save. The shared [admission helper](../provideradmissioncap/provideradmissioncap.go) waits only for a typed host refusal before callback entry; it does not make a supplier write safe to repeat.

## Verify an integration

Run contract tests from the repository root with its prepared plugin workspace:

```text
go -C temp/output/.build test lina-plugin-idc-core/backend/cap/productfulfillmentcap -count=1
```

Then verify the actual connector and its owner using a dedicated local supplier fixture: merchant save, customer selection and price, payment settlement, final supplier payload, query recovery, ownership and management readiness. Include explicit rejection and uncertain-result cases. Existing generic-form and paid-server examples are in [core plugin E2E tests](../../../hack/tests/e2e/). Passing a mock does not establish real supplier availability, billing or lifecycle support. Third-party writes require their own authorization.

## Native imported sales

`ProviderDeclaration.saleImportProtocol` explicitly admits native financial catalogs with `native_finance_v1`, or V10 common-module products with `native_v10_common_v1`, through the resource owner. Admission still matches the exact provider release, declaration, interface revision and owner-generated catalog evidence; plugin names do not grant this capability. The protocol requires per-unit ongoing resource ownership.

The frozen `provisionresourcecap.TaskRequest.NativePurchaseProtocol` identifies the exact protocol, and `NativePurchase` carries one paid `channelcommercecap.Purchase`, separately from server plans. The resource owner persists its native task, callback correlation and `downstream_token`; it does not create a server pool, upstream customer account or login password. Generic imported entitlements do not become server projections.

`ImportFulfillment.maximumPurchaseCycles` is a portable source ceiling. Zero preserves prior snapshots. Native financial sales attest one cycle, and all subsequent imports and exports retain that limit. Cart resolution and checkout quotation reject excess cycles before accepting payment.


Static imports with a source cycle ceiling or visibility/default policy use snapshot protocol `2`; old connectors reject them instead of discarding restrictions. Dynamic pricing uses protocol `3` with `pricingMode=remote_evaluation`, whose schedules contain no static amounts. Raw financial-system price tables do not substitute for authenticated account prices. Configuration quotes, carts and checkout acceptance read current prices through frozen interface evidence inside the resource owner, then apply merchant markup or whole-item replacements. Changed source price or configuration evidence invalidates a quote; replaying an accepted order never requotes it. Re-export of dynamically priced imports is explicitly unavailable: multi-level dynamic resale is not supported.
