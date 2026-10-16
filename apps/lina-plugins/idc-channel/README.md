# Open API

English | [简体中文](README.zh-CN.md)

`idc-channel`is a built-in source plugin owning customer-bound `AppID/AppSecret` credentials, method permissions, and request evidence. Public `idc-core` capabilities own customers, product prices, orders, money, and services.

## Administration

In Customer Business → Open API, select an existing customer, choose allowed operations, and generate credentials. Secrets appear once and can be rotated or revoked; applications can be disabled. The native Vue page contains guidance and examples. Authenticate with HTTP Basic using AppID as the username and AppSecret as the password. Production connections require HTTPS.

## Purchases and ownership

1. Browse products, read their configuration, remotely evaluate selections, and read billing terms for the selected currency.
2. POST `/channel/quotes` to freeze the bound customer's price. Customer and agent pricing use the same rules as storefront purchases.
3. POST `/channel/orders` with the returned `quoteNo`. Orders, invoices, stock reservations, and same-currency balance debits commit atomically; insufficient balance rejects the transaction.
4. Query the order by its identifier or original request identifier, then list its services. Reconcile uncertain responses before any retry.

Orders, debits, and services belong to the credential's customer. The API does not create downstream end customers, exchange currencies, or fall back to credit. Retries must retain identical request identifiers and content. Renewals use the service-operation endpoint with persisted progress queries.

For file configuration fields, POST `/channel/catalog/products/{publicSlug}/file-uploads` to reserve an upload, PUT raw bytes to `/channel/file-uploads/{uploadToken}/content`, then use the returned file token in its configuration field. Files belong to the bound customer's active account. Foreign customers, mismatched fields and files consumed by another order are rejected. Size, media type and safety policy inspection must pass before quoting or checkout.

GET `/channel/orders/{orderNo}/deliveries` returns one-time deliveries and all linked services, including each service's currency and billing period. After an unknown renewal response, GET `/channel/services/{serviceNo}/renewals/by-request/{requestId}` reconciles the original request without charging for a second renewal.

## API boundaries

The prefix is `/x/idc-channel/api/v1`. Cursor catalog pages contain at most 24 products; numbered `/channel/catalog/products` pages allow up to 100. Configuration snapshots contain only public fields and opaque selection tokens. Billing previews show ordinary prices; frozen quotes contain the actual customer price.

Configuration evaluation, billing terms and import candidates use GET. The `values` query is a URL-encoded JSON array string with at most 128 items and 32768 UTF-8 bytes; omitting it means no selections. Delivery and renewal recovery timestamps are Unix milliseconds or null when unavailable.

Authenticated capabilities, order deliveries and renewal recovery also return `observedAt` as Unix milliseconds from the source HTTP runtime. Managed connectors use this response observation for connection verification, capability discovery and renewal status; they reject missing observations instead of relying on sandbox wall time.

Filter services by `orderNo` and continue with the last `serviceNo` as `afterServiceNo`. Query progress with GET `/channel/services/{serviceNo}/operations/{operationNo}`. GET `/channel/orders/by-request/{requestId}` reads only the current application's order and waits for a concurrent submission transaction using that identifier.

The plugin consumes `idc-core/backend/cap/channelcommercecap` and never reads another plugin's private tables. Unverified ZJMF and WHMCS compatibility remains disabled. Same-system automatic import and procurement additionally require `idc-peer` and governed configuration mappings; the foundational API alone is not the completed delivery.

## Customer open platform

Authorized customers automatically receive an Open platform menu with the API address, `AppID`, policies, expiry and documentation. Secret rotation shows the replacement once, immediately revokes the previous secret and preserves administrator policies. Disabled applications retain history and cannot be restored by customers.

Call history starts when this feature is deployed. It includes successful calls and failures whose secret proves application ownership. Invalid secrets cannot safely be attributed to another customer. Logs exclude bodies, query parameters, credentials and confidential output. Source-plugin initialization uses public router readiness and menu-store APIs for its declared surface without host changes.
