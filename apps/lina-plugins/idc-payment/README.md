# idc-payment

`idc-payment` is the built-in source plugin for IDC payment integrations.

English | [简体中文](README.zh-CN.md)

## Scope

The plugin owns payment channels, payment and refund attempts, callback evidence, active queries, and reconciliation.

## Dependencies

The plugin depends only on the public `idccap` contract from `idc-core`.

## Delivery

Version `v0.1.0` is delivered as `source + builtin` with `platform_only`, `supports_multi_tenant: false`, and `global` governance.

## Phase Boundary

Phase one provides registration, dependency, i18n, test, and capability-switch scaffolding. It does not yet implement payment flows or persist business data.
