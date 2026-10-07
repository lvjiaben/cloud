# IDC V10 Provider

This managed WASM connector integrates **IDCSmart Business System V10** through its independent supplier protocol. It is not ZJMF Finance, WHMCS `/v1`, or the ZJMFCloud administrator API.

## Implemented contract

- Each interface instance has its own endpoint, API username, and API password. Administrators may create multiple instances.
- The host signs in through `POST /api/v1/auth`, retains the returned JWT, and adds `Authorization: Bearer ...` only inside the confidential outbound proxy.
- Product discovery uses `GET /api/v1/product`.
- V10 module configuration HTML is converted inside the isolated connector into bounded host candidates. Raw HTML and internal configuration JSON are never shown to customers.
- Provisioning follows the official query-first console workflow: delete/check `/console/v1/cart`, add the selected product, settle the cart, and pay through `/console/v1/pay` when required.
- Suspend, resume, terminate, and resource detail use the reviewed `/console/v1/host/...` and `/console/v1/refund` routes.
- Unverified module-specific actions remain disabled instead of being guessed.

The callback token, supplier credentials, JWT, and raw supplier responses stay in `idc-resource`. Only field-whitelisted business projections cross the WASM boundary. A timeout after a potentially accepted request returns an unknown result for reconciliation; the connector does not blindly submit again.

## Source evidence

The implementation is based on the official IDCSmart Business System 10.4.6 source, especially `SupplierModel`, `UpstreamLogic`, and `ResModuleLogic`. The connector keeps the V10 routes separate from the native ZJMF Finance `/zjmf_api_login` flow.
