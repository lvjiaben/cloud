# LinaPro IDC Local Compliance Gateway

This directory provides the standalone local compliance gateway process delivered with every complete LinaPro installation.

English | [简体中文](README.zh-CN.md)

## Correction Status

The valid target is a standalone process packaged with and operated inside each customer's own environment. Each installing IDC merchant supplies its own Tencent eSign, NuoNuo, or alternative provider account, application credentials, legal subject, callback endpoint, database, monitoring, backup, and billing relationship. The software vendor does not proxy customer compliance traffic or settle customer provider usage.

The existing sales-bound registration, remote instance identity, cross-customer routing, platform-operator authority, and platform-to-merchant settlement code was built under a superseded architecture. It is not a production capability and must not be extended. OpenSpec tasks `10.4` through `10.7` now govern its inventory, removal, and conversion to local ownership.

Generic security work may be reused after review: bounded inputs, idempotency, replay protection, persistent rate limits, supplier-write circuit breaking, callback verification, monotonic convergence, active queries, unknown-result protection, encrypted evidence, and evidence-bound manual takeover. Local process authentication must be generated during installation and must not depend on a sales record, remote registration token, or commercial license code.

The Tencent production client uses the official `2021-05-26` Go SDK contract and can be enabled only from this buyer installation's private application file. The Tencent subject canonicalizer accepts only a server-side one-time authorization projection; it does not expose a browser field for enterprise or operator identifiers. NuoNuo remains disabled behind its separate external-evidence gate. The signed local routes for subject bind, revoke, and reopen remain fail-closed when the corresponding provider recognizer is unavailable.

The local usage collection service is always assembled and records an explicit state for both fixed providers. The current Tencent client can perform explicit read-only capability probes and registers the active-query worker only when `flow.query` is selected, but the production usage collector map remains empty in this slice; NuoNuo also remains `disabled`. Readiness remains independent of supplier availability. A later reviewed collector runs only inside the current installation: an immediate pass followed by a 15-minute cadence, at most two provider claims, PostgreSQL-authoritative 60-second leases, a 45-second call deadline, and bounded backoff from one to thirty minutes. One atomic commit accepts at most one plan, 200 usage rows, eight original-currency reconciliation rows, one SLA observation, and 256 KiB of encrypted evidence. Budget alerts use only an explicitly enabled local policy; the gateway does not invent a budget, convert currency, or call a software-vendor fallback.

## Build

Run the cross-platform Go release tool from this directory and provide a component-bundle directory that does not exist:

```text
go run ./hack/tools/release -out ./dist/idc-compliance-local-gateway -version dev -commit unknown -built-at unknown
```

For a Windows target, pass `-goos windows`; the tool selects the `.exe` file name. The tool enables `CGO_ENABLED=0`, disables workspace and VCS injection, refuses to overwrite an existing bundle directory, and creates a complete bundle containing `bin/`, `config/`, `migrations/`, `docs/`, and `manifest.json`. The manifest records every file's SHA-256 and declares that the component is mandatory in the product package, runs only inside the installer's environment, and uses the same program for vendor-owned and customer installations.

## Run

The default listen address is `127.0.0.1:9088`. The following process settings are available:

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
| `LINAPRO_COMPLIANCE_GATEWAY_TENCENT_ESSBASIC_CONFIG_FILE` | none |

The colocated LinaPro source plugin uses `LINAPRO_COMPLIANCE_GATEWAY_INTERNAL_BASE_URL`; its reviewed value is fixed to `http://127.0.0.1:9088`. The three signed write routes are `/internal/v1/supplier-subject-bindings`, `/internal/v1/supplier-subject-revocations`, and `/internal/v1/supplier-subject-reopenings`. They accept no browser traffic and expose only opaque local operation and binding identifiers.

Process timeouts must be between one second and two minutes. The database, installation identifier, process keyring, supplier-identity keyring, and callback-reconciliation keyring paths are mandatory, absolute, and distinct. Generate the installation identifier and process keyring with `idc-compliance-local-gateway-keyring -out <keyring-path> -installation-id-out <installation-id-path>`, generate three independently versioned supplier-subject key families with `idc-compliance-local-gateway-supplier-keyring -out <supplier-keyring-path>`, and generate five independently versioned callback and reconciliation families with `idc-compliance-local-gateway-evidence-keyring -out <evidence-keyring-path>`: evidence encryption, external-event digest, external-reference encryption, external-reference lookup digest, and manual-takeover operation digest. Preserve matching identifiers and all keyrings when restoring the same installation, but regenerate them for a new merchant clone. `LINAPRO_COMPLIANCE_GATEWAY_PROCESS_AUTH_WINDOW` is explicit and must be between `1s` and the `60s` hard cap; signed issue time may lead PostgreSQL authoritative time by at most `30s`. On Unix, all five required files must not grant group or other permissions. The optional Tencent application file is also absolute, private, and distinct from every required file; it has no configurable endpoint, proxy, or subject fields. Startup validates the local identifier and three keyrings, verifies the database, applies migrations, and assembles the Tencent client only when that optional file is present. The public Tencent callback route is registered only when `callback.receive`, a verification token, and an exact 32-byte `CallbackUrlKey` are all present; it accepts only encrypted `FlowStatusChange` callbacks at `/callbacks/v1/tencent-essbasic`. NuoNuo recognizers, queriers, collectors, and public callbacks remain disabled.

Read-only reconciliation claims at most 50 routes with PostgreSQL time, `SKIP LOCKED`, a 60-second 256-bit lease, and failure backoff capped at 30 minutes. Only routes with a protected external reference are claimable. A late result whose route version or lease changed cannot overwrite a callback. Manual takeover has no public route in this slice: a future management adapter must supply the exact permission, current-merchant employee and data-scope proof, a full login from the last five minutes, the last-read route version, one of the five fixed reason codes, one to sixteen same-route immutable evidence records, and a 256-bit operation key. Takeover only stops automation; it cannot force success, force failure, replace an external reference, or retry a supplier write.

## Complete Delivery

The merchant source plugin declares the local gateway as a mandatory product-package component in `manifest/delivery/local-compliance-gateway.yaml`; the source plugin does not embed a generated binary for one operating system. Release assembly must run this module's release tool for each target platform and ship the resulting component bundle with the host, business plugins, and `web-customer`. The bilingual runbook inside the bundle defines installation, supervision, backup, restore, and uninstall boundaries.

The old Portal remains a migration baseline and is not included in the local gateway component bundle. Its pages, tests, migration, and eventual removal remain governed only by `FB-10` and tasks `15.12.a` through `15.12.b`; this task neither removes it early nor treats it as the final customer frontend.
