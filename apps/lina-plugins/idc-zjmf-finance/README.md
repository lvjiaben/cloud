# idc-zjmf-finance

`idc-zjmf-finance` is a managed dynamic server connector for the native ZJMF Finance API. It is separate from ZJMFCloud, ZJMF V10, and DCIM connectors and never uses the legacy `modules/addons/idcsmart_api` or `/v1` compatibility API.

English | [简体中文](README.zh-CN.md)

## Interface instances

Administrators can create multiple independent interface instances in `idc-resource`. Endpoint and credentials remain owned by that resource channel. Product releases freeze only the chosen interface projection and reviewed upstream product mapping.

This connector does not duplicate server menus or declare empty pages. Interface management, server lists, and customer server pages remain owned by `idc-resource`; a connector-owned surface may be declared only when a real independent page and its packaged static assets are delivered together.

Authentication always posts `username` and `password` to `/zjmf_api_login`. The confidential host proxy extracts `jwt` and injects `Authorization: Bearer` without exposing credentials or tokens to WASM, pages, logs, or fulfillment evidence.

The resource channel must use `login_session`, token path `["jwt"]`, header `Authorization`, scheme `Bearer`, refresh path `["status"]`, refresh value `["405"]`, and `serializeInvocations: true`. The resource owner validates these governed settings, retries authentication at most once, and holds one PostgreSQL-backed per-channel invocation lease across the complete guest call.

## Product configuration

Products come from `GET /cart/all`. Configuration comes only from `GET /cart/get_product_config?pid=<selected product>`. A missing upstream product is rejected, and lookups for different product IDs remain separate.

ZJMF Finance lets merchants name configuration options freely. Native resale retains their numeric field and choice identifiers, quantity ranges, source dependencies and customer fields; it does not infer region, network or operating-system roles. The product identity contains only its source product ID and source configuration revision.

## Fulfillment safety

The verified initial flow uses `/user_info`, cart reconciliation, `/cart/add_to_shop`, `/cart/settle`, and credit payment. The complete flow must hold a durable per-interface serialization lease. A failure before settlement may fail safely; a timeout after settlement without an order reference becomes an unknown result requiring manual reconciliation and is never submitted again automatically.

The connector implements this initial purchase flow with owner-only correlation carrying a governed HTTPS callback URL and numeric intent ID. It accepts exactly one operation secret slot, `downstream_token`; the confidential proxy injects its plaintext only into the supplier form. Every replay starts with `/cart/clear`, recovers an existing active server or invoice, and does not repeat cart or settlement side effects. Success is returned only after the upstream supplies a positive server ID. Renewal and resize remain unsupported because they create separate upstream financial transactions.

## Verification

Run from the plugin directory:

```powershell
$env:GOWORK = "off"
go test ./... -count=1
go vet ./...
go build ./...
$env:GOOS = "wasip1"
$env:GOARCH = "wasm"
go build -o idc-zjmf-finance.wasm .
```

Production write verification must use a dedicated test account and test product. Do not use existing customer orders or servers for destructive verification.


## Catalog read boundaries

The user-supplied Finance 3.5.8 source confirms that `/cart/get_product_config` separates base prices from configuration surcharges. Hour and day intervals retain the declared multiplier; absent durations never default to one. Every native Finance import uses protocol v3 `remote_evaluation`: only configuration and cycle metadata are frozen. Raw base prices and choice charges are never exported as a complete selling price. Explicit staff previews quote valid defaults; missing required customer input leaves the sample unavailable instead of inventing zero. Native resale uses the separate `NativePurchasePlan`, not a server hardware plan. Source configuration and choice charges remain read-only. Complete representable catalogs retain quantity tiers, conditional candidate sets and customer fields; unsupported source rules fail closed. Account billing previews call `POST /cart/get_total` and use explicit account discount amounts rather than the published raw tariff. Purchase quantity and cycle count are each one per native intent; import protocol v3 preserves the maximum cycle count and requires a compatible dynamic billing owner; unsupported nested re-export is rejected. See the [protocol evidence](../../../localdocs/idcsmart-finance-reference.md).

Finance linkage type `20` retains a `tree_select` candidate hierarchy. Terminal tokens restore all ancestor `configoption` pairs, keep cumulative period/setup charges, and reject nonterminal or cross-branch selections. Advanced constraints can address the original child level without flattening the public tree.
