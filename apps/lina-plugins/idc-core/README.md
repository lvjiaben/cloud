# LinaPro IDC Core

## Scope

This source plugin owns the stable IDC business boundaries for customers, product catalog, orders and billing, support coordination, merchant settings, and shared public contracts.

## Dependencies

The plugin depends only on the LinaPro host. Other IDC plugins may depend on its public contracts under `backend/cap`.

## Runtime security keyring

Before enabling the plugin routes and secret envelope service, deployment must provide `LINAPRO_IDC_CORE_SECURITY_KEYRING` as one canonical JSON object:

```json
{"active_key_id":"v1","reservation_key":"<canonical base64 of 32 random bytes>","keys":{"v1":"<different canonical base64 of 32 random bytes>"}}
```

Generate every root with a cryptographically secure random source and keep it outside source control, database rows, logs, and support bundles. `reservation_key` must remain stable. Versioned roots derive separate audit-replay, audit-integrity, and secret-envelope keys through HKDF-SHA-256. To rotate, add a new key version, keep old versions for existing audit verification and envelope unwrapping, then change `active_key_id` and complete bounded envelope rewrapping before retiring an old version. The plugin fails closed when this variable is missing or invalid; it has no generated, database, or hard-coded fallback.

Source consumers obtain callback-only secret access through the single-owner `secretcap/spi` declaration and freeze seam. The resource owner derives one deterministic, non-reversible scope per channel and injects only that accessor and encrypted-output sink into a Provider. A scope-bound manager accepts lifecycle plaintext only through a synchronous, single-call producer and never offers listing or handle discovery. Its optional generated-credential extension supports only the fixed 256-bit URL-safe policy: the secret owner generates and encrypts the value once, then identical replays recover only the original handle. The registrar has no runtime lookup, and no dynamic-plugin plaintext bridge is published.

Resource-owned product interfaces project an opaque `pfi_*` identity and an immutable `rci_*` resource configuration revision into product automation. Core maps each `rci_*` value to one append-only interface configuration revision. A published product version freezes that mapping, and a paid fulfillment request carries the exact `rci_*` value in its canonical digest. Product configuration revisions and resource endpoint/account/secret-reference revisions are separate facts; editing a resource channel must never redirect an older order to the newest supplier configuration.

## Catalog foundation

The catalog owner persists merchant-defined category trees and general product identities separately from immutable published-version history. Categories organize products but never determine a product kind. The general core owns names, descriptions, billing rules, exact decimal-string money at HTTP boundaries, explicit-currency prices, tax policy, publication state, and frozen commercial snapshots. It has no fixed server, certificate, CDN, domain, phone, delivery, or logistics field set.

From FB-52 0.10 onward, an unpublished product version may bind at most one compatible supplier-interface instance through the product-fulfillment extension point. The selected plugin declares its interface configuration, merchant product configuration, customer purchase fields, validation, fulfillment actions, status, and governed frontend surfaces. Server CPU, memory, operating system, disk, bandwidth, region, line, IP, inventory pool, and server operations stay in the server-plugin contract; SSL and phone plugins own their different fields and pages. Replacing a supplier creates a new product version and never rewrites historical orders. Prices never use floating-point money or currency fallback, and raw supplier payloads, credentials, costs, or internal identifiers never become public catalog content.

The retired development-only path that generated catalog drafts from resource or Provider snapshots has no compatibility entry. Migration `089-retire-provider-catalog-draft.sql` cleans databases in place only when no `provider_import` product version remains. If such a version exists, the migration fails closed: export any test data that must be kept, rebuild the development database from current migrations, and restore only data independent of the retired path. Do not relabel those versions as manual or migrated products.

## Delivery boundary

This stable business owner is delivered as a mandatory built-in source plugin with Chinese and English resources. Replaceable supplier connectors are separately managed dynamic plugins: they install, upgrade, stop new sales, disable, and uninstall through the single LinaPro plugin manager, while their business configuration remains in the catalog or owning-domain page. The built-in owner is not listed as an uninstallable managed plugin.
