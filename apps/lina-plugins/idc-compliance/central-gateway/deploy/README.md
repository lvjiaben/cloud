# Local Compliance Gateway Installation and Lifecycle

This directory ships with every complete LinaPro installation. The software vendor's own installation and customer installations use the same program component. Each installer runs the gateway in its own environment and owns its database, secrets, supplier accounts, callbacks, monitoring, and backups.

## Install and start

1. Verify the `SHA-256` of every file listed in `manifest.json`.
2. Copy `config/local-gateway.env.example` into the installation's private configuration directory and apply least-privilege permissions.
3. Create a private file containing the installation-local PostgreSQL DSN and set `LINAPRO_COMPLIANCE_GATEWAY_DATABASE_DSN_FILE` to its absolute path.
4. Generate the installation identifier and process-authentication keyring with `bin/idc-compliance-local-gateway-keyring -out <absolute-keyring-path> -installation-id-out <absolute-installation-id-path>`. Set `LINAPRO_COMPLIANCE_GATEWAY_PROCESS_AUTH_KEYRING_FILE` and `LINAPRO_COMPLIANCE_GATEWAY_INSTALLATION_ID_FILE` to those paths, and explicitly set `LINAPRO_COMPLIANCE_GATEWAY_PROCESS_AUTH_WINDOW` between `1s` and `60s`. Never obtain a sales registration token or business gateway identity from a software-vendor service.
5. Generate the separate supplier-subject keyring with `bin/idc-compliance-local-gateway-supplier-keyring -out <absolute-supplier-keyring-path>` and set `LINAPRO_COMPLIANCE_GATEWAY_SUPPLIER_IDENTITY_KEYRING_FILE` to that path. The file contains three independently versioned key families and must not reuse the process keyring.
6. Generate the separate callback-reconciliation keyring with `bin/idc-compliance-local-gateway-evidence-keyring -out <absolute-evidence-keyring-path>` and set `LINAPRO_COMPLIANCE_GATEWAY_CALLBACK_EVIDENCE_KEYRING_FILE` to that path. Its five independently versioned families cover evidence encryption, external-event digest, external-reference encryption, external-reference lookup digest, and manual-takeover operation digest; none may reuse either existing keyring.
7. To enable Tencent eSign, copy `config/tencent-essbasic.json.example` into the private secret directory, replace its application credential placeholders with this buyer installation's own values, select only capabilities purchased and authorized for this account, and set `LINAPRO_COMPLIANCE_GATEWAY_TENCENT_ESSBASIC_CONFIG_FILE` to the absolute path. The file deliberately has no endpoint, proxy, enterprise subject, or operator subject field. The enterprise and operator identifiers come only from the server-side Tencent authorization result and are encrypted by the supplier-identity service.
8. Run `bin/idc-compliance-local-gateway` as a dedicated unprivileged system account. It listens on `127.0.0.1:9088` by default.
9. Give the colocated LinaPro source plugin access to the same installation identifier, process-authentication keyring, and authentication window. Set its `LINAPRO_COMPLIANCE_GATEWAY_INTERNAL_BASE_URL` to the fixed loopback value `http://127.0.0.1:9088`; never publish the internal routes through a reverse proxy.
10. When Tencent callbacks are enabled, configure the current buyer application's public callback URL as `/callbacks/v1/tencent-essbasic`, enable `callback.receive`, and provide both the buyer-owned verification token and exact 32-byte `CallbackUrlKey` in the private Tencent configuration file. Missing either secret keeps the route unregistered; plaintext callbacks are not accepted.
11. Configure the service supervisor to check `GET /health/live` and then `GET /health/ready`. Readiness means only that the local database, migrations, installation identifier, keyrings, and any configured Tencent SDK client and callback verifier can be assembled. It does not claim that a Tencent subject is active or a remote capability check passed.

The bundle contains the reviewed Tencent eSign `2021-05-26` official SDK client. It never calls Tencent merely because a configuration file exists: read-only capability checks are explicit, flow creation must pass the local database write guard, and the active-query worker runs only when `flow.query` is configured. The Tencent usage collector is not enabled by this client slice, and NuoNuo remains fully disabled; do not edit database state to bypass those gates. A later reviewed collector failure must retain the last complete projection and must not make process readiness fail.

## Process key rotation and revocation

Create a replacement file with `-out <new-absolute-path> -rotate <current-absolute-path>`. Rotation never accepts `-installation-id-out`: the existing installation identifier must remain unchanged. Atomically update both local processes to the replacement keyring and restart them; the previous key stays verify-only during the controlled overlap. After no request signed by the old process can remain in flight, create another replacement with `-revoke <old-key-id>`, deploy it atomically, restart both processes, and archive or destroy superseded secret files under the installer's secret-retention policy. The command refuses in-place overwrite so an interrupted rotation cannot corrupt the active keyring.

## Supplier identity key rotation

Use `bin/idc-compliance-local-gateway-supplier-keyring -out <new-absolute-path> -rotate <current-absolute-path> -family <key-family>` to rotate exactly one of `subject_encryption`, `subject_lookup`, or `identity_operation`. New writes use the new active version while retained versions continue to decrypt history or recover duplicate and idempotent operations. Atomically deploy the replacement file and restart the gateway; startup backfills retained lookup versions for current active subjects. The command deliberately has no revoke or destruction flag. A later retention executor must first prove ciphertext or lookup migration and idempotency-retention completion, and terminal key destruction remains a separate approved lifecycle action.

## Monitoring

Send only liveness, readiness, process exits, restart count, CPU, memory, file descriptors, database connections, bounded local query and usage-collection backlog, fixed query and collection failure codes, and takeover backlog to the installer's monitoring system. Monitoring, logs, and support bundles must not contain database DSNs, keys, supplier credentials, external references, lease tokens, customer materials, callback payloads, free-text supplier errors, or business evidence payloads.

## Backup and restore

The installer backs up the local gateway database, installation identifier, process keyring, supplier-identity keyring, callback-evidence keyring, configuration versions, purpose-separated secrets, supplier authorization metadata, immutable evidence, and audit records. Secret backups must be encrypted separately and authorized independently from data backups. Restore the same installation identifier and matching keyrings with the database when recovering the same instance; generate a new identifier and all three keyrings before using a clone as a new merchant instance. Then verify migrations, evidence integrity, and supplier authorization in read-only mode. Supplier writes remain blocked until verification completes.

## Stop and uninstall

Stop new business first, drain or close in-flight tasks, and then stop the process. Uninstallation removes only the binary and service definition by default and preserves databases, secrets, evidence, and audit records. Data erasure requires legal-hold, statutory-retention, in-flight-task, supplier-revocation, and evidence-export checks. The installer must never silently delete business data.

## Independent-operation boundary

This component must not connect to a software-vendor central business gateway, central settlement, platform sales registration, or platform-operator service. Commercial licensing governs only software use, versions, and upgrade entitlement; it does not participate in supplier identity, business signatures, callback routing, usage, or settlement.
