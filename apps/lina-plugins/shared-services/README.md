# LinaPro Shared Services

`shared-services` is a built-in source plugin that owns two reusable, IDC-neutral capabilities:

- `challengecap`: purpose-bound human-verification challenges and short-lived, single-use proofs.
- `inboxcap`: idempotent publication of rendered in-app messages to an explicit host user ID.

The plugin does not read IDC customer, employee, ticket, billing, or service tables. Consumer plugins must declare a hard plugin dependency and use only the public contracts under `backend/cap/`. The capability owner stores its own records and fails closed when the plugin or a selected provider is unavailable.

## Human verification

The default provider creates a local arithmetic image. A caller may request the accessible arithmetic prompt instead. Challenges expire after five minutes and allow at most five attempts. A successful completion returns a proof valid for two minutes; consuming that proof is atomic and succeeds only once for the original caller, purpose, and binding.

Deployments may select Cloudflare Turnstile with environment configuration:

| Variable | Meaning |
| --- | --- |
| `LINAPRO_SHARED_CHALLENGE_PROVIDER` | `local` or `turnstile` |
| `LINAPRO_SHARED_TURNSTILE_SITE_KEY` | Public Turnstile site key |
| `LINAPRO_SHARED_TURNSTILE_SECRET` | Private Turnstile verification secret |

Turnstile verification always uses Cloudflare's fixed official endpoint. Missing configuration, timeouts, malformed responses, action mismatches, and rejected tokens fail closed.

## In-app messages

The publisher accepts an explicit host user ID, stable source fields, an idempotency key, rendered title and body, a business category, and an optional same-site path. It atomically stores one authoritative message and its publication evidence. Concurrent or repeated calls with the same content return the original result; reusing the same idempotency identity with different content is rejected.

`Publish` acknowledges durable message intent only; its `PublishedAt` does not mean the message is already visible in the host inbox. The existing publication contract remains unchanged.

Call `source.NewCurrentSync(ctx, services, callerPluginID)` and invoke `CurrentSync` from the real authenticated user's request. The method accepts only a limit (zero defaults to 50; maximum 50), selects that platform user's pending intents, and forwards their title and plain-text body to the public host notification capability. The body is HTML-escaped and line breaks become fixed `<br>` elements for the host preview; stored intent content and its digest remain unchanged. Anonymous, tenant, and impersonation contexts are rejected. Employee data scopes do not broaden this self-only boundary and are not required for customer requests. Consumers apply their own business permissions before synchronization; self-only scope never grants access to other recipients.

`SyncedCount` counts messages committed to the host inbox in this call; `PendingCount` counts that user's remaining intents. Host delivery and its unique message-ID evidence share the default database transaction, including nested host writes. Concurrent callers skip locked rows and recheck delivery evidence; failed or unknown-response calls can be retried without duplicating committed messages. A periodic authenticated refresh can pick up intents created later by background workers. Synchronization does not forge an employee identity or send mail. Host read state and business notification policy remain outside this capability.

The host notification's plugin attribution comes from the caller-bound host capability. Its source type is `plugin`, its source ID is the shared message number, and its payload retains the original publisher, business source, and optional same-site action path. Background publication remains durable until an authenticated recipient synchronization occurs.

## Verification

Run the plugin checks from the repository root:

```powershell
.\make.cmd plugins.check
.\make.cmd lint.go dir=apps/lina-plugins/shared-services plugins=1
```

Run Go verification from this plugin directory:

```powershell
$env:GOWORK = 'off'
go test ./...
go vet ./...
```

Set `LINAPRO_SHARED_SERVICES_TEST_DSN` to a PostgreSQL test connection to run the isolated-schema persistence and concurrency test. The test creates and removes only a randomly named schema.
