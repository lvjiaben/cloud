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

The connector declares administration and customer certificate pages. Each page can call only the exact `idc-core` service capability listed by its frozen surface manifest. The host resolves the owner namespace, checks the current session, permission, data scope, and customer ownership, and then issues a short-lived grant for one exact method and path. Service mutations run as durable, query-first tasks with bounded retries, result-unknown handling, and staff takeover.

Certificate-chain download uses a host-owned, two-minute, one-use ticket. The host downloads and sanitizes the supplier archive into private temporary storage and returns only public certificates and chains. The WASM module never receives the supplier ZIP, private keys, supplier credentials, or browser token. Renewal remains unavailable in this release.

## Build and test

```powershell
$env:GOWORK='off'
go test ./...
$env:GOWORK=$null
Set-Location E:\golang\cloud
go run ./hack/tools/linactl wasm dir=apps/lina-plugins/yunsu-ssl out=temp/output/yunsu-ssl
```

Production verification requires a lawful isolated Yunsu account and dedicated test certificate orders. Existing customer certificates must never be used.
