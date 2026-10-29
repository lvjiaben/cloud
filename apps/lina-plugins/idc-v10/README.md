# IDC V10 Provider

This managed WASM connector integrates **IDCSmart Business System V10** through its independent supplier protocol. It is not ZJMF Finance, WHMCS `/v1`, or the ZJMFCloud administrator API.

## Implemented contract

- Each interface instance has its own endpoint, API username, and API password. Administrators may create multiple instances.
- The host signs in through `POST /api/v1/auth`, retains the returned JWT, and adds `Authorization: Bearer ...` only inside the confidential outbound proxy.
- Product discovery uses `GET /api/v1/product`.
- Selected products are resolved through `GET /api/v1/product/:id/resource`. Only the explicitly identified `idcsmart_common` protocol is admitted for generic resale; remote module ZIPs are never downloaded or executed.
- Provisioning follows the official query-first console workflow: delete/check `/console/v1/cart`, add the selected product, settle the cart, and pay through `/console/v1/pay` when required.
- Suspend, resume, terminate, and resource detail use the reviewed `/console/v1/host/...` and `/console/v1/refund` routes.
- Unverified module-specific actions remain disabled instead of being guessed.

The callback token, supplier credentials, JWT, and raw supplier responses stay in `idc-resource`. Only field-whitelisted business projections cross the WASM boundary. A timeout after a potentially accepted request returns an unknown result for reconciliation; the connector does not blindly submit again.

## Source evidence

The implementation is based on the official IDCSmart Business System 10.4.6 source, especially `SupplierModel`, `UpstreamLogic`, and `ResModuleLogic`. The connector keeps the V10 routes separate from the native ZJMF Finance `/zjmf_api_login` flow.


## Evaluated common-module resale

Version `v0.2.4` uses the official 10.4.6 common module's `reidcsmart_common/.../configoption` and `/calculate` APIs. The wrapper supports both native and already-resold products. This API exposes selected whole-product prices rather than an arbitrary option-price table, so the connector returns `remote_evaluation`, keeps all source configuration read-only, and requests current account prices with `is_downstream=1`. It does not promote a product's starting price to a complete static schedule or invent a peer quote token. The exact source module must be supported before import.

Native parameters preserve the real `config_options[cycle]`, scalar choices, array quantities and multiselects, bounded repeated quantities, and `self_defined_field` customer inputs. Source protocol limits and unrepresentable fields fail closed; the existing query-first cart workflow retains resource-owned callback correlation. Local HTTPS protocol tests do not constitute successful transactions on an actual supplier account.
