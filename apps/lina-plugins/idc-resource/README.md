# LinaPro IDC Resource

## Scope

This source plugin owns provider accounts, normalized provider-capacity observations, server instances, resource allocation, and durable asynchronous operation boundaries.

## Runtime foundation

The plugin now owns versioned Provider contracts and startup declarations, encrypted channel-secret bindings, channels, routing pools, authoritative upstream mappings, resource tasks, capability snapshots, provider-capacity observations, financial observations, and health observations. A channel reader uses two bounded queries to construct only enabled channels; disabled channels never construct a Provider.

Secret plaintext is never stored by this plugin. Only the core-encrypted opaque reference is retained in the owner-only binding table and converted back to a non-serializable `secretcap.Handle` inside the construction boundary.

Provider login sessions are also resource-owner state. PostgreSQL authoritatively stores the channel generation, expiry, refresh lease digest, and an opaque `secretcap` reference; token plaintext remains encrypted. Fresh Provider instances and separate application nodes therefore converge on one login, while generation-fenced invalidation prevents a stale `401` from discarding a newer token. Refresh work happens outside the database transaction under a 150-second lease, and lease expiry permits recovery without making a business write replayable.

Provider network access is owner-controlled: requests are restricted to the configured HTTPS origin, redirects are disabled, TLS verification is mandatory, and every dial connects directly to a freshly resolved and validated public IP. Private, loopback, link-local, shared, benchmark, documentation, multicast, reserved, and mixed public/private DNS answers are rejected.

Provider write tasks are created idempotently with only a caller-key digest, claimed in bounded skip-locked batches using the PostgreSQL clock, and completed through attempt- and lease-fenced transitions. An expired in-flight write is frozen as `result_unknown` and is never automatically replayed.

Provider write health is also PostgreSQL-authoritative. Each completed upstream call appends an immutable, non-sensitive observation and advances one bounded per-channel window. Five samples with three failures, two unknown results, or three consecutive failures open a five-minute write-only protection window. Protected tasks remain pending without consuming attempts, while exact task and resource reconciliation stays available. A worker checks all channels in one bounded query, so this gate does not introduce per-task database reads.

Exact task/resource queries and provider-capacity collection append their own health observations only after the owner has durably converged or rescheduled the local operation. Invalid responses are classified without raw payloads, non-converging but valid resource reads remain healthy, and an authentication-classified failure atomically records both the original operation and authentication dimensions.

Provider capacity is observed only by this resource owner. Each collection persists a fixed page size, exact opaque continuation cursor, page count, item count, and chained fingerprint. Every page is validated and inserted in one bounded batch; only the terminal page transaction can supersede the last complete observation. A failed, duplicate, skipped, filtered, oversized, or malformed traversal never changes the current usable capacity observation. This data does not create or synchronize product stock.

Resource observations use a dedicated PostgreSQL `observation_claimed_at` business timestamp as their freshness boundary. The exact value is returned as `ClaimedAt`; GoFrame's automatically maintained `updated_at` is not reused for this purpose.

Upstream customer accounts converge through a durable owner state machine. Every create eligibility decision starts with an exact username lookup; only explicit absence permits one bounded create. An unknown write is never replayed, and a documented rejection still requires another exact lookup before a subsequent create. Username and password cross the boundary only as channel-scoped `secretcap` handles. The create acknowledgement is published as a mapping only after exact lookup verification; both handles are revoked after success, while the random password is revoked before manual reconciliation.

Provider task administration is local and bounded. The queue uses keyset pagination with at most one hundred rows, while a detail loads one task, its latest normalized upstream binding, and at most fifty immutable redacted evidence records. Manual takeover is available only from `accepted`, `running`, or `result_unknown`; it requires an optimistic task version, one constrained reason, and one through sixteen evidence records belonging to the same task. The transaction closes automation, revokes retained task-input bindings, appends an immutable responsibility assignment and business audit, and clears observation claims. It never calls a Provider, retries a write, accepts free text, or decides success or failure.

## Dependencies

The plugin depends on LinaPro and `idc-core` version `>=0.1.60 <1.0.0`. Version `v0.1.60` provides the versioned, safe lifecycle projection for residual resources after partial reclamation; the resource plugin continues to use the core plugin's Provider-task permissions and secret-owner capability. The resource plugin imports only public core capability contracts; other IDC plugins may import `resourcecap` but cannot import this plugin's internal implementation.

## Delivery and Phase boundary

Version `v0.1.8` is delivered as a platform-only, global, built-in source plugin with Chinese and English resources. It includes bounded Provider-health and task-reconciliation pages, the latest normalized non-secret observations, and evidence-driven manual responsibility transfer with exact permissions and optimistic concurrency. An active circuit cannot be manually closed or shortened by relaxing policy, and manual takeover cannot force a result or replay a Provider write. Remaining capability-specific reconciliation, cross-owner reporting, and full resource orchestration remain gated implementation tasks.
