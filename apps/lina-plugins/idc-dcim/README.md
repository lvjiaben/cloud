# DCIM Server Connector

`idc-dcim` is a managed dynamic connector for DCIM server suppliers. An administrator may create any number of interface instances. Every saved interface revision explicitly freezes either the `legacy` or `bms` protocol; the connector never guesses, falls back, or combines them.

## Security boundary

The interface endpoint and all credentials stay in the `idc-resource` confidential outbound proxy. Only the non-secret `protocol` setting enters WASM memory. The connector sends declarative relative HTTP requests and field allowlists. It does not receive a raw access token, endpoint, username, password, access user, or access token, and does not include them in results, evidence, or logs.

Authentication is selected from the frozen protocol by the owner:

- `legacy`: `username` and `password` are injected into every request form.
- `bms`: `access-user` and `access-token` are injected into every request header.

Administrators fill structured fields only. They cannot provide executable code, raw headers, or arbitrary authentication JSON.

## Proven routes

The legacy contract supports bounded discovery through `POST /index.php?m=api&a=getSaleGroup` and `getAllMirrorOs`. Its old provisioning and lifecycle calls depend on a finance-system local host ID, callback token, and confidential response values that the new owner contract does not expose. Those actions therefore fail closed as unsupported.

The BMS contract supports server groups and operating-system discovery, creation through `POST /bms/source`, resource observation through `GET /bms/source/{id}`, start, shutdown, reboot, suspend, resume, and delete. Existing-resource actions first perform an exact resource observation. Network ambiguity returns an unknown result for manual reconciliation; it is never reported as success.

Password input for BMS creation is an owner-held operation secret. Only the canonical slot name `password` crosses into WASM, and the proxy injects its value into the supplier form.

## Product configuration

The plugin declares DCIM-specific product and purchase fields. They do not become fields of the generic product, cart, order, billing, or service core. Server disks remain typed server components supplied by `idc-resource`; the connector maps them to the BMS form only while building a frozen provisioning request.

This connector does not add its own management page. Product editing and server operations remain in the governed pages owned by `idc-core` and `idc-resource`.

## Clean-room reference

Route names, HTTP methods, authentication placement, and request field names were independently derived from the user-provided decoded ZJMF Finance source, especially `app/common/logic/Dcim.php`. No supplier source code is copied into this plugin.

## Verification

Run from this directory:

```text
go test ./... -count=1
go vet ./...
go build ./...
make build
```

## Merchant provisioning parameters

Direct provisioning accepts explicitly named, non-secret merchant parameters frozen with the order. Their native names and scalar contents reach the connector form; unbound customer fields stay private to the order. Credentials, the upstream customer identity and paid unit quantity remain owner controlled. Supplier acceptance is determined by its actual response. Upstream resale configurations remain read only.
