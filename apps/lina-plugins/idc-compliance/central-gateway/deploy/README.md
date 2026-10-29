# Local Compliance Gateway Installation and Lifecycle

This directory ships with every complete LinaPro installation. The software vendor's own installation and customer installations use the same program component. Each installer runs the gateway in its own environment and owns its database, secrets, supplier accounts, callbacks, monitoring, and backups.

## Install and start

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


The electronic-sign adapter, public callback and contract routes are retired. NuoNuo capabilities remain closed until their independent API evidence gates pass. Historical evidence is preserved without resuming retired signature processing.
