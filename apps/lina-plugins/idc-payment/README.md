# idc-payment

`idc-payment` is the built-in source plugin for IDC payment integrations.

English | [简体中文](README.zh-CN.md)

## Scope

The plugin owns payment channels, payment and refund attempts, callback evidence, active queries, and reconciliation.

## Dependencies

The plugin depends only on the public `idccap` contract from `idc-core`.

## Delivery

Version `v0.1.0` is delivered as `source + builtin` with `platform_only`, `supports_multi_tenant: false`, and `global` governance.

## Payment addresses

In **Payment Channels → Payment notification and return addresses**, configure the public notification origin separately from the customer browser return origin. Notifications may pass through an independent public gateway; browser returns should point to the customer center. Saving applies immediately to subsequent requests without publishing or restarting. Leave the notification origin empty until a public gateway is available; new online payments remain disabled.

The gateway must forward `/x/idc-payment/api/v1/callbacks/payments/{channelNo}` and `/x/idc-payment/api/v1/callbacks/refunds/{channelNo}` with the original body and signature headers. Payment notifications also accept EPay's documented signed `GET` protocol and preserve the original query string; other providers and refunds retain `POST`. Duplicate query keys are rejected. Browser returns never determine payment success; verified notifications or authoritative queries drive financial state.

### EPay entry points and mode editing

EPay supports the default cashier and explicit Alipay or WeChat entry points. Explicit entry points include `type=alipay` or `type=wxpay` in the signed `submit.php` form, following the provider's [V1 protocol](https://www.ezfp.cn/doc_old.html). Existing channels can update their enabled modes without re-entering credentials. The update preserves the encrypted secret handle, connection verification, and historical payment facts; current provider declarations, adapter capabilities, staff authorization, and the channel version are checked again.

A generated provider trade number proves request acceptance, not a usable cashier or settlement. Record upstream error pages separately; a temporary notification origin can validate initiation while real callback and settlement acceptance remain pending.

## Managed payment package lifecycle

A `payment-method/v1` guest exposes only the read-only `capabilities.describe` declaration. This built-in plugin executes payments, refunds, callbacks, queries, and reconciliation through its adapters and retains channels, secret references, transaction evidence, and history. The payment owner registers the public business-impact handler and returns an empty transactional-impact collection for this descriptor-only guest. The host still checks every other owner, authorization, stopped admission, and dependencies. Adding execution methods requires reassessing and testing the impact policy; this result cannot be reused for executable payment extensions.
