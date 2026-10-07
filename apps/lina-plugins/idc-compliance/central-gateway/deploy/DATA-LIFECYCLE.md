# Local Gateway Data Lifecycle and Responsibility Matrix

## Responsibility matrix

| Activity | Current IDC merchant | Third-party supplier | LinaPro software vendor |
| --- | --- | --- | --- |
| Select account, legal subject, plan, and budget | Accountable and responsible | Supplies its own contract and authoritative records | No business role |
| Process contracts, invoices, callbacks, usage, and evidence | Data controller for its installation | Processor or independent controller as stated in the merchant-supplier agreement | No access by default |
| Operate database, purpose-separated keyrings, backups, monitoring, and restore | Accountable and responsible | No local infrastructure access | May provide software support only under a separate, time-bounded support authorization |
| Revoke supplier authorization and stop new processing | Accountable and responsible | Executes provider-side revocation | No authority to override |
| Evidence export, legal hold, retention, and deletion | Accountable and responsible | Supplies or deletes provider-held copies under its contract | No routine custody |

Commercial software licensing controls only software use and upgrades. It never makes the software vendor a settlement counterparty, callback relay, or business data processor.

## Backup boundary

Back up the PostgreSQL database, private runtime configuration, DSN file, process-auth keyring, supplier-identity keyring, callback-evidence keyring, purpose-separated secret store, and the component `manifest.json`. Encrypt database and secret backups separately, store their checksums outside the backup set, and restrict restoration to current-merchant employees. Supplier credentials, subject plaintext, decrypted evidence, DSNs, and key bytes must never appear in backup logs or filenames.

Before a backup, record the migration ledger and the maximum immutable evidence timestamps. After a backup, verify archive readability, manifest checksums, database dump integrity, and independent secret recovery. The installer sets and tests its own RPO and RTO; LinaPro does not send backups to a software-vendor service.

## Restore and disaster recovery

Restore into an isolated network with supplier writes blocked. Restore the database and secret store, verify all component hashes and migration checksums, load the process, supplier-identity, and callback-reconciliation keyrings, and run liveness/readiness checks. Validate subject ciphertext decryption, versioned lookup digests, callback evidence decryption, provider-scoped event digests, protected external-reference decryption and lookup, immutable evidence hashes, query schedules without live supplier calls, current supplier authorization events, legal holds, retention policies, plan/usage/reconciliation projections, and pending or result-unknown operations. Clear expired query leases through the normal claimant rather than editing evidence or takeover history. Record the result in `idc_gateway_restore_verification`; its database constraint requires supplier writes to remain blocked during verification. Enabling a real adapter requires a separate reviewed operation after restore.

The local lifecycle service persists the recovery write block for both providers before verification. The verifier compares the complete embedded migration ledger, checks all seven lifecycle policies, verifies every retained subject against its keyring and lookup digests, and decrypts every local callback evidence row. It records both successful and failed checks locally. The restore drill must use a restored isolated database and matching keyrings; setting a boolean without the persisted two-provider stop state is rejected.

## Supplier termination and revocation

Stop new work first, isolate in-flight and result-unknown operations, export required evidence, and obtain a current-merchant employee's recent full authentication. Revoke provider-side access in the merchant's supplier console, then append the local `processing_stopped` and `revoked` authorization events and keep the adapter disabled. Supplier-subject revocation and local credential lifecycle are separate authorized operations: revoking a subject never implicitly revokes a `secretcap` handle or destroys a retained key version. A software-vendor operator cannot perform or bypass this sequence.

The signed local revocation path changes `idc_gateway_supplier_processing_state` to `stopped` before it revokes the opaque local subject. Supplier write admission and the read-only collection worker both read this database state. Stable retries complete a partially finished stop/revoke sequence without calling a supplier, deleting ciphertext history, or destroying a key version.

Reopening is a separate recently authenticated supplier authorization. It creates a new opaque binding, appends `processing_resumed`, changes the provider processing state back to `allowed`, and re-evaluates whether a reviewed collector is actually installed before scheduling collection. A revocation request cannot resume processing, and a reopening retry cannot create a second subject or a second resume event.

## Evidence export and legal hold

An export is a bounded, employee-authorized local operation. It must select a declared scope, decrypt only the required evidence inside the current installation, produce an encrypted archive, and write an immutable manifest hash and expiry. Never export process keys, DSNs, credential envelopes, unrelated customers, or raw provider responses beyond the authorized scope.

One execution exports at most 16 explicitly scoped local callback-evidence records. The caller supplies a temporary 256-bit archive key through the authorized in-process boundary; the service clears that key, produces an `AES-256-GCM` envelope, and stores only the scope digest, encrypted-archive manifest digest, status, employee, and expiry. The archive key is never persisted.

Place or release legal holds as immutable events. A hold suspends scheduled deletion for the exact data class and optional aggregate HMAC without changing original evidence. Export and hold history remain in the local database and are not reported to LinaPro's software vendor.

## Retention and deletion

Configure each data class in `idc_gateway_data_lifecycle_policy`. Scheduled deletion must check active legal holds, incomplete operations, supplier revocation, export obligations, and statutory retention before deleting or cryptographically erasing data. Process nonces expire quickly; supplier-subject bindings and identity operations retain append-only history, while supplier evidence, usage, cost, SLA, and audits follow separately approved periods. An old subject-encryption key may be revoked only after all referenced ciphertext has been migrated or approved for disposition. Old lookup and identity-idempotency keys require lookup migration and idempotency-retention verification first. Instance or subject revocation never destroys keys; terminal cryptographic erasure is a separate reviewed action. Retained digests must not permit reconstruction of secret or personal plaintext.

Each execution is limited to 1,000 rows and writes an immutable selection digest and outcome. Process nonces and separately approved usage, cost, and SLA projections may be removed. Supplier callbacks, supplier evidence, and audit classes are always retained by this executor even if service binaries are uninstalled. Any active hold fails closed; until aggregate-to-record mappings are available, an aggregate-scoped hold conservatively blocks the whole data class. A `crypto_erase` policy records that a separate reviewed terminal action is required and never destroys a shared key implicitly.

## Service stop and uninstall

Stop new processing, reconcile or isolate in-flight work, revoke supplier authorization when the service relationship ends, complete required exports, take a final verified backup, and stop the supervisor. Normal uninstall removes binaries and service definitions only. Database, secret, evidence, hold, and audit erasure is a separate current-merchant decision and must never be automatic.
