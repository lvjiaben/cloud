# Same-system connector

English | [简体中文](README.zh-CN.md)

`idc-peer`connects to another installation through its public Open API using credentials bound to one upstream customer. It never creates upstream accounts for the downstream installation's end customers.

## Connection workflow

Create a customer-bound application in the upstream Open API page and allow catalog, quotes, orders, services and renewal operations. Configure its address and credentials in this installation's resource interface. The resource owner's confidential outbound boundary injects `AppID/AppSecret`; the guest never receives plaintext credentials.

Imports use the complete public sale contract, including configuration rules, public choices, billing periods, tax and pricing. Dynamic choices remain subject to upstream evaluation. Procurement refreshes the billing token for the frozen currency and period, freezes the bound customer's actual price and consumes only the matching currency balance. Insufficient balance never creates debt orders.

## Delivery and recovery

Unknown purchase responses are reconciled by the original request identifier; changing that identifier could charge twice. One-time deliveries create no invented ongoing entitlement. Multiple services retain their individual upstream service identifiers.

Renewal matches the frozen upstream service, currency, billing period and expiry, submits an upstream renewal and queries its original request identifier. It never extends only the local expiry. Configuration files stream through core-owned consumed-order evidence and the resource confidential outbound boundary; the guest receives neither object paths nor credentials.

An uploaded upstream file token expires after seven days if it has not been consumed by an order. Normal retries reuse that token. An outage beyond its lifetime causes an explicit failure and manual follow-up; the worker never changes request identifiers to charge again. Check the original upstream request's order result before recovering it.

Complete two-installation acceptance for the current version before delivery. Passing unit tests alone does not prove that acceptance.

## Validation

Connection verification and discovery consume `observedAt` from the authenticated upstream capabilities response. Renewal tasks use authenticated delivery and renewal-query observations. Missing or invalid observations fail closed; the managed WASM connector never manufactures wall time locally.

Run `GOWORK=off go test ./backend/... -count=1` in this directory to verify the managed manifest, purchase protocol, balance rejection, read-only recovery after a lost response, and service ownership. These tests do not replace integration testing against two real installations.

Restricted sale snapshots use protocol `2` and preserve `fulfillment.maximumPurchaseCycles` through the explicit response projection. A missing limit in protocol `2` is rejected; ordinary protocol `1` snapshots retain their previous behavior. This lets an older connector reject an unsupported restricted source instead of silently offering extra cycles.
