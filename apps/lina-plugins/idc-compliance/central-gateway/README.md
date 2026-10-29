# LinaPro IDC Local Compliance Gateway

This directory provides the standalone local compliance gateway process delivered with every complete LinaPro installation.

English | [简体中文](README.zh-CN.md)

## Correction Status

The existing sales-bound registration, remote instance identity, cross-customer routing, platform-operator authority, and platform-to-merchant settlement code was built under a superseded architecture. It is not a production capability and must not be extended. OpenSpec tasks `10.4` through `10.7` now govern its inventory, removal, and conversion to local ownership.

Generic security work may be reused after review: bounded inputs, idempotency, replay protection, persistent rate limits, supplier-write circuit breaking, callback verification, monotonic convergence, active queries, unknown-result protection, encrypted evidence, and evidence-bound manual takeover. Local process authentication must be generated during installation and must not depend on a sales record, remote registration token, or commercial license code.

## Build

Run the cross-platform Go release tool from this directory and provide a component-bundle directory that does not exist:

```text
go run ./hack/tools/release -out ./dist/idc-compliance-local-gateway -version dev -commit unknown -built-at unknown
```

For a Windows target, pass `-goos windows`; the tool selects the `.exe` file name. The tool enables `CGO_ENABLED=0`, disables workspace and VCS injection, refuses to overwrite an existing bundle directory, and creates a complete bundle containing `bin/`, `config/`, `migrations/`, `docs/`, and `manifest.json`. The manifest records every file's SHA-256 and declares that the component is mandatory in the product package, runs only inside the installer's environment, and uses the same program for vendor-owned and customer installations.

## Run

| Environment variable | Default |
|---|---|
| `LINAPRO_COMPLIANCE_GATEWAY_LISTEN_ADDR` | `127.0.0.1:9088` |
| `LINAPRO_COMPLIANCE_GATEWAY_READ_HEADER_TIMEOUT` | `5s` |
| `LINAPRO_COMPLIANCE_GATEWAY_READ_TIMEOUT` | `10s` |
| `LINAPRO_COMPLIANCE_GATEWAY_WRITE_TIMEOUT` | `10s` |
| `LINAPRO_COMPLIANCE_GATEWAY_IDLE_TIMEOUT` | `30s` |
| `LINAPRO_COMPLIANCE_GATEWAY_SHUTDOWN_TIMEOUT` | `10s` |
| `LINAPRO_COMPLIANCE_GATEWAY_DATABASE_DSN_FILE` | none |
| `LINAPRO_COMPLIANCE_GATEWAY_INSTALLATION_ID_FILE` | none |
| `LINAPRO_COMPLIANCE_GATEWAY_PROCESS_AUTH_KEYRING_FILE` | none |
| `LINAPRO_COMPLIANCE_GATEWAY_SUPPLIER_IDENTITY_KEYRING_FILE` | none |
| `LINAPRO_COMPLIANCE_GATEWAY_CALLBACK_EVIDENCE_KEYRING_FILE` | none |
| `LINAPRO_COMPLIANCE_GATEWAY_PROCESS_AUTH_WINDOW` | none |

The default listen address is `127.0.0.1:9088`. The following process settings are available:

The colocated LinaPro source plugin uses `LINAPRO_COMPLIANCE_GATEWAY_INTERNAL_BASE_URL`; its reviewed value is fixed to `http://127.0.0.1:9088`. The three signed write routes are `/internal/v1/supplier-subject-bindings`, `/internal/v1/supplier-subject-revocations`, and `/internal/v1/supplier-subject-reopenings`. They accept no browser traffic and expose only opaque local operation and binding identifiers.

Read-only reconciliation claims at most 50 routes with PostgreSQL time, `SKIP LOCKED`, a 60-second 256-bit lease, and failure backoff capped at 30 minutes. Only routes with a protected external reference are claimable. A late result whose route version or lease changed cannot overwrite a callback. Manual takeover has no public route in this slice: a future management adapter must supply the exact permission, current-merchant employee and data-scope proof, a full login from the last five minutes, the last-read route version, one of the five fixed reason codes, one to sixteen same-route immutable evidence records, and a 256-bit operation key. Takeover only stops automation; it cannot force success, force failure, replace an external reference, or retry a supplier write.

## Complete Delivery

The merchant source plugin declares the local gateway as a mandatory product-package component in `manifest/delivery/local-compliance-gateway.yaml`; the source plugin does not embed a generated binary for one operating system. Release assembly must run this module's release tool for each target platform and ship the resulting component bundle with the host, business plugins, and `web-customer`. The bilingual runbook inside the bundle defines installation, supervision, backup, restore, and uninstall boundaries.

The old Portal remains a migration baseline and is not included in the local gateway component bundle. Its pages, tests, migration, and eventual removal remain governed only by `FB-10` and tasks `15.12.a` through `15.12.b`; this task neither removes it early nor treats it as the final customer frontend.


The electronic-sign adapter, public callback and contract routes are retired. NuoNuo capabilities remain closed until their independent API evidence gates pass. Historical evidence is preserved without resuming retired signature processing.
