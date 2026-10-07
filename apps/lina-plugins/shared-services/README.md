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

Publishing does not claim that a user has read the message. Read-state projection and business notification policy remain outside this capability.

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
