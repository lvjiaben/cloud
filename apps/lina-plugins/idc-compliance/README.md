# idc-compliance

`idc-compliance` is the built-in source plugin for IDC compliance workflows.

English | [简体中文](README.zh-CN.md)

## Scope

The plugin owns identity-verification cases, enterprise review, tax invoicing, and required material retention.

## Dependencies

The plugin consumes only the public `idccap`, `managementcap`, and `secretcap` contracts from `idc-core`. It does not depend on core implementation packages.

## Delivery

Version `v0.1.8` is delivered as `source + builtin` with `platform_only`, `supports_multi_tenant: false`, and `global` governance.

## Local Gateway Delivery Boundary

Every LinaPro installation must receive and operate its own compliance gateway process, database migrations, configuration, supplier credentials, callbacks, monitoring, and recovery boundary. The software vendor's own installation and every customer installation use the same complete program. Customer compliance traffic must not be routed through a vendor-operated business gateway.

`manifest/delivery/local-compliance-gateway.yaml` declares the local gateway as a mandatory component of every product package. Its release tool creates a target-platform bundle containing the binary, configuration template, database migrations, and bilingual lifecycle runbook. `central-gateway` remains a historical source-directory name during correction and does not imply central deployment. The physical rename follows removal of the central identity and settlement code so obsolete model names cannot be mistaken for a compatibility contract.

The directory still contains implementation produced under a superseded platform-proxy assumption. Sales-bound registration, remote instance identity, cross-customer routing, platform-operator authority, and vendor-to-customer prepaid or postpaid settlement are no longer valid product requirements and must not be extended. Reusable idempotency, replay protection, circuit breaking, callback convergence, active-query, and evidence controls may be retained only after they are constrained to the installing merchant's local ownership boundary. See the OpenSpec independent-delivery correction record before continuing implementation.

## Data Boundary

The compliance data foundation owns verification cases, encrypted enterprise-material metadata, supplier-neutral tax invoices, red-letter tasks with immutable allocation traces, and append-only external evidence. `idc-core` keeps only business-gate projections and opaque references.

Raw face images or video, identity documents, invoice originals, third-party credentials, and provider payloads are never stored in the database. Permitted file artifacts use private encrypted object references and `SHA-256` integrity digests; exact identity and invoice-number lookup uses keyed digests, never unsalted public hashes. Legal records do not use soft deletion. Database triggers protect archived tax-invoice content and ownership metadata from mutation, reject physical deletion, and still allow explicit retention and legal-hold governance. External evidence is strictly append-only.

## Aliyun FACE Boundary

The Aliyun financial-grade identity verification adapter uses the fixed official endpoint and protocol contract. Credentials are accepted only through callback-scoped buffers, stored through `secretcap`, and never returned by the service. Configuration replacement uses a fail-closed two-phase transition and an idempotency fence so a process interruption cannot expose an uncommitted credential as active.

The adapter never requests or persists face images or video. A browser redirect is not proof of success; only a signed server-side query may advance a verification result. Provider configuration is restricted to an authorized installation administrator, and the installing IDC merchant supplies and pays for its own provider account.

The HTTP controller, administrator wizard, customer verification workflow, recovery scheduler, status dictionaries, and plugin-owned end-to-end assets are composed inside the merchant plugin. They use the installing merchant's supplier accounts and share credentials or runtime state only with that installation's local gateway.

## Material Retention and Consent

Electronic contracts and the standalone privacy-governance workspace are retired. Identity and enterprise verification management retain explicit retention periods, expiry scans and manually confirmed deletion. Collection requires its current retention policy and separate customer consent, without the retired assessment publication gate. Legal holds, tax retention and access audits remain enforced; scanning does not delete originals. Upgrades preserve historical contract and governance evidence without exposing or processing retired business records.
