# Local order diagnosis

Run from the repository root:

```text
go run ./.agents/tools/context.go order --order ORD_example --session-file temp/private-session.json
```

Use an explicitly selected private JSON session file with an `accessToken` field, or set `LINA_DIAG_TOKEN` in the process environment and omit `--session-file`. The tool never discovers credentials, signs in, refreshes sessions, or prints tokens. Do not put credentials on the command line or in tracked files. `--base-url` defaults to `http://127.0.0.1:6030` and accepts loopback origins only.

The report contains exact invoice amounts in minor units, order and fulfillment states, local automation phase/result codes, resource task status, and saved upstream task/resource references. It reads the existing staff APIs, retaining their exact action and data-scope checks. The optional `execution` projection requires the updated core backend; an older running backend reports `execution_unavailable` for an automatic delivery in progress instead of claiming complete evidence.

At most three fulfillment tasks and three distinct resource tasks are inspected, with at most eight `GET` requests. Each response is limited to 1 MiB, each request to ten seconds, and the whole workflow to 45 seconds. Redirects and proxies are disabled. Large orders, missing permissions, expired sessions, invalid responses, and missing execution evidence produce a partial report and a nonzero exit. Narrow the investigation explicitly rather than scanning unrelated customers.

This command reads local saved facts only. It never calls a supplier, writes a database, retries provisioning, or confirms that an unknown upstream result has succeeded. It excludes contact details, notes, raw responses, and secrets. `complete` means all bounded queries completed, not that the order has been delivered. A paid invoice and a successful API invocation do not prove server creation.

Run the self-contained tests from the repository root:

```text
go test apps/lina-plugins/idc-core/hack/tools/order-diagnose/main.go apps/lina-plugins/idc-core/hack/tools/order-diagnose/diagnose.go apps/lina-plugins/idc-core/hack/tools/order-diagnose/diagnose_test.go -count=1
```
