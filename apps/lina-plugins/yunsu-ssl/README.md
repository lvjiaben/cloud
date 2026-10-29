# yunsu-ssl

`yunsu-ssl` is the managed dynamic fulfillment connector for Yunsu SSL certificates.

English | [简体中文](README.zh-CN.md)

## Current capabilities

The connector joins generic commerce through `product-fulfillment@v1` owned by `idc-core`. It currently provides:

- product candidates from `POST /product/list`;
- immutable DV/OV/EV and DNS/file capability facts frozen from the selected supplier product instead of merchant re-entry;
- durable asynchronous issuance through `POST /payment/balance` after payment confirmation or staff approval;
- issuance-state queries through `POST /order/sync`;
- pure pre-issuance configuration validation without a supplier request;
- new automatic publication is blocked until supplier domain types and quantity limits are documented; manual delivery remains available and existing certificates retain their frozen configuration;
- ordinary, multi-domain, wildcard, and IP certificate constraints, including matching DNS/file validation capabilities;
- mandatory customer-supplied, signature-checked CSR input that must exactly match the requested identifiers;
- complete organization and contact information for OV and EV products;
- strict adapters for `POST /order/detail`, `POST /order/update-dcv`, `POST /order/revalidate`, and `POST /order/cancel`;
- cancellation and validation changes only before issuance, with cancellation never implying a refund;
- result-unknown handling on submission timeout without an automatic repeat charge;
- host-injected API key and secret headers that never enter WASM, logs, or ordinary responses.

## Security boundary

The supplier endpoint must use HTTPS. The host pins every request to the configured origin, verifies TLS, and rejects redirects, system proxies, private addresses, and DNS rebinding. The guest submits only configuration field names and safe business data. Supplier responses are field-whitelisted and rejected if credential material is echoed.

## Owner-governed service pages

The connector declares a customer certificate page. Administration is embedded in the corresponding customer product management drawer and uses the existing `idc-core` entitlement APIs. The customer page can call only the exact `idc-core` service capability listed by its frozen surface manifest. The host resolves the owner namespace, checks the current session, permission, data scope, and customer ownership, and then issues a short-lived grant for one exact method and path. Service mutations run as durable, query-first tasks with bounded retries, result-unknown handling, and staff takeover.

Certificate-chain download uses a host-owned, two-minute, one-use ticket. The host downloads and sanitizes the supplier archive into private temporary storage and returns only public certificates and chains. The WASM module never receives the supplier ZIP, private keys, supplier credentials, or browser token. Renewal remains unavailable in this release.

## Build and test

Run from the repository root on Windows, Linux, or macOS:

```text
go run ./hack/tools/linactl build dir=apps/lina-plugins/yunsu-ssl
go run ./hack/tools/linactl test dir=apps/lina-plugins/yunsu-ssl
pnpm --dir apps/lina-plugins/yunsu-ssl/frontend test
```

The `build` command installs the locked frontend dependencies, builds the customer certificate page, and embeds them into `temp/output/yunsu-ssl.wasm`. A clean checkout must use `build` before the low-level `wasm` packaging command; `wasm` alone requires the declared `frontend/dist` assets to exist. Upload the package in plugin management, install it, then enable it to expose the customer surface and product management actions. Supplier credentials and certificate products are configured separately; enabling the plugin does not create a supplier order.

Production verification requires a lawful isolated Yunsu account and dedicated test certificate orders. Existing customer certificates must never be used.

## Purchased-content integration

The existing `certificates` customer surface also declares `purchased-services.customer.v1`. Supporting customer templates embed the same certificate page in the selected member navigation block without adding a duplicate menu. The plugin retains its own domain, validation, certificate status and action rules; server IP and operating-system columns are never imposed. The container fixes the current directory and fulfillment plugin on the owner list request, while the existing exact page grant and current-customer checks remain mandatory. Core `>=0.1.171` applies the paid-only boundary.
