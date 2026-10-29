# LinaPro IDC Core

## Scope

This source plugin owns the stable IDC business boundaries for customers, product catalog, orders and billing, support coordination, merchant settings, and shared public contracts.

For a bounded, read-only local payment and fulfillment summary, use the [order diagnostic tool](hack/tools/order-diagnose/README.md). It preserves existing staff permissions and never submits provisioning.

## Dependencies

The plugin depends only on the LinaPro host. Other IDC plugins may depend on its public contracts under `backend/cap`.

## Configuration quantity pricing

Quantity configuration fields use an explicit immutable algorithm: `per_unit` applies the matching band's unit rate to the entire quantity; `band_total` charges that band's total; `graduated` sums the actual quantity within each continuous `(minimum, maximum]` band. For example, ten free units followed by a rate of 2 makes fifteen units cost 10, independently of product base charges. Graduated setup fees belong only to the first band and are charged once for a nonzero purchase; renewals use each band's renewal rate and no setup fee. Zero quantity has no configuration charge or setup fee.

The merchant selects the algorithm while first defining quantity prices in the same configuration editor. Existing dimension algorithms and frozen order selections retain their original meaning. The shared `channelcommercecap` import contract preserves `graduated` without downgrading it to a matching-band algorithm, and validates its integer field and first-band setup fee. Migration `144-graduated-configuration-pricing.sql` extends only the existing algorithm constraint.

## Operating data

The top-level Operating Data page reports actual receipts and paid orders for today, yesterday, the current month, and all history in the merchant timezone. Top-ups and cash receipts count as income; spending an existing balance does not count again. Amounts remain separated by original currency. Calendar-month trends compare the same date in the previous month without fabricating absent dates, and product rankings use paid order quantity snapshots.

Online customers are deduplicated through the host-governed session seam. Server counts cover active cloud and bare-metal servers expiring within seven days; tickets are grouped by their current resolved or closed state. Every metric and chart opens a matching paginated projection in its original business page, with fresh host permission checks. `GET /admin/operations/data` and `GET /admin/operations/data/records` replace the scattered legacy homepage endpoints. Pages contain at most one hundred rows, and the existing homepage route remains valid.

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

## Catalog frontend verification

After installing the repository workspace dependencies, run `pnpm -C apps/lina-plugins/idc-core/frontend run typecheck:catalog` from the repository root. This checks the catalog client, product editor and imported Vue components against the actual installed host adapters. It is a source-plugin development check; the independent frontend runtime keeps its existing type-check and build commands.

## Delivery boundary

This stable business owner is delivered as a mandatory built-in source plugin with Chinese and English resources. Replaceable supplier connectors are separately managed dynamic plugins: they install, upgrade, stop new sales, disable, and uninstall through the single LinaPro plugin manager, while their business configuration remains in the catalog or owning-domain page. The built-in owner is not listed as an uninstallable managed plugin.

## Configuring support staff

1. Create an organization department under Permissions.
2. Create a support role with department data scope and ticket list, detail, reply, note, claim, and resolve permissions. Supervisors additionally need Manage Department Tickets plus each required transfer, close, or reopen action.
3. Create a separate backend user and assign the department and role. Positions record employment information; they do not grant permissions.
4. An administrator with broad data scope and support configuration permission links the queue to the organization department in Support Settings. Existing unlinked queues need an explicit mapping.
5. Employees claim tickets from their department queue. Transfers and reopening return tickets to the queue. Automatic assignment, individual targets, and the separate staff qualification feature have been removed. Escalations and low ratings flag follow-up in the queue; customer notifications remain.

Role permissions accumulate; all-data scope does not grant actions. Department changes and revoked permissions are checked on subsequent requests. The five permission administration pages provide a floating help icon at the lower right through a published plugin slot, without host changes.

## Product agent prices and custom prices

Use **Edit** on a product row, then the **Agent Prices** tab inside the product editor to set that product’s prices for agent levels. **Custom Prices** in the toolbar lets administrators select a customer and product, then create, query, edit or delete customer-specific prices. Use the small Agent Price dialog in Agent Sales → Agent Levels to set a default charge percentage per level, such as 80 for 80% of regular prices; leave blank and confirm to clear. Priority is customer custom price, product level price, default level percentage, regular price; web purchases, the open `API`, and upstream catalog exports share these rules. Setting or clearing the default requires pricing management permission. Editing a level name or status does not change its price; immutable versions are retained and conflicting updates require closing and reopening before saving. Product level and customer custom prices identify their currency and billing period; the default level percentage applies to uncovered product terms. The configured item price includes configuration but excludes tax and setup fees. Deletion revokes the rule for future quotes while preserving historical quotes and orders. Administrators need catalog access and pricing management permissions; custom customer prices additionally require customer query permission.

## Trial lifecycle admission

The optional `trialPauseResume` flag in the owner-defined `product-fulfillment-contract` declaration defaults to false. A provider may declare it only when its entitlement owner implements real system-owned pause and resume operations and a usable formal renewal path. Ongoing management alone does not imply trial support. No existing provider is enabled by inference. Publication rejects active trial plans on manual products or on a bound release without this declaration; ordinary plans and inactive trial history remain unaffected.

For local trials, the first successful activation initializes the trial dates from the immutable order period in UTC. Provisioning waits, catalog changes, repeated activation and later resumes cannot consume or restart that trial duration. This date initialization does not by itself provide expiry scheduling or paid conversion.

Products are edited in a wide drawer with five tabs: basic information, package price, product configuration, agent prices, and automatic delivery. Basic information requires a member navigation menu and directly shows inventory tracking and optional companion products. Server login rules are available through a switch only when declared by the connection. A parameter list opens modal editors for typed values and their prices, with interface shortcuts as a secondary tool. Package prices use an Add modal; the product cost entry is removed. Configured website settings provide copyable quick-order and sales-category links. Upstream product selection follows declared interface capabilities; complete source definitions remain fixed while local prices stay editable. Incomplete supplier definitions are rejected before mutation. Saves apply immediately while paid orders retain their purchased specifications and amounts.

The authorized merchant configuration endpoint accepts optional `priceRows` in the existing configuration update. Configuration, newly required price dimensions, exact currency/period schedules and immediate application share one transaction. Configuration and price permissions remain separate; a stale price or invalid schedule rolls back the entire update. The editor obtains existing price identities in its schema response instead of loading candidates for every row. The public pricing endpoints retain opaque identities. Blank amounts require explicit entry; zero means explicitly included. Existing quantity modes keep their original meanings: selected-band unit rate multiplied by total quantity, or a total fee for the selected band.

All cloud products use the same product, commercial configuration, pricing, order, payment and delivery foundation. Supplier plugins declare supported typed parameters, validation, fulfillment and service actions; the core has no fixed product-kind enum. Unbound customer information stays with the order and is not sent upstream. The cloud-server flow is the first integrated acceptance path; a generic certificate-form fixture separately verifies that authoring does not require server fields.

See the [connector contract guide](backend/cap/productfulfillmentcap/README.md) when adding a new cloud product. It separates supplier parameters, commercial options, paid delivery and ongoing service ownership, and links the actual contracts and isolated verification paths.

Manual products can maintain ordinary typed fields, sellable choices and their prices without an automation interface. They use the same quotation, payment, frozen selection and manual delivery records. Selecting an interface later preserves these commercial fields and prices; validated configuration saving admits the actual interface parameters for future purchases. Existing paid manual orders retain their original delivery mode and amount.

Merchant string dropdowns may bind product text parameters only when the provider explicitly declares `productChoiceFields` and supports pure batch validation. At most 512 alternatives are checked in one invocation without credentials or outbound access; providers without this opt-in retain their original behavior.

Local single-choice options may explicitly map their stable commercial identifiers to declared product choices or supported text profiles through `productChoiceBindings`. Prices and customer selections keep commercial identifiers; provider validation, related-choice filters and execution use the frozen mapped values. The same child editor maintains names, prices and upstream mappings. Missing overrides retain the existing same-value behavior. Removing or replacing an interface clears its mappings while retaining ordinary commercial choices and prices; paid orders keep their original mapping. Numeric unit conversions, customer core-field aliases, protected files and imported-contract conversion are outside this mapping contract.

`provideradmissioncap` shares a stateless, bounded admission wait between product and resource adapters. Only a typed host refusal before callback entry may wait for up to five seconds; entered callbacks, other errors and cancellation stop immediately. Direct provider method calls are not automatically replayed. Pure product validation reports localized busy, unavailable or incompatible-result errors and logs fixed stage categories without arbitrary connector messages or credentials. Failed saves retain merchant input for an explicit retry.

## Customer resource notes and confirmation

Customer product-directory pages expose one bounded owned service page, a matching literal-keyword count, the current actual price snapshot and independent customer-note revisions. Migration `150` extends the existing entitlement aggregate. Note writes lock the exact customer-owned row, require its note revision and append a digest-only audit in the same transaction; fulfillment versions and paid terms do not change.

Source resource consumers use `customerauthcap.SessionConfirmationAccess` for exact high-risk native operations. Prepare a digest covering the server, action, normalized non-secret parameters and request key. Consume the returned proof inside `WithCurrentEntitlement`, in the same transaction as resource admission and task creation. A valid current session has no recent-login-age restriction. Policy reads, proof consumption and owner writes share transaction authority; confirmation never replaces resource ownership, refund or capability checks.


Catalog availability uses one batch through the existing injected `CandidateSource.CatalogAvailability` to read resource-owned local method evidence. Product-fulfillment `provision/query` methods do not describe resource catalog methods. The projection accepts at most 100 already-authorized interfaces without reading credentials or contacting suppliers.


## Resource navigation shortcuts

Resource menu shortcuts open the existing customer product workflow and supplier configuration operations; they retain product ownership and the renewal work-items entry.
