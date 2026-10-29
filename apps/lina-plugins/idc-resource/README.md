# LinaPro IDC Resource

## Scope

This source plugin owns provider accounts, normalized provider-capacity observations, server instances, resource allocation, and durable asynchronous operation boundaries.

Server details, accessories, renewals, and product tasks are available from the corresponding customer product. Independent server and operations routes are retired while their permission grants remain. Native server directories and renewal candidates are restricted by entitlement; legacy tasks also support service numbers. Task details and takeover validate the supplied product or interface context; renewal previews and confirmations validate the product context.

Supplier configuration contains global overview, health, capacity, tasks, and invocation records, each loaded under its existing permission. Operations-only staff do not read the configuration directory. Interface details restrict tasks and logs to that interface; logs without a reliable product association are never presented as product logs.

## Runtime foundation

The plugin now owns versioned Provider contracts and startup declarations, encrypted channel-secret bindings, channels, routing pools, authoritative upstream mappings, resource tasks, capability snapshots, provider-capacity observations, financial observations, and health observations. A channel reader uses two bounded queries to construct only enabled channels; disabled channels never construct a Provider.

Secret plaintext is never stored by this plugin. Only the core-encrypted opaque reference is retained in the owner-only binding table and converted back to a non-serializable `secretcap.Handle` inside the construction boundary.

Provider login sessions are also resource-owner state. PostgreSQL authoritatively stores the channel generation, expiry, refresh lease digest, and an opaque `secretcap` reference; token plaintext remains encrypted. Fresh Provider instances and separate application nodes therefore converge on one login, while generation-fenced invalidation prevents a stale `401` from discarding a newer token. Refresh work happens outside the database transaction under a 150-second lease, and lease expiry permits recovery without making a business write replayable.

Provider network access is owner-controlled: requests are restricted to the explicitly configured HTTP or HTTPS origin without automatic downgrade; redirects are disabled and HTTPS follows the confirmed interface policy of encryption without certificate verification. Every dial connects directly to a freshly resolved and validated public IP. Private, loopback, link-local, shared, benchmark, documentation, multicast, reserved, and mixed public/private DNS answers are rejected.

Provider write tasks are created idempotently with only a caller-key digest, claimed in bounded skip-locked batches using the PostgreSQL clock, and completed through attempt- and lease-fenced transitions. An expired in-flight write is frozen as `result_unknown` and is never automatically replayed.

Provider write health is also PostgreSQL-authoritative. Each completed upstream call appends an immutable, non-sensitive observation and advances one bounded per-channel window. Five samples with three failures, two unknown results, or three consecutive failures open a five-minute write-only protection window. Protected tasks remain pending without consuming attempts, while exact task and resource reconciliation stays available. A worker checks all channels in one bounded query, so this gate does not introduce per-task database reads.

Exact task/resource queries and provider-capacity collection append their own health observations only after the owner has durably converged or rescheduled the local operation. Invalid responses are classified without raw payloads, non-converging but valid resource reads remain healthy, and an authentication-classified failure atomically records both the original operation and authentication dimensions.

Provider capacity is observed only by this resource owner. Each collection persists a fixed page size, exact opaque continuation cursor, page count, item count, and chained fingerprint. Every page is validated and inserted in one bounded batch; only the terminal page transaction can supersede the last complete observation. A failed, duplicate, skipped, filtered, oversized, or malformed traversal never changes the current usable capacity observation. This data does not create or synchronize product stock.

Resource observations use a dedicated PostgreSQL `observation_claimed_at` business timestamp as their freshness boundary. The exact value is returned as `ClaimedAt`; GoFrame's automatically maintained `updated_at` is not reused for this purpose.

Upstream customer accounts converge through a durable owner state machine. Every create eligibility decision starts with an exact username lookup; only explicit absence permits one bounded create. An unknown write is never replayed, and a documented rejection still requires another exact lookup before a subsequent create. Username and password cross the boundary only as channel-scoped `secretcap` handles. The create acknowledgement is published as a mapping only after exact lookup verification; both handles are revoked after success, while the random password is revoked before manual reconciliation.

Provider task administration is local and bounded. The queue uses keyset pagination with at most one hundred rows, while a detail loads one task, its latest normalized upstream binding, and at most fifty immutable redacted evidence records. Manual takeover is available only from `accepted`, `running`, or `result_unknown`; it requires an optimistic task version, one constrained reason, and one through sixteen evidence records belonging to the same task. The transaction closes automation, revokes retained task-input bindings, appends an immutable responsibility assignment and business audit, and clears observation claims. It never calls a Provider, retries a write, accepts free text, or decides success or failure.

## Dependencies

The plugin depends on LinaPro and `idc-core` version `>=0.1.171 <1.0.0`. Version `v0.1.60` provides the versioned, safe lifecycle projection for residual resources after partial reclamation; the resource plugin continues to use the core plugin's Provider-task permissions and secret-owner capability. The resource plugin imports only public core capability contracts; other IDC plugins may import `resourcecap` but cannot import this plugin's internal implementation.

## Peer connector lifecycle recovery

The provider configuration page exposes an explicit peer connector check to staff with `plugin:query` and `plugin:business-impact`. Reconfirmation also requires `plugin:enable`, installed/enabled status, a normal runtime, and matching nonempty effective/discovered/current versions without a pending upgrade. It rereads the detail after confirmation, invokes the existing enable API, verifies the version and new-business state, then requests the official business-impact report. Failed or changed state clears the result. This is a standard enable/authorization operation that may refresh the current same-version build, not a checksum-pinned repair API.

No host code or private authorization tables are changed. Restarting the host or changing another connector's status may require reconfirmation again. The report is advisory; the official disable operation still rechecks business drain and rejects unfinished work. The action never resumes stopped new business or disables the connector.

## Delivery and Phase boundary

Version `v0.1.8` is delivered as a platform-only, global, built-in source plugin with Chinese and English resources. It includes bounded Provider-health and task-reconciliation pages, the latest normalized non-secret observations, and evidence-driven manual responsibility transfer with exact permissions and optimistic concurrency. An active circuit cannot be manually closed or shortened by relaxing policy, and manual takeover cannot force a result or replay a Provider write. Remaining capability-specific reconciliation, cross-owner reporting, and full resource orchestration remain gated implementation tasks.

Resource connector metadata may declare `apiPathPrefix: /v1`. The owner validates this single version segment, freezes it in host-only settings, and resolves root addresses for both login and business requests without duplicating an existing version. The guest receives neither this transport policy nor secret values.

Region, node and image choices use a shared database projection so page reads do not wait for supplier networks. Keys isolate the current interface, connection revision and connector evidence; asynchronous refresh uses shared leases and persists only validated option fields. Missing, expired or disabled sources never produce fabricated options. New purchases use the current resource connection while historical orders retain their frozen connection.

Supplier option directories retain the last successfully synchronized result for the same enabled connection during refresh failures. They are configuration references, not live capacity guarantees. Background retries continue; connection revisions and connector evidence remain isolated, and disabled interfaces cannot read their old directories.

A corrected paid server keeps its original commerce identity. Entitlement delivery verifies the purchased interface evidence independently from the succeeded task's executed interface, connector version and declaration. Compatible versions may have different declaration digests; the selected channel must still match the same supplier and resource protocol, the frozen correction, and the successful local receipt. Binding is idempotent and never resends an upstream creation request.


## Native imported-product procurement

The public `resourcecap.NativePurchasePlan` is an independent `provision.parameters.build` input carrying an owner-frozen `channelcommercecap.Purchase`. It contains no server hardware, principal, hostname, password, callback secret or procurement authority. `BuildNativePurchaseParameters` validates both the plan and the mapper's bounded JSON result; resource-owned persistent correlation and confidential token injection continue to govern execution and retries.

Native Finance admission requires the exact release's explicit `native_finance_v1` metadata, its source product/revision identity schema, reversible source field mappings and a validated import snapshot. It does not relax `ServerFulfillmentContract` or trust a plugin ID. Native intents have quantity one and purchase-cycle count one; `MaximumPurchaseCycles=1` is preserved by the versioned import contract so older readers reject an unsupported version instead of silently dropping the source limit. New Finance snapshots use v3 remote evaluation and carry no public component amounts. Catalog hour/day intervals use the public billing schedule's unit-specific bounds, including a Finance 720-hour month.

V10 common admission uses the explicit `native_v10_common_v1` protocol and import v3 `remote_evaluation`: the owner converts a validated evaluated view into source schema and amount-free billing metadata. The internal `ImportedBillingSource` bridge checks the frozen product/interface revision, exact product and resource authority, and current source configuration before invoking only `catalog.billing-terms`; it does not borrow staff access or accept arbitrary provider methods. Native plans now require their typed protocol.

Staff-owned product reads explicitly set the typed `CatalogProductQuery.IncludePricePreview` flag to request a valid default-selection account-price sample. Frozen imported billing reads omit this flag and quote only the selected values; missing required customer input does not create a zero-price sample.

The managed cloud connector's `resource.query` may read reviewed guest hostname and login-user labels only through an authenticated `GET /clouds/{positive-id}` without additional request inputs. Global credential-field restrictions remain active for other paths, methods and connectors. Normalized runtime `Data` remains permitted by the existing result policy; password, token and private-path fields never enter customer facts.

## Purchased server page

The `customer-ui` package owns the purchased-server columns, lifecycle dots, expiry date and remaining-day colors, OS icons, scoped search, pagination, and independently versioned customer notes. It declares one `customer.products` surface with `purchased-services.customer.v1`; customer templates host this page rather than defining server columns themselves. The standard frontend build builds the admin assets, then installs the locked customer UI dependencies and builds this page into `frontend/dist/customer`; one public asset declaration publishes both. Run `pnpm --dir apps/lina-plugins/idc-resource/frontend run build:customer` from the repository root to build only the customer page. The resource page uses the public Vben isolated adapter, keeps authentication in the parent, and requests resource facts in batches. Core `>=0.1.171` supplies paid-only directories and provider filtering; explicit empty-menu content configuration requires core `>=0.1.172`. The server page uses compact inline search without a redundant title or refresh toolbar, and fits its bounded grid to the container viewport.


## Resource navigation shortcuts

The resource menu includes a Supplier Operations shortcut to the existing configuration operations tab. It loads only the selected authorized tab; the shortcut uses the existing overview permission and does not grant configuration or write access.
