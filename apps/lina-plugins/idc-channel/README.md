# idc-channel

`idc-channel` is the built-in source plugin for IDC downstream-channel integrations.

English | [简体中文](README.zh-CN.md)

## Scope

The plugin owns downstream access tokens, product publication, request evidence, compatibility adapters, and webhook delivery. Core remains authoritative for orders, money, and services.

## Dependencies

The plugin depends only on the public `idccap` contract from `idc-core`.

## Delivery

Version `v0.1.0` is delivered as `source + builtin` with `platform_only`, `supports_multi_tenant: false`, and `global` governance.

## Delivered API surface

The admin page enters through a version-pinned HTML entry inside the isolated plugin sandbox. Data requests use the page bridge, and the plugin never receives the host login token.

The plugin exposes the mounted `/x/idc-channel/api/v1/channel/...` contract for:

- one-time token issuance and revocation (only a SHA-256 digest is persisted);
- a sanitized published catalog (price, currency, stock and product-version projection);
- idempotent channel order creation and order lookup, with ownership mode and currency retained;
- a verified capability matrix. LinaPro's generic API is enabled; the three external adapters remain explicitly disabled until formal contract evidence is available.

Order creation is intentionally a safe first step: new orders enter `pending_identity` and must pass the host's identity and financial gates before any fulfillment call. Request keys are bound to the complete order payload, so reusing a key with changed data is rejected.
